// Package handlers contains HTTP handler functions for the GO2shiny server.
package handlers

import (
"context"
"fmt"
"html/template"
"net/http"
"strconv"
"sync"
"time"

"golang.org/x/sync/singleflight"

"github.com/christiaanpauw/GO2shiny/internal/db"
)

// dashboardData holds the data passed to the dashboard template.
type dashboardData struct {
Year int
}

// Dashboard handles GET /dashboard and renders the base layout with the
// dashboard content block. The actual KPI data loads asynchronously via HTMX.
func Dashboard(tmpl *template.Template) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
data := dashboardData{Year: time.Now().Year()}
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.WriteHeader(http.StatusOK)
if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
http.Error(w, "template error", http.StatusInternalServerError)
}
}
}

// kpiView is the view model passed to the kpi_cards template.
type kpiView struct {
Year                 int
TotalExports         string
TotalImports         string
TradeBalance         string
TradeBalancePositive bool
YoYChange            string
YoYPositive          bool
}

// kpiCacheEntry holds a cached KPISummary and its expiry time.
type kpiCacheEntry struct {
value     db.KPISummary
expiresAt time.Time
}

// kpiCache is a TTL in-memory store for KPI summaries, keyed by year.
// A singleflight.Group prevents thundering-herd duplicate DB fetches when
// the cache expires under concurrent load.
type kpiCache struct {
mu      sync.Mutex
entries map[int]kpiCacheEntry
ttl     time.Duration
sf      singleflight.Group
}

func newKPICache(ttl time.Duration) *kpiCache {
return &kpiCache{
entries: make(map[int]kpiCacheEntry),
ttl:     ttl,
}
}

// getOrFetch returns a cached KPISummary for year if it is still valid;
// otherwise it calls fetch exactly once (even under concurrent requests for
// the same key), stores the result, and returns it.
func (c *kpiCache) getOrFetch(
ctx context.Context,
year int,
fetch func(context.Context, int) (db.KPISummary, error),
) (db.KPISummary, error) {
c.mu.Lock()
entry, ok := c.entries[year]
if ok && time.Now().Before(entry.expiresAt) {
c.mu.Unlock()
return entry.value, nil
}
c.mu.Unlock()

// Coalesce concurrent fetches for the same year into a single DB call.
key := strconv.Itoa(year)
v, err, _ := c.sf.Do(key, func() (any, error) {
return fetch(ctx, year)
})
if err != nil {
return db.KPISummary{}, err
}

val, _ := v.(db.KPISummary)

c.mu.Lock()
// Evict expired entries on each write to bound memory usage.
now := time.Now()
for k, e := range c.entries {
if now.After(e.expiresAt) {
delete(c.entries, k)
}
}
c.entries[year] = kpiCacheEntry{value: val, expiresAt: now.Add(c.ttl)}
c.mu.Unlock()

return val, nil
}

// KPIHandler returns an http.HandlerFunc for GET /partials/kpis.
//
// The handler requires a "year" query parameter (integer, 1900–9999). It uses
// an in-memory cache with the given TTL to avoid repeated DB round-trips.
// If querier is nil the handler responds with 503 Service Unavailable.
func KPIHandler(querier db.KPIQuerier, tmpl *template.Template, ttl time.Duration) http.HandlerFunc {
if querier == nil {
return func(w http.ResponseWriter, r *http.Request) {
http.Error(w, "database not available", http.StatusServiceUnavailable)
}
}

cache := newKPICache(ttl)

return func(w http.ResponseWriter, r *http.Request) {
yearStr := r.URL.Query().Get("year")
if yearStr == "" {
http.Error(w, "year parameter required", http.StatusBadRequest)
return
}

year, err := strconv.Atoi(yearStr)
if err != nil || year < 1900 || year > 9999 {
http.Error(w, "invalid year parameter", http.StatusBadRequest)
return
}

summary, err := cache.getOrFetch(r.Context(), year, querier.GetKPISummary)
if err != nil {
http.Error(w, "failed to load KPI data", http.StatusInternalServerError)
return
}

view := toKPIView(summary)
w.Header().Set("Content-Type", "text/html; charset=utf-8")
if err := tmpl.ExecuteTemplate(w, "kpi_cards", view); err != nil {
http.Error(w, "template error", http.StatusInternalServerError)
}
}
}

// toKPIView converts a KPISummary into a kpiView for template rendering.
func toKPIView(s db.KPISummary) kpiView {
return kpiView{
Year:                 s.Year,
TotalExports:         formatNZD(s.TotalExports),
TotalImports:         formatNZD(s.TotalImports),
TradeBalance:         formatNZD(s.TradeBalance),
TradeBalancePositive: s.TradeBalance >= 0,
YoYChange:            formatPct(s.YoYChange),
YoYPositive:          s.YoYChange >= 0,
}
}

// formatNZD formats a float64 as a human-readable NZD string
// (e.g. "NZD 12.5B", "NZD 7.2M", "NZD 450.0K", "NZD 900").
func formatNZD(v float64) string {
neg := v < 0
abs := v
if neg {
abs = -v
}

var s string
switch {
case abs >= 1e9:
s = fmt.Sprintf("NZD %.1fB", abs/1e9)
case abs >= 1e6:
s = fmt.Sprintf("NZD %.1fM", abs/1e6)
case abs >= 1e3:
s = fmt.Sprintf("NZD %.1fK", abs/1e3)
default:
s = fmt.Sprintf("NZD %.0f", abs)
}

if neg {
return "-" + s
}
return s
}

// formatPct formats a float64 as a percentage string with sign
// (e.g. "+3.2%" or "-1.5%").
func formatPct(v float64) string {
if v >= 0 {
return fmt.Sprintf("+%.1f%%", v)
}
return fmt.Sprintf("%.1f%%", v)
}
