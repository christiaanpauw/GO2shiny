package handlers_test

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/christiaanpauw/GO2shiny/internal/db"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// mockCommodityQuerier is a test double for db.CommodityQuerier.
type mockCommodityQuerier struct {
	commodityTotals []db.CommodityTotal
	hsCodeTotals    []db.HSCodeTotal
	err             error
}

func (m *mockCommodityQuerier) GetCommodityTotals(_ context.Context, yearFrom, yearTo int, typeIE, typeGS string) ([]db.CommodityTotal, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commodityTotals, nil
}

func (m *mockCommodityQuerier) GetHSCodeTotals(_ context.Context, yearFrom, yearTo int, typeIE string, hsDigits int) ([]db.HSCodeTotal, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Filter by digit length to simulate the DB behaviour.
	var filtered []db.HSCodeTotal
	for _, ht := range m.hsCodeTotals {
		if len(ht.HSCode) == hsDigits {
			filtered = append(filtered, ht)
		}
	}
	return filtered, nil
}

// commodityTestTmpl returns a minimal template set containing "base.html" and "content".
func commodityTestTmpl() *template.Template {
	const src = `{{define "base.html"}}{{block "content" .}}{{end}}{{end}}` +
		`{{define "content"}}commodity-{{.Direction}}{{end}}`
	return template.Must(template.New("base.html").Parse(src))
}

// newCommodityRequest builds a request with the direction chi URL param set.
func newCommodityRequest(direction string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/commodity/"+direction, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("direction", direction)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// TestCommodityExportsPage verifies that GET /commodity/exports returns 200 OK.
func TestCommodityExportsPage(t *testing.T) {
	h := handlers.CommodityPage(commodityTestTmpl())
	req := newCommodityRequest("exports")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// TestCommodityImportsPage verifies that GET /commodity/imports returns 200 OK.
func TestCommodityImportsPage(t *testing.T) {
	h := handlers.CommodityPage(commodityTestTmpl())
	req := newCommodityRequest("imports")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// TestCommodityHSPage verifies that GET /commodity/hs returns 200 OK.
func TestCommodityHSPage(t *testing.T) {
	h := handlers.CommodityPage(commodityTestTmpl())
	req := newCommodityRequest("hs")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// TestCommodityPageInvalidDirection verifies that an unknown direction returns 400.
func TestCommodityPageInvalidDirection(t *testing.T) {
	h := handlers.CommodityPage(commodityTestTmpl())
	req := newCommodityRequest("unknown")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// TestCommodityAPI verifies that GET /api/commodity returns a sorted list of commodities
// with NZD values.
func TestCommodityAPI(t *testing.T) {
	mock := &mockCommodityQuerier{
		commodityTotals: []db.CommodityTotal{
			{Commodity: "Dairy products", ValueNZD: 18_400},
			{Commodity: "Meat and edible meat offal", ValueNZD: 9_200},
		},
	}

	h := handlers.CommodityAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/commodity?direction=Exports", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var totals []db.CommodityTotal
	if err := json.NewDecoder(w.Body).Decode(&totals); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(totals) != 2 {
		t.Fatalf("want 2 commodities, got %d", len(totals))
	}

	if totals[0].Commodity != "Dairy products" {
		t.Errorf("want first commodity=Dairy products, got %q", totals[0].Commodity)
	}

	if totals[0].ValueNZD != 18_400 {
		t.Errorf("want Dairy products value_nzd=18400, got %f", totals[0].ValueNZD)
	}
}

// TestCommodityAPINilQuerier verifies that a nil querier returns 503.
func TestCommodityAPINilQuerier(t *testing.T) {
	h := handlers.CommodityAPIHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/commodity", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

// TestHSCodeDrillDown verifies that GET /api/commodity?hs_digits=4 returns only
// 4-digit HS codes.
func TestHSCodeDrillDown(t *testing.T) {
	mock := &mockCommodityQuerier{
		hsCodeTotals: []db.HSCodeTotal{
			{HSCode: "04", ValueNZD: 5_000},     // 2-digit
			{HSCode: "0401", ValueNZD: 3_000},   // 4-digit
			{HSCode: "0402", ValueNZD: 2_000},   // 4-digit
			{HSCode: "040110", ValueNZD: 1_500}, // 6-digit
		},
	}

	h := handlers.CommodityAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/commodity?direction=Exports&hs_digits=4", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var totals []db.HSCodeTotal
	if err := json.NewDecoder(w.Body).Decode(&totals); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(totals) != 2 {
		t.Fatalf("want 2 four-digit HS codes, got %d", len(totals))
	}

	for _, ht := range totals {
		if len(ht.HSCode) != 4 {
			t.Errorf("want 4-digit HS code, got %q (len=%d)", ht.HSCode, len(ht.HSCode))
		}
	}
}

// TestHSCodeDrillDownInvalidDigits verifies that an invalid hs_digits value returns 400.
func TestHSCodeDrillDownInvalidDigits(t *testing.T) {
	mock := &mockCommodityQuerier{}
	h := handlers.CommodityAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/commodity?hs_digits=3", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
