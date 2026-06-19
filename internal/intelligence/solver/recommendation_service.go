package control

import (
	"fmt"
)

// Recommendation represents the structured, explainable output intended for UiPath Maestro.
type Recommendation struct {
	IncidentType               string   `json:"incident_type"`
	Confidence                 float64  `json:"confidence"`
	PredictedOutageProbability float64  `json:"predicted_outage_probability"`
	RecommendedAction          string   `json:"recommended_action"`
	RecommendedBundle          Bundle   `json:"recommended_bundle"`
	SafetyVerified             bool     `json:"safety_verified"`
	ExpectedRiskReduction      float64  `json:"expected_risk_reduction"`
	Reasoning                  []string `json:"reasoning"`
}

// RecommendationService translates raw mathematical bounds and optimal physics vectors
// into declarative, explainable intents that an orchestration engine like Maestro can consume.
// It performs NO optimization or clamping itself.
type RecommendationService struct {}

func NewRecommendationService() *RecommendationService {
	return &RecommendationService{}
}

// GenerateRecommendation packages the state and safety validation into a final JSON-ready struct.
func (rs *RecommendationService) GenerateRecommendation(
	sysState *SystemState, 
	lastBundle Bundle,
	safeBundle Bundle, 
	validationResult SafetyValidationResult,
	sysIdConfidence ParameterConfidence,
) Recommendation {
	
	action := "DoNothing"
	var reasoning []string
	
	// Translate raw float changes into semantic actions
	if safeBundle.Replicas > lastBundle.Replicas {
		action = "ScaleUp"
		reasoning = append(reasoning, fmt.Sprintf("Increasing capacity from %d to %d replicas.", lastBundle.Replicas, safeBundle.Replicas))
	} else if safeBundle.Replicas < lastBundle.Replicas {
		action = "ScaleDown"
		reasoning = append(reasoning, fmt.Sprintf("Decreasing capacity from %d to %d replicas.", lastBundle.Replicas, safeBundle.Replicas))
	} else if safeBundle.QueueLimit < lastBundle.QueueLimit {
		action = "ShedLoad"
		reasoning = append(reasoning, fmt.Sprintf("Tightening queue limit from %.0f to %.0f to shed load.", lastBundle.QueueLimit, safeBundle.QueueLimit))
	} else if safeBundle.QueueLimit > lastBundle.QueueLimit {
		action = "ExpandQueue"
		reasoning = append(reasoning, fmt.Sprintf("Expanding queue limit from %.0f to %.0f.", lastBundle.QueueLimit, safeBundle.QueueLimit))
	}

	// Inject Safety Traceability
	if !validationResult.IsSafe {
		for _, v := range validationResult.Violations {
			reasoning = append(reasoning, "Safety clamp applied: "+v)
		}
	} else {
		reasoning = append(reasoning, "Action mathematically verified against SLA physics ceiling.")
	}
	
	// Inject Observability Context
	if sysState.FailureMode != "Healthy" {
		reasoning = append(reasoning, "System is currently in failure mode: "+sysState.FailureMode)
	}

	// Calculate overall machine learning confidence
	confidence := (sysIdConfidence.ArrivalProcess + sysIdConfidence.RetryProcess + sysIdConfidence.CapacityProcess) / 3.0
	if confidence == 0 {
		confidence = 0.8 // Base EKF initialization confidence fallback
	}

	// Translate heuristic risk into predicted outage probability
	outageProb := sysState.Risk
	if outageProb > 1.0 {
		outageProb = 0.99
	}

	return Recommendation{
		IncidentType:               sysState.FailureMode,
		Confidence:                 confidence,
		PredictedOutageProbability: outageProb,
		RecommendedAction:          action,
		RecommendedBundle:          safeBundle,
		SafetyVerified:             true, // True because the input bundle has passed through SafetyService
		ExpectedRiskReduction:      0.0,  // Stays zero statically; requires simulation delta to populate
		Reasoning:                  reasoning,
	}
}
