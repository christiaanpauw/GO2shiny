# Approach: Recreating the NZ Trade Intelligence Dashboard in Go

## 1. Original Dashboard Analysis

### What the Shiny App Does

The [New Zealand Trade Intelligence Dashboard](https://gallery.shinyapps.io/nz-trade-dash) (originally built by Wei Zhang for MBIE using R/Shiny) provides interactive visualisation of New Zealand's international trade data. Key features:

| Feature | Description |
|---|---|
| **Value Boxes** | KPI tiles showing total goods/services exports, imports, and trade balance |
| **Main Dashboard** | High-level overview charts — time-series trends, treemaps of commodity breakdown |
| **Market Intelligence tab** | Per-country/region analysis with searchable multi-select filter |
| **Commodity Intelligence tab** | Exports/Imports drilled down by commodity or HS code |
| **Interactive charts** | Highcharter (Highcharts) time-series, bar charts, treemaps |
| **Data tables** | DT (DataTables) with sorting, filtering, pagination, CSV download |
| **World map** | Trade-flow arc map drawn with geosphere |
| **Social sharing** | Header buttons for Twitter, LinkedIn, etc. |
| **Progress indicator** | Shiny progress bar on first load (25 steps) |
| **Data source** | Pre-loaded `.rda` binary files sourced from Stats NZ + UN Comtrade |

### Shiny-specific Patterns to Replace

| Shiny Pattern | Go Equivalent |
|---|---|
| Reactive inputs (`input$*`) | URL query params / HTMX request params |
| `renderPlot` / `renderHighchart` | JSON API endpoint → ECharts/Chart.js on client |
| `renderDataTable` | JSON API endpoint → Tabulator.js on client |
| `conditionalPanel` | CSS `hidden` toggle driven by HTMX or Alpine.js |
| `withProgress` | CSS skeleton/spinner shown until first data fetch completes |
| `renderValueBox` | Server-side HTML partial returned by HTMX `hx-get` |
| `.rda` data files | PostgreSQL tables (imported once; queried at runtime) |

---

## 2. Technology Decisions

### Backend — Go + Chi

**Decision:** Use [Go](https://go.dev) with the [Chi](https://github.com/go-chi/chi) router.

**Rationale:**
- Chi is lightweight (~1 300 lines), idiomatic, and fully compatible with `net/http` handlers — no lock-in.
- Middleware composability (logger, recoverer, GZIP, CORS, rate-limit) matches Shiny's server-side session model.
- Go's compiled binary deploys as a single executable — no runtime dependencies.
- Go's `html/template` gives XSS-safe server-side rendering without a heavy framework.

### Database — PostgreSQL + pgx

**Decision:** Store all trade data in PostgreSQL, accessed via [pgx v5](https://github.com/jackc/pgx).

**Rationale:**
- The `.rda` data files contain tabular numeric data — a natural fit for relational tables.
- PostgreSQL supports time-series aggregations, window functions, and `COPY FROM` bulk imports natively.
- `pgxpool` provides connection pooling out of the box.
- pgx v5 is the current community standard; it avoids the `database/sql` reflection overhead.

Proposed schema (simplified):

```sql
-- Goods & services trade (from dtf_shiny_full)
CREATE TABLE trade_flows (
    id          BIGSERIAL PRIMARY KEY,
    year        SMALLINT    NOT NULL,
    quarter     CHAR(2),                  -- 'Q1'..'Q4', NULL for annual
    country     TEXT        NOT NULL,
    region      TEXT,
    type_ie     TEXT        NOT NULL,     -- 'Exports' | 'Imports'
    type_gs     TEXT        NOT NULL,     -- 'Goods'   | 'Services'
    commodity   TEXT,
    hs_code     TEXT,
    value_nzd   NUMERIC(18,3) NOT NULL    -- NZD millions
);

-- Reference: country → region mapping
CREATE TABLE countries (
    country TEXT PRIMARY KEY,
    region  TEXT NOT NULL,
    iso3    CHAR(3)
);
```

Data import: a one-off Go CLI command reads the original CSV exports from the `.rda` files and `COPY`s them into PostgreSQL.

### Frontend — HTMX + ECharts + Tabulator.js + Alpine.js

| Library | Version | Purpose |
|---|---|---|
| [HTMX](https://htmx.org) | 2.x | Partial HTML swaps (tab switching, filter changes) without a JS framework |
| [ECharts](https://echarts.apache.org) | 5.x | Time-series, bar, treemap, geo/map charts — Canvas-based, very fast |
| [Tabulator.js](https://tabulator.info) | 6.x | Sortable/filterable/paginated data tables with CSV export |
| [Alpine.js](https://alpinejs.dev) | 3.x | Minimal reactive state for dropdowns, toggle panels, modal visibility |
| [Tabler UI](https://tabler.io) | 1.x | Open-source Bootstrap 5 admin template — stat cards, sidebar, nav |

**Why not React/Vue/Svelte?** The original app is server-driven. HTMX + Alpine.js achieves the same feel with far less JavaScript, keeping the architectural model close to the Shiny original and making Go templating the single source of truth.

**Why ECharts over Highcharts?** The original uses Highcharts (commercial licence). ECharts is open-source, similarly feature-rich, and has native treemap and geo-map support.

---

## 3. Project Structure

```
GO2shiny/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: wires router, DB pool, config
├── internal/
│   ├── config/
│   │   └── config.go            # Reads env vars (DB_URL, PORT, etc.)
│   ├── db/
│   │   ├── db.go                # pgxpool setup, helper query functions
│   │   └── migrations/          # .sql migration files (goose or plain SQL)
│   ├── handlers/
│   │   ├── dashboard.go         # Main dashboard page + value-box endpoints
│   │   ├── market.go            # Market Intelligence tab
│   │   ├── commodity.go         # Commodity Intelligence tab
│   │   └── api.go               # JSON API endpoints for charts & tables
│   ├── models/
│   │   └── trade.go             # Structs: TradeFlow, Country, CommoditySummary
│   └── templates/
│       └── renderer.go          # Template cache + ExecuteTemplate helper
├── web/
│   ├── static/
│   │   ├── css/
│   │   │   └── app.css          # Tabler overrides + custom styles
│   │   └── js/
│   │       └── charts.js        # ECharts initialisation helpers
│   └── templates/
│       ├── base.html            # Layout: header, sidebar, footer, script tags
│       ├── dashboard.html       # Main dashboard content block
│       ├── market.html          # Market Intelligence content block
│       ├── commodity.html       # Commodity Intelligence content block
│       └── partials/
│           ├── kpi_cards.html   # Value-box KPI tiles
│           ├── chart_block.html # Generic chart container partial
│           └── table_block.html # Tabulator table container partial
├── scripts/
│   └── import_data.go           # One-off data import CLI (CSV → PostgreSQL)
├── approach.md
├── agent.md
├── go.mod
├── go.sum
└── .env.example
```

---

## 4. Page & Route Design

```
GET  /                        → redirect → /dashboard
GET  /dashboard               → Main Dashboard page (full HTML)
GET  /market                  → Market Intelligence page
GET  /commodity/exports       → Commodity Intelligence – Exports
GET  /commodity/imports       → Commodity Intelligence – Imports
GET  /commodity/hs            → Commodity Intelligence – HS Code

# HTMX partial endpoints (return HTML fragments)
GET  /partials/kpis           → KPI value boxes (hx-trigger="load")
GET  /partials/market-report  → Country report section

# JSON API (consumed by ECharts / Tabulator)
GET  /api/trade/summary       → Annual totals by type_ie, type_gs
GET  /api/trade/timeseries    → Time-series for chart; ?country=&type=
GET  /api/trade/treemap       → Commodity breakdown for treemap
GET  /api/trade/countries     → Country-level totals (for map + table)
GET  /api/trade/table         → Paginated table data; ?page=&size=&q=
GET  /api/commodity           → Commodity list with values; ?direction=exports
```

---

## 5. Rendering Strategy

```
Browser                 Go/Chi Server              PostgreSQL
   │                        │                          │
   │── GET /dashboard ──────▶│                          │
   │                        │── SELECT kpi data ───────▶│
   │                        │◀─ rows ───────────────────│
   │◀── full HTML page ──────│                          │
   │                        │                          │
   │  (ECharts divs empty)  │                          │
   │                        │                          │
   │── hx-get /partials/kpis▶│                          │
   │◀── KPI HTML fragment ───│                          │
   │                        │                          │
   │── fetch /api/trade/ts ─▶│                          │
   │                        │── SELECT time-series ────▶│
   │                        │◀─ rows ───────────────────│
   │◀── JSON ───────────────│                          │
   │  charts.js renders     │                          │
   │  ECharts with JSON     │                          │
```

**First load** shows skeleton cards (CSS animation) while HTMX fires `hx-trigger="load"` requests in parallel. This replaces Shiny's `withProgress` spinner.

---

## 6. Filter Interaction Pattern

The sidebar filters (country multi-select, commodity select, date range) are driven by:

1. **Alpine.js** maintains local state for selected values.
2. On change, Alpine updates a hidden form that HTMX watches (`hx-include`).
3. HTMX sends `hx-get` to the relevant partial or API endpoint with filter params appended.
4. Go handler reads params from `r.URL.Query()`, builds parameterised SQL, returns updated fragment.

This mirrors the Shiny `reactive()` / `observe()` pattern without a WebSocket.

---

## 7. Data Import Plan

The original app ships pre-computed `.rda` binary files. The migration path:

1. **Export step (R):** Run a small R script (`scripts/export_rda_to_csv.R`) that loads each `.rda` and writes a UTF-8 CSV.
2. **Import step (Go CLI):** `go run ./scripts/import_data.go` reads the CSVs and uses `pgx COPY` for bulk insert.
3. **Schema migration:** Use [goose](https://github.com/pressly/goose) or plain `.sql` files in `internal/db/migrations/`.

---

## 8. Security & Operational Considerations

| Concern | Approach |
|---|---|
| SQL injection | Exclusively use parameterised queries (`$1`, `$2`, …) via pgx |
| XSS | Use `html/template` (not `text/template`) throughout |
| CSRF | Not required for a read-only dashboard; add `gorilla/csrf` if write forms are added |
| Environment secrets | Never hard-code; read from env vars or a `.env` file (excluded from git) |
| Rate limiting | `chi/middleware.Throttle` or `golang.org/x/time/rate` |
| HTTPS | Terminate at a reverse proxy (Caddy/nginx); Go binary listens on HTTP internally |
| GZIP | `chi/middleware.Compress` — important for large JSON API payloads |

---

## 9. Development Phases

| Phase | Deliverable |
|---|---|
| **1 – Skeleton** | `go.mod`, Chi router, static file server, base HTML template, Tabler UI wired |
| **2 – Database** | Schema migrations, pgxpool setup, data import CLI |
| **3 – KPIs** | Value-box API endpoint + HTMX partial |
| **4 – Charts** | Time-series and treemap ECharts connected to JSON API |
| **5 – Tables** | Tabulator.js table with server-side pagination + CSV export |
| **6 – Filters** | Sidebar filters (country, commodity, date) wired end-to-end |
| **7 – Market tab** | Country/region intelligence page |
| **8 – Commodity tab** | Exports / Imports / HS code sub-tabs |
| **9 – Polish** | Responsive layout, loading skeletons, error states, social share links |
| **10 – Deployment** | Dockerfile, environment config, health-check endpoint |

---

## 10. Key Differences from the Shiny Original

| Aspect | Shiny | Go / Chi |
|---|---|---|
| Execution model | R process per user session (WebSocket) | Stateless HTTP; concurrent via goroutines |
| State management | Server-side reactive graph | URL params + client-side Alpine.js |
| Charting | Highcharts (commercial) | ECharts (open-source, Apache 2.0) |
| Tables | DataTables via DT package | Tabulator.js directly |
| Data storage | `.rda` binary files loaded at startup | PostgreSQL — queryable, updatable |
| Deployment | shinyapps.io / Shiny Server | Single Go binary or Docker container |
| Scalability | One R process per session | Thousands of concurrent requests |
| Licence | GPL (R packages) + Highcharts (commercial) | MIT / Apache — fully open |
