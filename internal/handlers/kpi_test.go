package handlers_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/christiaanpauw/GO2shiny/internal/db"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// mockKPIQuerier is a test double for db.KPIQuerier that records call counts.
type mockKPIQuerier struct {
	callCount int
	summary   db.KPISummary
	err       error
}

func (m *mockKPIQuerier) GetKPISummary(_ context.Context, year int) (db.KPISummary, error) {
	m.callCount++
	s := m.summary
	s.Year = year
	return s, m.err
}

// kpiTestTmpl returns a minimal template set containing the "kpi_cards" template.
func kpiTestTmpl() *template.Template {
	const src = `<div class="kpi-test">` +
		`<span id="exports">{{.TotalExports}}</span>` +
		`<span id="imports">{{.TotalImports}}</span>` +
		`<span id="balance">{{.TradeBalance}}</span>` +
		`<span id="yoy">{{.YoYChange}}</span>` +
		`</div>`
	return template.Must(template.New("kpi_cards").Parse(src))
}

// TestKPIEndpoint verifies that GET /partials/kpis?year=2023 returns 200 OK
// with the expected HTML structure and NZD-formatted values.
func TestKPIEndpoint(t *testing.T) {
	mock := &mockKPIQuerier{
		summary: db.KPISummary{
			TotalExports: 12_000,
			TotalImports: 10_000,
			TradeBalance: 2_000,
			YoYChange:    5.0,
		},
	}

	h := handlers.KPIHandler(mock, kpiTestTmpl(), time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/partials/kpis?year=2023", nil)
	w := httptest.NewRecorder()

	h(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "kpi-test") {
		t.Errorf("response body missing expected markup: %q", body)
	}
	if !strings.Contains(body, "NZD") {
		t.Errorf("response body missing NZD formatting: %q", body)
	}
	if !strings.Contains(body, "5.0%") {
		t.Errorf("response body missing YoY formatting: %q", body)
	}
}

// TestKPIEndpointInvalidParams verifies that missing or invalid year values
// cause the handler to return 400 Bad Request.
func TestKPIEndpointInvalidParams(t *testing.T) {
	mock := &mockKPIQuerier{}
	h := handlers.KPIHandler(mock, kpiTestTmpl(), time.Minute)

	cases := []struct {
		name string
		url  string
	}{
		{"missing year", "/partials/kpis"},
		{"non-numeric year", "/partials/kpis?year=abc"},
		{"year too low", "/partials/kpis?year=1800"},
		{"year too high", "/partials/kpis?year=99999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			h(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", w.Code)
			}
		})
	}
}

// TestKPICache verifies that a second request for the same year within the
// cache TTL does not trigger a second database call.
func TestKPICache(t *testing.T) {
	mock := &mockKPIQuerier{
		summary: db.KPISummary{TotalExports: 1_000, TotalImports: 800},
	}

	h := handlers.KPIHandler(mock, kpiTestTmpl(), time.Minute)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/partials/kpis?year=2023", nil)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("call %d: want 200, got %d", i+1, w.Code)
		}
	}

	if mock.callCount != 1 {
		t.Errorf("want 1 DB call (cache hit on second request), got %d", mock.callCount)
	}
}

// TestKPIHandlerNilQuerier verifies that a nil querier causes the handler to
// respond with 503 Service Unavailable.
func TestKPIHandlerNilQuerier(t *testing.T) {
	h := handlers.KPIHandler(nil, kpiTestTmpl(), time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/partials/kpis?year=2023", nil)
	w := httptest.NewRecorder()

	h(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
