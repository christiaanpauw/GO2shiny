# Agent Guidelines: Go Development Best Practices

These guidelines apply to every coding session on this project. They are the standing rules that the development agent (and all contributors) must follow throughout the entire development lifecycle.

---

## 1. Project Conventions

### Module & Package Layout

- Follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout) convention:
  - `cmd/<name>/main.go` — binary entry points only; no business logic.
  - `internal/` — all application packages (not importable by external modules).
  - `web/` — HTML templates, static assets (CSS, JS, images).
  - `scripts/` — one-off CLI utilities (data import, migrations).
- Package names must be **short, lowercase, single-word** — no underscores, no camelCase.
- Avoid `util`, `common`, `helpers` package names; name by domain instead (`trade`, `commodity`, `auth`).

### Naming

| Thing | Convention | Example |
|---|---|---|
| Exported types | PascalCase | `TradeFlow`, `CommoditySummary` |
| Unexported | camelCase | `buildQuery`, `rowsToJSON` |
| Constants | PascalCase or ALL_CAPS for env keys | `MaxPageSize`, `DB_URL` |
| Files | snake_case | `trade_flow.go`, `db_test.go` |
| Test files | `<file>_test.go` same package | `handlers_test.go` |
| SQL identifiers | snake_case | `trade_flows`, `type_ie` |

---

## 2. Code Quality

### Formatting & Linting

- **Always** run `gofmt` (or `goimports`) before committing — CI must pass.
- Use `go vet ./...` as the minimum lint step.
- Configure [golangci-lint](https://golangci-lint.run/) with at minimum:
  - `errcheck` — never silently discard errors.
  - `staticcheck` — catches subtle bugs.
  - `govet` — alignment and correctness checks.
  - `revive` — style and idiomatic Go.
  - `gosec` — security issues.

### Error Handling

- **Never** ignore errors. Always check and handle or explicitly propagate.
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)` — use `%w` not `%v` so callers can `errors.Is`/`errors.As`.
- Return errors to the caller; log only at the boundary (handler or main).
- HTTP handlers must write a response even on error — use a helper:

  ```go
  func respondError(w http.ResponseWriter, code int, err error) {
      log.Printf("error: %v", err)
      http.Error(w, http.StatusText(code), code)
  }
  ```

### No Panics in Production Code

- Never use `panic` except in `init()` or test helpers where the invariant truly cannot be violated at runtime.
- Use `chi/middleware.Recoverer` as the outermost middleware to catch any unexpected panics.

---

## 3. HTTP Handlers

### Handler Signature

All handlers must use the standard `http.HandlerFunc` signature:

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

- Inject dependencies (DB pool, template cache, config) via a handler struct — never use global variables.
- Use `chi.URLParam(r, "id")` for path parameters, `r.URL.Query().Get("key")` for query parameters.

### Request Validation

- Always validate and sanitise query parameters before passing to SQL.
- Use allow-lists for `type_ie`, `type_gs`, `region` etc. — reject unknown values with `400 Bad Request`.
- Parse integers with `strconv.Atoi` and check errors; never cast blindly.

### Response Content-Type

- JSON responses: `w.Header().Set("Content-Type", "application/json")`.
- HTML partials: `w.Header().Set("Content-Type", "text/html; charset=utf-8")`.
- Always set the header **before** writing the body.

---

## 4. Database (PostgreSQL / pgx)

### Connection

- Use `pgxpool.New` — never `pgx.Connect` directly in request handlers (connection-per-request is expensive).
- Read `DATABASE_URL` from the environment; never hard-code credentials.
- Call `pool.Ping(ctx)` at startup and fail fast if the database is unreachable.

### Queries

- **Always use parameterised queries** — no string concatenation or `fmt.Sprintf` in SQL:

  ```go
  // CORRECT
  rows, err := pool.Query(ctx, "SELECT value_nzd FROM trade_flows WHERE country = $1", country)

  // WRONG — SQL injection risk
  rows, err := pool.Query(ctx, "SELECT value_nzd FROM trade_flows WHERE country = '"+country+"'")
  ```

- Use `pgx/v5` (not v4) for new code — it has a cleaner API and better performance.
- Always `defer rows.Close()` immediately after checking the `Query` error.
- Use `pgx` named structs with `pgx.RowToStructByName` or scan manually; avoid `database/sql`.

### Migrations

- Manage schema changes with numbered SQL files in `internal/db/migrations/`.
- Use [goose](https://github.com/pressly/goose) or apply migrations at startup via embedded SQL — never alter the schema manually in production.
- Migrations must be idempotent where possible (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`).

### Contexts

- Every database call must receive a `context.Context` derived from the request: `r.Context()`.
- Use `context.WithTimeout` for long-running queries to avoid runaway requests.

---

## 5. HTML Templates

- Use `html/template` (not `text/template`) — it auto-escapes content and prevents XSS.
- Pre-parse and cache all templates at startup; never call `template.ParseFiles` inside a request handler (performance cost and race conditions).
- Use a single base layout (`base.html`) with named blocks (`{{block "content" .}}`) for each page.
- Pass data to templates via dedicated view-model structs — never pass raw database rows.

  ```go
  type DashboardView struct {
      KPIs      KPISummary
      MaxYear   string
      Countries []string
  }
  ```

---

## 6. Security

### Input Handling

- Validate all user inputs (query params, path params, form values) against expected formats.
- Use allow-lists for enum-like values (`type_ie` must be `"Exports"` or `"Imports"` — nothing else).
- Reject oversized inputs (max page size, max string length) to prevent DoS.

### HTTP Headers

Add security headers via middleware on every response:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
```

### Secrets

- Never commit secrets, API keys, or database credentials to the repository.
- Store secrets in environment variables.
- Add `.env` to `.gitignore`; commit only `.env.example` with placeholder values.

### Dependencies

- Run `go mod tidy` after adding or removing imports.
- Run `govulncheck ./...` before each release to check for known CVEs in dependencies.
- Pin indirect dependencies — do not use `@latest` in `go.mod`.

---

## 7. Testing

### Unit Tests

- Place tests in `<pkg>_test.go` files in the same directory as the code under test.
- Use the standard `testing` package; avoid third-party assertion libraries unless the project already uses one.
- Table-driven tests are preferred for functions with multiple input/output cases.

### HTTP Handler Tests

- Use `net/http/httptest.NewRecorder()` and `httptest.NewServer()` — never start a real server in tests.
- Test both happy paths and error conditions (invalid params, DB errors).

### Database Tests

- Use a test PostgreSQL instance (Docker Compose is fine for CI).
- Wrap each test in a transaction and roll back at the end — keeps tests isolated and fast.
- Never use the production database for tests.

### Coverage

- Aim for ≥ 80% statement coverage on `internal/` packages.
- Run `go test -race ./...` — the race detector must be clean.

---

## 8. Logging & Observability

- Use the standard `log/slog` package (Go 1.21+) with structured key-value pairs — no `fmt.Println` for production logs.
- Log at `INFO` level for normal operations, `WARN` for recoverable issues, `ERROR` for failures.
- Include request-scoped context in logs (request ID, user, duration) via middleware.
- Expose a `/health` endpoint that returns `200 OK` with `{"status":"ok"}` for liveness probes.

---

## 9. Performance

- Enable `chi/middleware.Compress` (GZIP) — JSON payloads for large trade datasets can be several hundred KB.
- Cache expensive aggregation queries in-memory with a short TTL (e.g. `sync.Map` + `time.Now`) — KPI totals change infrequently.
- Use `pgxpool` with `MaxConns` tuned to the deployment environment (default 4 is too low for production).
- Serve static assets with appropriate `Cache-Control` headers (long TTL + content hash in filename for cache busting).

---

## 10. Development Workflow

### Local Setup

```bash
# 1. Start PostgreSQL
docker compose up -d db

# 2. Run migrations
go run ./scripts/migrate.go up

# 3. Import sample data
go run ./scripts/import_data.go --source ./data/sample

# 4. Start dev server with live reload
air  # uses .air.toml; falls back to: go run ./cmd/server
```

### Branch & Commit Convention

- Branch name: `feature/<short-name>`, `fix/<short-name>`, `chore/<short-name>`.
- Commit messages: `<type>(<scope>): <imperative summary>` — e.g. `feat(handlers): add market intelligence endpoint`.
- Keep commits focused — one logical change per commit.
- Never commit directly to `main`; always open a PR.

### CI Checks (must pass before merge)

1. `go build ./...`
2. `go vet ./...`
3. `golangci-lint run`
4. `go test -race ./...`
5. `govulncheck ./...`

---

## 11. Dependency Policy

| Category | Preferred Library | Notes |
|---|---|---|
| HTTP router | `go-chi/chi/v5` | Only add; never replace with a heavier framework |
| PostgreSQL driver | `jackc/pgx/v5` | Use pgxpool for connection pooling |
| Schema migration | `pressly/goose/v3` | Plain SQL files preferred over Go migrations |
| Configuration | `caarlos0/env/v11` or stdlib `os.Getenv` | No Viper unless truly needed |
| Logging | `log/slog` (stdlib) | Only add zerolog/zap if structured perf logging becomes critical |
| Testing | stdlib `testing` + `net/http/httptest` | Add `testify/require` only if it simplifies significantly |

**Principle:** Prefer the standard library. Add a third-party dependency only when the stdlib is genuinely insufficient. Every dependency is a maintenance burden.

---

## 12. Documentation

- Every exported function, type, and constant must have a Go doc comment (`// FunctionName …`).
- Keep `README.md` up to date with setup instructions and environment variable reference.
- Document non-obvious SQL queries with a short comment explaining the business logic.
- Update `approach.md` if architectural decisions change.
