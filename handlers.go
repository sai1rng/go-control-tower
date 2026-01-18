package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// --- SSE Handlers (Fault Injection) ---

// handleFaultSSE is a generic handler for SSE streams.
// It is used by both /host/inject and /docker/fault.
func handleFaultSSE(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	// 1. Set SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 2. Parse Query
	query := r.URL.Query()
	keys := query["key"]
	vals := query["val"]

	// Default to Linux if not specified (Faults are usually Linux-heavy in this context)
	targetOS := "linux"
	// If the path contains 'host/service' (Windows), we might need logic,
	// but here we are strictly doing faults. You can add ?os=windows if needed.

	ips, err := getInstances(keys, vals, targetOS)
	if err != nil || len(ips) == 0 {
		sendSSEMessage(w, flusher, "error", "No matching instances found")
		return
	}

	// 3. Notify Start
	sendSSEMessage(w, flusher, "start", fmt.Sprintf("Targeting %d nodes", len(ips)))

	// 4. Stream Results
	// r.URL.Path passes the exact path (e.g. /host/inject) to the agent
	resultsChan := StreamBroadcast(ips, LinuxAgentPort, "GET", r.URL.Path, query, nil)

	for res := range resultsChan {
		jsonData, _ := json.Marshal(res)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}

	// 5. End
	fmt.Fprintf(w, "event: end\ndata: \"done\"\n\n")
	flusher.Flush()
}

// --- JSON Handlers (Management) ---

// handleWindowsService handles POST /host/service (Start/Stop/Status)
func handleWindowsService(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method must be POST", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	handleJSONBroadcast(w, r, "windows", WindowsAgentPort, "POST", "/host/service", body)
}

// handleDockerJSON handles standard JSON requests for Docker (List, Start, Stop, Status)
func handleDockerJSON(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	// Determine method and body based on the request to Control Tower
	method := r.Method
	var body []byte
	if method == "POST" {
		body, _ = io.ReadAll(r.Body)
	}

	// r.URL.Path is forwarded exactly (e.g. /docker/start)
	handleJSONBroadcast(w, r, "linux", LinuxAgentPort, method, r.URL.Path, body)
}

// --- Helpers ---

// handleJSONBroadcast encapsulates the common logic for finding nodes and waiting for JSON results
func handleJSONBroadcast(w http.ResponseWriter, r *http.Request, osType, port, method, path string, body []byte) {
	query := r.URL.Query()
	keys := query["key"]
	vals := query["val"]

	ips, err := getInstances(keys, vals, osType)
	if err != nil {
		http.Error(w, fmt.Sprintf("AWS Error: %v", err), http.StatusInternalServerError)
		return
	}

	results := broadcastRequest(ips, port, method, path, query, body)
	respondJSON(w, ips, results)
}

func respondJSON(w http.ResponseWriter, ips []string, results []NodeResult) {
	successCount := 0
	for _, res := range results {
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			successCount++
		}
	}

	msg := ""
	if len(ips) == 0 {
		msg = "No matching instances found. Did you provide ?key=...&val=... tags?"
	}

	resp := APIResponse{
		Total:        len(ips),
		SuccessCount: successCount,
		FailedCount:  len(ips) - successCount,
		Results:      results,
		Message:      msg,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func setupCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func sendSSEMessage(w http.ResponseWriter, f http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: \"%s\"\n\n", event, data)
	f.Flush()
}

// handleHealth is a simple local health check
func handleHealth(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
