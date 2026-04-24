// Package db provides database connectivity, migration utilities, and query functions.
package db

import (
	"context"
	"fmt"
)

// CountryTotal holds per-country aggregated trade totals for the market intelligence page.
type CountryTotal struct {
	Country      string  `json:"country"`
	Region       string  `json:"region"`
	Exports      float64 `json:"exports"`
	Imports      float64 `json:"imports"`
	TradeBalance float64 `json:"trade_balance"`
}

// CountryTimePoint holds annual export and import totals for a single country/year pair.
type CountryTimePoint struct {
	Country string  `json:"country"`
	Year    int     `json:"year"`
	Exports float64 `json:"exports"`
	Imports float64 `json:"imports"`
}

// MarketQuerier is the interface required by the market intelligence HTTP handlers.
type MarketQuerier interface {
	// GetCountryTotals returns per-country trade aggregates for the given year range,
	// optionally filtered by typeIE ("Exports", "Imports", "Both", or "").
	// Results are ordered by total trade value (exports + imports) descending.
	GetCountryTotals(ctx context.Context, yearFrom, yearTo int, typeIE string) ([]CountryTotal, error)

	// GetCountryTimeSeries returns annual export and import totals per country for
	// the given countries and year range, ordered by country then year ascending.
	// An empty countries slice returns data for all countries.
	GetCountryTimeSeries(ctx context.Context, countries []string, yearFrom, yearTo int, typeIE, typeGS string) ([]CountryTimePoint, error)
}

// GetCountryTotals returns per-country aggregated trade totals ordered by total trade volume.
func (q *PoolQuerier) GetCountryTotals(ctx context.Context, yearFrom, yearTo int, typeIE string) ([]CountryTotal, error) {
	const query = `
		SELECT
			t.country,
			COALESCE(c.region, '') AS region,
			COALESCE(SUM(CASE WHEN t.type_ie = 'Exports' THEN t.value_nzd ELSE 0 END), 0) AS exports,
			COALESCE(SUM(CASE WHEN t.type_ie = 'Imports' THEN t.value_nzd ELSE 0 END), 0) AS imports
		FROM trade_flows t
		LEFT JOIN countries c ON c.country = t.country
		WHERE t.year BETWEEN $1 AND $2
		  AND ($3 = '' OR $3 = 'Both' OR t.type_ie = $3)
		GROUP BY t.country, c.region
		ORDER BY (exports + imports) DESC
	`

	rows, err := q.Pool.Query(ctx, query, yearFrom, yearTo, typeIE)
	if err != nil {
		return nil, fmt.Errorf("query country totals: %w", err)
	}
	defer rows.Close()

	var totals []CountryTotal
	for rows.Next() {
		var ct CountryTotal
		if err := rows.Scan(&ct.Country, &ct.Region, &ct.Exports, &ct.Imports); err != nil {
			return nil, fmt.Errorf("scan country total: %w", err)
		}
		ct.TradeBalance = ct.Exports - ct.Imports
		totals = append(totals, ct)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate country totals: %w", err)
	}

	return totals, nil
}

// GetCountryTimeSeries returns annual export and import totals for the given countries and
// year range, ordered by country then year ascending.
// When countries is empty, data for all countries is returned.
func (q *PoolQuerier) GetCountryTimeSeries(ctx context.Context, countries []string, yearFrom, yearTo int, typeIE, typeGS string) ([]CountryTimePoint, error) {
	// Use ANY($5) to filter by countries when the slice is non-empty.
	// pgx maps []string to a PostgreSQL text array automatically.
	const queryFiltered = `
		SELECT
			country,
			year,
			COALESCE(SUM(CASE WHEN type_ie = 'Exports' THEN value_nzd ELSE 0 END), 0) AS exports,
			COALESCE(SUM(CASE WHEN type_ie = 'Imports' THEN value_nzd ELSE 0 END), 0) AS imports
		FROM trade_flows
		WHERE year BETWEEN $1 AND $2
		  AND ($3 = '' OR $3 = 'Both' OR type_ie = $3)
		  AND ($4 = '' OR $4 = 'Total' OR type_gs = $4)
		  AND country = ANY($5)
		GROUP BY country, year
		ORDER BY country, year
	`
	const queryAll = `
		SELECT
			country,
			year,
			COALESCE(SUM(CASE WHEN type_ie = 'Exports' THEN value_nzd ELSE 0 END), 0) AS exports,
			COALESCE(SUM(CASE WHEN type_ie = 'Imports' THEN value_nzd ELSE 0 END), 0) AS imports
		FROM trade_flows
		WHERE year BETWEEN $1 AND $2
		  AND ($3 = '' OR $3 = 'Both' OR type_ie = $3)
		  AND ($4 = '' OR $4 = 'Total' OR type_gs = $4)
		GROUP BY country, year
		ORDER BY country, year
	`

	var sqlQuery string
	var args []any
	if len(countries) > 0 {
		sqlQuery = queryFiltered
		args = []any{yearFrom, yearTo, typeIE, typeGS, countries}
	} else {
		sqlQuery = queryAll
		args = []any{yearFrom, yearTo, typeIE, typeGS}
	}

	rows, err := q.Pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query country time series: %w", err)
	}
	defer rows.Close()

	var points []CountryTimePoint
	for rows.Next() {
		var p CountryTimePoint
		if err := rows.Scan(&p.Country, &p.Year, &p.Exports, &p.Imports); err != nil {
			return nil, fmt.Errorf("scan country time point: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate country time points: %w", err)
	}

	return points, nil
}
