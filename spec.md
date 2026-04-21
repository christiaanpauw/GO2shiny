# Specification: NZ Trade Intelligence Dashboard (Go Rebuild)

> **Version:** 1.0  
> **Status:** Draft  
> **Last updated:** 2026-04-21

---

## 1. Purpose & Scope

Rebuild the [New Zealand Trade Intelligence Dashboard](https://gallery.shinyapps.io/nz-trade-dash) — originally authored by Wei Zhang (MBIE) in R/Shiny — as a production-ready Go web application.

The rebuilt application must:

- Reproduce every user-visible feature of the original Shiny app.
- Run as a stateless Go binary (or Docker container) with PostgreSQL as its sole external dependency.
- Replace commercial libraries (Highcharts) with open-source equivalents (ECharts).
- Be maintainable, testable, and deployable without R or Shiny Server.

---

## 2. Stakeholders

| Role | Person / Team | Concern |
|---|---|---|
| Product owner | christiaanpauw | Feature parity with original app |
| Developer | Copilot / contributors | Implementability, code quality |
| End users | Trade analysts, MBIE staff | Fast, accurate, interactive data exploration |

---

## 3. Functional Requirements

### 3.1 Dashboard (Main) Page

| ID | Requirement |
|---|---|
| F-01 | Display four KPI value-boxes: **Total Exports**, **Total Imports**, **Trade Balance**, **Year on Year change**. |
| F-02 | KPI boxes must load asynchronously via HTMX after initial page render (skeleton animation shown until data arrives). |
| F-03 | Display a **time-series line chart** of annual trade values (exports vs imports) for all goods and services. |
| F-04 | Display a **treemap chart** of the top commodity groups for the selected year and direction. |
| F-05 | All charts and KPIs must respond to the global filter state (see §3.5). |

### 3.2 Market Intelligence Page

| ID | Requirement |
|---|---|
| F-10 | Display a **country/partner selector** (multi-select, searchable) listing all trading partners. |
| F-11 | Display a per-country **time-series chart** of trade values. |
| F-12 | Display a **data table** of country-level totals with sortable columns. |
| F-13 | Display an optional **world map** showing trade flow arcs or country-level choropleth. |

### 3.3 Commodity Intelligence Page

| ID | Requirement |
|---|---|
| F-20 | Provide three sub-tabs: **Exports**, **Imports**, **HS Code**. |
| F-21 | Each sub-tab shows a **bar chart** and a **data table** of commodity breakdown. |
| F-22 | HS Code sub-tab allows drill-down by 2-digit, 4-digit, and 6-digit HS code. |

### 3.4 Data Tables (all pages)

| ID | Requirement |
|---|---|
| F-30 | All data tables must support **client-side column sorting**. |
| F-31 | All data tables must support **server-side pagination** (default 25 rows per page; configurable). |
| F-32 | All data tables must provide a **CSV download** button. |
| F-33 | Tables must support a **global search / filter** text input. |

### 3.5 Global Filters (Sidebar)

| ID | Requirement |
|---|---|
| F-40 | **Date range** selector: year from / year to (minimum 1990, maximum latest available year). |
| F-41 | **Direction** toggle: Exports / Imports / Both. |
| F-42 | **Type** selector: Goods / Services / Total. |
| F-43 | **Country** multi-select with search (available on Market Intelligence page). |
| F-44 | Filter changes must update all charts, KPIs, and tables on the active page without a full page reload (HTMX). |
| F-45 | Active filter state must be reflected in the URL (query string) so pages are shareable / bookmarkable. |

### 3.6 Navigation

| ID | Requirement |
|---|---|
| F-50 | Sidebar navigation with links to: Dashboard, Market Intelligence, Commodity Intelligence. |
| F-51 | Active page highlighted in sidebar. |
| F-52 | Responsive layout: sidebar collapses to hamburger menu on mobile. |

### 3.7 Data & API

| ID | Requirement |
|---|---|
| F-60 | All data is stored in PostgreSQL; no `.rda` or in-memory data files at runtime. |
| F-61 | A one-off CLI tool (`scripts/import_data.go`) imports CSV exports from the original `.rda` files. |
| F-62 | A JSON REST API (`/api/…`) serves chart and table data to the frontend. |
| F-63 | All API endpoints must return data within **500 ms** at the p95 for datasets up to 5 years. |

### 3.8 Health & Observability

| ID | Requirement |
|---|---|
| F-70 | `GET /health` returns `200 OK` with `{"status":"ok"}` for liveness probes. |
| F-71 | Structured JSON logs (via `log/slog`) for every request including method, path, status, duration. |

---

## 4. Non-Functional Requirements

### 4.1 Performance

| ID | Requirement |
|---|---|
| NF-01 | Initial page load (first-byte) ≤ 200 ms on a single-core VPS with ≤ 10 concurrent users. |
| NF-02 | API responses ≤ 500 ms p95 for all parameterised queries against the full dataset. |
| NF-03 | Static assets served with `Cache-Control: max-age=31536000, immutable` (content-hashed filenames). |
| NF-04 | JSON API responses compressed with GZIP when `Accept-Encoding: gzip` is present. |

### 4.2 Security

| ID | Requirement |
|---|---|
| NF-10 | No SQL injection: exclusively parameterised queries via `pgx`. |
| NF-11 | No XSS: use `html/template` exclusively; never `text/template` for user-visible output. |
| NF-12 | No secrets committed to the repository (enforced by `.gitignore` and CI scan). |
| NF-13 | HTTP security headers on every response: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`. |
| NF-14 | Rate limiting on the API tier via `chi/middleware.Throttle`. |

### 4.3 Reliability

| ID | Requirement |
|---|---|
| NF-20 | The application must recover from panics without crashing (`chi/middleware.Recoverer`). |
| NF-21 | DB connection pool with automatic reconnect; fail-fast at startup if DB is unreachable. |
| NF-22 | All HTTP handlers must return a response (even on error); no dangling connections. |

### 4.4 Maintainability

| ID | Requirement |
|---|---|
| NF-30 | Test coverage ≥ 80% statement coverage on all `internal/` packages. |
| NF-31 | `go test -race ./...` must pass cleanly (no data races). |
| NF-32 | All exported functions, types, and constants have Go doc comments. |
| NF-33 | Schema migrations managed via numbered SQL files + goose; never manual schema edits. |

### 4.5 Deployment

| ID | Requirement |
|---|---|
| NF-40 | Application packaged as a single Go binary (no CGO required). |
| NF-41 | A `Dockerfile` (multi-stage) builds and runs the binary. |
| NF-42 | All configuration via environment variables (see §6). |
| NF-43 | A `docker-compose.yml` for local development with PostgreSQL. |

---

## 5. Data Model

### 5.1 Core Tables

```sql
-- Goods & services trade flows (from dtf_shiny_full)
CREATE TABLE trade_flows (
    id          BIGSERIAL PRIMARY KEY,
    year        SMALLINT      NOT NULL,
    quarter     CHAR(2),                    -- 'Q1'..'Q4', NULL for annual rows
    country     TEXT          NOT NULL,
    region      TEXT,
    type_ie     TEXT          NOT NULL,     -- 'Exports' | 'Imports'
    type_gs     TEXT          NOT NULL,     -- 'Goods'   | 'Services'
    commodity   TEXT,
    hs_code     TEXT,
    value_nzd   NUMERIC(18,3) NOT NULL      -- NZD millions
);

CREATE INDEX idx_trade_flows_year       ON trade_flows (year);
CREATE INDEX idx_trade_flows_country    ON trade_flows (country);
CREATE INDEX idx_trade_flows_type_ie    ON trade_flows (type_ie);

-- Reference: country → region mapping
CREATE TABLE countries (
    country TEXT PRIMARY KEY,
    region  TEXT NOT NULL,
    iso3    CHAR(3)
);
```

### 5.2 Allowed Enum Values

| Column | Allowed values |
|---|---|
| `type_ie` | `Exports`, `Imports` |
| `type_gs` | `Goods`, `Services` |
| `quarter` | `Q1`, `Q2`, `Q3`, `Q4`, `NULL` |

---

## 6. API Contract

### Base URL

```
http://localhost:8080
```

### Endpoints

| Method | Path | Description | Query Params |
|---|---|---|---|
| `GET` | `/` | Redirect to `/dashboard` | — |
| `GET` | `/dashboard` | Main dashboard (full HTML) | `year_from`, `year_to`, `direction`, `type_gs` |
| `GET` | `/market` | Market Intelligence (full HTML) | `countries[]` |
| `GET` | `/commodity/exports` | Commodity – Exports | `year_from`, `year_to` |
| `GET` | `/commodity/imports` | Commodity – Imports | `year_from`, `year_to` |
| `GET` | `/commodity/hs` | Commodity – HS Code | `year_from`, `year_to`, `hs_digits` |
| `GET` | `/partials/kpis` | HTMX KPI value boxes | `year_from`, `year_to`, `direction`, `type_gs` |
| `GET` | `/partials/market-report` | HTMX country report section | `countries[]` |
| `GET` | `/api/trade/summary` | Annual totals JSON | `year_from`, `year_to`, `type_gs` |
| `GET` | `/api/trade/timeseries` | Time-series JSON | `country`, `type_ie`, `type_gs` |
| `GET` | `/api/trade/treemap` | Treemap JSON | `year`, `type_ie`, `type_gs` |
| `GET` | `/api/trade/countries` | Country totals JSON | `year_from`, `year_to`, `type_ie` |
| `GET` | `/api/trade/table` | Paginated table JSON | `page`, `size`, `q`, `year_from`, `year_to` |
| `GET` | `/api/commodity` | Commodity list JSON | `direction`, `year_from`, `year_to` |
| `GET` | `/health` | Liveness probe | — |

### Example JSON Responses

```jsonc
// GET /api/trade/summary
{
  "years": [2019, 2020, 2021, 2022, 2023],
  "exports": [62.1, 58.4, 63.2, 71.5, 74.8],
  "imports": [65.3, 54.2, 62.8, 79.1, 82.3]
}

// GET /api/trade/treemap
{
  "name": "Exports 2023",
  "children": [
    { "name": "Dairy products", "value": 18.4 },
    { "name": "Meat and edible meat offal", "value": 9.2 }
  ]
}

// GET /api/trade/table (paginated)
{
  "total": 2340,
  "page": 1,
  "size": 25,
  "rows": [
    { "year": 2023, "country": "China", "type_ie": "Exports", "type_gs": "Goods", "commodity": "Dairy", "value_nzd": 7.1 }
  ]
}

// GET /health
{ "status": "ok" }
```

---

## 7. Configuration (Environment Variables)

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | PostgreSQL connection string |
| `PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `MAX_DB_CONNS` | `10` | pgxpool MaxConns |
| `CACHE_TTL_SECONDS` | `300` | KPI/summary in-memory cache TTL |

---

## 8. Browser Support

| Browser | Minimum version |
|---|---|
| Chrome / Edge | Last 2 major versions |
| Firefox | Last 2 major versions |
| Safari | Last 2 major versions |
| Mobile Safari / Chrome Android | Last 2 major versions |

---

## 9. Acceptance Criteria

The application is considered complete when:

1. All requirements marked `F-*` (§3) and `NF-*` (§4) are met.
2. CI pipeline passes on every commit: `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`.
3. Every feature has at least one automated test (unit or integration).
4. The app can be started locally with `docker compose up` and serves the dashboard at `http://localhost:8080`.
5. A demo dataset (≥ 3 years of NZ trade data) is importable via `scripts/import_data.go`.
6. The `/health` endpoint returns `200` when the DB is reachable, `503` when it is not.

---

## 10. Out of Scope

- User authentication / authorisation (the original app is public read-only).
- Write operations on trade data via the UI.
- Real-time data feeds (data is imported in batch).
- Internationalisation / localisation.
- Social-sharing buttons (Twitter, LinkedIn) — low priority; revisit after core is stable.
