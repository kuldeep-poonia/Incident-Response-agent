# Autonomous Incident Response Agent (UiPath Maestro)

An enterprise-grade, Multi-Agent Site Reliability Engineering (SRE) system that autonomously detects, analyzes, and mitigates Kubernetes cluster degradation before humans even wake up. 

**Winner Candidate for UiPath AgentHack: Track 2 (Maestro BPMN)**

## 🚀 What This Project Does
Modern SREs are overwhelmed by alert fatigue. When a microservice queue backs up, humans take 15+ minutes to log in, read Datadog charts, figure out the root cause, and run scaling scripts. 

Our **Autonomous Incident Response Agent** reduces Mean Time to Mitigate (MTTM) from 15 minutes to **200 milliseconds**. 

We built a backend that runs 4 deterministic mathematical AI agents (using Model Predictive Control and Extended Kalman Filters) and 1 generative AI agent (DeepSeek-V3). These are perfectly orchestrated together using **UiPath Maestro BPMN**. 

## 🛠️ UiPath Components Used
- **UiPath Maestro BPMN**: The core orchestration layer that defines the incident response process. We use an advanced **Parallel Gateway** to separate the slow Generative AI LLM analysis from the high-speed Mathematical pipeline, preventing network latency from blocking critical auto-scaling actions.
- **UiPath Action Center (Human-in-the-loop)**: We use Maestro's **Exclusive Gateway** (Flow Decision) to evaluate the mathematical "Confidence Score" of the mitigation. If confidence is > 80%, the agent auto-mitigates (Zero-Touch). If confidence is < 80%, Maestro suspends the workflow and generates an Action Center Form for a human SRE to approve.
- **UiPath Coding Agents (Bonus)**: The entire Go-based mathematical backend, the decoupled REST APIs, and the BPMN architecture analysis were co-developed using the **Antigravity Coding Agent** inside VSCode.

## 🧠 Agent Architecture
Our solution leverages a hybrid multi-agent approach:
1. **Predict Agent (Math):** Ingests raw telemetry and uses an Extended Kalman Filter to estimate the hidden physics of the cluster (Arrival rates, Capacity Velocity).
2. **RCA Agent (GenAI - DeepSeek):** Analyzes the topological bottleneck and generates a human-readable "Root Cause Analysis" string.
3. **Decision Agent (Math):** Uses Model Predictive Control (MPC) to optimize Kubernetes Replicas and Queue Limits to survive the traffic spike.
4. **Safety Agent (Math):** Clamps the proposed decision against hard physical constraints (Little's Law).
5. **Recommend Agent (Math):** Translates the raw arrays into an explainable JSON payload with a calculated Confidence Score.

## ⚙️ Setup & Prerequisites

### Prerequisites
- Go 1.21+
- UiPath Automation Cloud Account
- DeepSeek API Key (Optional: The backend falls back to mock responses if omitted)

### Running the Backend & UI Dashboard
1. Clone the repository.
2. Run the server:
   ```bash
   go run ./cmd/sre-server/main.go
   ```
   *The server will start on `http://localhost:8080`.*
3. Open your browser and navigate to `http://localhost:8080` to view the **Autonomous Action Center Dashboard**. This is a premium, single-file HTML/CSS/JS frontend that perfectly mirrors the parallel orchestrator logic of the Maestro BPMN, firing concurrent requests to the backend agents and dynamically rendering the mathematical confidence scores.

### Setting up UiPath Maestro
1. Open UiPath Studio Web.
2. Import the Maestro BPMN layout.
3. Map the Service Tasks to the deployed backend endpoints (e.g., `POST /agent/predict`).

## 📜 License
This project is licensed under the MIT License - see the LICENSE file for details.
