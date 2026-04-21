// Package models defines data structures for trade intelligence data.
package models

// TradeFlow represents a single trade flow record from the trade_flows table.
type TradeFlow struct {
	ID        int64   `db:"id"`
	Year      int16   `db:"year"`
	Quarter   *string `db:"quarter"`
	Country   string  `db:"country"`
	Region    *string `db:"region"`
	TypeIE    string  `db:"type_ie"`
	TypeGS    string  `db:"type_gs"`
	Commodity *string `db:"commodity"`
	HSCode    *string `db:"hs_code"`
	ValueNZD  float64 `db:"value_nzd"`
}

// Country represents a row in the countries reference table.
type Country struct {
	Country string  `db:"country"`
	Region  string  `db:"region"`
	ISO3    *string `db:"iso3"`
}

// CommoditySummary is a rolled-up view of trade value by commodity.
type CommoditySummary struct {
	Commodity string  `db:"commodity"`
	TypeIE    string  `db:"type_ie"`
	TypeGS    string  `db:"type_gs"`
	Year      int16   `db:"year"`
	ValueNZD  float64 `db:"value_nzd"`
}
