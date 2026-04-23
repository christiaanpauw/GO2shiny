// Package handlers contains HTTP handler functions for the GO2shiny server.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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
// If absent, the provided defaultYear is returned.
// Returns 0 and writes a 400 response if the value is present but invalid.
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
//   - year      (optional, default: current year — backward-compat alias for year_from=year_to=year)
//   - year_from (optional, default: 1990)
//   - year_to   (optional, default: current year)
//   - type_ie   (optional: Exports|Imports|Both)
//   - type_gs   (optional: Goods|Services|Total)
func SummaryAPIHandler(querier db.KPIQuerier) http.HandlerFunc {
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

		// Backward compat: if a bare `year` param is supplied, treat it as a
		// single-year window (year_from = year_to = year).
		if s := r.URL.Query().Get("year"); s != "" {
			y, err := strconv.Atoi(s)
			if err != nil || y < 1900 || y > 9999 {
				http.Error(w, "invalid year parameter", http.StatusBadRequest)
				return
			}
			fp.YearFrom = y
			fp.YearTo = y
		}

		summary, err := querier.GetKPISummary(r.Context(), fp.YearFrom, fp.YearTo, fp.TypeIE, fp.TypeGS)
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
//   - type_ie   (optional: Exports|Imports|Both)
//   - type_gs   (optional: Goods|Services|Total)
func TimeSeriesAPIHandler(querier db.ChartQuerier) http.HandlerFunc {
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

		points, err := querier.GetTimeSeries(r.Context(), fp.YearFrom, fp.YearTo, fp.TypeIE, fp.TypeGS)
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
//   - page      (optional, default: 1)
//   - size      (optional, default: 25; capped at 100)
//   - q         (optional, free-text search)
//   - year_from (optional, default: 1990)
//   - year_to   (optional, default: current year)
//   - type_ie   (optional: Exports|Imports|Both)
//   - type_gs   (optional: Goods|Services|Total)
func TableAPIHandler(querier db.TableQuerier) http.HandlerFunc {
	if querier == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
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

		fp, ok := parseFilterParams(w, r)
		if !ok {
			return
		}

		result, err := querier.GetTablePage(r.Context(), page, size, search, fp.TypeIE, fp.TypeGS, fp.YearFrom, fp.YearTo)
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
//   - year      (optional, default: year_to from filter params)
//   - direction (optional, "Exports" or "Imports", default: derived from type_ie or "Exports")
//   - type_gs   (optional: Goods|Services|Total)
func TreemapAPIHandler(querier db.ChartQuerier) http.HandlerFunc {
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

		// year: explicit `year` param takes precedence, then fp.YearTo.
		year, ok := parseYear(w, r, fp.YearTo)
		if !ok {
			return
		}

		// direction: explicit param takes precedence, then type_ie filter.
		direction := r.URL.Query().Get("direction")
		if direction == "" {
			if fp.TypeIE == "Imports" {
				direction = "Imports"
			} else {
				direction = "Exports"
			}
		}
		if direction != "Exports" && direction != "Imports" {
			http.Error(w, "direction must be 'Exports' or 'Imports'", http.StatusBadRequest)
			return
		}

		node, err := querier.GetTreemap(r.Context(), year, direction, fp.TypeGS)
		if err != nil {
			http.Error(w, "failed to load treemap", http.StatusInternalServerError)
			return
		}

		writeJSON(w, node)
	}
}
