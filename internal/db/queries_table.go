// Package db provides database connectivity, migration utilities, and query functions.
package db

import (
	"context"
	"fmt"
	"strings"
)

// TableRow represents a single row returned by the trade table API.
type TableRow struct {
	Year      int     `json:"year"`
	Country   string  `json:"country"`
	TypeIE    string  `json:"type_ie"`
	TypeGS    string  `json:"type_gs"`
	Commodity string  `json:"commodity"`
	ValueNZD  float64 `json:"value_nzd"`
}

// TablePage is the paginated response returned by GetTablePage.
type TablePage struct {
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
	Rows  []TableRow `json:"rows"`
}

// TableQuerier is the interface required by the table HTTP handler.
type TableQuerier interface {
	GetTablePage(ctx context.Context, page, size int, q string) (TablePage, error)
}

// GetTablePage returns a paginated, optionally filtered page of trade_flows rows.
// If q is non-empty the results are filtered by a case-insensitive substring
// match across country, type_ie, type_gs, and commodity columns.
// The size parameter is expected to be already capped by the caller.
func (pq *PoolQuerier) GetTablePage(ctx context.Context, page, size int, q string) (TablePage, error) {
	offset := (page - 1) * size

	var (
		countQuery string
		dataQuery  string
		args       []any
	)

	if q == "" {
		countQuery = `SELECT COUNT(*) FROM trade_flows`
		dataQuery = `
			SELECT year, country, type_ie, type_gs, COALESCE(commodity, '') AS commodity, value_nzd
			FROM trade_flows
			ORDER BY year DESC, country, type_ie
			LIMIT $1 OFFSET $2
		`
		args = []any{size, offset}
	} else {
		escaped := strings.ReplaceAll(q, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, "_", `\_`)
		escaped = strings.ReplaceAll(escaped, "%", `\%`)
		pattern := "%" + escaped + "%"
		countQuery = `
			SELECT COUNT(*) FROM trade_flows
			WHERE country ILIKE $1
			   OR type_ie ILIKE $1
			   OR type_gs ILIKE $1
			   OR commodity ILIKE $1
		`
		dataQuery = `
			SELECT year, country, type_ie, type_gs, COALESCE(commodity, '') AS commodity, value_nzd
			FROM trade_flows
			WHERE country ILIKE $1
			   OR type_ie ILIKE $1
			   OR type_gs ILIKE $1
			   OR commodity ILIKE $1
			ORDER BY year DESC, country, type_ie
			LIMIT $2 OFFSET $3
		`
		args = []any{pattern, size, offset}
	}

	// Fetch total count.
	var total int
	var countArgs []any
	if q != "" {
		countArgs = []any{args[0]}
	}
	if err := pq.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return TablePage{}, fmt.Errorf("count table rows: %w", err)
	}

	// Fetch page rows.
	rows, err := pq.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return TablePage{}, fmt.Errorf("query table rows: %w", err)
	}
	defer rows.Close()

	tableRows := make([]TableRow, 0)
	for rows.Next() {
		var r TableRow
		if err := rows.Scan(&r.Year, &r.Country, &r.TypeIE, &r.TypeGS, &r.Commodity, &r.ValueNZD); err != nil {
			return TablePage{}, fmt.Errorf("scan table row: %w", err)
		}
		tableRows = append(tableRows, r)
	}
	if err := rows.Err(); err != nil {
		return TablePage{}, fmt.Errorf("iterate table rows: %w", err)
	}

	return TablePage{
		Total: total,
		Page:  page,
		Size:  size,
		Rows:  tableRows,
	}, nil
}
