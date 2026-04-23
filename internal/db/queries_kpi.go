// Package db provides database connectivity, migration utilities, and query functions.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// KPISummary holds the four dashboard KPI values for a given year.
type KPISummary struct {
	Year         int
	TotalExports float64
	TotalImports float64
	TradeBalance float64
	// YoYChange is the percentage change in total trade (exports + imports)
	// compared with the prior year. Zero if no prior-year data exists.
	YoYChange float64
}

// KPIQuerier is the interface required by the KPI HTTP handler.
type KPIQuerier interface {
	GetKPISummary(ctx context.Context, year int) (KPISummary, error)
}

// PoolQuerier implements KPIQuerier using a *pgxpool.Pool.
type PoolQuerier struct {
	Pool *pgxpool.Pool
}

// GetKPISummary queries annual trade totals for the given year and year-1,
// then computes the four KPI metrics.
func (q *PoolQuerier) GetKPISummary(ctx context.Context, year int) (KPISummary, error) {
	// Fetch exports and imports totals for the requested year and the prior
	// year in a single round-trip so we can compute the YoY percentage change.
	const query = `
		SELECT
			year,
			SUM(CASE WHEN type_ie = 'Exports' THEN value_nzd ELSE 0 END) AS total_exports,
			SUM(CASE WHEN type_ie = 'Imports' THEN value_nzd ELSE 0 END) AS total_imports
		FROM trade_flows
		WHERE year IN ($1, $1 - 1)
		GROUP BY year
		ORDER BY year
	`

	rows, err := q.Pool.Query(ctx, query, year)
	if err != nil {
		return KPISummary{}, fmt.Errorf("query KPI: %w", err)
	}
	defer rows.Close()

	type yearRow struct {
		year    int
		exports float64
		imports float64
	}

	var results []yearRow
	for rows.Next() {
		var r yearRow
		if err := rows.Scan(&r.year, &r.exports, &r.imports); err != nil {
			return KPISummary{}, fmt.Errorf("scan KPI row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return KPISummary{}, fmt.Errorf("iterate KPI rows: %w", err)
	}

	summary := KPISummary{Year: year}

	// Locate current-year and prior-year rows.
	for _, r := range results {
		if r.year == year {
			summary.TotalExports = r.exports
			summary.TotalImports = r.imports
		}
	}
	summary.TradeBalance = summary.TotalExports - summary.TotalImports

	// Compute YoY change as a percentage of total trade.
	currentTotal := summary.TotalExports + summary.TotalImports
	for _, r := range results {
		if r.year == year-1 {
			prevTotal := r.exports + r.imports
			if prevTotal != 0 {
				summary.YoYChange = (currentTotal - prevTotal) / prevTotal * 100
			}
		}
	}

	return summary, nil
}
