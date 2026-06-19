package observability

import (
	"time"

	"github.com/qphysics/phaseshift/internal/observability/telemetry"
	"github.com/qphysics/phaseshift/internal/observability/topology"
)

// ObservabilityService serves as the Observability Agent
// Responsibilities: service state, dependency state, blast radius
type ObservabilityService struct {
	Store *telemetry.Store
	Graph *topology.Graph
}

func NewObservabilityService() *ObservabilityService {
	return &ObservabilityService{
		Store: telemetry.NewStore(1024, 5000, 5*time.Minute),
		Graph: topology.New(),
	}
}

func (s *ObservabilityService) Ingest(p *telemetry.MetricPoint) {
	s.Store.Ingest(p)
}

func (s *ObservabilityService) GetServiceState(serviceID string, n int, freshness time.Duration) *telemetry.ServiceWindow {
	return s.Store.Window(serviceID, n, freshness)
}

func (s *ObservabilityService) UpdateTopology() {
	windows := s.Store.AllWindows(60, 5*time.Minute)
	s.Graph.Update(windows)
}

func (s *ObservabilityService) GetDependencyState() topology.GraphSnapshot {
	return s.Graph.Snapshot()
}

func (s *ObservabilityService) GetBlastRadius() topology.CriticalPath {
	snap := s.Graph.Snapshot()
	return snap.CriticalPath
}
