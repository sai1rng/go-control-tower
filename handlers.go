package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// --- CORS Helper ---

// enableCORS sets standard CORS headers.
// Returns true if the request was an OPTIONS preflight (and handled), false otherwise.
func enableCORS(w http.ResponseWriter, r *http.Request) bool {
	// Allow requests from ANY origin
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// Allow common methods
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	// Allow common headers (Content-Type is crucial for JSON, Cache-Control for SSE)
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Cache-Control")

	// Handle Preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// --- SSE Handlers (Fault Injection) ---

func handleFaultSSE(w http.ResponseWriter, r *http.Request) {
	// 1. Strict CORS Check
	if enableCORS(w, r) {
		return
	}

	// 2. Set SSE Specific Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Required for some proxies/browsers to not buffer SSE
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	query := r.URL.Query()
	keys := query["key"]
	vals := query["val"]

	// Default to Linux if not specified
	targetOS := "linux"
	// Optional: Check if user requested windows via query param
	if query.Get("os") == "windows" {
		targetOS = "windows"
	}

	ips, err := getInstances(keys, vals, targetOS)
	if err != nil || len(ips) == 0 {
		sendSSEMessage(w, flusher, "error", "No matching instances found")
		return
	}

	// Notify Start
	sendSSEMessage(w, flusher, "start", fmt.Sprintf("Targeting %d nodes", len(ips)))

	// Stream Results
	// r.URL.Path passes the exact path (e.g. /host/inject) to the agent
	resultsChan := StreamBroadcast(ips, LinuxAgentPort, "GET", r.URL.Path, query, nil)

	for res := range resultsChan {
		jsonData, _ := json.Marshal(res)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}

	fmt.Fprintf(w, "event: end\ndata: \"done\"\n\n")
	flusher.Flush()
}

// --- JSON Handlers (Management) ---

// handleWindowsService handles POST /host/service
func handleWindowsService(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method must be POST", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	handleJSONBroadcast(w, r, "windows", WindowsAgentPort, "POST", "/host/service", body)
}

// handleDockerJSON handles Docker List/Start/Stop/Status
func handleDockerJSON(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}

	// Determine method based on the request coming IN to Control Tower
	method := r.Method
	var body []byte

	// Read body only if it's a POST/PUT
	if method == "POST" || method == "PUT" {
		body, _ = io.ReadAll(r.Body)
	}

	// Forward the exact path (/docker/list, /docker/start, etc.)
	handleJSONBroadcast(w, r, "linux", LinuxAgentPort, method, r.URL.Path, body)
}

// --- Helpers ---

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
		msg = "No matching instances found."
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

func sendSSEMessage(w http.ResponseWriter, f http.Flusher, event, data string) {
	// Note: We do NOT need to set CORS headers here because
	// handleFaultSSE already called enableCORS() at the top.
	fmt.Fprintf(w, "event: %s\ndata: \"%s\"\n\n", event, data)
	f.Flush()
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
