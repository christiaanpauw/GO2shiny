// Package handlers contains HTTP handler functions for the GO2shiny server.
package handlers

import (
	"html/template"
	"net/http"
	"time"

	"github.com/christiaanpauw/GO2shiny/internal/db"
)

// marketData holds the data passed to the market intelligence page template.
type marketData struct {
	Year     int
	YearFrom int
	YearTo   int
}

// Market handles GET /market and renders the full market intelligence page.
func Market(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now().Year()
		data := marketData{Year: now, YearFrom: 1990, YearTo: now}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}
}

// marketReportData holds the data passed to the market_report partial template.
type marketReportData struct {
	Countries []db.CountryTotal
	Selected  []string
}

// MarketReportPartial handles GET /partials/market-report.
//
// Query parameters:
//   - countries[] (repeatable) — country names to include in the report.
//   - year_from, year_to, type_ie, type_gs — standard sidebar filter params.
//
// Returns an HTMX-swappable HTML fragment containing a country data table.
// When querier is nil, responds with 503 Service Unavailable.
func MarketReportPartial(querier db.MarketQuerier, tmpl *template.Template) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		fp, ok := parseFilterParams(w, r)
		if !ok {
			return
		}

		selectedCountries := r.URL.Query()["countries[]"]

		totals, err := querier.GetCountryTotals(r.Context(), fp.YearFrom, fp.YearTo, fp.TypeIE)
		if err != nil {
			http.Error(w, "failed to load country data", http.StatusInternalServerError)
			return
		}

		// Filter to selected countries when provided.
		if len(selectedCountries) > 0 {
			selectedSet := make(map[string]bool, len(selectedCountries))
			for _, c := range selectedCountries {
				selectedSet[c] = true
			}
			filtered := totals[:0]
			for _, ct := range totals {
				if selectedSet[ct.Country] {
					filtered = append(filtered, ct)
				}
			}
			totals = filtered
		}

		data := marketReportData{
			Countries: totals,
			Selected:  selectedCountries,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "market_report", data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}
}

// CountriesAPIHandler handles GET /api/trade/countries.
//
// Query parameters:
//   - year_from, year_to — year range (defaults: 1990 – current year).
//   - type_ie — direction filter ("Exports", "Imports", "Both", or "").
//
// Returns a JSON array of CountryTotal objects ordered by total trade volume.
// When querier is nil, responds with 503 Service Unavailable.
func CountriesAPIHandler(querier db.MarketQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		fp, ok := parseFilterParams(w, r)
		if !ok {
			return
		}

		totals, err := querier.GetCountryTotals(r.Context(), fp.YearFrom, fp.YearTo, fp.TypeIE)
		if err != nil {
			http.Error(w, "failed to load country data", http.StatusInternalServerError)
			return
		}

		if totals == nil {
			totals = []db.CountryTotal{}
		}

		writeJSON(w, totals)
	}
}

// CountryTimeSeriesAPIHandler handles GET /api/market/timeseries.
//
// Query parameters:
//   - countries[] (repeatable) — country names to include; omit for all countries.
//   - year_from, year_to, type_ie, type_gs — standard sidebar filter params.
//
// Returns a JSON array of CountryTimePoint objects for the requested countries.
// When querier is nil, responds with 503 Service Unavailable.
func CountryTimeSeriesAPIHandler(querier db.MarketQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		fp, ok := parseFilterParams(w, r)
		if !ok {
			return
		}

		countries := r.URL.Query()["countries[]"]

		points, err := querier.GetCountryTimeSeries(r.Context(), countries, fp.YearFrom, fp.YearTo, fp.TypeIE, fp.TypeGS)
		if err != nil {
			http.Error(w, "failed to load country time series", http.StatusInternalServerError)
			return
		}

		if points == nil {
			points = []db.CountryTimePoint{}
		}

		writeJSON(w, points)
	}
}
