package handlers

import (
	"net/http"
	"strconv"
	"time"
)

// validTypeIE is the allow-list for the type_ie (direction) filter parameter.
var validTypeIE = map[string]bool{
	"":        true,
	"Exports": true,
	"Imports": true,
	"Both":    true,
}

// validTypeGS is the allow-list for the type_gs (goods/services type) filter parameter.
var validTypeGS = map[string]bool{
	"":         true,
	"Goods":    true,
	"Services": true,
	"Total":    true,
}

// FilterParams holds the parsed and validated sidebar filter parameters.
type FilterParams struct {
	YearFrom int
	YearTo   int
	TypeIE   string // "Exports", "Imports", "Both", or "" (all)
	TypeGS   string // "Goods", "Services", "Total", or "" (all)
}

// parseFilterParams reads filter query parameters, validates them, and returns
// a FilterParams. If any parameter is invalid it writes a 400 response and returns false.
func parseFilterParams(w http.ResponseWriter, r *http.Request) (FilterParams, bool) {
	q := r.URL.Query()
	now := time.Now().Year()
	fp := FilterParams{
		YearFrom: 1990,
		YearTo:   now,
	}

	if s := q.Get("year_from"); s != "" {
		y, err := strconv.Atoi(s)
		if err != nil || y < 1900 || y > 9999 {
			http.Error(w, "invalid year_from parameter", http.StatusBadRequest)
			return FilterParams{}, false
		}
		fp.YearFrom = y
	}

	if s := q.Get("year_to"); s != "" {
		y, err := strconv.Atoi(s)
		if err != nil || y < 1900 || y > 9999 {
			http.Error(w, "invalid year_to parameter", http.StatusBadRequest)
			return FilterParams{}, false
		}
		fp.YearTo = y
	}

	if fp.YearFrom > fp.YearTo {
		http.Error(w, "year_from must be <= year_to", http.StatusBadRequest)
		return FilterParams{}, false
	}

	fp.TypeIE = q.Get("type_ie")
	if !validTypeIE[fp.TypeIE] {
		http.Error(w, "invalid type_ie parameter", http.StatusBadRequest)
		return FilterParams{}, false
	}

	fp.TypeGS = q.Get("type_gs")
	if !validTypeGS[fp.TypeGS] {
		http.Error(w, "invalid type_gs parameter", http.StatusBadRequest)
		return FilterParams{}, false
	}

	return fp, true
}
