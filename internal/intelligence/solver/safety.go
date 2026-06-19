package control

import "math"

// SafetyService is the extracted Safety Agent.
// It acts as the final boundary check, enforcing Little's Law physics
// and SLA constraints to prevent the orchestrator from causing cascading failures.
type SafetyService struct {}

func NewSafetyService() *SafetyService {
	return &SafetyService{}
}

// SafetyAudit performs a dry-run check of the proposed bundle against physical constraints.
// It returns true if the bundle violates safety constraints and requires clamping.
func (ss *SafetyService) SafetyAudit(
	proposedBundle Bundle, 
	currentSysState *SystemState, 
	lastDecision Bundle,
) bool {
	safeBundle := ss.ApplySafetyConstraints(proposedBundle, currentSysState, lastDecision)
	return safeBundle.QueueLimit != proposedBundle.QueueLimit ||
		safeBundle.Replicas != proposedBundle.Replicas ||
		safeBundle.RetryLimit != proposedBundle.RetryLimit ||
		safeBundle.CacheAggression != proposedBundle.CacheAggression
}

// SafetyValidationResult provides a comprehensive report of the proposed action.
type SafetyValidationResult struct {
	IsSafe           bool
	OriginalBundle   Bundle
	SafeBundle       Bundle
	Violations       []string
}

// ValidateAction provides a comprehensive safety report of the proposed action,
// exposing explainability to UiPath Maestro.
func (ss *SafetyService) ValidateAction(
	proposedBundle Bundle, 
	currentSysState *SystemState, 
	lastDecision Bundle,
) SafetyValidationResult {
	safeBundle := ss.ApplySafetyConstraints(proposedBundle, currentSysState, lastDecision)
	
	var violations []string
	if safeBundle.QueueLimit < proposedBundle.QueueLimit {
		violations = append(violations, "Queue limit violated absolute SLA ceiling or max-rise slew rate.")
	} else if safeBundle.QueueLimit > proposedBundle.QueueLimit {
		violations = append(violations, "Queue limit violated minimum safety floor or max-drop slew rate.")
	}
	
	return SafetyValidationResult{
		IsSafe:         len(violations) == 0,
		OriginalBundle: proposedBundle,
		SafeBundle:     safeBundle,
		Violations:     violations,
	}
}

// ApplySafetyConstraints enforces the SLA-Aware Little's Law Mesh Ceiling and slew rates.
// It calculates the strict physics boundaries and clamps the proposed bundle accordingly,
// ensuring the output is mathematically guaranteed to be safe.
func (ss *SafetyService) ApplySafetyConstraints(
	proposedBundle Bundle, 
	currentSysState *SystemState, 
	lastDecision Bundle,
) Bundle {
	safeBundle := proposedBundle

	// SLA-Aware Little's Law Physics Ceiling
	currentPhysicalCapacity := math.Max(0.001, float64(currentSysState.Replicas)*currentSysState.ServiceRate)
	maxSlaQueue := currentPhysicalCapacity * currentSysState.SLATarget

	absoluteMaxQueue := math.Max(25.0, maxSlaQueue * 1.5) 
	
	if safeBundle.QueueLimit > absoluteMaxQueue {
		safeBundle.QueueLimit = absoluteMaxQueue
	}
	if safeBundle.QueueLimit < 25.0 {
		safeBundle.QueueLimit = 25.0
	}

	// Slew Rate Protections (Prevent Envoy Crashes)
	currentQ := float64(lastDecision.QueueLimit)
	maxDrop := math.Max(10.0, currentQ * 0.20)
	if currentSysState.FailureMode != "Healthy" { maxDrop = currentQ * 0.90 } 
	
	if safeBundle.QueueLimit < currentQ - maxDrop {
		safeBundle.QueueLimit = currentQ - maxDrop
	}
	
	maxRise := math.Max(5.0, currentQ * 0.05)
	if currentSysState.RetryPressure > 10.0 { 
		maxRise = 2.0 
	} else if maxRise > 25.0 { 
		maxRise = 25.0 
	}
	
	if safeBundle.QueueLimit > currentQ + maxRise {
		safeBundle.QueueLimit = currentQ + maxRise
	}

	return safeBundle
}
