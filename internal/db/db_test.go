package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/christiaanpauw/GO2shiny/internal/db"
)

// TestPoolPingFails verifies that Open returns an error quickly when
// DATABASE_URL points to an unreachable host.  No real database is required.
func TestPoolPingFails(t *testing.T) {
	// connect_timeout=2 caps the TCP dial so the test completes in seconds.
	const dsn = "postgres://nobody:nopass@invalid-host-does-not-exist.local:5432/nodb" +
		"?connect_timeout=2&sslmode=disable"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.Open(ctx, dsn)
	if err == nil {
		t.Fatal("expected an error for an unreachable host, got nil")
	}
}
