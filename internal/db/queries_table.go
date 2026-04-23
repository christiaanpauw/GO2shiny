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
	GetTablePage(ctx context.Context, page, size int, q, typeIE, typeGS string, yearFrom, yearTo int) (TablePage, error)
}

// GetTablePage returns a paginated, optionally filtered page of trade_flows rows.
// Filtering supports year range, type_ie, type_gs, and free-text search (q).
// The size parameter is expected to be already capped by the caller.
func (pq *PoolQuerier) GetTablePage(ctx context.Context, page, size int, q, typeIE, typeGS string, yearFrom, yearTo int) (TablePage, error) {
	offset := (page - 1) * size

	// Build dynamic WHERE clause.
	var conds []string
	var args []any

	// Year range (always applied; defaults are 1990–now).
	conds = append(conds, fmt.Sprintf("year BETWEEN $%d AND $%d", len(args)+1, len(args)+2))
	args = append(args, yearFrom, yearTo)

	// Direction filter.
	if typeIE != "" && typeIE != "Both" {
		conds = append(conds, fmt.Sprintf("type_ie = $%d", len(args)+1))
		args = append(args, typeIE)
	}

	// Goods/services type filter.
	if typeGS != "" && typeGS != "Total" {
		conds = append(conds, fmt.Sprintf("type_gs = $%d", len(args)+1))
		args = append(args, typeGS)
	}

	// Free-text search across key text columns.
	if q != "" {
		escaped := strings.ReplaceAll(q, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, "_", `\_`)
		escaped = strings.ReplaceAll(escaped, "%", `\%`)
		pattern := "%" + escaped + "%"
		// The same parameter index is intentional: all four ILIKE conditions
		// reference the single pattern argument appended below.
		idx := len(args) + 1
		conds = append(conds, fmt.Sprintf(
			"(country ILIKE $%d OR type_ie ILIKE $%d OR type_gs ILIKE $%d OR commodity ILIKE $%d)",
			idx, idx, idx, idx,
		))
		args = append(args, pattern)
	}

	whereClause := "WHERE " + strings.Join(conds, " AND ")
	n := len(args)

	countQuery := "SELECT COUNT(*) FROM trade_flows " + whereClause

	dataQuery := fmt.Sprintf(`
		SELECT year, country, type_ie, type_gs, COALESCE(commodity, '') AS commodity, value_nzd
		FROM trade_flows
		%s
		ORDER BY year DESC, country, type_ie
		LIMIT $%d OFFSET $%d
	`, whereClause, n+1, n+2)

	// Fetch total count.
	var total int
	if err := pq.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return TablePage{}, fmt.Errorf("count table rows: %w", err)
	}

	// Fetch page rows (append LIMIT and OFFSET args).
	dataArgs := make([]any, n+2)
	copy(dataArgs, args)
	dataArgs[n] = size
	dataArgs[n+1] = offset

	rows, err := pq.Pool.Query(ctx, dataQuery, dataArgs...)
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
