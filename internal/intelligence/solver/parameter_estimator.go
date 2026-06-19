package control

type ParameterEstimator struct {
	ArrivalTheta float64
	ArrivalMean  float64
	ArrivalSigma float64
	RetryAlpha float64
	RetryBeta  float64
	RetryGamma float64
	LearningRate float64
	MinArrivalTheta float64
	MaxArrivalTheta float64
	MinArrivalSigma float64
	MaxArrivalSigma float64
	MinRetryAlpha float64
	MaxRetryAlpha float64
	MinRetryBeta float64
	MaxRetryBeta float64
	MinRetryGamma float64
	MaxRetryGamma float64
	MaxArrivalThetaStep float64
	MaxArrivalMeanStep  float64
	MaxArrivalSigmaStep float64
	MaxRetryAlphaStep float64
	MaxRetryBetaStep  float64
	MaxRetryGammaStep float64
}

func NewParameterEstimator() *ParameterEstimator {
	return &ParameterEstimator{
		LearningRate: 0.05,
		ArrivalTheta: 0.25,
		ArrivalMean:  100,
		ArrivalSigma: 0.10,
		RetryAlpha: 0.30,
		RetryBeta:  0.25,
		RetryGamma: 0.50,
		MinArrivalTheta: 0.001,
		MaxArrivalTheta: 5.0,
		MinArrivalSigma: 0.0001,
		MaxArrivalSigma: 100.0,
		MinRetryAlpha: 0.0,
		MaxRetryAlpha: 10.0,
		MinRetryBeta: 0.0,
		MaxRetryBeta: 10.0,
		MinRetryGamma: 0.0,
		MaxRetryGamma: 10.0,
		MaxArrivalThetaStep: 0.05,
		MaxArrivalMeanStep:  10.0,
		MaxArrivalSigmaStep: 0.05,
		MaxRetryAlphaStep: 0.05,
		MaxRetryBetaStep:  0.05,
		MaxRetryGammaStep: 0.05,
	}
}

func (pe *ParameterEstimator) Update(params ParameterEstimate, conf ParameterConfidence) {
	thetaTarget := blend(pe.ArrivalTheta, clampf(params.ArrivalTheta, pe.MinArrivalTheta, pe.MaxArrivalTheta), pe.LearningRate*conf.ArrivalProcess)
	pe.ArrivalTheta = rateLimit(pe.ArrivalTheta, thetaTarget, pe.MaxArrivalThetaStep)

	arrivalMeanTarget := blend(pe.ArrivalMean, params.ArrivalMean, pe.LearningRate*conf.ArrivalProcess)
	pe.ArrivalMean = rateLimit(pe.ArrivalMean, arrivalMeanTarget, pe.MaxArrivalMeanStep)

	arrivalSigmaTarget := blend(pe.ArrivalSigma, clampf(params.ArrivalSigma, pe.MinArrivalSigma, pe.MaxArrivalSigma), pe.LearningRate*conf.ArrivalProcess)
	pe.ArrivalSigma = rateLimit(pe.ArrivalSigma, arrivalSigmaTarget, pe.MaxArrivalSigmaStep)

	retryAlphaTarget := blend(pe.RetryAlpha, clampf(params.RetryAlpha, pe.MinRetryAlpha, pe.MaxRetryAlpha), pe.LearningRate*conf.RetryProcess)
	pe.RetryAlpha = rateLimit(pe.RetryAlpha, retryAlphaTarget, pe.MaxRetryAlphaStep)

	retryBetaTarget := blend(pe.RetryBeta, clampf(params.RetryBeta, pe.MinRetryBeta, pe.MaxRetryBeta), pe.LearningRate*conf.RetryProcess)
	pe.RetryBeta = rateLimit(pe.RetryBeta, retryBetaTarget, pe.MaxRetryBetaStep)

	retryGammaTarget := blend(pe.RetryGamma, clampf(params.RetryGamma, pe.MinRetryGamma, pe.MaxRetryGamma), pe.LearningRate*conf.RetryProcess)
	pe.RetryGamma = rateLimit(pe.RetryGamma, retryGammaTarget, pe.MaxRetryGammaStep)
}

func (pe *ParameterEstimator) Apply(cfg *SimConfig) {
	cfg.ArrivalTheta = pe.ArrivalTheta
	cfg.ArrivalMean = pe.ArrivalMean
	cfg.ArrivalSigma = pe.ArrivalSigma
	cfg.RetryAlpha = pe.RetryAlpha
	cfg.RetryBeta = pe.RetryBeta
	cfg.RetryGamma = pe.RetryGamma
}

func (pe *ParameterEstimator) Snapshot() ParameterEstimate {
	return ParameterEstimate{
		ArrivalTheta: pe.ArrivalTheta,
		ArrivalMean:  pe.ArrivalMean,
		ArrivalSigma: pe.ArrivalSigma,
		RetryAlpha: pe.RetryAlpha,
		RetryBeta:  pe.RetryBeta,
		RetryGamma: pe.RetryGamma,
	}
}

func blend(current float64, target float64, alpha float64) float64 {
	if alpha < 0 { alpha = 0 }
	if alpha > 1 { alpha = 1 }
	return current + alpha*(target-current)
}

func clampf(v float64, min float64, max float64) float64 {
	if v < min { return min }
	if v > max { return max }
	return v
}

func rateLimit(current float64, target float64, maxStep float64) float64 {
	delta := target - current
	if delta > maxStep { return current + maxStep }
	if delta < -maxStep { return current - maxStep }
	return target
}
