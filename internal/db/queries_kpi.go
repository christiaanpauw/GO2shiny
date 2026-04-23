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
	GetKPISummary(ctx context.Context, yearFrom, yearTo int, typeIE, typeGS string) (KPISummary, error)
}

// PoolQuerier implements KPIQuerier using a *pgxpool.Pool.
type PoolQuerier struct {
	Pool *pgxpool.Pool
}

// GetKPISummary aggregates trade totals over [yearFrom, yearTo], optionally filtered by
// typeIE ("Exports", "Imports", "Both", or "") and typeGS ("Goods", "Services", "Total", or "").
// YoY change compares [yearFrom, yearTo] against [yearFrom-1, yearTo-1].
func (q *PoolQuerier) GetKPISummary(ctx context.Context, yearFrom, yearTo int, typeIE, typeGS string) (KPISummary, error) {
	// Aggregate with optional type_ie / type_gs filters.
	const query = `
		SELECT
			COALESCE(SUM(CASE WHEN type_ie = 'Exports' THEN value_nzd ELSE 0 END), 0) AS total_exports,
			COALESCE(SUM(CASE WHEN type_ie = 'Imports' THEN value_nzd ELSE 0 END), 0) AS total_imports
		FROM trade_flows
		WHERE year BETWEEN $1 AND $2
		  AND ($3 = '' OR $3 = 'Both' OR type_ie = $3)
		  AND ($4 = '' OR $4 = 'Total' OR type_gs = $4)
	`

	var curExports, curImports float64
	if err := q.Pool.QueryRow(ctx, query, yearFrom, yearTo, typeIE, typeGS).
		Scan(&curExports, &curImports); err != nil {
		return KPISummary{}, fmt.Errorf("query KPI: %w", err)
	}

	// Prior period for YoY comparison.
	var prevExports, prevImports float64
	if err := q.Pool.QueryRow(ctx, query, yearFrom-1, yearTo-1, typeIE, typeGS).
		Scan(&prevExports, &prevImports); err != nil {
		return KPISummary{}, fmt.Errorf("query KPI prior period: %w", err)
	}

	summary := KPISummary{
		Year:         yearTo,
		TotalExports: curExports,
		TotalImports: curImports,
		TradeBalance: curExports - curImports,
	}

	currentTotal := curExports + curImports
	prevTotal := prevExports + prevImports
	if prevTotal != 0 {
		summary.YoYChange = (currentTotal - prevTotal) / prevTotal * 100
	}

	return summary, nil
}
