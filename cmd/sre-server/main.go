package main

import (
	"log"
	"net/http"

	"github.com/kuldeep-poonia/Incident-Response-agent/internal/api"
	"github.com/kuldeep-poonia/Incident-Response-agent/internal/engine"
)

func main() {
	autobot := engine.NewAutonomousEngine()
	serverAPI := &api.API{Engine: autobot}

	// Maestro Integration Endpoints
	http.HandleFunc("/agent/predict", serverAPI.HandleAgentPredict)
	http.HandleFunc("/agent/decide", serverAPI.HandleAgentDecide)
	http.HandleFunc("/agent/safety", serverAPI.HandleAgentSafety)
	http.HandleFunc("/agent/recommend", serverAPI.HandleAgentRecommend)
	http.HandleFunc("/agent/rca", serverAPI.HandleAgentRCA)

	// Serve the UI Dashboard
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	log.Println("🤖 Agentic SRE Backend listening on :8080")
	log.Println("🌐 Dashboard available at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
