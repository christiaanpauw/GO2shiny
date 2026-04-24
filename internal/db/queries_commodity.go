// Package db provides database connectivity, migration utilities, and query functions.
package db

import (
	"context"
	"fmt"
)

// CommodityTotal holds per-commodity aggregated trade values.
type CommodityTotal struct {
	Commodity string  `json:"commodity"`
	ValueNZD  float64 `json:"value_nzd"`
}

// HSCodeTotal holds per-HS code aggregated trade values.
type HSCodeTotal struct {
	HSCode   string  `json:"hs_code"`
	ValueNZD float64 `json:"value_nzd"`
}

// CommodityQuerier is the interface required by the commodity intelligence HTTP handlers.
type CommodityQuerier interface {
	// GetCommodityTotals returns per-commodity aggregated trade values for the given year
	// range and direction, ordered by total value descending.
	GetCommodityTotals(ctx context.Context, yearFrom, yearTo int, typeIE, typeGS string) ([]CommodityTotal, error)

	// GetHSCodeTotals returns per-HS code aggregated trade values for HS codes whose
	// length equals hsDigits (2, 4, or 6), ordered by total value descending.
	GetHSCodeTotals(ctx context.Context, yearFrom, yearTo int, typeIE string, hsDigits int) ([]HSCodeTotal, error)
}

// GetCommodityTotals returns per-commodity aggregated trade values ordered by total value descending.
func (q *PoolQuerier) GetCommodityTotals(ctx context.Context, yearFrom, yearTo int, typeIE, typeGS string) ([]CommodityTotal, error) {
	const query = `
		SELECT
			COALESCE(commodity, 'Unknown') AS commodity,
			COALESCE(SUM(value_nzd), 0)   AS value_nzd
		FROM trade_flows
		WHERE year BETWEEN $1 AND $2
		  AND ($3 = '' OR $3 = 'Both' OR type_ie = $3)
		  AND ($4 = '' OR $4 = 'Total' OR type_gs = $4)
		  AND commodity IS NOT NULL
		GROUP BY commodity
		ORDER BY value_nzd DESC
	`

	rows, err := q.Pool.Query(ctx, query, yearFrom, yearTo, typeIE, typeGS)
	if err != nil {
		return nil, fmt.Errorf("query commodity totals: %w", err)
	}
	defer rows.Close()

	var totals []CommodityTotal
	for rows.Next() {
		var ct CommodityTotal
		if err := rows.Scan(&ct.Commodity, &ct.ValueNZD); err != nil {
			return nil, fmt.Errorf("scan commodity total: %w", err)
		}
		totals = append(totals, ct)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate commodity totals: %w", err)
	}

	return totals, nil
}

// GetHSCodeTotals returns per-HS code aggregated trade values for HS codes with the
// given number of digits (2, 4, or 6), ordered by total value descending.
func (q *PoolQuerier) GetHSCodeTotals(ctx context.Context, yearFrom, yearTo int, typeIE string, hsDigits int) ([]HSCodeTotal, error) {
	const query = `
		SELECT
			COALESCE(hs_code, '') AS hs_code,
			COALESCE(SUM(value_nzd), 0) AS value_nzd
		FROM trade_flows
		WHERE year BETWEEN $1 AND $2
		  AND ($3 = '' OR $3 = 'Both' OR type_ie = $3)
		  AND hs_code IS NOT NULL
		  AND LENGTH(hs_code) = $4
		GROUP BY hs_code
		ORDER BY value_nzd DESC
	`

	rows, err := q.Pool.Query(ctx, query, yearFrom, yearTo, typeIE, hsDigits)
	if err != nil {
		return nil, fmt.Errorf("query hs code totals: %w", err)
	}
	defer rows.Close()

	var totals []HSCodeTotal
	for rows.Next() {
		var ht HSCodeTotal
		if err := rows.Scan(&ht.HSCode, &ht.ValueNZD); err != nil {
			return nil, fmt.Errorf("scan hs code total: %w", err)
		}
		totals = append(totals, ht)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hs code totals: %w", err)
	}

	return totals, nil
}
