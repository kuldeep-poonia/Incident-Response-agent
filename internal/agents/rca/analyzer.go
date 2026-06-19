package rca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)


// GITHUB MODELS API SCHEMA (OpenAI Compatible)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}


// TELEMETRY PARSING SCHEMA (Matches chaos_simulation_test.go)

type TelemetryTick struct {
	TimeSec         int     `json:"time_sec"`
	Event           string  `json:"event"`
	PodsReady       float64 `json:"pods_ready"`
	PodsTarget      int     `json:"pods_target"`
	EnvoyActualQ    float64 `json:"envoy_actual_q"`
	EnvoyCommandQ   float64 `json:"envoy_cmd_q"`
	PhysicalQueue   float64 `json:"physical_queue"`
	RetryPool       float64 `json:"retry_pool"`
	PhysicalLatency float64 `json:"physical_latency"`
	CtrlLatency     float64 `json:"ctrl_latency"`
	RiskScore       float64 `json:"risk_score"`
}

type TelemetryReport struct {
	ScenarioName string          `json:"scenario_name"`
	Ticks        []TelemetryTick `json:"ticks"`
}


// CORE INCIDENT ANALYZER

func AnalyzeIncident(report TelemetryReport) error {
	// 1. Load Environment Variables
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		return fmt.Errorf("GITHUB_TOKEN not found in .env")
	}

	

	if len(report.Ticks) == 0 {
		return fmt.Errorf("no telemetry data found in report")
	}

	// 3. Extract Incident Metadata
	var criticalEvents []string
	var peakRisk float64
	var peakLatency float64

	for _, tick := range report.Ticks {
		if tick.Event != "-" {
			criticalEvents = append(criticalEvents, fmt.Sprintf("T+%03ds: %s", tick.TimeSec, tick.Event))
		}
		if tick.RiskScore > peakRisk {
			peakRisk = tick.RiskScore
		}
		if tick.PhysicalLatency > peakLatency {
			peakLatency = tick.PhysicalLatency
		}
	}

	finalTick := report.Ticks[len(report.Ticks)-1]

	// 4. Construct AI Prompt
	prompt := fmt.Sprintf(`You are an elite Principal Site Reliability Engineer (SRE). 
We just ran a chaos engineering simulation on our Kubernetes/Envoy cluster. 

Please analyze this incident telemetry and output a post-mortem.

--- TELEMETRY SUMMARY ---
Scenario: %s
Duration: %d seconds
Critical Incident Timeline:
%v

--- PEAK METRICS ---
Peak Risk Score: %.2f
Peak Latency: %.2fs

--- FINAL SYSTEM STATE (At T+%ds) ---
Pods (Ready/Target): %.1f / %d
Physical Queue: %.2f
Retry Pool: %.2f
Physical Latency: %.2fs
Risk Score: %.2f

--- REQUIRED OUTPUT ---
Based strictly on the data above, generate a concise, professional diagnostic containing:
1. **Severity**: (Critical, High, Medium, Low)
2. **Root Cause**: What triggered the failure cascade?
3. **Business Impact**: How did this affect users and infrastructure costs?
4. **Recommended Actions**: 2-3 engineering fixes to prevent this in the future.
`, report.ScenarioName, len(report.Ticks), criticalEvents, peakRisk, peakLatency, 
finalTick.TimeSec, finalTick.PodsReady, finalTick.PodsTarget, finalTick.PhysicalQueue, 
finalTick.RetryPool, finalTick.PhysicalLatency, finalTick.RiskScore)

	// 5. Send to GitHub Models API
	fmt.Println("🧠 Pinging DeepSeek-V3 via GitHub Models API for Root Cause Analysis...")
	
	reqBody := ChatRequest{
		Model: "DeepSeek-V3", // Standard GitHub Model alias for DeepSeek V3
		Messages: []ChatMessage{
			{Role: "system", Content: "You are an elite Kubernetes and Control Theory Site Reliability Engineer."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2, // Low temperature for highly analytical, deterministic output
		MaxTokens:   1000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal API request: %w", err)
	}

	// GitHub Models standard inference endpoint
	apiURL := "https://models.inference.ai.azure.com/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+githubToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("no response generated by AI")
	}

	// 6. Print the Post-Mortem
	
	fmt.Println(" AUTOMATED AI INCIDENT POST-MORTEM (DeepSeek-V3)")
	
	fmt.Println(chatResp.Choices[0].Message.Content)
	

	return nil
}

