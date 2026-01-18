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

// StreamBroadcast sends requests concurrently and pushes results to a channel (For SSE).
// Returns a read-only channel that closes when all requests are done.
func StreamBroadcast(ips []string, port, method, path string, originalParams url.Values, body []byte) <-chan NodeResult {
	resChan := make(chan NodeResult, len(ips))
	var wg sync.WaitGroup

	// Timeout covers long running faults (e.g. 60s CPU stress)
	client := http.Client{Timeout: 70 * time.Second}

	go func() {
		for _, ip := range ips {
			wg.Add(1)
			go func(targetIP string) {
				defer wg.Done()

				// Build URL
				reqURL := fmt.Sprintf("http://%s:%s%s", targetIP, port, path)

				// Forward allowed query parameters
				if len(originalParams) > 0 {
					u, _ := url.Parse(reqURL)
					q := u.Query()
					for k, v := range originalParams {
						// Filter out routing keys used by Control Tower
						if k != "key" && k != "val" {
							q.Set(k, v[0])
						}
					}
					u.RawQuery = q.Encode()
					reqURL = u.String()
				}

				var req *http.Request
				var err error
				if body != nil && len(body) > 0 {
					req, err = http.NewRequest(method, reqURL, bytes.NewBuffer(body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req, err = http.NewRequest(method, reqURL, nil)
				}

				if err != nil {
					resChan <- NodeResult{IP: targetIP, Status: "InternalErr", Message: err.Error()}
					return
				}

				resp, err := client.Do(req)
				if err != nil {
					resChan <- NodeResult{IP: targetIP, Status: "NetworkErr", Message: err.Error()}
					return
				}
				defer resp.Body.Close()

				respBytes, _ := io.ReadAll(resp.Body)
				statusLabel := "Failed"
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					statusLabel = "Success"
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
	}()

	return resChan
}

// broadcastRequest is a wrapper around StreamBroadcast for standard JSON responses.
// It waits for all results and returns a slice.
func broadcastRequest(ips []string, port, method, path string, originalParams url.Values, body []byte) []NodeResult {
	stream := StreamBroadcast(ips, port, method, path, originalParams, body)
	var results []NodeResult
	for res := range stream {
		results = append(results, res)
	}
	return results
}
