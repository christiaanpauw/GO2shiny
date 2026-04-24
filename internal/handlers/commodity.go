// Package handlers contains HTTP handler functions for the GO2shiny server.
package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/christiaanpauw/GO2shiny/internal/db"
)

// commodityData holds the data passed to the commodity intelligence page template.
type commodityData struct {
	Year      int
	YearFrom  int
	YearTo    int
	Direction string // "exports", "imports", or "hs"
}

// validCommodityDirections is the allow-list for the commodity page direction URL parameter.
var validCommodityDirections = map[string]bool{
	"exports": true,
	"imports": true,
	"hs":      true,
}

// CommodityPage handles GET /commodity/{direction} and renders the commodity intelligence page.
// The direction URL parameter must be "exports", "imports", or "hs".
// When the direction is invalid, it responds with 400 Bad Request.
func CommodityPage(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		direction := chi.URLParam(r, "direction")
		if !validCommodityDirections[direction] {
			http.Error(w, "invalid direction: must be exports, imports, or hs", http.StatusBadRequest)
			return
		}

		now := time.Now().Year()
		data := commodityData{
			Year:      now,
			YearFrom:  1990,
			YearTo:    now,
			Direction: direction,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}
}

// validHSDigits is the allow-list for the hs_digits query parameter.
var validHSDigits = map[int]bool{
	2: true,
	4: true,
	6: true,
}

// CommodityAPIHandler handles GET /api/commodity.
//
// Query parameters:
//   - direction  — "Exports" or "Imports" (default: "Exports").
//   - year_from  — year range start (default: 1990).
//   - year_to    — year range end (default: current year).
//   - type_gs    — Goods/Services/Total filter.
//   - hs_digits  — when present (2, 4, or 6), returns HS code totals instead of commodity totals.
//
// When hs_digits is provided, returns a JSON array of HSCodeTotal objects.
// Otherwise, returns a JSON array of CommodityTotal objects ordered by value descending.
// When querier is nil, responds with 503 Service Unavailable.
func CommodityAPIHandler(querier db.CommodityQuerier) http.HandlerFunc {
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

		// direction: explicit param takes precedence; default to Exports.
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

		// hs_digits: when present, return HS code totals.
		if s := r.URL.Query().Get("hs_digits"); s != "" {
			digits, err := strconv.Atoi(s)
			if err != nil || !validHSDigits[digits] {
				http.Error(w, "hs_digits must be 2, 4, or 6", http.StatusBadRequest)
				return
			}

			totals, err := querier.GetHSCodeTotals(r.Context(), fp.YearFrom, fp.YearTo, direction, digits)
			if err != nil {
				http.Error(w, "failed to load hs code data", http.StatusInternalServerError)
				return
			}

			if totals == nil {
				totals = []db.HSCodeTotal{}
			}

			writeJSON(w, totals)
			return
		}

		// No hs_digits — return commodity totals.
		totals, err := querier.GetCommodityTotals(r.Context(), fp.YearFrom, fp.YearTo, direction, fp.TypeGS)
		if err != nil {
			http.Error(w, "failed to load commodity data", http.StatusInternalServerError)
			return
		}

		if totals == nil {
			totals = []db.CommodityTotal{}
		}

		writeJSON(w, totals)
	}
}
