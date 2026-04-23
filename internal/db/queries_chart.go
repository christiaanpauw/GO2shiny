// Package db provides database connectivity, migration utilities, and query functions.
package db

import (
	"context"
	"fmt"
)

// TimeSeriesPoint holds annual export and import totals for a single year.
type TimeSeriesPoint struct {
	Year    int     `json:"year"`
	Exports float64 `json:"exports"`
	Imports float64 `json:"imports"`
}

// TreemapNode represents a node in the commodity treemap hierarchy.
// Leaf nodes carry a Value; branch nodes carry Children instead.
type TreemapNode struct {
	Name     string        `json:"name"`
	Value    float64       `json:"value,omitempty"`
	Children []TreemapNode `json:"children,omitempty"`
}

// ChartQuerier is the interface required by the chart API handlers.
type ChartQuerier interface {
	GetTimeSeries(ctx context.Context, yearFrom, yearTo int) ([]TimeSeriesPoint, error)
	GetTreemap(ctx context.Context, year int, direction string) (TreemapNode, error)
}

// GetTimeSeries returns annual export and import totals for every year in
// [yearFrom, yearTo], ordered by year ascending.
func (q *PoolQuerier) GetTimeSeries(ctx context.Context, yearFrom, yearTo int) ([]TimeSeriesPoint, error) {
	const query = `
		SELECT
			year,
			SUM(CASE WHEN type_ie = 'Exports' THEN value_nzd ELSE 0 END) AS exports,
			SUM(CASE WHEN type_ie = 'Imports' THEN value_nzd ELSE 0 END) AS imports
		FROM trade_flows
		WHERE year BETWEEN $1 AND $2
		GROUP BY year
		ORDER BY year
	`

	rows, err := q.Pool.Query(ctx, query, yearFrom, yearTo)
	if err != nil {
		return nil, fmt.Errorf("query time series: %w", err)
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Year, &p.Exports, &p.Imports); err != nil {
			return nil, fmt.Errorf("scan time series row: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate time series rows: %w", err)
	}

	return points, nil
}

// GetTreemap returns the top commodity groups for the given year and trade
// direction ("Exports" or "Imports"), structured as a root TreemapNode whose
// Children are the individual commodity leaves ordered by value descending.
func (q *PoolQuerier) GetTreemap(ctx context.Context, year int, direction string) (TreemapNode, error) {
	const query = `
		SELECT COALESCE(commodity, 'Other') AS name, SUM(value_nzd) AS value
		FROM trade_flows
		WHERE year = $1 AND type_ie = $2
		GROUP BY commodity
		ORDER BY value DESC
		LIMIT 20
	`

	rows, err := q.Pool.Query(ctx, query, year, direction)
	if err != nil {
		return TreemapNode{}, fmt.Errorf("query treemap: %w", err)
	}
	defer rows.Close()

	root := TreemapNode{
		Name:     fmt.Sprintf("%s %d", direction, year),
		Children: []TreemapNode{},
	}
	for rows.Next() {
		var child TreemapNode
		if err := rows.Scan(&child.Name, &child.Value); err != nil {
			return TreemapNode{}, fmt.Errorf("scan treemap row: %w", err)
		}
		root.Children = append(root.Children, child)
	}
	if err := rows.Err(); err != nil {
		return TreemapNode{}, fmt.Errorf("iterate treemap rows: %w", err)
	}

	return root, nil
}
