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
	SafeService *SafetyService
	RecService  *RecommendationService
	RcaService  *RCAService
	
	SRObserver  *ServiceRateObserver

	CtrlCfg     ControllerConfig
	RegimeCfg   RegimeConfig   

	LastDecision Bundle
	LastValidation SafetyValidationResult
	MasterSeed   int64
}

func NewController(baseSeed int64, minDeps, maxDeps int, ctrlCfg ControllerConfig) *Controller {
	if baseSeed == 0 { baseSeed = time.Now().UnixNano() }

	return &Controller{
		PredService:  NewPredictionService(ctrlCfg),
		DecService:   NewDecisionService(DefaultOptimizerConfig()),
		SafeService:  NewSafetyService(),
		RecService:   NewRecommendationService(),
		RcaService:   NewRCAService(),
		
		SRObserver:   &ServiceRateObserver{CurrentServiceRate: 10.0},
		LastDecision: Bundle{Replicas: minDeps, QueueLimit: 1000.0, RetryLimit: 3, CacheAggression: 0.0},
		MasterSeed:   baseSeed,
		
		CtrlCfg:     ctrlCfg,
		RegimeCfg:   DefaultRegimeConfig(),
	}
}

// Recommend remains for backward compatibility with AutonomousEngine tests.
// It wraps the new explainable recommendation pipeline and extracts the raw bundle.
func (c *Controller) Recommend(zTelemetry MeasurementVector, currentSysState *SystemState, mem *RegimeMemory, dt float64) Bundle {
	rec := c.GenerateExplainableRecommendation(zTelemetry, currentSysState, mem, dt)
	return rec.RecommendedBundle
}

// GenerateExplainableRecommendation is the new Maestro-ready API endpoint
// that returns the fully structured, safety-verified, and explainable decision.
func (c *Controller) GenerateExplainableRecommendation(zTelemetry MeasurementVector, currentSysState *SystemState, mem *RegimeMemory, dt float64) Recommendation {
	c.AgentPredict(zTelemetry, currentSysState, mem, dt)
	optimalBundle := c.AgentDecide(currentSysState, mem)
	validationResult := c.AgentSafety(optimalBundle, currentSysState)
	return c.AgentRecommend(validationResult, currentSysState)
}

func (c *Controller) AgentPredict(zTelemetry MeasurementVector, currentSysState *SystemState, mem *RegimeMemory, dt float64) {
	if dt <= 0.0 { dt = c.CtrlCfg.DefaultIntegrationDt }
	c.PredService.PredictCurrentState(&zTelemetry, currentSysState, c.LastDecision, dt)
	currentActiveReplicas := currentSysState.Replicas
	if currentActiveReplicas < 1 { currentActiveReplicas = 1 }
	filteredCapacity := c.PredService.EKF.X[2] 
	currentSysState.ServiceRate = c.SRObserver.Observe([5]float64(zTelemetry), currentActiveReplicas, c.CtrlCfg)
	currentSysState.Utilisation = currentSysState.PredictedArrival / math.Max(filteredCapacity, 0.001)

	currentSysState.FailureMode = "Healthy"
	if currentSysState.Latency > currentSysState.SLATarget * 5.0 {
		if filteredCapacity < currentSysState.PredictedArrival * 0.5 {
			currentSysState.FailureMode = "CapacityExhausted"
		} else {
			currentSysState.FailureMode = "DegradedDownstream"
		}
	}

	qRisk := currentSysState.QueueDepth / math.Max(1.0, float64(c.LastDecision.QueueLimit))
	lRisk := currentSysState.Latency / math.Max(0.001, currentSysState.SLATarget) 
	rRisk := currentSysState.RetryPressure / 5.0 
	currentSysState.Risk = qRisk + lRisk + rRisk 

	if mem != nil { mem.Update(*currentSysState, currentSysState.SLATarget, 0.0, c.RegimeCfg) }
	c.PredService.UpdateModel(currentSysState, dt)
}

func (c *Controller) AgentDecide(currentSysState *SystemState, mem *RegimeMemory) Bundle {
	simCfg := c.PredService.GetSimConfig()
	return c.DecService.GenerateOptimalDecision(currentSysState, simCfg, mem, c.MasterSeed)
}

func (c *Controller) AgentSafety(optimalBundle Bundle, currentSysState *SystemState) SafetyValidationResult {
	val := c.SafeService.ValidateAction(optimalBundle, currentSysState, c.LastDecision)
	c.LastValidation = val
	return val
}

func (c *Controller) AgentRecommend(validationResult SafetyValidationResult, currentSysState *SystemState) Recommendation {
	if validationResult.SafeBundle.Replicas == 0 && c.LastValidation.SafeBundle.Replicas != 0 {
		validationResult = c.LastValidation
	}
	
	safeBundle := validationResult.SafeBundle
	sysIdConfidence := c.PredService.SysID.Confidence()
	recommendation := c.RecService.GenerateRecommendation(currentSysState, c.LastDecision, safeBundle, validationResult, sysIdConfidence)

	c.LastDecision = safeBundle
	c.LastValidation = validationResult
	c.MasterSeed++
	return recommendation
}
