package handlers

import (
	"html/template"
	"net/http"
	"time"
)

// dashboardData holds the data passed to the dashboard template.
type dashboardData struct {
	Year int
}

// Dashboard handles GET /dashboard and renders the base layout with an empty
// content block. No database access is required in Phase 1.
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
