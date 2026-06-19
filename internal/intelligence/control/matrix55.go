package control

type Matrix55 [5][5]float64
type Matrix65 [6][5]float64
type Matrix56 [5][6]float64

func identity55() Matrix55 {
	var m Matrix55
	for i := 0; i < 5; i++ {
		m[i][i] = 1
	}
	return m
}

func transpose55(a Matrix55) Matrix55 {
	var out Matrix55
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			out[j][i] = a[i][j]
		}
	}
	return out
}

func add55(a Matrix55, b Matrix55) Matrix55 {
	var out Matrix55
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			out[i][j] = a[i][j] + b[i][j]
		}
	}
	return out
}

func sub55(a Matrix55, b Matrix55) Matrix55 {
	var out Matrix55
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			out[i][j] = a[i][j] - b[i][j]
		}
	}
	return out
}

func mul55(a Matrix55, b Matrix55) Matrix55 {
	var out Matrix55
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			for k := 0; k < 5; k++ {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

// Gaussian Elimination
func inverse55(a Matrix55) (Matrix55, bool) {
	var aug [5][10]float64
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			aug[i][j] = a[i][j]
		}
		aug[i][i+5] = 1
	}

	for col := 0; col < 5; col++ {
		pivot := col
		for row := col + 1; row < 5; row++ {
			if absf(aug[row][col]) > absf(aug[pivot][col]) {
				pivot = row
			}
		}
		if absf(aug[pivot][col]) < 1e-12 {
			return Matrix55{}, false
		}
		if pivot != col {
			aug[col], aug[pivot] = aug[pivot], aug[col]
		}
		pivotValue := aug[col][col]
		for j := 0; j < 10; j++ {
			aug[col][j] /= pivotValue
		}
		for row := 0; row < 5; row++ {
			if row == col {
				continue
			}
			factor := aug[row][col]
			for j := 0; j < 10; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	var inverse Matrix55
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			inverse[i][j] = aug[i][j+5]
		}
	}
	return inverse, true
}

func mul65x55(a Matrix65, b Matrix55) Matrix65 {
	var out Matrix65
	for i := 0; i < 6; i++ {
		for j := 0; j < 5; j++ {
			for k := 0; k < 5; k++ {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

func mul56x65(a Matrix56, b Matrix65) Matrix55 {
	var out Matrix55
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			for k := 0; k < 6; k++ {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}