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
	EKF         *ExtendedKalmanFilter
	SysID       *SystemIdentifier
	ParamEst    *ParameterEstimator 
	MPC         *RobustMPC
	
	SRObserver  *ServiceRateObserver
	Calibrator  *CalibrationManager
	GateKeeper  *ConfidenceGate

	CtrlCfg     ControllerConfig
	SimCfg      SimConfig      
	EconCfg     EconomicParams 
	
	GenCfg      GeneratorConfig
	RegimeCfg   RegimeConfig   

	LastDecision Bundle
	MasterSeed   int64
}

func NewController(baseSeed int64, minDeps, maxDeps int, ctrlCfg ControllerConfig) *Controller {
	if baseSeed == 0 { baseSeed = time.Now().UnixNano() }

	return &Controller{
		EKF:          NewExtendedKalmanFilter(DefaultEKFConfig()),
		SysID:        NewSystemIdentifier(),
		ParamEst:     NewParameterEstimator(), 
		MPC:          NewRobustMPC(DefaultOptimizerConfig()),
		
		SRObserver:   &ServiceRateObserver{CurrentServiceRate: 10.0},
		Calibrator:   &CalibrationManager{TimeElapsed: 0.0, IsCalibrated: false},
		GateKeeper:   &ConfidenceGate{},
		LastDecision: Bundle{Replicas: minDeps, QueueLimit: 1000.0, RetryLimit: 3, CacheAggression: 0.0},
		MasterSeed:   baseSeed,
		
		CtrlCfg:     ctrlCfg,
		RegimeCfg:   DefaultRegimeConfig(),
		EconCfg:     DefaultEconomicParams(),
		
		GenCfg:      DefaultGeneratorConfig(),
		SimCfg: SimConfig{
			HorizonSteps:      ctrlCfg.DefaultHorizonSteps,
			Dt:                ctrlCfg.DefaultIntegrationDt,
			BaseLatency:       0.005,
			MaxQueueDelay:     10.000, 
			NaturalFrequency:  0.25,
			DampingRatio:      0.707,
			ArrivalTheta:      0.20,
			ArrivalMean:       100.0,
			ArrivalSigma:      0.10,
			RetryAlpha:        0.30,
			RetryBeta:         0.25,
			RetryGamma:        0.50,
			EfficiencyDecay:   0.15,
			RetryFeedbackGain: 0.20,
		},
	}
}

func (c *Controller) Recommend(zTelemetry MeasurementVector, currentSysState *SystemState, mem *RegimeMemory, dt float64) Bundle {
	if dt <= 0.0 { dt = c.SimCfg.Dt }

	// Byzantine Fault Shield
	for i := 0; i < 5; i++ {
		if math.IsNaN(zTelemetry[i]) || math.IsInf(zTelemetry[i], 0) {
			switch i {
			case 0: zTelemetry[0] = currentSysState.QueueDepth
			case 1: zTelemetry[1] = currentSysState.Latency
			case 2: zTelemetry[2] = currentSysState.RetryPressure
			case 3: zTelemetry[3] = math.Max(0.001, float64(currentSysState.Replicas)*currentSysState.ServiceRate)
			case 4: zTelemetry[4] = currentSysState.PredictedArrival
			}
		}
	}


	var inputU ControlVector
	inputU[0] = float64(c.LastDecision.Replicas) * currentSysState.ServiceRate
	inputU[1] = c.LastDecision.QueueLimit
	inputU[2] = c.LastDecision.CacheAggression

	c.EKF.Predict(inputU, *currentSysState, c.SimCfg, dt)
	c.EKF.Update(zTelemetry)

	filteredState := c.EKF.X

	currentSysState.QueueDepth = math.Max(0.0, filteredState[0])
	currentSysState.RetryPressure = math.Max(0.0, filteredState[1])
	currentSysState.PredictedArrival = math.Max(0.001, filteredState[3])
	currentSysState.CapacityVelocity = filteredState[4]

	currentActiveReplicas := currentSysState.Replicas
	if currentActiveReplicas < 1 { currentActiveReplicas = 1 }

	currentSysState.ServiceRate = c.SRObserver.Observe([5]float64(zTelemetry), currentActiveReplicas, c.CtrlCfg)
	currentSysState.Utilisation = currentSysState.PredictedArrival / math.Max(filteredState[2], 0.001)

	predictedLatency := ComputeLatency(currentSysState.QueueDepth, currentSysState.PredictedArrival, filteredState[2], c.SimCfg.BaseLatency, c.SimCfg.MaxQueueDelay)
	observedLatency := zTelemetry[1] 

	currentSysState.Latency = (0.1 * math.Min(predictedLatency, 10.0)) + (0.9 * observedLatency)

	currentSysState.FailureMode = "Healthy"
	if currentSysState.Latency > currentSysState.SLATarget * 5.0 {
		if filteredState[2] < currentSysState.PredictedArrival * 0.5 {
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


	currentObs := Observation{ Queue: currentSysState.QueueDepth, Latency: currentSysState.Latency, Retry: currentSysState.RetryPressure, Arrival: currentSysState.PredictedArrival, Capacity: filteredState[2], Time: c.Calibrator.TimeElapsed }
	if c.Calibrator.Track(dt, c.CtrlCfg) {
		c.SysID.Update(currentObs)
		if c.GateKeeper.Validate(c.SysID.Confidence(), c.CtrlCfg) {
			c.ParamEst.Update(c.SysID.Parameters(), c.SysID.Confidence())
			c.ParamEst.Apply(&c.SimCfg)
		}
	}

	physicalRequiredCapacity := currentSysState.PredictedArrival / math.Max(0.001, currentSysState.ServiceRate)
	if float64(currentSysState.Replicas) > physicalRequiredCapacity * 1.5 {
		c.GenCfg.MaxScaleSurge = 0 
	} else if currentSysState.Utilisation < 0.5 && currentSysState.QueueDepth < 10.0 {
		c.GenCfg.MaxScaleSurge = 5 
	} else {
		c.GenCfg.MaxScaleSurge = 100 
	}

	candidates := GenerateBundles(*currentSysState, c.GenCfg, c.SimCfg)
	optimalBundle := c.MPC.Optimize(*currentSysState, candidates, c.SimCfg, c.EconCfg, mem, c.MasterSeed)

	

	// ========================================================================
	// THE FINAL CURE: SLA-AWARE LITTLE'S LAW MESH CEILING
	// We override the capitalist MPC with a strict Engineering Physics Ceiling.
	// Max Allowed Queue = Realtime Capacity * SLA Target
	// ========================================================================
	currentPhysicalCapacity := math.Max(0.001, float64(currentSysState.Replicas)*currentSysState.ServiceRate)
	maxSlaQueue := currentPhysicalCapacity * currentSysState.SLATarget

	// We ruthlessly clamp the ceiling to the exact SLA boundary, but allow a 
	// microscopic physical floor (25) to prevent Euler-RED Singularities.
	absoluteMaxQueue := math.Max(25.0, maxSlaQueue * 1.5) // Allow 50% buffer to prevent jitter
	
	if optimalBundle.QueueLimit > absoluteMaxQueue {
		optimalBundle.QueueLimit = absoluteMaxQueue
	}
    // Force the physical floor
	if optimalBundle.QueueLimit < 25.0 {
		optimalBundle.QueueLimit = 25.0
	}

	// Apply Slew Rates to ensure we don't crash Envoy while enforcing the ceiling
	currentQ := float64(c.LastDecision.QueueLimit)
	maxDrop := math.Max(10.0, currentQ * 0.20)
	if currentSysState.FailureMode != "Healthy" { maxDrop = currentQ * 0.90 } // Instant load shed
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



// Append this to the end of uipath/control/policy_controller.go

// Recommend runs the core intelligence loop (EKF + MPC) based on real-world telemetry
// to generate an optimal action, but relies on external execution (UiPath).
func (c *Controller) Recommend(zTelemetry MeasurementVector, currentSysState *SystemState, mem *RegimeMemory, dt float64) Bundle {
	if dt <= 0.0 { dt = c.SimCfg.Dt }

	// 1. Byzantine Fault Shield
	for i := 0; i < 5; i++ {
		if math.IsNaN(zTelemetry[i]) || math.IsInf(zTelemetry[i], 0) {
			switch i {
			case 0: zTelemetry[0] = currentSysState.QueueDepth
			case 1: zTelemetry[1] = currentSysState.Latency
			case 2: zTelemetry[2] = currentSysState.RetryPressure
			case 3: zTelemetry[3] = math.Max(0.001, float64(currentSysState.Replicas)*currentSysState.ServiceRate)
			case 4: zTelemetry[4] = currentSysState.PredictedArrival
			}
		}
	}

	// 2. State Estimation (EKF Predict & Update)
	var inputU ControlVector
	inputU[0] = float64(c.LastDecision.Replicas) * currentSysState.ServiceRate
	inputU[1] = c.LastDecision.QueueLimit
	inputU[2] = c.LastDecision.CacheAggression

	c.EKF.Predict(inputU, *currentSysState, c.SimCfg, dt)
	c.EKF.Update(zTelemetry)
	filteredState := c.EKF.X

	// Map filtered state back to current system state
	currentSysState.QueueDepth = math.Max(0.0, filteredState[0])
	currentSysState.RetryPressure = math.Max(0.0, filteredState[1])
	currentSysState.PredictedArrival = math.Max(0.001, filteredState[3])
	currentSysState.CapacityVelocity = filteredState[4]

	currentActiveReplicas := currentSysState.Replicas
	if currentActiveReplicas < 1 { currentActiveReplicas = 1 }

	currentSysState.ServiceRate = c.SRObserver.Observe([5]float64(zTelemetry), currentActiveReplicas, c.CtrlCfg)
	currentSysState.Utilisation = currentSysState.PredictedArrival / math.Max(filteredState[2], 0.001)

	predictedLatency := ComputeLatency(currentSysState.QueueDepth, currentSysState.PredictedArrival, filteredState[2], c.SimCfg.BaseLatency, c.SimCfg.MaxQueueDelay)
	observedLatency := zTelemetry[1] 
	currentSysState.Latency = (0.1 * math.Min(predictedLatency, 10.0)) + (0.9 * observedLatency)

	// Risk calculation
	qRisk := currentSysState.QueueDepth / math.Max(1.0, float64(c.LastDecision.QueueLimit))
	lRisk := currentSysState.Latency / math.Max(0.001, currentSysState.SLATarget) 
	rRisk := currentSysState.RetryPressure / 5.0 
	currentSysState.Risk = qRisk + lRisk + rRisk 

	if mem != nil { mem.Update(*currentSysState, currentSysState.SLATarget, 0.0, c.RegimeCfg) }

	// 3. Online Learning (SysID)
	currentObs := Observation{ Queue: currentSysState.QueueDepth, Latency: currentSysState.Latency, Retry: currentSysState.RetryPressure, Arrival: currentSysState.PredictedArrival, Capacity: filteredState[2], Time: c.Calibrator.TimeElapsed }
	if c.Calibrator.Track(dt, c.CtrlCfg) {
		c.SysID.Update(currentObs)
		if c.GateKeeper.Validate(c.SysID.Confidence(), c.CtrlCfg) {
			c.ParamEst.Update(c.SysID.Parameters(), c.SysID.Confidence())
			c.ParamEst.Apply(&c.SimCfg)
		}
	}

	// 4. Candidate Generation & MPC Optimization
	physicalRequiredCapacity := currentSysState.PredictedArrival / math.Max(0.001, currentSysState.ServiceRate)
	if float64(currentSysState.Replicas) > physicalRequiredCapacity * 1.5 {
		c.GenCfg.MaxScaleSurge = 0 
	} else if currentSysState.Utilisation < 0.5 && currentSysState.QueueDepth < 10.0 {
		c.GenCfg.MaxScaleSurge = 5 
	} else {
		c.GenCfg.MaxScaleSurge = 100 
	}

	candidates := GenerateBundles(*currentSysState, c.GenCfg, c.SimCfg)
	optimalBundle := c.MPC.Optimize(*currentSysState, candidates, c.SimCfg, c.EconCfg, mem, c.MasterSeed)

	// Mesh Ceiling Enforcements
	currentPhysicalCapacity := math.Max(0.001, float64(currentSysState.Replicas)*currentSysState.ServiceRate)
	absoluteMaxQueue := math.Max(25.0, (currentPhysicalCapacity * currentSysState.SLATarget) * 1.5)
	if optimalBundle.QueueLimit > absoluteMaxQueue { optimalBundle.QueueLimit = absoluteMaxQueue }
	if optimalBundle.QueueLimit < 25.0 { optimalBundle.QueueLimit = 25.0 }

	c.LastDecision = optimalBundle
	c.MasterSeed++
	
	// CRITICAL: We return the decision, but we DO NOT simulate `ApplyActuatorDynamics`.
	// UiPath owns the execution now.
	return optimalBundle
}