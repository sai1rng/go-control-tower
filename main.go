package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Configuration
const (
	AgentPort  = "5000"         // Port where go-windows-service-handler is listening on nodes
	ServerPort = ":8080"        // Port this Control Tower listens on
	AWSRegion  = "eu-central-1" // Defined region
)

// Response structure
type NodeResult struct {
	IP      string `json:"ip"`
	Status  string `json:"status"` // Success or Failed
	Message string `json:"message"`
}

type APIResponse struct {
	Total   int          `json:"total_nodes,omitempty"`
	Success int          `json:"success_count,omitempty"`
	Failed  int          `json:"failed_count,omitempty"`
	Results []NodeResult `json:"results,omitempty"`
	Message string       `json:"message,omitempty"`
}

var ec2Client *ec2.Client

func main() {
	// 1. Initialize AWS Session with explicit Region
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(AWSRegion))
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}
	ec2Client = ec2.NewFromConfig(cfg)

	// 2. Setup Router
	http.HandleFunc("/control", handleControl)

	fmt.Printf("Control Tower listening on %s (Region: %s)\n", ServerPort, AWSRegion)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}

func handleControl(w http.ResponseWriter, r *http.Request) {
	// Allow simple GET requests with query params
	// Example: /control?key=Product&val=Payment&action=stop
	query := r.URL.Query()
	tagKey := query.Get("key")
	tagVal := query.Get("val")
	action := query.Get("action")

	if tagKey == "" || tagVal == "" || action == "" {
		http.Error(w, "Missing required params: key, val, action", http.StatusBadRequest)
		return
	}

	// Validate Action
	if action != "start" && action != "stop" && action != "status" {
		http.Error(w, "Invalid action. Use: start, stop, status", http.StatusBadRequest)
		return
	}

	// 1. Find Target Nodes
	log.Printf("Searching for nodes with %s=%s in %s...", tagKey, tagVal, AWSRegion)
	ips, err := getInstances(tagKey, tagVal)
	if err != nil {
		http.Error(w, fmt.Sprintf("AWS Error: %v", err), http.StatusInternalServerError)
		return
	}

	if len(ips) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Message: "No instances found matching tags."})
		return
	}

	// 2. Broadcast Command
	results := broadcastCommand(ips, action)

	// 3. Summarize and Respond
	successCount := 0
	for _, res := range results {
		if res.Status == "Success" {
			successCount++
		}
	}

	resp := APIResponse{
		Total:   len(ips),
		Success: successCount,
		Failed:  len(ips) - successCount,
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- AWS Logic ---

func getInstances(key, value string) ([]string, error) {
	var ips []string
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: aws.String("tag:" + key), Values: []string{value}},
			{Name: aws.String("instance-state-name"), Values: []string{"running"}},
		},
	}

	paginator := ec2.NewDescribeInstancesPaginator(ec2Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				// Prefer Private IP if available (assuming Control Tower is in the same VPC)
				if inst.PrivateIpAddress != nil {
					ips = append(ips, *inst.PrivateIpAddress)
				} else if inst.PublicIpAddress != nil {
					ips = append(ips, *inst.PublicIpAddress)
				}
			}
		}
	}
	return ips, nil
}

// --- Broadcast Logic ---

func broadcastCommand(ips []string, action string) []NodeResult {
	var wg sync.WaitGroup
	// Channel to safely collect results
	resChan := make(chan NodeResult, len(ips))

	for _, ip := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()

			// Construct URL: http://10.0.1.5:5000/stop
			url := fmt.Sprintf("http://%s:%s/%s", targetIP, AgentPort, action)

			client := http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(url)

			res := NodeResult{IP: targetIP}

			if err != nil {
				res.Status = "Failed"
				res.Message = err.Error()
			} else {
				defer resp.Body.Close()
				bodyBytes, _ := io.ReadAll(resp.Body)

				if resp.StatusCode == 200 {
					res.Status = "Success"
					res.Message = string(bodyBytes)
				} else {
					res.Status = "Failed"
					res.Message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
				}
			}
			resChan <- res
		}(ip)
	}

	wg.Wait()
	close(resChan)

	// Collect results from channel to slice
	var results []NodeResult
	for res := range resChan {
		results = append(results, res)
	}
	return results
}
