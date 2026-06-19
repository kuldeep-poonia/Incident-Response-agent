package control

import (
	"math"
	"testing"
)

// ============================================================================
// TEST 1: BYZANTINE SENSOR FAILURE (NaN / Inf Injection)
// Proves the controller survives corrupted Prometheus metrics without panicking
// or poisoning its internal state matrices.
// ============================================================================
func TestEKF_ByzantineSensorSurvival(t *testing.T) {
	ctrl := NewController(42, 10, 100, DefaultControllerConfig())
	mem := NewRegimeMemory(DefaultRegimeConfig())
	sysState := SystemState{
		Replicas: 10, QueueLimit: 1000, RetryLimit: 3, PredictedArrival: 100.0, 
		ServiceRate: 10.0, SLATarget: 0.100, MinReplicas: 10, MaxReplicas: 100, MaxRetry: 5, Survival: 1.0,
	}

	// Define corrupted payloads
	poisonPills := []MeasurementVector{
		{math.NaN(), 0.05, 0, 100, 100},              // NaN Queue
		{100, math.Inf(1), 0, 100, 100},              // Infinite Latency
		{math.Inf(-1), -10.0, math.NaN(), 0, 0},      // Total Garbage
		{0, 0, 0, 0, 0},                              // Dead Sensors (Zeroes)
	}

	for i, poison := range poisonPills {
		// If the math shield fails, this will panic or permanently corrupt ekf.X
		_ = ctrl.Recommend(poison, &sysState, mem, 1.0)
		
		for j := 0; j < 9; j++ {
			if math.IsNaN(ctrl.PredService.EKF.X[j]) || math.IsInf(ctrl.PredService.EKF.X[j], 0) {
				t.Fatalf("CRITICAL VULNERABILITY: EKF Poisoned at step %d on state X[%d]", i, j)
			}
		}
	}
}

// ============================================================================
// TEST 2: EKF CONVERGENCE & HALLUCINATION RECOVERY
// Proves the EKF can snap to reality if its internal state diverges massively.
// ============================================================================
func TestEKF_ModelRealityGapConvergence(t *testing.T) {
	ctrl := NewController(42, 10, 100, DefaultControllerConfig())
	
	// Force the EKF to believe the system is perfectly healthy
	ctrl.PredService.EKF.X[0] = 0.0     // Queue = 0
	ctrl.PredService.EKF.X[1] = 0.0     // Retry = 0
	ctrl.PredService.EKF.X[3] = 10.0    // Arrival = 10
	
	// Suddenly inject a catastrophic physical reality
	disasterTelemetry := MeasurementVector{
		5000.0, // Massive Queue
		10.0,   // Saturated Latency
		150.0,  // Massive Retry Pressure
		50.0,   // Low Capacity
		4000.0, // Massive Arrival
	}

	// Tick once
	ctrl.PredService.EKF.Update(disasterTelemetry)

	// ASSERTION: The EKF must abandon its nominal math and snap to the sensors
	if ctrl.PredService.EKF.X[0] < 4000.0 {
		t.Fatalf("EKF HALLUCINATION: Failed to snap Queue to reality. Believes: %.2f", ctrl.PredService.EKF.X[0])
	}
	if ctrl.PredService.EKF.X[3] < 3000.0 {
		t.Fatalf("EKF HALLUCINATION: Failed to snap Arrival to reality. Believes: %.2f", ctrl.PredService.EKF.X[3])
	}
}
