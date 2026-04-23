// Package handlers contains HTTP handler functions for the application.
package handlers

import (
	"encoding/json"
	"net/http"
)

// Health handles GET /health and returns a simple liveness probe response.
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
