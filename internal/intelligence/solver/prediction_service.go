package control

import "math"

// PredictionService is the extracted Prediction Agent.
// It wraps the Extended Kalman Filter and System Identification logic,
// exposing them as a queryable service independent of the Decision loop.
type PredictionService struct {
	EKF        *ExtendedKalmanFilter
	SysID      *SystemIdentifier
	ParamEst   *ParameterEstimator
	Calibrator *CalibrationManager
	GateKeeper *ConfidenceGate
	SimCfg     SimConfig
	CtrlCfg    ControllerConfig
}

func NewPredictionService(ctrlCfg ControllerConfig) *PredictionService {
	return &PredictionService{
		EKF:        NewExtendedKalmanFilter(DefaultEKFConfig()),
		SysID:      NewSystemIdentifier(),
		ParamEst:   NewParameterEstimator(),
		Calibrator: &CalibrationManager{TimeElapsed: 0.0, IsCalibrated: false},
		GateKeeper: &ConfidenceGate{},
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
		CtrlCfg: ctrlCfg,
	}
}

// PredictCurrentState uses the EKF to filter telemetry and estimate the true kinematic state of the system.
func (ps *PredictionService) PredictCurrentState(
	zTelemetry *MeasurementVector, 
	currentSysState *SystemState, 
	lastDecision Bundle, 
	dt float64,
) {
	if dt <= 0.0 { dt = ps.SimCfg.Dt }

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

	// 2. EKF Predict & Update
	var inputU ControlVector
	inputU[0] = float64(lastDecision.Replicas) * currentSysState.ServiceRate
	inputU[1] = lastDecision.QueueLimit
	inputU[2] = lastDecision.CacheAggression

	ps.EKF.Predict(inputU, *currentSysState, ps.SimCfg, dt)
	ps.EKF.Update(*zTelemetry)
	filteredState := ps.EKF.X

	// 3. Update current system state with filtered beliefs
	currentSysState.QueueDepth = math.Max(0.0, filteredState[0])
	currentSysState.RetryPressure = math.Max(0.0, filteredState[1])
	currentSysState.PredictedArrival = math.Max(0.001, filteredState[3])
	currentSysState.CapacityVelocity = filteredState[4]

	predictedLatency := ComputeLatency(currentSysState.QueueDepth, currentSysState.PredictedArrival, filteredState[2], ps.SimCfg.BaseLatency, ps.SimCfg.MaxQueueDelay)
	observedLatency := zTelemetry[1] 
	currentSysState.Latency = (0.1 * math.Min(predictedLatency, 10.0)) + (0.9 * observedLatency)
}

// UpdateModel learns from the environment to update the simulation parameters (SysID).
func (ps *PredictionService) UpdateModel(currentSysState *SystemState, dt float64) {
	filteredCapacity := ps.EKF.X[2]
	
	currentObs := Observation{ 
		Queue: currentSysState.QueueDepth, 
		Latency: currentSysState.Latency, 
		Retry: currentSysState.RetryPressure, 
		Arrival: currentSysState.PredictedArrival, 
		Capacity: filteredCapacity, 
		Time: ps.Calibrator.TimeElapsed,
	}

	if ps.Calibrator.Track(dt, ps.CtrlCfg) {
		ps.SysID.Update(currentObs)
		if ps.GateKeeper.Validate(ps.SysID.Confidence(), ps.CtrlCfg) {
			ps.ParamEst.Update(ps.SysID.Parameters(), ps.SysID.Confidence())
			ps.ParamEst.Apply(&ps.SimCfg)
		}
	}
}

// GetSimConfig returns the latest dynamically learned simulation parameters.
func (ps *PredictionService) GetSimConfig() SimConfig {
	return ps.SimCfg
}
