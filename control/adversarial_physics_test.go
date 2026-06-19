package control

import (
	"math"
	"testing"
)

// ============================================================================
// TEST 3: ACTUATOR LOCKUP (THE FROZEN KUBE-APISERVER)
// Proves the controller will use the Envoy Mesh to save the system if K8s
// refuses to scale pods (e.g., Node auto-scaler out of quota).
// ============================================================================
func TestAdversarial_FrozenActuator(t *testing.T) {
	ctrl := NewController(42, 10, 1000, DefaultControllerConfig())
	mem := NewRegimeMemory(DefaultRegimeConfig())
	plant := NewChaosPlant(42) // FIXED: Passed the seed to avoid the compiler error
	
	sysState := SystemState{ Replicas: 10, QueueLimit: 1000, RetryLimit: 3, ServiceRate: 10.0, SLATarget: 0.100, Survival: 1.0, MaxReplicas: 1000 }

	// Lock the plant's pods at 10 permanently.
	plant.ReadyReplicas = 10

	for sec := 0; sec < 60; sec++ {
		plant.TrueArrival = 2000.0 // 20x Load

		// FORCE ACTUATOR FAILURE: Delete provisioning pods so they never spin up
		plant.ProvisioningPods = nil 
		plant.ReadyReplicas = 10.0

		zTelemetry := plant.Tick(1.0, ctrl.LastDecision)
		sysState = ctrl.Tick(zTelemetry, &sysState, mem, 1.0)
	}

	// ASSERTION: Because K8s is broken, the controller MUST have crushed the Envoy queue to shed load.
	if ctrl.LastDecision.QueueLimit > 500 {
		t.Fatalf("ACTUATOR BLINDNESS: Controller failed to use Mesh defense when K8s froze. QueueLimit: %.2f", ctrl.LastDecision.QueueLimit)
	}
}

// ============================================================================
// TEST 4: RESONANT FREQUENCY ATTACK (WORST-CASE SINE WAVE)
// Proves the controller doesn't enter destructive oscillation.
// ============================================================================
func TestAdversarial_ResonantSineWave(t *testing.T) {
	ctrl := NewController(42, 10, 1000, DefaultControllerConfig())
	mem := NewRegimeMemory(DefaultRegimeConfig())
	plant := NewChaosPlant(42) // FIXED: Passed the seed to avoid the compiler error
	
	sysState := SystemState{ Replicas: 10, QueueLimit: 1000, RetryLimit: 3, ServiceRate: 10.0, SLATarget: 0.100, Survival: 1.0, MaxReplicas: 1000 }

	// The controller's internal Damping/Frequency config
	tau := ctrl.ActuatorCfg.WarmupTau // ~30s
	
	// We attack the system with a sine wave perfectly matching its actuation delay
	// to induce mathematical resonance (the hardest thing for a controller to handle).
	for sec := 0.0; sec < 300.0; sec += 1.0 {
		// Oscillate load between 100 and 2000 every 30 seconds
		plant.TrueArrival = 1050.0 + 950.0*math.Sin((2.0*math.Pi/tau)*sec)

		zTelemetry := plant.Tick(1.0, ctrl.LastDecision)
		sysState = ctrl.Tick(zTelemetry, &sysState, mem, 1.0)
	}

	// ASSERTION: The controller must not have amplified the wave into a collapse.
	if sysState.Survival < 0.10 {
		t.Fatalf("RESONANCE COLLAPSE: The adversarial wave destroyed the controller.")
	}
}