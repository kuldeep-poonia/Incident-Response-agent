# Autonomous Incident Response Agent (UiPath Maestro)

An enterprise-grade, Multi-Agent Site Reliability Engineering (SRE) system that autonomously detects, analyzes, and mitigates Kubernetes cluster degradation before humans even wake up.

## Agent Type

**This solution uses both Coded Agents and Low-code Agents.**

* **Coded Agents:** Go backend services, mathematical AI agents, REST APIs, and backend logic.
* **Low-code Agents:** UiPath Maestro BPMN orchestration, Action Center human-in-the-loop workflow, and Studio Web automation.

**Submission for UiPath AgentHack 2026 – Track 2 (Maestro BPMN)**

## 🚀 What This Project Does

Modern SREs are overwhelmed by alert fatigue. When a microservice queue backs up, humans take 15+ minutes to log in, read Datadog charts, figure out the root cause, and run scaling scripts.

Our **Autonomous Incident Response Agent** reduces Mean Time to Mitigate (MTTM) from 15 minutes to **200 milliseconds**.

We built a backend that runs **4 deterministic mathematical AI agents** (using Model Predictive Control and Extended Kalman Filters) and **1 generative AI agent (DeepSeek-V3)**. These are orchestrated together using **UiPath Maestro BPMN**.

## 🛠️ UiPath Components Used

* **UiPath Maestro BPMN:** The core orchestration layer that defines the incident response process. We use an advanced **Parallel Gateway** to separate the slow Generative AI LLM analysis from the high-speed mathematical pipeline, preventing network latency from blocking critical auto-scaling actions.
* **UiPath Action Center (Human-in-the-loop):** We use Maestro's **Exclusive Gateway (Flow Decision)** to evaluate the mathematical confidence score of the mitigation. If confidence is **>80%**, the workflow automatically mitigates the incident. If confidence is **<80%**, Maestro pauses the workflow and generates an Action Center approval for a human SRE.
* **UiPath Coding Agents:** The Go-based mathematical backend, REST APIs, and BPMN architecture were co-developed using the **Antigravity Coding Agent** inside VS Code.

## 🧠 Agent Architecture

Our solution leverages a hybrid multi-agent approach:

1. **Predict Agent (Math):** Ingests telemetry and uses an Extended Kalman Filter to estimate hidden cluster state.
2. **RCA Agent (GenAI – DeepSeek):** Generates a human-readable Root Cause Analysis.
3. **Decision Agent (Math):** Uses Model Predictive Control (MPC) to compute the optimal remediation strategy.
4. **Safety Agent (Math):** Validates the proposed action against hard physical constraints such as Little's Law.
5. **Recommend Agent (Math):** Produces an explainable recommendation with a confidence score.

## ⚙️ Setup & Prerequisites

### Prerequisites

* Go 1.21+
* UiPath Automation Cloud Account
* DeepSeek API Key (Optional)

### Running the Backend & UI Dashboard

1. Clone the repository.
2. Run:

```bash
go run ./cmd/sre-server/main.go
```

The server starts on **http://localhost:8080**.

Open **http://localhost:8080** to access the Autonomous Action Center Dashboard.

### Setting up UiPath Maestro

1. Open UiPath Studio Web.
2. Import the Maestro BPMN workflow.
3. Map the Service Tasks to the deployed backend REST endpoints (for example `POST /agent/predict`).

## 📜 License

This project is licensed under the MIT License.
