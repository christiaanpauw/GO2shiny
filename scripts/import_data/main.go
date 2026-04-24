// Command import_data reads trade flow and country CSV files and bulk-copies
// them into the PostgreSQL database using the pgx COPY protocol.
//
// Usage:
//
//	go run ./scripts/import_data \
//	    -trade  data/sample/trade_flows.csv \
//	    -countries data/sample/countries.csv
//
// The DATABASE_URL environment variable must be set (or passed via -db flag).
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	tradeFile := flag.String("trade", "data/sample/trade_flows.csv", "path to trade_flows CSV")
	countriesFile := flag.String("countries", "data/sample/countries.csv", "path to countries CSV")
	flag.Parse()

	if *dbURL == "" {
		slog.Error("DATABASE_URL must be set (or pass -db flag)")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		slog.Error("create pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping database", "err", err)
		os.Exit(1)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		slog.Error("acquire connection", "err", err)
		os.Exit(1)
	}
	defer conn.Release()

	if err := importCountries(ctx, conn.Conn(), *countriesFile); err != nil {
		slog.Error("import countries", "err", err)
		os.Exit(1)
	}

	if err := importTradeFlows(ctx, conn.Conn(), *tradeFile); err != nil {
		slog.Error("import trade flows", "err", err)
		os.Exit(1)
	}

	slog.Info("import complete")
}

// importCountries bulk-copies rows from countriesFile into the countries table.
// The operation is idempotent: rows whose primary key already exists are skipped.
func importCountries(ctx context.Context, conn *pgx.Conn, filename string) error {
	rows, err := readCSV(filename)
	if err != nil {
		return err
	}

	// Skip header row.
	if len(rows) > 0 {
		rows = rows[1:]
	}

	copyRows := make([][]any, 0, len(rows))
	for _, r := range rows {
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

	// Copy into a temporary staging table, then upsert into the real table so
	// that re-runs are idempotent (duplicate countries are silently skipped).
	if _, err = conn.Exec(ctx, `CREATE TEMP TABLE tmp_countries (LIKE countries)`); err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS tmp_countries`) //nolint:errcheck

	n, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"tmp_countries"},
		[]string{"country", "region", "iso3"},
		pgx.CopyFromRows(copyRows),
	)
	if err != nil {
		return fmt.Errorf("COPY tmp_countries: %w", err)
	}

	if _, err = conn.Exec(ctx, `INSERT INTO countries SELECT * FROM tmp_countries ON CONFLICT (country) DO NOTHING`); err != nil {
		return fmt.Errorf("insert countries: %w", err)
	}

	slog.Info("imported countries", "rows", n)
	return nil
}

// importTradeFlows bulk-copies rows from filename into the trade_flows table.
// The table is truncated first so that re-runs remain idempotent.
func importTradeFlows(ctx context.Context, conn *pgx.Conn, filename string) error {
	rows, err := readCSV(filename)
	if err != nil {
		return err
	}

	// Skip header row.
	if len(rows) > 0 {
		rows = rows[1:]
	}

	// Truncate to avoid duplicate rows when the importer is re-run.
	if _, err = conn.Exec(ctx, `TRUNCATE trade_flows`); err != nil {
		return fmt.Errorf("truncate trade_flows: %w", err)
	}

	copyRows := make([][]any, 0, len(rows))
	for i, r := range rows {
		if len(r) < 9 {
			slog.Warn("skipping malformed row", "line", i+2, "cols", len(r))
			continue
		}

		year, err := strconv.ParseInt(r[0], 10, 16)
		if err != nil {
			slog.Warn("invalid year", "line", i+2, "value", r[0])
			continue
		}

		valueNZD, err := strconv.ParseFloat(r[8], 64)
		if err != nil {
			slog.Warn("invalid value_nzd", "line", i+2, "value", r[8])
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

	n, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"trade_flows"},
		[]string{"year", "quarter", "country", "region", "type_ie", "type_gs", "commodity", "hs_code", "value_nzd"},
		pgx.CopyFromRows(copyRows),
	)
	if err != nil {
		return fmt.Errorf("COPY trade_flows: %w", err)
	}

	slog.Info("imported trade flows", "rows", n)
	return nil
}

// readCSV opens filename and returns all records including the header row.
func readCSV(filename string) ([][]string, error) {
	f, err := os.Open(filename) //nolint:gosec // file path comes from CLI flag
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}
	defer f.Close()

	return csv.NewReader(f).ReadAll()
}
