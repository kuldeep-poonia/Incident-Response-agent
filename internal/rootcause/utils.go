package rootcause

import "math"

// clamp01 strictly enforces the invariant x ∈ [0, 1].
func clamp01(x float64) float64 {
	if x < 0 { return 0 }
	if x > 1 { return 1 }
	return x
}

// pos enforces the physical non-negativity constraint x ∈ [0, ∞).
func pos(x float64) float64 {
	if x < 0 { return 0 }
	return x
}

// norm rigorously projects an unbounded signal x ∈ [0, ∞) onto the compact set [0, 1).
func norm(x float64) float64 {
	if x <= 0 { return 0 }
	return x / (1.0 + x)
}

// boundedAgg computes a mathematically rigorous Smooth Maximum (Log-Sum-Exp) 
func boundedAgg(vals ...float64) float64 {
	n := len(vals)
	if n == 0 { return 0 }

	beta := 5.0 // Smoothness hardness parameter
	sumExp := 0.0

	for _, v := range vals {
		sumExp += math.Exp(beta * v)
	}

	smoothMax := (1.0 / beta) * math.Log(sumExp / float64(n))
	return smoothMax / (1.0 + smoothMax)
}
