package handlers_test

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/christiaanpauw/GO2shiny/internal/db"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// mockMarketQuerier is a test double for db.MarketQuerier.
type mockMarketQuerier struct {
	countryTotals     []db.CountryTotal
	countryTimeSeries []db.CountryTimePoint
	err               error
}

func (m *mockMarketQuerier) GetCountryTotals(_ context.Context, yearFrom, yearTo int, typeIE string) ([]db.CountryTotal, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.countryTotals, nil
}

func (m *mockMarketQuerier) GetCountryTimeSeries(_ context.Context, countries []string, yearFrom, yearTo int, typeIE, typeGS string) ([]db.CountryTimePoint, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(countries) == 0 {
		return m.countryTimeSeries, nil
	}
	set := make(map[string]bool, len(countries))
	for _, c := range countries {
		set[c] = true
	}
	var pts []db.CountryTimePoint
	for _, p := range m.countryTimeSeries {
		if set[p.Country] {
			pts = append(pts, p)
		}
	}
	return pts, nil
}

// marketTestTmpl returns a minimal template set containing the "market_report" template.
func marketTestTmpl() *template.Template {
	const src = `{{define "market_report"}}` +
		`<div class="market-report-test">` +
		`{{range .Countries}}<span class="country">{{.Country}}</span>{{end}}` +
		`</div>{{end}}`
	return template.Must(template.New("market_report").Parse(src))
}

// TestMarketPageRenders verifies that GET /market returns 200 OK.
func TestMarketPageRenders(t *testing.T) {
	tmpl := template.Must(template.New("base.html").Parse(`{{block "content" .}}{{end}}`))

	req := httptest.NewRequest(http.MethodGet, "/market", nil)
	w := httptest.NewRecorder()

	handlers.Market(tmpl)(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
}

// TestCountriesAPI verifies that GET /api/trade/countries returns a valid JSON
// array of country objects with the expected fields.
func TestCountriesAPI(t *testing.T) {
	mock := &mockMarketQuerier{
		countryTotals: []db.CountryTotal{
			{Country: "China", Region: "Asia", Exports: 7_000, Imports: 3_000, TradeBalance: 4_000},
			{Country: "Australia", Region: "Oceania", Exports: 4_500, Imports: 5_000, TradeBalance: -500},
		},
	}

	h := handlers.CountriesAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/countries", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var countries []db.CountryTotal
	if err := json.NewDecoder(w.Body).Decode(&countries); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(countries) != 2 {
		t.Fatalf("want 2 countries, got %d", len(countries))
	}

	china := countries[0]
	if china.Country != "China" {
		t.Errorf("want first country=China, got %q", china.Country)
	}
	if china.Exports != 7_000 {
		t.Errorf("want China exports=7000, got %f", china.Exports)
	}
	if china.Imports != 3_000 {
		t.Errorf("want China imports=3000, got %f", china.Imports)
	}
	if china.TradeBalance != 4_000 {
		t.Errorf("want China trade_balance=4000, got %f", china.TradeBalance)
	}
}

// TestCountriesAPINilQuerier verifies that a nil querier returns 503.
func TestCountriesAPINilQuerier(t *testing.T) {
	h := handlers.CountriesAPIHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/countries", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

// TestMarketReportPartial verifies that GET /partials/market-report?countries[]=China
// returns a 200 HTML response containing the expected country name.
func TestMarketReportPartial(t *testing.T) {
	mock := &mockMarketQuerier{
		countryTotals: []db.CountryTotal{
			{Country: "China", Region: "Asia", Exports: 7_000, Imports: 3_000, TradeBalance: 4_000},
			{Country: "Australia", Region: "Oceania", Exports: 4_500, Imports: 5_000, TradeBalance: -500},
		},
	}

	h := handlers.MarketReportPartial(mock, marketTestTmpl())
	req := httptest.NewRequest(http.MethodGet, "/partials/market-report?countries[]=China", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "China") {
		t.Errorf("response body missing 'China': %q", body)
	}
	if strings.Contains(body, "Australia") {
		t.Errorf("response body should not include unselected 'Australia': %q", body)
	}
}

// TestMarketReportPartialNoFilter verifies that without a countries[] filter
// all countries are returned.
func TestMarketReportPartialNoFilter(t *testing.T) {
	mock := &mockMarketQuerier{
		countryTotals: []db.CountryTotal{
			{Country: "China", Region: "Asia", Exports: 7_000, Imports: 3_000, TradeBalance: 4_000},
			{Country: "Australia", Region: "Oceania", Exports: 4_500, Imports: 5_000, TradeBalance: -500},
		},
	}

	h := handlers.MarketReportPartial(mock, marketTestTmpl())
	req := httptest.NewRequest(http.MethodGet, "/partials/market-report", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "China") {
		t.Errorf("response body missing 'China': %q", body)
	}
	if !strings.Contains(body, "Australia") {
		t.Errorf("response body missing 'Australia': %q", body)
	}
}

// TestMarketReportPartialNilQuerier verifies that a nil querier returns 503.
func TestMarketReportPartialNilQuerier(t *testing.T) {
	h := handlers.MarketReportPartial(nil, marketTestTmpl())
	req := httptest.NewRequest(http.MethodGet, "/partials/market-report", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

// TestCountryTimeSeriesAPI verifies that GET /api/market/timeseries
// returns a JSON array of CountryTimePoint objects.
func TestCountryTimeSeriesAPI(t *testing.T) {
	mock := &mockMarketQuerier{
		countryTimeSeries: []db.CountryTimePoint{
			{Country: "China", Year: 2021, Exports: 6_000, Imports: 2_500},
			{Country: "China", Year: 2022, Exports: 7_000, Imports: 3_000},
			{Country: "Australia", Year: 2021, Exports: 4_000, Imports: 4_800},
		},
	}

	h := handlers.CountryTimeSeriesAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/market/timeseries?countries[]=China", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var pts []db.CountryTimePoint
	if err := json.NewDecoder(w.Body).Decode(&pts); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Only China points should be returned.
	if len(pts) != 2 {
		t.Fatalf("want 2 China points, got %d", len(pts))
	}
	for _, p := range pts {
		if p.Country != "China" {
			t.Errorf("want Country=China, got %q", p.Country)
		}
	}
}

// TestCountryTimeSeriesAPINilQuerier verifies that a nil querier returns 503.
func TestCountryTimeSeriesAPINilQuerier(t *testing.T) {
	h := handlers.CountryTimeSeriesAPIHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/market/timeseries", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
