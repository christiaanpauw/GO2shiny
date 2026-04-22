// Package db provides database connectivity and migration utilities.
package db

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open creates a new pgxpool connection pool, pings the database, and applies
// any pending goose migrations before returning the pool.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}

	if err := runMigrations(pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, nil
}

// runMigrations applies all pending goose migrations embedded in the binary.
func runMigrations(pool *pgxpool.Pool) error {
	goose.SetBaseFS(migrations)

	db := stdlib.OpenDBFromPool(pool)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}
