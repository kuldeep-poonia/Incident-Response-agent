package rootcause

// RootCauseService serves as the Root Cause Agent
// Responsibilities: anomaly classification, cascade detection, incident severity
type RootCauseService struct {}

func NewRootCauseService() *RootCauseService {
	return &RootCauseService{}
}

func (s *RootCauseService) AnalyzeIncident(
	backlog float64, 
	backlogRate float64, 
	latency float64, 
	latencyRate float64, 
	retryRate float64, 
	oscillation float64, 
	utilization float64,
	confidence float64,
) (float64, string, AnomalyType) {
	
	// 1. Instability Analysis
	instInput := InstabilityInput{
		Backlog:     backlog,
		BacklogRate: backlogRate,
		Latency:     latency,
		LatencyRate: latencyRate,
		RetryRate:   retryRate,
		Oscillation: oscillation,
		Utilization: utilization,
	}
	score, severity := ComputeInstability(instInput)
	
	// 2. Anomaly Classification (Cascade detection)
	anomalyType := Classify(AnomalyInput{
		Instability:   score,
		Confidence:    confidence,
		BacklogGrowth: backlogRate,
		LatencyTrend:  latencyRate,
		RetryPressure: retryRate,
		Oscillation:   oscillation,
	})
	
	return score, severity, anomalyType
}
