package control

import "math"

type ActionBundle = Bundle

type ActionBounds struct {
	MinReplicas int
	MaxReplicas int
	MinQueue    int
	MaxQueue    int
	MinRetry    int
	MaxRetry    int
	MinCache    float64
	MaxCache    float64
}

// Renamed to avoid conflict with bundle_generator.go
type LegacyGeneratorConfig struct {
	BaseRadius int
	Seed       int64
}

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

type BundleConfig struct {
	ReplicaRadius      int
	QueueRadius        int
	CacheRadius        int
	MaxScaleStep       int
	MinReplicas        int
	MaxReplicas        int
	QueueStep          float64
	MinQueue           float64
	MaxQueue           float64
	MinRetry           int
	MaxRetry           int
	CacheStep          float64
	MinCache           float64
	MaxCache           float64
	RetryAmplification float64
	EfficiencyDecay    float64
	TargetUtil         float64
	QueueWeight        float64
	ReplicaMovePenalty float64
	QueueMovePenalty   float64
	RetryMovePenalty   float64
	CacheMovePenalty   float64
	MinBundleDistance  float64
	TopK               int
	GenerationKeepProb float64
	Seed               int64
}

func (r *RegimeMemory) UpdateCostTrend(delta float64) {
	r.CostTrendEWMA = 0.8*r.CostTrendEWMA + 0.2*delta
}

func (r *RegimeMemory) RecordAction(next ActionBundle) {
	dist := math.Abs(float64(next.Replicas-r.LastAction.Replicas)) +
		0.2*math.Abs(next.QueueLimit-r.LastAction.QueueLimit) +
		0.5*math.Abs(float64(next.RetryLimit-r.LastAction.RetryLimit)) +
		math.Abs(next.CacheAggression-r.LastAction.CacheAggression)

	r.OscillationEWMA = 0.85*r.OscillationEWMA + 0.15*dist
	r.LastAction = next
}

func defaultRegimeConfigLegacy() RegimeConfig {
	return RegimeConfig{
		EWMAAlpha:        0.20,
		HistorySize:      64,
		BaseUtilStress:   0.70,
		BaseRiskUnstable: 0.60,
		HysteresisMargin: 0.05,
	}
}

func GenerateLocalBundles(
	current SystemState,
	cfg LegacyGeneratorConfig,
	mem *RegimeMemory,
) []Bundle {
	// Logic retained to wire legacy calls without breaking main Generator.
	return []Bundle{} // Returns empty slice natively, deferred to new generator logic
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}