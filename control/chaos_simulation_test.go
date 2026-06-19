package control

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// JSON EXPORT SCHEMAS
// ============================================================================
type SimulationTick struct {
	TimeSec         int     `json:"time_sec"`
	Event           string  `json:"event"`
	PodsReady       float64 `json:"pods_ready"`
	PodsTarget      int     `json:"pods_target"`
	EnvoyActualQ    float64 `json:"envoy_actual_q"`
	EnvoyCommandQ   float64 `json:"envoy_cmd_q"`
	PhysicalQueue   float64 `json:"physical_queue"`
	RetryPool       float64 `json:"retry_pool"`
	PhysicalLatency float64 `json:"physical_latency"`
	CtrlLatency     float64 `json:"ctrl_latency"`
	RiskScore       float64 `json:"risk_score"`
}

type SimulationReport struct {
	ScenarioName string           `json:"scenario_name"`
	Ticks        []SimulationTick `json:"ticks"`
}

type MonteCarloReport struct {
	TotalRuns       int     `json:"total_runs"`
	DurationSeconds float64 `json:"duration_seconds"`
	ExpectedCost    float64 `json:"expected_cost_usd"`
	VaR95           float64 `json:"value_at_risk_95_usd"`
	CVaR95          float64 `json:"tail_risk_cvar_95_usd"`
}

// ============================================================================
// THE CHAOS PLANT (PHYSICS ENGINE)
// ============================================================================
type ChaosPlant struct {
	PhysicalQueue          float64
	RetryPool              float64
	ReadyReplicas          float64
	ProvisioningPods       []float64
	TrueArrival            float64
	BaseServiceRate        float64
	NetworkPartitionActive bool
	DownstreamFailing      bool
	ActualQueueLimit       float64
	ActualRetryLimit       float64
	LastTelemetry          MeasurementVector
	RNG                    *rand.Rand
}

func NewChaosPlant(seed int64) *ChaosPlant {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &ChaosPlant{
		PhysicalQueue:    0,
		RetryPool:        0,
		ReadyReplicas:    10,
		TrueArrival:      100.0,
		BaseServiceRate:  10.0,
		ActualQueueLimit: 1000,
		ActualRetryLimit: 3,
		RNG:              rand.New(rand.NewSource(seed)),
	}
}

func (p *ChaosPlant) Tick(dt float64, cmd Bundle) MeasurementVector {
	targetDiff := int(cmd.Replicas) - int(p.ReadyReplicas) - len(p.ProvisioningPods)
	if targetDiff > 0 {
		for i := 0; i < targetDiff; i++ {
			p.ProvisioningPods = append(p.ProvisioningPods, 15.0+(p.RNG.Float64()*15.0))
		}
	}

	var survivingPods []float64
	for _, timer := range p.ProvisioningPods {
		if timer-dt <= 0 {
			p.ReadyReplicas++
		} else {
			survivingPods = append(survivingPods, timer-dt)
		}
	}
	p.ProvisioningPods = survivingPods

	if !p.NetworkPartitionActive {
		alpha := 1.0 - math.Exp(-dt/5.0)
		p.ActualQueueLimit += (float64(cmd.QueueLimit) - p.ActualQueueLimit) * alpha
		p.ActualRetryLimit += (float64(cmd.RetryLimit) - p.ActualRetryLimit) * alpha
	}

	alphaContention := 0.05
	betaCoherency := 0.00
	if p.DownstreamFailing {
		betaCoherency = 0.08
	}

	N := math.Max(1.0, p.ReadyReplicas)
	uslMultiplier := N / (1.0 + alphaContention*(N-1.0) + betaCoherency*N*(N-1.0))
	actualServiceCapacity := p.BaseServiceRate * uslMultiplier

	retryStrikeRate := p.RetryPool / math.Max(1.0, p.ActualRetryLimit)
	totalOfferedLoad := p.TrueArrival + retryStrikeRate

	pressureRatio := p.PhysicalQueue / math.Max(1.0, p.ActualQueueLimit)
	dropProbability := 1.0 / (1.0 + math.Exp(-10.0*(pressureRatio-1.0)))
	if p.ActualQueueLimit <= 1.0 {
		dropProbability = 1.0
	}

	admitted := totalOfferedLoad * (1.0 - dropProbability)
	rejected := totalOfferedLoad * dropProbability

	processed := math.Min(admitted+p.PhysicalQueue, actualServiceCapacity)
	p.PhysicalQueue += (admitted - processed) * dt
	p.PhysicalQueue = math.Max(0.0, p.PhysicalQueue)

	clientAbandonRate := 0.15
	p.RetryPool += (rejected - retryStrikeRate - (p.RetryPool * clientAbandonRate)) * dt
	p.RetryPool = math.Max(0.0, p.RetryPool)

	latency := 0.005
	utilization := admitted / math.Max(0.001, actualServiceCapacity)
	if utilization > 0.01 {
		safeUtil := math.Min(utilization, 0.99)
		latency += (safeUtil / (1.0 - safeUtil)) * (1.0 / p.BaseServiceRate)
	}
	if actualServiceCapacity > 0.001 {
		latency += p.PhysicalQueue / actualServiceCapacity
	}

	noise := func(val float64) float64 { return val * (1.0 + ((p.RNG.Float64() - 0.5) * 0.10)) }

	p.LastTelemetry = MeasurementVector{
		math.Max(0.0, noise(p.PhysicalQueue)),
		noise(math.Min(latency, 10.0)),
		noise(p.RetryPool / 100.0),
		noise(actualServiceCapacity),
		noise(p.TrueArrival),
	}
	return p.LastTelemetry
}

// ============================================================================
// DEATH SPIRAL SIMULATION (WITH JSON EXPORT)
// ============================================================================
func TestChaos_CascadingDeathSpiral(t *testing.T) {
	fmt.Println("\n======================================================================================================================")
	fmt.Println(" ENTERPRISE CHAOS SIMULATION: FLASH CROWD -> NODE DEATH -> PARTITION -> DOWNSTREAM CASCADE")
	fmt.Println("======================================================================================================================")
	fmt.Printf("%-5s | %-16s | %-13s | %-15s | %-17s | %-15s | %-6s\n",
		"Time", "Events", "Pods(Rdy/Tgt)", "Envoy(AQ/CQ)", "Phys(Queue/Pool)", "Lat(Phys/Ctrl)", "Risk")
	fmt.Println("----------------------------------------------------------------------------------------------------------------------")

	ctrlCfg := DefaultControllerConfig()
	ctrl := NewController(42, 10, 1000, ctrlCfg)
	mem := NewRegimeMemory(DefaultRegimeConfig())
	plant := NewChaosPlant(42)

	ctrl.EconCfg.InfraUSDPerSec = 0.0001
	ctrl.EconCfg.SLABreachUSDPerSec = 1.0

	sysState := SystemState{
		Replicas: 10, QueueLimit: 1000, RetryLimit: 3, PredictedArrival: 100.0,
		ServiceRate: 10.0, SLATarget: 0.100, MinReplicas: 10, MaxReplicas: 1000, MaxRetry: 5, Survival: 1.0,
	}

	file, err := os.Create("simulation_telemetry.jsonl")
if err != nil {
    t.Fatalf("failed to create telemetry file: %v", err)
}
defer file.Close()

encoder := json.NewEncoder(file)

	for sec := 0; sec <= 120; sec++ {
		eventMarker := "-"

		if sec == 15 {
			plant.TrueArrival = 4000.0
			eventMarker = "🔥 Flash Crowd"
		}
		if sec == 30 {
			plant.ReadyReplicas = 5.0
			eventMarker = "💀 Node Failure"
		}
		if sec == 45 {
			plant.NetworkPartitionActive = true
			eventMarker = "⚡ Net Partition"
		}
		if sec == 60 {
			plant.DownstreamFailing = true
			eventMarker = "💥 Cascade Fail"
		}
		if sec == 80 {
			plant.NetworkPartitionActive = false
			eventMarker = "🩹 Net Restored"
		}
		if sec == 100 {
			plant.DownstreamFailing = false
			plant.TrueArrival = 500.0
			eventMarker = "📉 Load Drops"
		}

		zTelemetry := plant.Tick(1.0, ctrl.LastDecision)
		sysState = ctrl.Tick(zTelemetry, &sysState, mem, 1.0)
		cmd := ctrl.LastDecision

		record := SimulationTick{
    TimeSec:         sec,
    Event:           eventMarker,
    PodsReady:       math.Round(plant.ReadyReplicas*100) / 100,
    PodsTarget:      cmd.Replicas,
    EnvoyActualQ:    math.Round(plant.ActualQueueLimit*100) / 100,
    EnvoyCommandQ:   math.Round(cmd.QueueLimit*100) / 100,
    PhysicalQueue:   math.Round(plant.PhysicalQueue*100) / 100,
    RetryPool:       math.Round(plant.RetryPool*100) / 100,
    PhysicalLatency: math.Round(plant.LastTelemetry[1]*1000) / 1000,
    CtrlLatency:     math.Round(sysState.Latency*1000) / 1000,
    RiskScore:       math.Round(mem.GainScheduledRisk*100) / 100,
}

if err := encoder.Encode(record); err != nil {
    t.Fatalf("failed to encode telemetry record: %v", err)
}

		if sec%5 == 0 || eventMarker != "-" {
			fmt.Printf("T+%03ds | %-16s | %4.0f / %-4d | AQ:%-4.0f CQ:%-4.0f | Q:%-5.0f Pool:%-4.0f | P:%-5.2fs C:%-5.2fs | %-6.2f\n",
				sec, eventMarker,
				plant.ReadyReplicas, cmd.Replicas,
				plant.ActualQueueLimit, cmd.QueueLimit,
				plant.PhysicalQueue, plant.RetryPool,
				plant.LastTelemetry[1], sysState.Latency, mem.GainScheduledRisk)
		}
	}

}


// monte carlo with json file output

func TestChaos_MonteCarlo_10000_Scenarios(t *testing.T) {
	fmt.Println("\n=========================================================================================================")
	fmt.Println(" statistical proof - 5k monte carlo chaos features (CVaR bounding)")
	fmt.Println("=========================================================================================================")

	totalRuns := 5000
	var finalCosts []float64
	var mu sync.Mutex
	start := time.Now()

	concurrency := runtime.NumCPU()
	if concurrency < 1 {
		concurrency = 1
	}
	runsPerWorker := totalRuns / concurrency
	var wg sync.WaitGroup

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRNG := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			localCosts := make([]float64, 0, runsPerWorker)

			for i := 0; i < runsPerWorker; i++ {
				ctrl := NewController(localRNG.Int63(), 10, 500, DefaultControllerConfig())
				mem := NewRegimeMemory(DefaultRegimeConfig())
				plant := NewChaosPlant(localRNG.Int63())

				plant.TrueArrival = 100.0 + (localRNG.Float64() * 5000.0)
				sysState := SystemState{Replicas: 10, QueueLimit: 1000, RetryLimit: 3, ServiceRate: 10.0, SLATarget: 0.100, Survival: 1.0}

				if localRNG.Float64() < 0.10 {
					plant.NetworkPartitionActive = true
				}
				if localRNG.Float64() < 0.05 {
					plant.DownstreamFailing = true
				}

				runCost := 0.0
				for step := 0; step < 10; step++ {
					zTelemetry := plant.Tick(1.0, ctrl.LastDecision)
					sysState = ctrl.Tick(zTelemetry, &sysState, mem, 1.0)
					runCost += (float64(ctrl.LastDecision.Replicas) * ctrl.EconCfg.InfraUSDPerSec) +
						(math.Max(0, sysState.Latency-sysState.SLATarget) * ctrl.EconCfg.SLABreachUSDPerSec)
				}
				localCosts = append(localCosts, runCost)
			}

			mu.Lock()
			finalCosts = append(finalCosts, localCosts...)
			mu.Unlock()
		}(c)
	}

	wg.Wait()
	duration := time.Since(start)

	sort.Float64s(finalCosts)
	meanCost := 0.0
	for _, c := range finalCosts {
		meanCost += c
	}
	meanCost /= float64(len(finalCosts))

	varIdx := int(math.Floor(0.95 * float64(len(finalCosts))))
	if varIdx >= len(finalCosts) {
		varIdx = len(finalCosts) - 1
	}
	cvarSum := 0.0
	for i := varIdx; i < len(finalCosts); i++ {
		cvarSum += finalCosts[i]
	}
	cvar := cvarSum / math.Max(1.0, float64(len(finalCosts)-varIdx))

	mcReport := MonteCarloReport{
		TotalRuns:       len(finalCosts),
		DurationSeconds: math.Round(duration.Seconds()*1000) / 1000,
		ExpectedCost:    math.Round(meanCost*10000) / 10000,
		VaR95:           math.Round(finalCosts[varIdx]*10000) / 10000,
		CVaR95:          math.Round(cvar*10000) / 10000,
	}

	mcData, err := json.MarshalIndent(mcReport, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal Monte Carlo JSON: %v", err)
	}
	err = os.WriteFile("monte_carlo_results.json", mcData, 0644)
	if err != nil {
		t.Fatalf("Failed to write Monte Carlo JSON file: %v", err)
	}

	fmt.Printf("Completed %d full MPC/EKF/USL physics rollouts in %v.\n", len(finalCosts), duration)
	fmt.Printf("📊 Average Expected Cost:   $%.4f\n", meanCost)
	fmt.Printf("🛡️  95%% Value at Risk:       $%.4f\n", finalCosts[varIdx])
	fmt.Printf("🚨 95%% Tail Risk (CVaR):    $%.4f\n", cvar)
	fmt.Println("💾 Saved statistical proof to 'monte_carlo_results.json'")
	fmt.Println("=========================================================================================================")
}