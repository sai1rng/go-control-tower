package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// 1. Initialize AWS Client (defined in aws_client.go)
	InitAWS()

	// 2. Register Routes (handlers defined in handlers.go)
	http.HandleFunc("/control/windows", handleWindowsControl)
	http.HandleFunc("/control/linux", handleLinuxControl)
	// NEW: Health Check
	http.HandleFunc("/health", handleHealth)
	fmt.Printf("Control Tower listening on %s (Region: %s)\n", ServerPort, AWSRegion)

	// 3. Start Server
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}
