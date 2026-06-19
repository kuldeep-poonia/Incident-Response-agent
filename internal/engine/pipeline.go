package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/qphysics/phaseshift/control"
	"github.com/qphysics/phaseshift/telemetry"
	"github.com/qphysics/phaseshift/topology"
)

type AutonomousEngine struct {
	Telemetry *telemetry.Store
	Topology  *topology.Graph

	mu          sync.RWMutex
	Controllers map[string]*control.Controller
	States      map[string]*control.SystemState
	Memories    map[string]*control.RegimeMemory
}

func NewAutonomousEngine() *AutonomousEngine {
	return &AutonomousEngine{
		Telemetry:   telemetry.NewStore(1024, 5000, 5*time.Minute),
		Topology:    topology.New(),
		Controllers: make(map[string]*control.Controller),
		States:      make(map[string]*control.SystemState),
		Memories:    make(map[string]*control.RegimeMemory),
	}
}

// 1. INGEST
func (e *AutonomousEngine) IngestMetric(p *telemetry.MetricPoint) {
	e.Telemetry.Ingest(p)
}

// 2. ANALYZE TOPOLOGY
func (e *AutonomousEngine) UpdateContext() topology.CriticalPath {
	// Build rolling windows (last 60 pts, 1 min cutoff)
	windows := e.Telemetry.AllWindows(60, 1*time.Minute)
	
	// Update graph edges based on windows
	e.Topology.Update(windows)
	
	// Extract the bottleneck
	snap := e.Topology.Snapshot()
	return snap.CriticalPath
}

// 3. GENERATE DECISION
func (e *AutonomousEngine) GenerateDecision(serviceID string) (control.Bundle, *control.SystemState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Get localized data for the service
	window := e.Telemetry.Window(serviceID, 60, 1*time.Minute)
	if window == nil {
		return control.Bundle{}, nil, fmt.Errorf("insufficient telemetry for %s", serviceID)
	}

	// Initialize controller memory if new
	ctrl, exists := e.Controllers[serviceID]
	if !exists {
		ctrl = control.NewController(time.Now().UnixNano(), 10, 500, control.DefaultControllerConfig())
		e.Controllers[serviceID] = ctrl
		e.States[serviceID] = &control.SystemState{}
		e.Memories[serviceID] = control.NewRegimeMemory(control.DefaultRegimeConfig())
	}
	state := e.States[serviceID]
	mem := e.Memories[serviceID]

	// Ensure system state limits are populated
	EnsureDefaultState(state)

	// Transform Telemetry -> Control
	z := AdaptWindowToMeasurement(window)

	// Run EKF & MPC Optimization (Dt = 1.0s assumed for interval)
	decision := ctrl.Recommend(z, state, mem, 1.0)

	// Save current system belief 
	e.States[serviceID] = state

	return decision, state, nil
}

// ApplyExecution is called when UiPath actually executes the command to keep math synchronized
func (e *AutonomousEngine) ApplyExecution(serviceID string, b control.Bundle) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if state, exists := e.States[serviceID]; exists {
		state.Replicas = b.Replicas
		state.QueueLimit = int(b.QueueLimit)
		state.RetryLimit = b.RetryLimit
		state.CacheAggression = b.CacheAggression
	}
}