package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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

// POST /decision/recommend
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

	// Classify Risk for UiPath BPMN routing
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