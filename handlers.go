package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func handleLinuxControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method must be POST", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	keys := query["key"]
	vals := query["val"]
	endpoint := query.Get("endpoint")

	// Basic validation (Keys are still needed for targeting groups like "Role:Worker")
	if len(keys) == 0 || len(keys) != len(vals) {
		http.Error(w, "Invalid or missing key/val pairs", http.StatusBadRequest)
		return
	}
	if endpoint == "" {
		http.Error(w, "Missing 'endpoint' param", http.StatusBadRequest)
		return
	}

	// ... [Switch Statement for Endpoint Mapping is same as before] ...
	// (Keeping the switch concise for this snippet)
	var agentMethod, agentPath string
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

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// CALL WITH "linux" -> This enforces the OS check
	log.Printf("Targeting Linux nodes. Tags: %v=%v", keys, vals)
	ips, err := getInstances(keys, vals, "linux")

	if err != nil {
		http.Error(w, fmt.Sprintf("AWS Error: %v", err), http.StatusInternalServerError)
		return
	}
	if len(ips) == 0 {
		respondJSON(w, ips, []NodeResult{})
		return
	}

	results := broadcastRequest(ips, LinuxAgentPort, agentMethod, agentPath, query, bodyBytes)
	respondJSON(w, ips, results)
}

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
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)

	// CALL WITH "windows" -> This enforces the OS check
	ips, err := getInstances(keys, vals, "windows")

	if err != nil || len(ips) == 0 {
		http.Error(w, "No nodes found or AWS error", http.StatusInternalServerError)
		return
	}

	results := broadcastRequest(ips, WindowsAgentPort, "POST", path, nil, bodyBytes)
	respondJSON(w, ips, results)
}

// ... respondJSON helper remains the same ...
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
		Total: len(ips), SuccessCount: successCount, FailedCount: len(ips) - successCount, Results: results, Message: msg,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
