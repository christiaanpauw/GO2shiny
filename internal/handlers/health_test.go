package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// TestHealthEndpoint verifies that GET /health returns 200 OK with the
// expected JSON body {"status":"ok"}.
func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handlers.Health(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("want Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf(`want status "ok", got %q`, body["status"])
	}
}
