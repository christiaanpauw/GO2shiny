package handlers_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/christiaanpauw/GO2shiny/internal/db"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// mockChartQuerier is a test double for db.ChartQuerier.
type mockChartQuerier struct {
	timeSeries []db.TimeSeriesPoint
	treemap    db.TreemapNode
	err        error
}

func (m *mockChartQuerier) GetTimeSeries(_ context.Context, yearFrom, yearTo int, typeIE, typeGS string) ([]db.TimeSeriesPoint, error) {
	if m.err != nil {
		return nil, m.err
	}
	var pts []db.TimeSeriesPoint
	for _, p := range m.timeSeries {
		if p.Year >= yearFrom && p.Year <= yearTo {
			pts = append(pts, p)
		}
	}
	return pts, nil
}

func (m *mockChartQuerier) GetTreemap(_ context.Context, _ int, _ string, _ string) (db.TreemapNode, error) {
	if m.err != nil {
		return db.TreemapNode{}, m.err
	}
	return m.treemap, nil
}

// TestSummaryAPI verifies that GET /api/trade/summary returns correct JSON
// schema and numeric values.
func TestSummaryAPI(t *testing.T) {
	mock := &mockKPIQuerier{
		summary: db.KPISummary{
			TotalExports: 1_000_000,
			TotalImports: 800_000,
			TradeBalance: 200_000,
			YoYChange:    3.5,
		},
	}

	h := handlers.SummaryAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/summary?year=2023", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var resp struct {
		Year         int     `json:"year"`
		TotalExports float64 `json:"total_exports"`
		TotalImports float64 `json:"total_imports"`
		TradeBalance float64 `json:"trade_balance"`
		YoYChange    float64 `json:"yoy_change"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Year != 2023 {
		t.Errorf("want year=2023, got %d", resp.Year)
	}
	if resp.TotalExports != 1_000_000 {
		t.Errorf("want total_exports=1000000, got %f", resp.TotalExports)
	}
	if resp.TotalImports != 800_000 {
		t.Errorf("want total_imports=800000, got %f", resp.TotalImports)
	}
	if resp.TradeBalance != 200_000 {
		t.Errorf("want trade_balance=200000, got %f", resp.TradeBalance)
	}
	if resp.YoYChange != 3.5 {
		t.Errorf("want yoy_change=3.5, got %f", resp.YoYChange)
	}
}

// TestSummaryAPINilQuerier verifies that a nil querier returns 503.
func TestSummaryAPINilQuerier(t *testing.T) {
	h := handlers.SummaryAPIHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/summary", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

// TestTimeSeriesAPI verifies that filters by year_from/year_to are applied
// correctly.
func TestTimeSeriesAPI(t *testing.T) {
	mock := &mockChartQuerier{
		timeSeries: []db.TimeSeriesPoint{
			{Year: 2020, Exports: 1_000, Imports: 900},
			{Year: 2021, Exports: 1_100, Imports: 950},
			{Year: 2022, Exports: 1_200, Imports: 1_000},
			{Year: 2023, Exports: 1_300, Imports: 1_050},
		},
	}

	h := handlers.TimeSeriesAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/timeseries?year_from=2021&year_to=2022", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var points []struct {
		Year    int     `json:"year"`
		Exports float64 `json:"exports"`
		Imports float64 `json:"imports"`
	}
	if err := json.NewDecoder(w.Body).Decode(&points); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("want 2 points (2021–2022), got %d", len(points))
	}
	if points[0].Year != 2021 {
		t.Errorf("want first point year=2021, got %d", points[0].Year)
	}
	if points[1].Year != 2022 {
		t.Errorf("want second point year=2022, got %d", points[1].Year)
	}
}

// TestTimeSeriesAPIInvalidParams verifies that invalid query parameters result
// in 400 Bad Request responses.
func TestTimeSeriesAPIInvalidParams(t *testing.T) {
	mock := &mockChartQuerier{}
	h := handlers.TimeSeriesAPIHandler(mock)

	cases := []struct {
		name string
		url  string
	}{
		{"invalid year_from", "/api/trade/timeseries?year_from=abc"},
		{"year_from too low", "/api/trade/timeseries?year_from=1800"},
		{"year_from too low boundary", "/api/trade/timeseries?year_from=1899"},
		{"invalid year_to", "/api/trade/timeseries?year_to=xyz"},
		{"year_to too high", "/api/trade/timeseries?year_to=99999"},
		{"year_to too high boundary", "/api/trade/timeseries?year_to=10000"},
		{"year_from > year_to", "/api/trade/timeseries?year_from=2023&year_to=2020"},
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

// TestTimeSeriesAPIBoundaryYears verifies that the inclusive boundary values
// 1900 and 9999 are accepted as valid year_from and year_to parameters.
func TestTimeSeriesAPIBoundaryYears(t *testing.T) {
	mock := &mockChartQuerier{}
	h := handlers.TimeSeriesAPIHandler(mock)

	cases := []struct {
		name string
		url  string
	}{
		{"year_from=1900", "/api/trade/timeseries?year_from=1900&year_to=1900"},
		{"year_to=9999", "/api/trade/timeseries?year_from=9999&year_to=9999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			h(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
		})
	}
}

// TestTreemapAPI verifies that the response has the hierarchical
// name/children/value structure required by ECharts.
func TestTreemapAPI(t *testing.T) {
	mock := &mockChartQuerier{
		treemap: db.TreemapNode{
			Name: "Exports 2023",
			Children: []db.TreemapNode{
				{Name: "Dairy", Value: 5_000_000},
				{Name: "Meat", Value: 3_000_000},
			},
		},
	}

	h := handlers.TreemapAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/treemap?year=2023&direction=Exports", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var node struct {
		Name     string `json:"name"`
		Children []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"children"`
	}
	if err := json.NewDecoder(w.Body).Decode(&node); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if node.Name != "Exports 2023" {
		t.Errorf("want name='Exports 2023', got %q", node.Name)
	}
	if len(node.Children) != 2 {
		t.Fatalf("want 2 children, got %d", len(node.Children))
	}
	if node.Children[0].Name != "Dairy" {
		t.Errorf("want first child='Dairy', got %q", node.Children[0].Name)
	}
	if node.Children[0].Value != 5_000_000 {
		t.Errorf("want first child value=5000000, got %f", node.Children[0].Value)
	}
}

// TestTreemapAPIInvalidDirection verifies that an unknown direction returns 400.
func TestTreemapAPIInvalidDirection(t *testing.T) {
	mock := &mockChartQuerier{}
	h := handlers.TreemapAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/treemap?direction=Both", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// TestAPIGZIP verifies that API endpoints return gzip-compressed responses
// when the client sends Accept-Encoding: gzip, via the chi compress middleware.
func TestAPIGZIP(t *testing.T) {
	mock := &mockChartQuerier{
		timeSeries: []db.TimeSeriesPoint{
			{Year: 2023, Exports: 1_000, Imports: 900},
		},
	}

	// Build a chi router with the same compression middleware used in production.
	r := chi.NewRouter()
	r.Use(middleware.Compress(5))
	r.Get("/api/trade/timeseries", handlers.TimeSeriesAPIHandler(mock))

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/trade/timeseries", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")

	// Use a transport that does not automatically decompress, so we can check
	// the Content-Encoding header and manually decompress.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("want Content-Encoding: gzip, got %q", resp.Header.Get("Content-Encoding"))
	}

	// Verify the body can be gzip-decompressed and decoded as valid JSON.
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("create gzip reader: %v", err)
	}
	defer gr.Close()

	var points []db.TimeSeriesPoint
	if err := json.NewDecoder(gr).Decode(&points); err != nil {
		t.Fatalf("decode gzipped response: %v", err)
	}
	if len(points) != 1 {
		t.Errorf("want 1 point, got %d", len(points))
	}
}
