package engine

import (
	"github.com/kuldeep-poonia/Incident-Response-agent/internal/observability/telemetry"
	"github.com/kuldeep-poonia/Incident-Response-agent/internal/intelligence/solver"
)

// AdaptWindowToMeasurement converts the time-series window into the Kalman Filter's state vector.
func AdaptWindowToMeasurement(w *telemetry.ServiceWindow) control.MeasurementVector {
	return control.MeasurementVector{
		float64(w.LastQueueDepth),        // [0] Queue (Requests)
		w.LastLatencyMs / 1000.0,         // [1] Latency (Seconds)
		w.LastErrorRate * 100.0,          // [2] RetryPressure (Proxy: Error % translates to retry load)
		w.MeanRequestRate,                // [3] Capacity (Successfully serviced requests/sec)
		w.LastRequestRate,                // [4] Arrival (Current inbound requests/sec)
	}
}

// EnsureDefaultState initializes a safe control state if the service is new.
func EnsureDefaultState(s *control.SystemState) {
	if s.Replicas == 0 {
		s.Replicas = 10
		s.QueueLimit = 1000
		s.RetryLimit = 3
		s.ServiceRate = 10.0
		s.SLATarget = 0.100 // 100ms
		s.MinReplicas = 2
		s.MaxReplicas = 500
		s.MaxRetry = 5
		s.Survival = 1.0
		s.PredictedArrival = 100.0
	}
}
