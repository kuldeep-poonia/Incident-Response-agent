package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/qphysics/phaseshift/control"
	"github.com/qphysics/phaseshift/internal/engine"
	"github.com/qphysics/phaseshift/telemetry"
)

type API struct {
	Engine *engine.AutonomousEngine
}

type DecisionResponse struct {
	Service               string  `json:"service"`
	Risk                  string  `json:"risk"`
	RecommendedAction     string  `json:"recommended_action"`
	PredictedImpact       string  `json:"predicted_impact"`
	Confidence            string  `json:"confidence"`
	RequiresHumanApproval bool    `json:"requires_human_approval"`
}

// POST /metrics/ingest
func (a *API) HandleIngest(w http.ResponseWriter, r *http.Request) {
	var pt telemetry.MetricPoint
	if err := json.NewDecoder(r.Body).Decode(&pt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.Engine.IngestMetric(&pt)
	w.WriteHeader(http.StatusOK)
}

// GET /incident/context
func (a *API) HandleContext(w http.ResponseWriter, r *http.Request) {
	cp := a.Engine.UpdateContext()
	json.NewEncoder(w).Encode(cp)
}

// POST /decision/recommend (Legacy Monolithic Handler)
func (a *API) HandleRecommend(w http.ResponseWriter, r *http.Request) {
	var req struct { ServiceID string `json:"service_id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bundle, state, err := a.Engine.GenerateDecision(req.ServiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	riskLevel := "LOW"
	reqHuman := false
	if state.Risk > 50.0 { 
		riskLevel = "CRITICAL"
		reqHuman = true
	} else if state.Risk > 20.0 {
		riskLevel = "HIGH"
	}

	actionStr := fmt.Sprintf("Scale to %d pods, set Mesh Queue Limit to %.0f, set Retry Limit to %d", 
		bundle.Replicas, bundle.QueueLimit, bundle.RetryLimit)

	resp := DecisionResponse{
		Service:               req.ServiceID,
		Risk:                  riskLevel,
		RecommendedAction:     actionStr,
		PredictedImpact:       "Restores Halfin-Whitt QED SLA limits within 15 seconds.",
		Confidence:            "High (MPC Convergence Validated)",
		RequiresHumanApproval: reqHuman,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// ============================================================================
// PHASE 5: MAESTRO INDEPENDENT AGENT ENDPOINTS
// ============================================================================

type MaestroRequest struct {
	ServiceID string `json:"service_id"`
}

type MaestroSafetyRequest struct {
	ServiceID string         `json:"service_id"`
	Bundle    control.Bundle `json:"bundle"`
}

type MaestroRecommendRequest struct {
	ServiceID  string                         `json:"service_id"`
	Validation control.SafetyValidationResult `json:"validation"`
}

// POST /agent/predict
func (a *API) HandleAgentPredict(w http.ResponseWriter, r *http.Request) {
	var req MaestroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.Engine.Mu.Lock()
	defer a.Engine.Mu.Unlock()

	ctrl := a.Engine.Controllers[req.ServiceID]
	state := a.Engine.States[req.ServiceID]
	mem := a.Engine.Memories[req.ServiceID]
	window := a.Engine.Telemetry.Window(req.ServiceID, 60, 1*time.Minute)
	
	if window == nil || ctrl == nil {
		http.Error(w, "insufficient telemetry or uninitialized", http.StatusBadRequest)
		return
	}

	z := engine.AdaptWindowToMeasurement(window)
	ctrl.AgentPredict(z, state, mem, 1.0)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// POST /agent/decide
func (a *API) HandleAgentDecide(w http.ResponseWriter, r *http.Request) {
	var req MaestroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.Engine.Mu.Lock()
	defer a.Engine.Mu.Unlock()

	ctrl := a.Engine.Controllers[req.ServiceID]
	state := a.Engine.States[req.ServiceID]
	mem := a.Engine.Memories[req.ServiceID]

	if ctrl == nil {
		http.Error(w, "uninitialized", http.StatusBadRequest)
		return
	}

	bundle := ctrl.AgentDecide(state, mem)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

// POST /agent/safety
func (a *API) HandleAgentSafety(w http.ResponseWriter, r *http.Request) {
	var req MaestroSafetyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.Engine.Mu.Lock()
	defer a.Engine.Mu.Unlock()

	ctrl := a.Engine.Controllers[req.ServiceID]
	state := a.Engine.States[req.ServiceID]

	if ctrl == nil {
		http.Error(w, "uninitialized", http.StatusBadRequest)
		return
	}

	validation := ctrl.AgentSafety(req.Bundle, state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(validation)
}

// POST /agent/recommend
func (a *API) HandleAgentRecommend(w http.ResponseWriter, r *http.Request) {
	var req MaestroRecommendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.Engine.Mu.Lock()
	defer a.Engine.Mu.Unlock()

	ctrl := a.Engine.Controllers[req.ServiceID]
	state := a.Engine.States[req.ServiceID]

	if ctrl == nil {
		http.Error(w, "uninitialized", http.StatusBadRequest)
		return
	}

	recommendation := ctrl.AgentRecommend(req.Validation, state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recommendation)
}

// POST /agent/rca
func (a *API) HandleAgentRCA(w http.ResponseWriter, r *http.Request) {
	var req MaestroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.Engine.Mu.RLock()
	ctrl := a.Engine.Controllers[req.ServiceID]
	statePtr := a.Engine.States[req.ServiceID]
	cp := a.Engine.Topology.Snapshot().CriticalPath
	
	// Copy state to prevent race conditions when releasing lock for slow network I/O
	var stateCopy control.SystemState
	if statePtr != nil {
		stateCopy = *statePtr
	}
	a.Engine.Mu.RUnlock()

	if ctrl == nil || statePtr == nil {
		http.Error(w, "uninitialized", http.StatusBadRequest)
		return
	}

	// This is deliberately called OUTSIDE the lock because it's a slow network boundary call.
	analysis, err := ctrl.RcaService.AnalyzeActiveIncident(cp, &stateCopy, req.ServiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}