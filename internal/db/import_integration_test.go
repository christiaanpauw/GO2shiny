//go:build integration

package db_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/christiaanpauw/GO2shiny/internal/db"
)

// TestImportCSV applies migrations, imports the sample CSV dataset into a
// real PostgreSQL instance, and verifies the expected row counts.
//
// Run with:
//
//	go test -tags=integration ./internal/db/...
func TestImportCSV(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer pool.Close()

	// Truncate tables to ensure a clean slate for the row-count assertions.
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE trade_flows"); err != nil {
		t.Fatalf("truncate trade_flows: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE countries"); err != nil {
		t.Fatalf("truncate countries: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	const (
		// Paths relative to this test file (internal/db/).
		countriesCSV  = "../../data/sample/countries.csv"
		tradeFlowsCSV = "../../data/sample/trade_flows.csv"
	)

	if err := csvImportCountries(ctx, conn.Conn(), countriesCSV); err != nil {
		t.Fatalf("import countries: %v", err)
	}

	if err := csvImportTradeFlows(ctx, conn.Conn(), tradeFlowsCSV); err != nil {
		t.Fatalf("import trade flows: %v", err)
	}

	var countriesCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM countries").Scan(&countriesCount); err != nil {
		t.Fatalf("count countries: %v", err)
	}
	const wantCountries = 15
	if countriesCount != wantCountries {
		t.Errorf("countries: want %d, got %d", wantCountries, countriesCount)
	}

	var tradeFlowsCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trade_flows").Scan(&tradeFlowsCount); err != nil {
		t.Fatalf("count trade_flows: %v", err)
	}
	const wantTradeFlows = 107
	if tradeFlowsCount != wantTradeFlows {
		t.Errorf("trade_flows: want %d, got %d", wantTradeFlows, tradeFlowsCount)
	}
}

// csvImportCountries bulk-copies rows from a countries CSV into the countries table.
func csvImportCountries(ctx context.Context, conn *pgx.Conn, filename string) error {
	records, err := readCSVFile(filename)
	if err != nil {
		return err
	}
	if len(records) > 0 {
		records = records[1:] // skip header
	}

	copyRows := make([][]any, 0, len(records))
	for _, r := range records {
		if len(r) < 3 {
			continue
		}
		var iso3 *string
		if r[2] != "" {
			v := r[2]
			iso3 = &v
		}
		copyRows = append(copyRows, []any{r[0], r[1], iso3})
	}

	_, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{"countries"},
		[]string{"country", "region", "iso3"},
		pgx.CopyFromRows(copyRows),
	)
	return err
}

// csvImportTradeFlows bulk-copies rows from a trade_flows CSV into the trade_flows table.
func csvImportTradeFlows(ctx context.Context, conn *pgx.Conn, filename string) error {
	records, err := readCSVFile(filename)
	if err != nil {
		return err
	}
	if len(records) > 0 {
		records = records[1:] // skip header
	}

	copyRows := make([][]any, 0, len(records))
	for _, r := range records {
		if len(r) < 9 {
			continue
		}

		year, err := strconv.ParseInt(r[0], 10, 16)
		if err != nil {
			continue
		}

		valueNZD, err := strconv.ParseFloat(r[8], 64)
		if err != nil {
			continue
		}

		nullableStr := func(s string) *string {
			if s == "" {
				return nil
			}
			return &s
		}

		copyRows = append(copyRows, []any{
			int16(year),
			nullableStr(r[1]), // quarter
			r[2],              // country
			nullableStr(r[3]), // region
			r[4],              // type_ie
			r[5],              // type_gs
			nullableStr(r[6]), // commodity
			nullableStr(r[7]), // hs_code
			valueNZD,          // value_nzd
		})
	}

	_, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{"trade_flows"},
		[]string{"year", "quarter", "country", "region", "type_ie", "type_gs", "commodity", "hs_code", "value_nzd"},
		pgx.CopyFromRows(copyRows),
	)
	return err
}

// readCSVFile opens filename and returns all records including the header row.
func readCSVFile(filename string) ([][]string, error) {
	f, err := os.Open(filename) //nolint:gosec // path is a hard-coded test fixture
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}
	defer f.Close()

	return csv.NewReader(f).ReadAll()
}
