package control

import (
	"math"
	"time"
)

type ControllerConfig struct {
	ServiceRateFilterAlpha float64 
	MinServiceRateSafety   float64 
	MaxServiceRateSafety   float64 
	SystemWarmupWindowSec  float64 
	MinConfidenceGateLimit float64 
	DefaultHorizonSteps  int
	DefaultIntegrationDt float64
}

func DefaultControllerConfig() ControllerConfig {
	return ControllerConfig{
		ServiceRateFilterAlpha: 0.05,
		MinServiceRateSafety:   0.001, 
		MaxServiceRateSafety:   10000.0,
		SystemWarmupWindowSec:  60.0,
		MinConfidenceGateLimit: 0.15,
		DefaultHorizonSteps:    10,
		DefaultIntegrationDt:   1.0,
	}
}

type ServiceRateObserver struct { CurrentServiceRate float64 }

func (sro *ServiceRateObserver) Observe(zTelemetry [5]float64, activeReplicas int, cfg ControllerConfig) float64 {
	measuredCapacity := zTelemetry[3]
	if measuredCapacity > 0.001 && activeReplicas > 0 {
		rawServiceObservation := measuredCapacity / float64(activeReplicas)
		sro.CurrentServiceRate = (cfg.ServiceRateFilterAlpha * rawServiceObservation) +
			((1.0 - cfg.ServiceRateFilterAlpha) * sro.CurrentServiceRate)
	}
	if sro.CurrentServiceRate < cfg.MinServiceRateSafety { sro.CurrentServiceRate = cfg.MinServiceRateSafety }
	if sro.CurrentServiceRate > cfg.MaxServiceRateSafety { sro.CurrentServiceRate = cfg.MaxServiceRateSafety }
	return sro.CurrentServiceRate
}

type CalibrationManager struct {
	TimeElapsed  float64
	IsCalibrated bool
}
func (cm *CalibrationManager) Track(dt float64, cfg ControllerConfig) bool {
	cm.TimeElapsed += dt; if cm.TimeElapsed > cfg.SystemWarmupWindowSec { cm.IsCalibrated = true }; return cm.IsCalibrated
}

type ConfidenceGate struct{}
func (cg *ConfidenceGate) Validate(conf ParameterConfidence, cfg ControllerConfig) bool {
	return conf.ArrivalProcess > cfg.MinConfidenceGateLimit && conf.RetryProcess > cfg.MinConfidenceGateLimit
}

type Controller struct {
	PredService *PredictionService
	DecService  *DecisionService
	
	SRObserver  *ServiceRateObserver

	CtrlCfg     ControllerConfig
	RegimeCfg   RegimeConfig   

	LastDecision Bundle
	MasterSeed   int64
}

func NewController(baseSeed int64, minDeps, maxDeps int, ctrlCfg ControllerConfig) *Controller {
	if baseSeed == 0 { baseSeed = time.Now().UnixNano() }

	return &Controller{
		PredService:  NewPredictionService(ctrlCfg),
		DecService:   NewDecisionService(DefaultOptimizerConfig()),
		
		SRObserver:   &ServiceRateObserver{CurrentServiceRate: 10.0},
		LastDecision: Bundle{Replicas: minDeps, QueueLimit: 1000.0, RetryLimit: 3, CacheAggression: 0.0},
		MasterSeed:   baseSeed,
		
		CtrlCfg:     ctrlCfg,
		RegimeCfg:   DefaultRegimeConfig(),
	}
}

func (c *Controller) Recommend(zTelemetry MeasurementVector, currentSysState *SystemState, mem *RegimeMemory, dt float64) Bundle {
	if dt <= 0.0 { dt = c.CtrlCfg.DefaultIntegrationDt }

	// 1 & 2. State Estimation via PredictionService
	c.PredService.PredictCurrentState(&zTelemetry, currentSysState, c.LastDecision, dt)

	currentActiveReplicas := currentSysState.Replicas
	if currentActiveReplicas < 1 { currentActiveReplicas = 1 }

	filteredCapacity := c.PredService.EKF.X[2] // Read raw capacity from the EKF state
	currentSysState.ServiceRate = c.SRObserver.Observe([5]float64(zTelemetry), currentActiveReplicas, c.CtrlCfg)
	currentSysState.Utilisation = currentSysState.PredictedArrival / math.Max(filteredCapacity, 0.001)

	// Failure Mode detection
	currentSysState.FailureMode = "Healthy"
	if currentSysState.Latency > currentSysState.SLATarget * 5.0 {
		if filteredCapacity < currentSysState.PredictedArrival * 0.5 {
			currentSysState.FailureMode = "CapacityExhausted"
		} else {
			currentSysState.FailureMode = "DegradedDownstream"
		}
	}

	// Risk calculation
	qRisk := currentSysState.QueueDepth / math.Max(1.0, float64(c.LastDecision.QueueLimit))
	lRisk := currentSysState.Latency / math.Max(0.001, currentSysState.SLATarget) 
	rRisk := currentSysState.RetryPressure / 5.0 
	currentSysState.Risk = qRisk + lRisk + rRisk 

	if mem != nil { mem.Update(*currentSysState, currentSysState.SLATarget, 0.0, c.RegimeCfg) }

	// 3. Online Learning (SysID) via PredictionService
	c.PredService.UpdateModel(currentSysState, dt)

	// 4. Decision Generation & MPC Optimization via DecisionService
	simCfg := c.PredService.GetSimConfig()
	optimalBundle := c.DecService.GenerateOptimalDecision(currentSysState, simCfg, mem, c.MasterSeed)

	// Mesh Ceiling Enforcements (The SLA-Aware Little's Law Physics Ceiling)
	currentPhysicalCapacity := math.Max(0.001, float64(currentSysState.Replicas)*currentSysState.ServiceRate)
	maxSlaQueue := currentPhysicalCapacity * currentSysState.SLATarget

	absoluteMaxQueue := math.Max(25.0, maxSlaQueue * 1.5) 
	
	if optimalBundle.QueueLimit > absoluteMaxQueue {
		optimalBundle.QueueLimit = absoluteMaxQueue
	}
	if optimalBundle.QueueLimit < 25.0 {
		optimalBundle.QueueLimit = 25.0
	}

	currentQ := float64(c.LastDecision.QueueLimit)
	maxDrop := math.Max(10.0, currentQ * 0.20)
	if currentSysState.FailureMode != "Healthy" { maxDrop = currentQ * 0.90 } 
	if optimalBundle.QueueLimit < currentQ - maxDrop {
		optimalBundle.QueueLimit = currentQ - maxDrop
	}
	
	maxRise := math.Max(5.0, currentQ * 0.05)
	if currentSysState.RetryPressure > 10.0 { 
		maxRise = 2.0 
	} else if maxRise > 25.0 { 
		maxRise = 25.0 
	}
	
	if optimalBundle.QueueLimit > currentQ + maxRise {
		optimalBundle.QueueLimit = currentQ + maxRise
	}

	c.LastDecision = optimalBundle
	c.MasterSeed++
	
	return optimalBundle
}