package control

import "math"

// DecisionService is the extracted Decision Agent.
// It wraps Candidate Generation and the RobustMPC (CVaR optimizer),
// outputting an optimal unconstrained decision.
type DecisionService struct {
	MPC     *RobustMPC
	GenCfg  GeneratorConfig
	EconCfg EconomicParams
}

func NewDecisionService(cfg OptimizerConfig) *DecisionService {
	return &DecisionService{
		MPC:     NewRobustMPC(cfg),
		GenCfg:  DefaultGeneratorConfig(),
		EconCfg: DefaultEconomicParams(),
	}
}

// GenerateOptimalDecision determines the appropriate candidate generation bounds based on current state,
// generates action candidates, and runs the RobustMPC optimizer to find the CVaR-optimal action.
// The mathematics, Monte Carlo evaluation, and Risk-Aversion behavior are strictly preserved.
func (ds *DecisionService) GenerateOptimalDecision(
	currentSysState *SystemState,
	simCfg SimConfig,
	mem *RegimeMemory,
	masterSeed int64,
) Bundle {
	
	physicalRequiredCapacity := currentSysState.PredictedArrival / math.Max(0.001, currentSysState.ServiceRate)
	
	// Dynamic Scale Surge Logic
	if float64(currentSysState.Replicas) > physicalRequiredCapacity * 1.5 {
		ds.GenCfg.MaxScaleSurge = 0 
	} else if currentSysState.Utilisation < 0.5 && currentSysState.QueueDepth < 10.0 {
		ds.GenCfg.MaxScaleSurge = 5 
	} else {
		ds.GenCfg.MaxScaleSurge = 100 
	}

	candidates := GenerateBundles(*currentSysState, ds.GenCfg, simCfg)
	optimalBundle := ds.MPC.Optimize(*currentSysState, candidates, simCfg, ds.EconCfg, mem, masterSeed)

	return optimalBundle
}
