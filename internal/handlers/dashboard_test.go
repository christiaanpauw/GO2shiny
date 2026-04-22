package handlers_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// TestDashboardPageReturns200 verifies that GET /dashboard returns 200 OK.
// A minimal template stub is used so that no filesystem access is required.
func TestDashboardPageReturns200(t *testing.T) {
	// Minimal template that satisfies the base.html contract used by Dashboard.
	tmpl := template.Must(template.New("base.html").Parse(`{{block "content" .}}{{end}}`))

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	handlers.Dashboard(tmpl)(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
}
