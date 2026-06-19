package main

import (
	"log"
	"net/http"

	"github.com/qphysics/phaseshift/internal/api"
	"github.com/qphysics/phaseshift/internal/engine"
)

func main() {
	autobot := engine.NewAutonomousEngine()
	serverAPI := &api.API{Engine: autobot}

	http.HandleFunc("/metrics/ingest", serverAPI.HandleIngest)
	http.HandleFunc("/incident/context", serverAPI.HandleContext)
	http.HandleFunc("/decision/recommend", serverAPI.HandleRecommend)
	
	// NEW: Exposing the DeepSeek-V3 Post-Mortem Agent
	http.HandleFunc("/incident/rca", serverAPI.HandleRCA)

	log.Println("🤖 Agentic SRE Backend listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}