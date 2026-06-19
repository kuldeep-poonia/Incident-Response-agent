package control

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// 
// HARDCORE REAL-WORLD PLANT SIMULATOR
// This simulates the cruel reality of Kubernetes and Envoy. It intentionally 
// includes model-mismatch, sensor noise, and sluggish container startups.
// 

type RealWorldPlant struct {
	Queue            float64
	RetryPressure    float64
	ReadyReplicas    float64
	Provisioning     float64
	TrueArrival      float64
	TrueServiceRate  float64
	K8sWarmupDelay   float64 // Hard 30-second delay for pods to spin up
	EnvoyConfigDelay float64 // 5-second delay for xDS config sync
	
	ActualQueueLimit float64
	ActualRetryLimit float64
}

func (p *RealWorldPlant) Tick(dt float64, cmd Bundle) MeasurementVector {
	// 1. Sluggish Kubernetes Actuation (Pods take time to become ready)
	diff := float64(cmd.Replicas) - (p.ReadyReplicas + p.Provisioning)
	if diff > 0 {
		p.Provisioning += diff // Immediately request pods from scheduler
	}
	// Warmup pods based on delay
	warmedUp := (p.Provisioning / p.K8sWarmupDelay) * dt
	p.Provisioning -= warmedUp
	p.ReadyReplicas += warmedUp

	// 2. Sluggish Envoy Mesh Propagation
	alpha := 1.0 - math.Exp(-dt/p.EnvoyConfigDelay)
	p.ActualQueueLimit = (alpha * float64(cmd.QueueLimit)) + ((1.0 - alpha) * p.ActualQueueLimit)
	p.ActualRetryLimit = (alpha * float64(cmd.RetryLimit)) + ((1.0 - alpha) * p.ActualRetryLimit)

	// 3. Brutal Real-World Physics (Load, Drops, and Efficiency Degradation)
	retryAmplification := 1.0 + 0.2*math.Log(1.0+p.RetryPressure)
	inboundTraffic := p.TrueArrival * retryAmplification
	
	// Shed load if queue is full
	admissionProbability := 1.0 / (1.0 + (p.Queue / math.Max(1.0, p.ActualQueueLimit)))
	admitted := inboundTraffic * admissionProbability

	// Service degradation: If utilization > 100%, threads thrash, CPU burns, capacity drops
	rawCapacity := math.Max(1.0, p.ReadyReplicas) * p.TrueServiceRate
	utilization := admitted / rawCapacity
	actualService := rawCapacity / (1.0 + 0.15*math.Max(0.0, utilization-1.0))

	// Queue Mass Balance
	p.Queue += (admitted - actualService) * dt
	p.Queue = math.Max(0.0, p.Queue)

	// Retry Pressure Accumulation (Clients get angry if latency > 100ms)
	latency := 0.005
	if admitted > 0 { latency += p.Queue / admitted }
	
	slaBreach := math.Max(0.0, latency-0.100) 
	p.RetryPressure += ((0.30 * (p.Queue / math.Max(1.0, p.ActualQueueLimit))) + (0.50 * slaBreach) - (0.25 * p.RetryPressure)) * dt
	p.RetryPressure = math.Max(0.0, p.RetryPressure)

	// 4. Noisy Telemetry Generation (Injecting +/- 5% sensor noise)
	noise := func(val float64) float64 { return val * (1.0 + ((rand.Float64() - 0.5) * 0.10)) }

	return MeasurementVector{
		noise(p.Queue),
		noise(math.Min(latency, 2.0)), // Latency capped at 2s timeout
		noise(p.RetryPressure),
		noise(actualService),
		noise(p.TrueArrival),
	}
}

// 
// THE TEST RUNNER
// 

func TestRealWorld_FlashCrowd_And_RetryStorm(t *testing.T) {
	
	fmt.Println(" REAL-WORLD SIMULATION: 10x BLACK FRIDAY FLASH CROWD (K8s Warmup = 30s)")
	
	fmt.Printf("%-5s | %-12s | %-16s | %-17s | %-10s | %-8s | %-12s\n", 
		"Time", "True Load", "Pods (Rdy/Tgt)", "Envoy (Q/Retry)", "Latency", "Risk", "Econ Cost/s")
	fmt.Println("---------------------------------------------------------------------------------------------------------")

	// 1. Initialize the Controller
	ctrl := NewController(42, 10, 500, DefaultControllerConfig())
	mem := NewRegimeMemory(DefaultRegimeConfig())
	
	// Define the starting system state
	sysState := SystemState{
		Replicas: 10, QueueLimit: 1000, RetryLimit: 3, CacheAggression: 0.0,
		PredictedArrival: 100.0, ServiceRate: 10.0, SLATarget: 0.100, // 100ms SLA
		MinReplicas: 10, MaxReplicas: 200, MinRetry: 0, MaxRetry: 5, Survival: 1.0,
	}

	// 2. Initialize the Cruel Real-World Plant
	plant := RealWorldPlant{
		Queue: 0, RetryPressure: 0, ReadyReplicas: 10, Provisioning: 0, 
		TrueArrival: 100.0, TrueServiceRate: 10.0, 
		K8sWarmupDelay: 30.0, EnvoyConfigDelay: 5.0, 
		ActualQueueLimit: 1000, ActualRetryLimit: 3,
	}

	dt := 1.0
	cumulativeCost := 0.0

	// 3. Run the Simulation Loop for 90 Seconds
	for sec := 0; sec <= 90; sec++ {
		
		// THE CHAOS INJECTION
		if sec == 20 {
			plant.TrueArrival = 2000.0 // Massive 20x Flash Crowd hits exactly at T=20
		}
		if sec == 60 {
			plant.TrueArrival = 500.0 // Traffic subsides slightly
		}

		// A. The physical world advances, sensors emit noisy telemetry
		zTelemetry := plant.Tick(dt, ctrl.LastDecision)

		// B. The Autopilot thinks, calculates SDEs, MPC optimization, and returns a command
		sysState = ctrl.Tick(zTelemetry, &sysState, mem, dt)
		cmd := ctrl.LastDecision

		// Calculate Current Second's Financial Burn
		currentCost := (float64(cmd.Replicas) * ctrl.EconCfg.InfraUSDPerSec) + 
			(math.Max(0, sysState.Latency-sysState.SLATarget) * ctrl.EconCfg.SLABreachUSDPerSec)
		cumulativeCost += currentCost

		// C. Print the exact truth every 5 seconds to observe the dynamic response
		if sec%5 == 0 {
			fmt.Printf("T+%03ds | %-12.1f | %4.0f rdy / %-5d | Q:%-4.0f Retry:%-3d | %-8.3fs | %-6.2f | $%-.4f/s\n",
				sec,
				plant.TrueArrival,
				plant.ReadyReplicas, cmd.Replicas,
				cmd.QueueLimit, cmd.RetryLimit,
				sysState.Latency,
				mem.GainScheduledRisk,
				currentCost,
			)
		}
	}

	fmt.Println("---------------------------------------------------------------------------------------------------------")
	fmt.Printf("SIMULATION COMPLETE. Total Financial Burn: $%.4f\n", cumulativeCost)
	fmt.Println("=========================================================================================================")

	// Assertion: If the controller failed to scale, latency would explode past 2.0s, and survival would be 0.
	if sysState.Survival < 0.1 {
		t.Fatalf("CONTROLLER FAILED: System collapsed under load. Survival: %.2f", sysState.Survival)
	}
}