// Package handlers contains HTTP handler functions for the GO2shiny server.
package handlers

import (
"context"
"fmt"
"html/template"
"net/http"
"sync"
"time"

"golang.org/x/sync/singleflight"

"github.com/christiaanpauw/GO2shiny/internal/db"
)

// dashboardData holds the data passed to the dashboard template.
type dashboardData struct {
Year     int
YearFrom int
YearTo   int
}

// Dashboard handles GET /dashboard and renders the base layout with the
// dashboard content block. The actual KPI data loads asynchronously via HTMX.
func Dashboard(tmpl *template.Template) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
now := time.Now().Year()
data := dashboardData{Year: now, YearFrom: 1990, YearTo: now}
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

// kpiCache is a TTL in-memory store for KPI summaries, keyed by a combined filter string.
// A singleflight.Group prevents thundering-herd duplicate DB fetches when
// the cache expires under concurrent load.
type kpiCache struct {
mu      sync.Mutex
entries map[string]kpiCacheEntry
ttl     time.Duration
sf      singleflight.Group
}

func newKPICache(ttl time.Duration) *kpiCache {
return &kpiCache{
entries: make(map[string]kpiCacheEntry),
ttl:     ttl,
}
}

// getOrFetch returns a cached KPISummary for the given FilterParams if still valid;
// otherwise it calls fetch exactly once (even under concurrent requests for
// the same key), stores the result, and returns it.
func (c *kpiCache) getOrFetch(
ctx context.Context,
fp FilterParams,
fetch func(context.Context, int, int, string, string) (db.KPISummary, error),
) (db.KPISummary, error) {
key := fmt.Sprintf("%d-%d-%s-%s", fp.YearFrom, fp.YearTo, fp.TypeIE, fp.TypeGS)

c.mu.Lock()
entry, ok := c.entries[key]
if ok && time.Now().Before(entry.expiresAt) {
c.mu.Unlock()
return entry.value, nil
}
c.mu.Unlock()

// Coalesce concurrent fetches for the same filter key into a single DB call.
v, err, _ := c.sf.Do(key, func() (any, error) {
return fetch(ctx, fp.YearFrom, fp.YearTo, fp.TypeIE, fp.TypeGS)
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
c.entries[key] = kpiCacheEntry{value: val, expiresAt: now.Add(c.ttl)}
c.mu.Unlock()

return val, nil
}

// KPIHandler returns an http.HandlerFunc for GET /partials/kpis.
//
// Filter parameters (all optional): year_from, year_to, type_ie, type_gs.
// Results are cached per unique filter combination using the given TTL.
// If querier is nil the handler responds with 503 Service Unavailable.
func KPIHandler(querier db.KPIQuerier, tmpl *template.Template, ttl time.Duration) http.HandlerFunc {
if querier == nil {
return func(w http.ResponseWriter, r *http.Request) {
http.Error(w, "database not available", http.StatusServiceUnavailable)
}
}

cache := newKPICache(ttl)

return func(w http.ResponseWriter, r *http.Request) {
fp, ok := parseFilterParams(w, r)
if !ok {
return
}

summary, err := cache.getOrFetch(r.Context(), fp, querier.GetKPISummary)
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
