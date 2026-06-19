package control

import "math"





// Renamed to avoid conflict with bundle_generator.go


// Unified SimConfig containing all physical and legacy fields
type SimConfig struct {
	HorizonSteps      int
	BaseLatency       float64
	Dt                float64
	NaturalFrequency  float64
	DampingRatio      float64
	ArrivalTheta      float64
	ArrivalMean       float64
	ArrivalSigma      float64
	RetryAlpha        float64
	RetryBeta         float64
	RetryGamma        float64
	DisturbanceStd    float64
	DisturbanceFreq   float64
	RetryFeedbackGain float64
	WarmupRate        float64
	EfficiencyDecay   float64
	MaxQueueDelay     float64
	HazardUtilGain    float64
	HazardBacklogGain float64
	HazardRetryGain   float64
	Seed              int64
}



func (r *RegimeMemory) UpdateCostTrend(delta float64) {
	r.CostTrendEWMA = 0.8*r.CostTrendEWMA + 0.2*delta
}

func (r *RegimeMemory) RecordAction(next Bundle) {
	dist := math.Abs(float64(next.Replicas-r.LastAction.Replicas)) +
		0.2*math.Abs(next.QueueLimit-r.LastAction.QueueLimit) +
		0.5*math.Abs(float64(next.RetryLimit-r.LastAction.RetryLimit)) +
		math.Abs(next.CacheAggression-r.LastAction.CacheAggression)

	r.OscillationEWMA = 0.85*r.OscillationEWMA + 0.15*dist
	r.LastAction = next
}




