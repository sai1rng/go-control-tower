package main

// Configuration
const (
	WindowsAgentPort = "8080"
	LinuxAgentPort   = "8080"
	ServerPort       = ":8000"
	AWSRegion        = "eu-central-1"
)

// NodeResult represents the response from a single agent
type NodeResult struct {
	IP         string `json:"ip"`
	StatusCode int    `json:"status_code"`
	Status     string `json:"status"` // Success, Failed, Conflict, etc.
	Message    string `json:"message"`
}

// APIResponse is the aggregated response sent back to the user
type APIResponse struct {
	Total        int          `json:"total_nodes"`
	SuccessCount int          `json:"success_count"`
	FailedCount  int          `json:"failed_count"`
	Results      []NodeResult `json:"results"`
	Message      string       `json:"message,omitempty"`
}
