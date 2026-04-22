//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/christiaanpauw/GO2shiny/internal/db"
)

// TestMigrationsApply verifies that goose migrations apply cleanly against a
// real PostgreSQL instance.  Run with:
//
//	go test -tags=integration ./internal/db/...
func TestMigrationsApply(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()
}
