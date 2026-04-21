// Package main is the entry point for the GO2shiny server.
// It wires together the router, database pool, and configuration,
// then starts listening for HTTP requests.
//
// See cmd/server/main.go for implementation details as development progresses.
package main

import (
	"fmt"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("GO2shiny server starting on :%s\n", port)
}
