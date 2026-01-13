package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// handleLinuxControl processes requests for Linux agents
func handleLinuxControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method must be POST", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse Params
	query := r.URL.Query()
	keys := query["key"]
	vals := query["val"]
	endpoint := query.Get("endpoint")

	if len(keys) == 0 || len(keys) != len(vals) {
		http.Error(w, "Invalid or missing key/val pairs", http.StatusBadRequest)
		return
	}
	if endpoint == "" {
		http.Error(w, "Missing 'endpoint' param", http.StatusBadRequest)
		return
	}

	// 2. Map Endpoint
	var agentMethod string
	var agentPath string

	switch endpoint {
	case "docker_list":
		agentMethod = "GET"
		agentPath = "/docker/list"
	case "docker_status":
		agentMethod = "POST"
		agentPath = "/docker/status"
	case "docker_start":
		agentMethod = "POST"
		agentPath = "/docker/start"
	case "docker_stop":
		agentMethod = "POST"
		agentPath = "/docker/stop"
	case "docker_fault":
		agentMethod = "POST"
		agentPath = "/docker/fault"
	case "host_inject":
		agentMethod = "POST"
		agentPath = "/host/inject"
	default:
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		return
	}

	// 3. Read Body & Find Nodes
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	log.Printf("Targeting Linux nodes. Tags: %v=%v | Endpoint: %s", keys, vals, endpoint)
	ips, err := getInstances(keys, vals)
	if err != nil {
		http.Error(w, fmt.Sprintf("AWS Error: %v", err), http.StatusInternalServerError)
		return
	}

	if len(ips) == 0 {
		respondJSON(w, ips, []NodeResult{}) // Empty response
		return
	}

	// 4. Broadcast
	results := broadcastRequest(ips, LinuxAgentPort, agentMethod, agentPath, query, bodyBytes)
	respondJSON(w, ips, results)
}

// handleWindowsControl processes requests for Windows agents
func handleWindowsControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method must be POST", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	keys := query["key"]
	vals := query["val"]
	endpoint := query.Get("endpoint")

	if len(keys) == 0 || len(keys) != len(vals) {
		http.Error(w, "Invalid key/val pairs", http.StatusBadRequest)
		return
	}

	var path string
	switch endpoint {
	case "inject":
		path = "/host/inject"
	case "service":
		path = "/host/service"
	default:
		http.Error(w, "Invalid endpoint. Use 'inject' or 'service'", http.StatusBadRequest)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	ips, err := getInstances(keys, vals)
	if err != nil || len(ips) == 0 {
		http.Error(w, "No nodes found or AWS error", http.StatusInternalServerError)
		return
	}

	// Windows Agent is purely POST based
	results := broadcastRequest(ips, WindowsAgentPort, "POST", path, nil, bodyBytes)
	respondJSON(w, ips, results)
}

// Helper to formulate standard JSON response
func respondJSON(w http.ResponseWriter, ips []string, results []NodeResult) {
	successCount := 0
	for _, res := range results {
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			successCount++
		}
	}

	msg := ""
	if len(ips) == 0 {
		msg = "No running instances found matching tags."
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
