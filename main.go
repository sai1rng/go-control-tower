package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// 1. Initialize AWS Client
	InitAWS()

	// 2. Register Routes

	// --- A. Streaming Fault Injection (SSE) ---
	// Handles /host/inject and /docker/fault
	// (Browsers require strict CORS for EventSource)
	http.HandleFunc("/host/inject", handleFaultSSE)
	http.HandleFunc("/docker/fault", handleFaultSSE)

	// --- B. Docker Management (JSON) ---
	http.HandleFunc("/docker/list", handleDockerJSON)
	http.HandleFunc("/docker/status", handleDockerJSON)
	http.HandleFunc("/docker/start", handleDockerJSON)
	http.HandleFunc("/docker/stop", handleDockerJSON)

	// --- C. Windows Service Control (JSON) ---
	http.HandleFunc("/host/service", handleWindowsService)

	// --- D. Utility ---
	http.HandleFunc("/health", handleHealth)

	// 3. Start Server
	fmt.Printf("Control Tower listening on %s (Region: %s)\n", ServerPort, AWSRegion)
	// Listen on all interfaces
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}
