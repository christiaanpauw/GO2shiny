// Package handlers contains HTTP handler functions for the GO2shiny server.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/christiaanpauw/GO2shiny/internal/db"
)

// summaryResponse is the JSON body returned by GET /api/trade/summary.
type summaryResponse struct {
	Year         int     `json:"year"`
	TotalExports float64 `json:"total_exports"`
	TotalImports float64 `json:"total_imports"`
	TradeBalance float64 `json:"trade_balance"`
	YoYChange    float64 `json:"yoy_change"`
}

// writeJSON encodes v as JSON into w and sets Content-Type.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "json encode error", http.StatusInternalServerError)
	}
}

// parseYear reads an optional "year" query parameter.
// If absent, the current year is returned.
// Returns -1 and writes a 400 response if the value is present but invalid.
func parseYear(w http.ResponseWriter, r *http.Request, defaultYear int) (int, bool) {
	s := r.URL.Query().Get("year")
	if s == "" {
		return defaultYear, true
	}
	y, err := strconv.Atoi(s)
	if err != nil || y < 1900 || y > 9999 {
		http.Error(w, "invalid year parameter", http.StatusBadRequest)
		return 0, false
	}
	return y, true
}

// SummaryAPIHandler returns an http.HandlerFunc for GET /api/trade/summary.
//
// Query parameters:
//   - year (optional, default: current year)
func SummaryAPIHandler(querier db.KPIQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		year, ok := parseYear(w, r, time.Now().Year())
		if !ok {
			return
		}

		summary, err := querier.GetKPISummary(r.Context(), year)
		if err != nil {
			http.Error(w, "failed to load summary", http.StatusInternalServerError)
			return
		}

		writeJSON(w, summaryResponse{
			Year:         summary.Year,
			TotalExports: summary.TotalExports,
			TotalImports: summary.TotalImports,
			TradeBalance: summary.TradeBalance,
			YoYChange:    summary.YoYChange,
		})
	}
}

// TimeSeriesAPIHandler returns an http.HandlerFunc for GET /api/trade/timeseries.
//
// Query parameters:
//   - year_from (optional, default: 1990)
//   - year_to   (optional, default: current year)
func TimeSeriesAPIHandler(querier db.ChartQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		yearFrom := 1990
		yearTo := time.Now().Year()

		if s := r.URL.Query().Get("year_from"); s != "" {
			y, err := strconv.Atoi(s)
			if err != nil || y < 1900 || y > 9999 {
				http.Error(w, "invalid year_from parameter", http.StatusBadRequest)
				return
			}
			yearFrom = y
		}
		if s := r.URL.Query().Get("year_to"); s != "" {
			y, err := strconv.Atoi(s)
			if err != nil || y < 1900 || y > 9999 {
				http.Error(w, "invalid year_to parameter", http.StatusBadRequest)
				return
			}
			yearTo = y
		}

		if yearFrom > yearTo {
			http.Error(w, "year_from must be <= year_to", http.StatusBadRequest)
			return
		}

		points, err := querier.GetTimeSeries(r.Context(), yearFrom, yearTo)
		if err != nil {
			http.Error(w, "failed to load time series", http.StatusInternalServerError)
			return
		}

		if points == nil {
			points = []db.TimeSeriesPoint{}
		}

		writeJSON(w, points)
	}
}

// maxTablePageSize is the upper bound on the number of rows returned per page.
const maxTablePageSize = 100

// defaultTablePageSize is the default number of rows returned when no size is
// specified.
const defaultTablePageSize = 25

// TableAPIHandler returns an http.HandlerFunc for GET /api/trade/table.
//
// Query parameters:
//   - page (optional, default: 1; must be a positive integer)
//   - size (optional, default: 25; capped at 100)
//   - q    (optional, free-text search across country, type_ie, type_gs, commodity)
func TableAPIHandler(querier db.TableQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		// Parse page.
		page := 1
		if s := q.Get("page"); s != "" {
			p, err := strconv.Atoi(s)
			if err != nil || p < 1 {
				http.Error(w, "invalid page parameter", http.StatusBadRequest)
				return
			}
			page = p
		}

		// Parse size.
		size := defaultTablePageSize
		if s := q.Get("size"); s != "" {
			sz, err := strconv.Atoi(s)
			if err != nil || sz < 1 {
				http.Error(w, "invalid size parameter", http.StatusBadRequest)
				return
			}
			if sz > maxTablePageSize {
				sz = maxTablePageSize
			}
			size = sz
		}

		search := q.Get("q")

		result, err := querier.GetTablePage(r.Context(), page, size, search)
		if err != nil {
			http.Error(w, "failed to load table data", http.StatusInternalServerError)
			return
		}

		writeJSON(w, result)
	}
}

// TreemapAPIHandler returns an http.HandlerFunc for GET /api/trade/treemap.
//
// Query parameters:
//   - year      (optional, default: current year)
//   - direction (optional, "Exports" or "Imports", default: "Exports")
func TreemapAPIHandler(querier db.ChartQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database not available", http.StatusServiceUnavailable)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		year, ok := parseYear(w, r, time.Now().Year())
		if !ok {
			return
		}

		direction := r.URL.Query().Get("direction")
		if direction == "" {
			direction = "Exports"
		}
		if direction != "Exports" && direction != "Imports" {
			http.Error(w, "direction must be 'Exports' or 'Imports'", http.StatusBadRequest)
			return
		}

		node, err := querier.GetTreemap(r.Context(), year, direction)
		if err != nil {
			http.Error(w, "failed to load treemap", http.StatusInternalServerError)
			return
		}

		writeJSON(w, node)
	}
}
