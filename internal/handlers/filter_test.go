package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/christiaanpauw/GO2shiny/internal/db"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// TestFilterTypeIEAllowList verifies that an invalid type_ie value returns 400.
func TestFilterTypeIEAllowList(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"invalid type_ie on kpi", "/partials/kpis?type_ie=invalid"},
		{"invalid type_ie on kpi (numeric)", "/partials/kpis?type_ie=1"},
		{"invalid type_gs on kpi", "/partials/kpis?type_gs=invalid"},
	}

	mock := &mockKPIQuerier{}
	h := handlers.KPIHandler(mock, kpiTestTmpl(), time.Minute)

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

// TestFilterYearRange verifies that year_from > year_to on the KPI endpoint
// returns 400 Bad Request.
func TestFilterYearRange(t *testing.T) {
	mock := &mockKPIQuerier{}
	h := handlers.KPIHandler(mock, kpiTestTmpl(), time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/partials/kpis?year_from=2023&year_to=2020", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "year_from must be <= year_to") {
		t.Errorf("want error message about year range, got %q", w.Body.String())
	}
}

// TestFilterUpdatesKPIs verifies that different year_from/year_to values produce
// separate DB calls (distinct cache keys) and return 200 with NZD-formatted data.
func TestFilterUpdatesKPIs(t *testing.T) {
	mock := &mockKPIQuerier{
		summary: db.KPISummary{
			TotalExports: 50_000,
			TotalImports: 30_000,
		},
	}

	// Use a long TTL; the two requests have different cache keys so both hit the DB.
	h := handlers.KPIHandler(mock, kpiTestTmpl(), time.Minute)

	ranges := []struct {
		url        string
		wantYearTo int
	}{
		{"/partials/kpis?year_from=2020&year_to=2021", 2021},
		{"/partials/kpis?year_from=2022&year_to=2023", 2023},
	}

	for _, tc := range ranges {
		req := httptest.NewRequest(http.MethodGet, tc.url, nil)
		w := httptest.NewRecorder()
		h(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("url %s: want 200, got %d", tc.url, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "NZD") {
			t.Errorf("url %s: response missing NZD formatting: %q", tc.url, body)
		}
	}

	// Each request has a unique cache key → two separate DB calls.
	if mock.callCount != 2 {
		t.Errorf("want 2 DB calls (different filter params), got %d", mock.callCount)
	}
}
