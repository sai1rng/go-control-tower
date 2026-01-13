package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// broadcastRequest sends requests to all target IPs concurrently
func broadcastRequest(ips []string, port, method, path string, originalParams url.Values, body []byte) []NodeResult {
	var wg sync.WaitGroup
	resChan := make(chan NodeResult, len(ips))

	// Timeout set to 65s to accommodate long-running faults (e.g. 60s CPU stress)
	client := http.Client{Timeout: 65 * time.Second}

	for _, ip := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()

			// Construct the URL
			reqURL := fmt.Sprintf("http://%s:%s%s", targetIP, port, path)

			// Forward allowed query parameters (excluding routing params)
			if len(originalParams) > 0 {
				u, _ := url.Parse(reqURL)
				q := u.Query()
				for k, v := range originalParams {
					if k != "key" && k != "val" && k != "endpoint" {
						q.Set(k, v[0])
					}
				}
				u.RawQuery = q.Encode()
				reqURL = u.String()
			}

			// Prepare Request
			var req *http.Request
			var err error
			if method == "POST" {
				req, err = http.NewRequest(method, reqURL, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, err = http.NewRequest(method, reqURL, nil)
			}

			if err != nil {
				resChan <- NodeResult{IP: targetIP, Status: "InternalErr", Message: err.Error()}
				return
			}

			// Execute Request
			resp, err := client.Do(req)
			if err != nil {
				resChan <- NodeResult{IP: targetIP, Status: "NetworkErr", Message: err.Error()}
				return
			}
			defer resp.Body.Close()

			respBytes, _ := io.ReadAll(resp.Body)

			// Determine Status Label
			statusLabel := "Failed"
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				statusLabel = "Success"
			} else if resp.StatusCode == 409 {
				statusLabel = "Conflict"
			}

			resChan <- NodeResult{
				IP:         targetIP,
				StatusCode: resp.StatusCode,
				Status:     statusLabel,
				Message:    string(respBytes),
			}
		}(ip)
	}

	wg.Wait()
	close(resChan)

	// Collect results from channel
	var results []NodeResult
	for res := range resChan {
		results = append(results, res)
	}
	return results
}
