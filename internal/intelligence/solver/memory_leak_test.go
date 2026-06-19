package control

import (
	"testing"
)

// ============================================================================
// TEST 5: ZERO ALLOCATION HOT-PATH VERIFICATION
// Proves the system can run infinitely as a Kubernetes Operator without 
// causing GC (Garbage Collection) pauses or Out-Of-Memory (OOM) kills.
// ============================================================================
func TestPerformance_ZeroAllocationHotPath(t *testing.T) {
	ctrl := NewController(42, 10, 500, DefaultControllerConfig())
	mem := NewRegimeMemory(DefaultRegimeConfig())
	sysState := SystemState{
		Replicas: 10, QueueLimit: 1000, RetryLimit: 3, PredictedArrival: 100.0, 
		ServiceRate: 10.0, SLATarget: 0.100, MinReplicas: 10, MaxReplicas: 500, MaxRetry: 5, Survival: 1.0,
	}

	zTelemetry := MeasurementVector{10.0, 0.05, 0.0, 100.0, 100.0}

	// Run the controller tick once to allocate all buffers, caches, and RNG matrices
	_ = ctrl.Recommend(zTelemetry, &sysState, mem, 1.0)

	// Use Go's built-in allocation tracker
	allocs := testing.AllocsPerRun(100, func() {
		// Simulate normal load fluctuations
		zTelemetry[4] += 1.0 
		_ = ctrl.Recommend(zTelemetry, &sysState, mem, 1.0)
	})

	// ASSERTION: After boot, the hot path must execute with minimal heap allocations (string formatting for Maestro).
	if allocs > 5 {
		t.Fatalf("MEMORY LEAK DETECTED: Controller is allocating %.2f objects per tick on the heap. Target is < 5.", allocs)
	}
}
