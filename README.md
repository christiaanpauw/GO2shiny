# GO2shiny

A Go rebuild of the [New Zealand Trade Intelligence Dashboard](https://gallery.shinyapps.io/nz-trade-dash), originally authored in R/Shiny. The application reproduces all user-visible features of the original dashboard as a stateless Go binary backed by PostgreSQL.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Quick Start with Docker Compose](#quick-start-with-docker-compose)
- [Local Development Setup](#local-development-setup)
- [Environment Variables](#environment-variables)
- [Database Migrations](#database-migrations)
- [Loading Sample Data](#loading-sample-data)
- [Project Structure](#project-structure)
- [HTTP Routes](#http-routes)
- [Running Tests](#running-tests)
- [Linting](#linting)

---

## Overview

| Aspect | Original (Shiny) | This Repo (Go) |
|---|---|---|
| Execution model | R process per user session | Stateless HTTP; concurrent via goroutines |
| State management | Server-side reactive graph | URL params + HTMX |
| Charting | Highcharts (commercial) | ECharts (Apache 2.0) |
| Tables | DataTables via DT package | Tabulator.js |
| Data storage | `.rda` binary files | PostgreSQL (queryable, updatable) |
| Deployment | shinyapps.io / Shiny Server | Single Go binary or Docker container |

---

## Prerequisites

| Tool | Minimum version |
|---|---|
| [Go](https://go.dev/dl/) | 1.23 |
| [PostgreSQL](https://www.postgresql.org/) | 16 |
| [Docker + Docker Compose](https://docs.docker.com/get-docker/) | (optional, for the quick-start path) |

---

## Quick Start with Docker Compose

This is the fastest way to run the full stack locally.

```bash
# 1. Copy the example environment file
cp .env.example .env

# 2. Start PostgreSQL and the application server
docker compose up --build
```

The server will be available at <http://localhost:8080>.

Docker Compose automatically:
- Starts a PostgreSQL 16 container with a `go2shiny` database.
- Waits for the database to be healthy before starting the app.
- Injects `DATABASE_URL` so the app connects to the Compose-managed database.

> **Note:** The `.env` file is loaded by Docker Compose for any additional overrides (e.g. `LOG_LEVEL`). The `DATABASE_URL` inside Compose is always overridden to point at the `db` service.

---

## Local Development Setup

### 1. Clone and install dependencies

```bash
git clone https://github.com/christiaanpauw/GO2shiny.git
cd GO2shiny
go mod download
```

### 2. Configure environment variables

```bash
cp .env.example .env
# Edit .env and set DATABASE_URL to point at your local PostgreSQL instance.
```

### 3. Run database migrations

Migrations are embedded in the binary and run automatically at startup **when using the application server**. To run them manually against a standalone PostgreSQL instance, use [goose](https://github.com/pressly/goose):

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest

goose -dir internal/db/migrations postgres "$DATABASE_URL" up
```

### 4. Load sample data (optional)

```bash
go run ./scripts/import_data \
    -trade    data/sample/trade_flows.csv \
    -countries data/sample/countries.csv
```

The `-db` flag or the `DATABASE_URL` environment variable must point at a running, migrated database.

### 5. Run the server

```bash
go run ./cmd/server
```

The server starts on port `8080` by default (override with the `PORT` environment variable). Open <http://localhost:8080> in your browser.

---

## Environment Variables

All configuration is provided through environment variables (or an `.env` file when using Docker Compose). Copy `.env.example` to `.env` and adjust as needed.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(none)_ | PostgreSQL connection string. If unset, KPI endpoints return `503`. |
| `PORT` | `8080` | HTTP port the server listens on. |
| `LOG_LEVEL` | `info` | Structured log level: `debug`, `info`, `warn`, or `error`. |
| `MAX_DB_CONNS` | `10` | Maximum connections in the pgxpool connection pool. |
| `CACHE_TTL_SECONDS` | `300` | In-memory cache TTL (seconds) for KPI/summary queries. |

---

## Database Migrations

SQL migration files live in `internal/db/migrations/` and are managed with [goose](https://github.com/pressly/goose).

| File | Description |
|---|---|
| `001_create_trade_flows.sql` | Creates the `trade_flows` table and indexes. |
| `002_create_countries.sql` | Creates the `countries` reference table. |

Run all pending migrations:

```bash
goose -dir internal/db/migrations postgres "$DATABASE_URL" up
```

Roll back the most recent migration:

```bash
goose -dir internal/db/migrations postgres "$DATABASE_URL" down
```

---

## Loading Sample Data

The `scripts/import_data` command bulk-copies CSV data into the database using the pgx `COPY` protocol.

```bash
# Using the DATABASE_URL environment variable
go run ./scripts/import_data \
    -trade     data/sample/trade_flows.csv \
    -countries data/sample/countries.csv

# Or specify the connection string directly
go run ./scripts/import_data \
    -db "postgres://user:password@localhost:5432/go2shiny?sslmode=disable" \
    -trade     data/sample/trade_flows.csv \
    -countries data/sample/countries.csv
```

---

## Project Structure

```
GO2shiny/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: wires router, DB pool, config
├── data/
│   └── sample/                  # Sample CSV files for local development
├── internal/
│   ├── config/
│   │   └── config.go            # Reads environment variables; applies defaults
│   ├── db/
│   │   ├── db.go                # pgxpool setup and helper query functions
│   │   ├── migrations/          # Goose SQL migration files
│   │   └── queries_kpi.go       # KPI / summary queries
│   ├── handlers/
│   │   ├── dashboard.go         # Dashboard page handler
│   │   ├── health.go            # Health-check endpoint
│   │   └── kpi*.go              # KPI partial handler
│   └── models/                  # Shared data model types
├── scripts/
│   └── import_data/
│       └── main.go              # CSV → PostgreSQL bulk-import tool
├── web/
│   ├── embed.go                 # Embeds templates/ and static/ into the binary
│   ├── static/                  # CSS, JS, and other static assets
│   └── templates/               # HTML templates (base layout + partials)
├── .env.example                 # Example environment variable file
├── .golangci.yml                # golangci-lint configuration
├── docker-compose.yml           # Docker Compose stack (app + PostgreSQL)
├── Dockerfile                   # Multi-stage Docker build
└── go.mod / go.sum              # Go module files
```

---

## HTTP Routes

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Redirects to `/dashboard` |
| `GET` | `/dashboard` | Main trade intelligence dashboard |
| `GET` | `/partials/kpis` | HTMX partial returning KPI value boxes |
| `GET` | `/health` | Health check — returns `200 OK` when the server is running |
| `GET` | `/static/*` | Embedded static assets (CSS, JS, images) |

---

## Running Tests

```bash
# Run all unit tests
go test ./...

# Run with verbose output
go test -v ./...

# Run integration tests (requires a running PostgreSQL instance)
DATABASE_URL="postgres://user:password@localhost:5432/go2shiny?sslmode=disable" \
    go test -tags integration ./...
```

---

## Linting

The project uses [golangci-lint](https://golangci-lint.run/) with the configuration in `.golangci.yml`.

```bash
# Install golangci-lint (if not already installed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run all configured linters
golangci-lint run ./...
```
