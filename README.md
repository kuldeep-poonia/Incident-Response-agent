# Autonomous Incident Response Agent

**An Enterprise Agentic SRE Platform built for the UiPath AgentHack.**

Unlike traditional auto-scalers that react blindly to CPU spikes, or LLM-only bots that hallucinate arbitrary infrastructure commands, this platform fuses **Hard Control Theory Physics** with **Agentic Workflows**. It predicts cascading failures *before* they happen, computes mathematically optimal mitigation strategies, and delegates orchestration to UiPath Maestro.

## Why This is Different

Most "AI Operations" tools are glorified log summarizers. They provide post-mortems after the outage has occurred. 
Our agent:
1. **Predicts:** Uses an Extended Kalman Filter (EKF) to strip Byzantine fault noise from live telemetry.
2. **Computes:** Uses Robust Model Predictive Control (MPC) and Conditional Value at Risk (CVaR) to find the mathematically optimal mitigation action.
3. **Validates:** Uses SLA-Aware Little's Law to physically clamp the decision, mathematically guaranteeing the mitigation won't cause an Envoy proxy crash.
4. **Advises:** Runs a parallel LLM RCA Agent to explain the physics in human terms.
5. **Orchestrates:** Packages the validated intent (`ShedLoad`, `ScaleUp`) for UiPath Maestro to execute.

## Hackathon Track Alignment

*   **Primary Track:** Enterprise Automation & AI
*   **Integration:** UiPath Maestro (BPMN Orchestration), UiPath Action Center (Human-in-the-loop).

## Architecture

*(See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for full sequence diagrams and Maestro API contracts).*

### The Multi-Agent Pipeline

The monolithic intelligence engine has been decoupled into five discrete, Maestro-consumable agents:

*   `Predict Agent`: Generates the filtered `SystemState`.
*   `Decision Agent`: Generates the raw `OptimalBundle`.
*   `Safety Agent`: Clamps the bundle using strict physics boundaries.
*   `RCA Agent`: Queries DeepSeek-V3 / GitHub Models for human root-cause explanations.
*   `Recommendation Agent`: Packages the final explainable payload.

## Installation & Running Locally

1.  **Dependencies:** Ensure Go 1.21+ is installed.
2.  **Environment Variables:** Add your GitHub Models token to `.env` for the RCA Agent:
    ```bash
    GITHUB_TOKEN=your_token_here
    ```
3.  **Run the Server:**
    ```bash
    go run cmd/sre-server/main.go
    ```
4.  **Run the Simulator/Tests:**
    ```bash
    go test ./...
    ```

## API Endpoints

The Maestro workflow orchestrates these independent endpoints sequentially:

*   `POST /agent/predict`: Generates the current state via EKF.
*   `POST /agent/rca`: Generates an advisory root cause explanation.
*   `POST /agent/decide`: Generates the optimal infrastructure change via MPC.
*   `POST /agent/safety`: Clamps the change via Little's Law.
*   `POST /agent/recommend`: Generates the semantic `ActionIntent` for execution.
*   `POST /decision/recommend`: (Legacy monolithic endpoint for load-testing simulation compatibility).
