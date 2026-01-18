package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// 1. Initialize AWS Client
	InitAWS()

	// 2. Register Routes matching User Requirements

	// --- A. Streaming Fault Injection (SSE) ---
	// "GET /host/inject" -> Broadcasts GET to agents (CPU, Mem, Network)
	http.HandleFunc("/host/inject", handleFaultSSE)

	// "GET /docker/fault" -> Broadcasts GET to agents (Docker specific faults)
	http.HandleFunc("/docker/fault", handleFaultSSE)

	// --- B. Docker Management (JSON) ---
	// "GET /docker/list"
	http.HandleFunc("/docker/list", handleDockerJSON)

	// "GET /docker/status" (expects ?container_id=...)
	http.HandleFunc("/docker/status", handleDockerJSON)

	// "POST /docker/start" (expects JSON body)
	http.HandleFunc("/docker/start", handleDockerJSON)

	// "POST /docker/stop" (expects JSON body)
	http.HandleFunc("/docker/stop", handleDockerJSON)

	// --- C. Windows Service Control (JSON) ---
	// "POST /host/service" (Start/Stop/Status for Windows Services)
	http.HandleFunc("/host/service", handleWindowsService)

	// --- D. Utility ---
	http.HandleFunc("/health", handleHealth)

	// 3. Start Server
	fmt.Printf("Control Tower listening on %s (Region: %s)\n", ServerPort, AWSRegion)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}
