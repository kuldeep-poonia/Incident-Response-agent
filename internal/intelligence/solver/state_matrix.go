package control

type StateMatrix struct {
	A [6][6]float64
	B [6][3]float64
}

func BuildStateMatrix(s DynamicState, cfg SimConfig) StateMatrix {
	var m StateMatrix

	// A MATRIX
	m.A[0][4] = 1.0
	m.A[0][3] = -1.0
	m.A[1][0] = 1.0
	m.A[2][1] = cfg.RetryGamma
	m.A[2][2] = -cfg.RetryBeta
	m.A[3][5] = 1.0
	m.A[5][3] = -cfg.NaturalFrequency * cfg.NaturalFrequency
	m.A[5][5] = -2 * cfg.DampingRatio * cfg.NaturalFrequency
	m.A[4][4] = -cfg.ArrivalTheta

	// B MATRIX
	m.B[5][0] = cfg.NaturalFrequency * cfg.NaturalFrequency
	m.B[0][1] = -0.10
	m.B[4][2] = -0.05

	return m
}

func StateDerivative(x [6]float64, u [3]float64, m StateMatrix) [6]float64 {
	var dx [6]float64
	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			dx[i] += m.A[i][j] * x[j]
		}
		for j := 0; j < 3; j++ {
			dx[i] += m.B[i][j] * u[j]
		}
	}
	return dx
}

func ToVectorLegacy(s SystemState, capacity float64) [6]float64 {
	return [6]float64{
		s.QueueDepth,
		s.Latency,
		s.RetryPressure,
		capacity,
		s.PredictedArrival,
		s.CapacityVelocity,
	}
}

func FromVectorLegacy(x [6]float64, s *SystemState) {
	s.QueueDepth = x[0]
	s.Latency = x[1]
	s.RetryPressure = x[2]
	// x[3] represents the system capacity mapping natively
	s.PredictedArrival = x[4]
	s.CapacityVelocity = x[5]
}