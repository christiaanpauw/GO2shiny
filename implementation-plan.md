# Implementation Plan: NZ Trade Intelligence Dashboard (Go Rebuild)

> **Version:** 1.0  
> **Status:** Active  
> **Last updated:** 2026-04-21

This document breaks the rebuild into ten sequential phases. Each phase has a clear set of tasks, an exit criterion, and a link back to the requirements in `spec.md`. Complete each phase fully before starting the next.

---

## Phase Checklist (top-level overview)

- [x] **Phase 0 – Repository & CI bootstrap**
- [x] **Phase 1 – Skeleton server**
- [x] **Phase 2 – Database & migrations**
- [x] **Phase 3 – KPI value-boxes**
- [x] **Phase 4 – Charts**
- [x] **Phase 5 – Data tables**
- [ ] **Phase 6 – Sidebar filters**
- [ ] **Phase 7 – Market Intelligence tab**
- [ ] **Phase 8 – Commodity Intelligence tab**
- [ ] **Phase 9 – Polish & UX hardening**
- [ ] **Phase 10 – Deployment packaging**

---

## Phase 0 – Repository & CI bootstrap ✅

**Goal:** Every subsequent commit is automatically vetted, built, and tested.

### Tasks

- [x] Create `spec.md` — formal specification.
- [x] Create `implementation-plan.md` (this file).
- [x] Create `go.mod` with module path `github.com/christiaanpauw/GO2shiny`.
- [x] Create `.golangci.yml` with a practical (non-pedantic) linter configuration.
- [x] Create `.github/workflows/ci.yml` — CI pipeline running on every push and pull-request.
- [x] Create `.gitignore` for Go + IDE artefacts and `.env`.
- [x] Create `.env.example` with placeholder values.

### Exit Criterion

CI pipeline runs and passes (`go build ./...`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`).

---

## Phase 1 – Skeleton server

**Goal:** A working HTTP server that can be built, run, and tested end-to-end, with no business logic yet.

### Spec references

F-50, F-51, F-70, NF-40, NF-41, NF-42

### Tasks

- [x] `go.mod` — add dependencies: `go-chi/chi/v5`, `jackc/pgx/v5`.
- [x] `cmd/server/main.go` — start Chi router on `$PORT`, wire middleware (Logger, Recoverer, Compress).
- [x] `internal/config/config.go` — read env vars (`DATABASE_URL`, `PORT`, `LOG_LEVEL`, etc.).
- [x] `web/templates/base.html` — Tabler UI layout: sidebar nav, header, content block, footer.
- [x] `web/static/css/app.css` — Tabler overrides and custom styles stub.
- [x] `web/static/js/charts.js` — ECharts initialisation helpers stub.
- [x] `internal/handlers/dashboard.go` — `GET /dashboard` renders `base.html` with empty content block.
- [x] `GET /health` handler — returns `{"status":"ok"}`.
- [x] Static file server: `GET /static/*` served from `web/static/`.
- [x] `Dockerfile` (multi-stage: `golang:1.23-alpine` → `alpine:3.20`).
- [x] `docker-compose.yml` — `app` service + `db` (postgres:16) service.
- [x] `.env.example` — placeholder values for all env vars.

### Tests

- [x] `TestHealthEndpoint` — GET `/health` returns 200 and correct JSON body.
- [x] `TestDashboardPageReturns200` — GET `/dashboard` returns 200 (no DB required; use `httptest`).
- [x] `TestStaticFilesServed` — GET `/static/css/app.css` returns 200.

### Exit Criterion

`docker compose up` starts the server, `curl http://localhost:8080/dashboard` returns `200 OK` with a valid HTML skeleton, and `curl http://localhost:8080/health` returns `{"status":"ok"}`.

---

## Phase 2 – Database & migrations

**Goal:** Schema is applied automatically at startup; trade data can be imported from CSV.

### Spec references

F-60, F-61, NF-21, NF-33

### Tasks

- [x] Add `pressly/goose/v3` dependency.
- [x] `internal/db/migrations/001_create_trade_flows.sql` — `trade_flows` table + indexes.
- [x] `internal/db/migrations/002_create_countries.sql` — `countries` reference table.
- [x] `internal/db/db.go` — `pgxpool.New`, `pool.Ping`, run embedded goose migrations at startup.
- [x] `internal/models/trade.go` — `TradeFlow`, `Country`, `CommoditySummary` structs.
- [x] `scripts/import_data/main.go` — CLI: read CSV, bulk `COPY` into `trade_flows` and `countries`.
- [x] `data/sample/` — a small synthetic CSV dataset (≥ 100 rows) for local dev and CI tests.

### Tests

- [x] `TestMigrationsApply` — applies migrations against a test DB (build tag: `integration`).
- [x] `TestImportCSV` — imports the sample dataset and verifies row counts.
- [x] `TestPoolPingFails` — pool startup fails fast when `DATABASE_URL` points to an unreachable host.

### Exit Criterion

`go run ./cmd/server` with a running PostgreSQL instance starts without error and logs `"migrations applied"` and `"database ping ok"`.

---

## Phase 3 – KPI value-boxes

**Goal:** The four KPI tiles appear on the dashboard, loaded asynchronously via HTMX.

### Spec references

F-01, F-02, NF-01, NF-02

### Tasks

- [x] `internal/db/queries_kpi.go` — SQL query: annual totals grouped by `type_ie` and `type_gs`.
- [x] `internal/handlers/dashboard.go` — `GET /partials/kpis` returns the KPI HTML partial.
- [x] `web/templates/partials/kpi_cards.html` — four Tabler stat-card tiles.
- [x] `web/templates/dashboard.html` — main content block with `hx-get="/partials/kpis"` and skeleton animation.
- [x] In-memory cache for KPI query results (TTL from `$CACHE_TTL_SECONDS`).

### Tests

- [x] `TestKPIEndpoint` — GET `/partials/kpis` with valid params returns 200 + correct HTML structure.
- [x] `TestKPIEndpointInvalidParams` — missing/invalid year returns 400.
- [x] `TestKPICache` — second call within TTL does not hit the DB.

### Exit Criterion

Loading `/dashboard` in a browser shows four animated skeleton cards that resolve to actual NZD values once HTMX fires.

---

## Phase 4 – Charts

**Goal:** Time-series and treemap charts are rendered by ECharts using JSON data from the API.

### Spec references

F-03, F-04, F-05, NF-02, NF-04

### Tasks

- [x] `internal/db/queries_chart.go` — queries for time-series and treemap data.
- [x] `internal/handlers/api.go` — `GET /api/trade/summary`, `GET /api/trade/timeseries`, `GET /api/trade/treemap`.
- [x] `web/static/js/charts.js` — ECharts initialisation: `initTimeSeries(divId, apiUrl)`, `initTreemap(divId, apiUrl)`.
- [x] `web/templates/dashboard.html` — add two chart container `<div>`s wired to `charts.js`.
- [x] GZIP middleware enabled for JSON API responses.

### Tests

- [x] `TestSummaryAPI` — returns correct JSON schema and numeric values.
- [x] `TestTimeSeriesAPI` — filters by `year_from`/`year_to` correctly.
- [x] `TestTreemapAPI` — returns hierarchical `name`/`children`/`value` structure.
- [x] `TestAPIGZIP` — `Accept-Encoding: gzip` response is compressed.

### Exit Criterion

`/dashboard` shows a populated time-series chart and a treemap with real data from the database.

---

## Phase 5 – Data tables

**Goal:** A paginated, sortable, downloadable data table is available on the dashboard.

### Spec references

F-30, F-31, F-32, F-33, NF-02

### Tasks

- [x] `internal/db/queries_table.go` — paginated query with optional full-text search.
- [x] `internal/handlers/api.go` — `GET /api/trade/table` with `page`, `size`, `q` params.
- [x] `web/templates/partials/table_block.html` — Tabulator.js container div.
- [x] `web/static/js/charts.js` (or `tables.js`) — Tabulator initialisation with remote pagination and CSV download.
- [x] Max page size enforced server-side (NF-12 / input validation).

### Tests

- [x] `TestTableAPIDefaults` — default pagination (page=1, size=25) returns correct JSON.
- [x] `TestTableAPISearch` — `?q=dairy` filters results correctly.
- [x] `TestTableAPIMaxPageSize` — requesting `size=9999` is capped at 100.
- [x] `TestTableAPIInvalidPage` — `?page=abc` returns 400.

### Exit Criterion

The dashboard table shows paginated rows, supports column sorting and text search, and the "Download CSV" button produces a valid file.

---

## Phase 6 – Sidebar filters

**Goal:** Changing the sidebar filter controls updates all charts, KPIs, and tables on the page without a full reload.

### Spec references

F-40–F-45, NF-01

### Tasks

- [ ] `web/templates/base.html` — sidebar filter form: year range, direction toggle, type selector.
- [ ] Alpine.js wires filter state to a hidden form; HTMX `hx-include` sends params on every request.
- [ ] URL query string updated on filter change (`history.pushState`) for shareability (F-45).
- [ ] All existing partial/API endpoints validated to honour new filter params.
- [ ] Allow-lists enforced for `type_ie`, `type_gs`, `region` (NF-10 / input validation).

### Tests

- [ ] `TestFilterTypeIEAllowList` — invalid `type_ie` value returns 400.
- [ ] `TestFilterYearRange` — `year_from` > `year_to` returns 400.
- [ ] `TestFilterUpdatesKPIs` — KPI partial reflects different totals for different `year_from`/`year_to`.

### Exit Criterion

Changing any sidebar filter visually updates all components on the page within 500 ms. Refreshing the browser with the current URL restores the same filter state.

---

## Phase 7 – Market Intelligence tab

**Goal:** The Market Intelligence page shows per-country analysis with a world map and data table.

### Spec references

F-10–F-13

### Tasks

- [ ] `internal/db/queries_market.go` — country-level aggregation query.
- [ ] `internal/handlers/market.go` — `GET /market`, `GET /partials/market-report`, `GET /api/trade/countries`.
- [ ] `web/templates/market.html` — country selector, time-series chart, data table.
- [ ] `web/templates/partials/market_report.html` — HTMX partial for country report.
- [ ] ECharts geo/map or scatter-map for world map (F-13).
- [ ] Sidebar country multi-select populated from `GET /api/trade/countries`.

### Tests

- [ ] `TestMarketPageRenders` — GET `/market` returns 200.
- [ ] `TestCountriesAPI` — returns valid list of country objects with totals.
- [ ] `TestMarketReportPartial` — `?countries[]=China` returns correct HTML.

### Exit Criterion

`/market` page loads, country multi-select is populated, selecting a country updates the chart and table, and the world map highlights the selected country.

---

## Phase 8 – Commodity Intelligence tab

**Goal:** The Commodity Intelligence page provides exports, imports, and HS code drill-down sub-tabs.

### Spec references

F-20–F-22

### Tasks

- [ ] `internal/db/queries_commodity.go` — commodity and HS code aggregation queries.
- [ ] `internal/handlers/commodity.go` — `GET /commodity/{direction}`, `GET /api/commodity`.
- [ ] `web/templates/commodity.html` — three sub-tabs with bar chart + table each.
- [ ] HS code drill-down: `?hs_digits=2|4|6` query param; validated server-side.

### Tests

- [ ] `TestCommodityExportsPage` — GET `/commodity/exports` returns 200.
- [ ] `TestCommodityAPI` — returns sorted list of commodities with NZD values.
- [ ] `TestHSCodeDrillDown` — `?hs_digits=4` returns 4-digit codes only.

### Exit Criterion

All three sub-tabs render data; the HS Code tab allows drilling from 2-digit to 6-digit breakdowns.

---

## Phase 9 – Polish & UX hardening

**Goal:** The application is production-ready from a UX and reliability perspective.

### Spec references

F-52, NF-01–NF-04, NF-13, NF-14

### Tasks

- [ ] Responsive sidebar (hamburger menu on mobile) via Tabler's built-in JS.
- [ ] Loading skeletons (CSS) shown until each HTMX partial resolves.
- [ ] Error states: if an API call fails, display a user-friendly message (not a blank div).
- [ ] HTTP security headers middleware (NF-13).
- [ ] Rate limiting: `chi/middleware.Throttle` for the API tier (NF-14).
- [ ] `govulncheck ./...` added to CI pipeline.
- [ ] Accessibility: ARIA labels on chart containers, keyboard-navigable sidebar.
- [ ] Browser compatibility smoke-test (manual, Chrome + Firefox + Safari).

### Tests

- [ ] `TestSecurityHeaders` — all required headers present on every response.
- [ ] `TestRateLimitExceeded` — > threshold requests in 1 s returns 429.

### Exit Criterion

All functional requirements F-01–F-71 verified. Lighthouse accessibility score ≥ 80. CI passes including `govulncheck`.

---

## Phase 10 – Deployment packaging

**Goal:** The application can be deployed to any Docker-compatible host with a single `docker compose up`.

### Spec references

NF-40–NF-43

### Tasks

- [ ] `Dockerfile` finalised: multi-stage, non-root user, minimal image.
- [ ] `docker-compose.yml`: `app` + `db` + `migrate` (one-shot migration container).
- [ ] CI: add `docker build` step to verify image builds on every PR.
- [ ] `README.md`: comprehensive setup instructions, environment variable reference, data import steps.
- [ ] Tag `v1.0.0` release once all phases are complete.

### Tests

- [ ] `TestDockerImageBuilds` (CI step) — `docker build .` exits 0.
- [ ] `TestHealthAfterCompose` — `docker compose up -d && curl /health` returns 200.

### Exit Criterion

A fresh `git clone` + `docker compose up` produces a fully working dashboard at `http://localhost:8080` with no manual steps other than providing a `.env` file from `.env.example`.

---

## Conventions

### Definition of Done (per phase)

A phase is **done** when:

1. All tasks are checked off.
2. All tests for that phase pass (`go test -race ./...`).
3. CI pipeline is green.
4. The exit criterion has been manually verified.
5. The checkbox in the top-level phase checklist above has been ticked.

### Branching

- Each phase is developed on a branch: `feature/phase-<N>-<short-name>`.
- PRs must pass CI before merging to `main`.
- Do not start Phase N+1 until Phase N is merged.

### Updating this Plan

If requirements change, update both `spec.md` and this file in the same commit. Preserve completed checkboxes.
