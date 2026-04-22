// Package main_test exercises the server router including static file serving.
package main_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	webfs "github.com/christiaanpauw/GO2shiny/web"
)

// TestStaticFilesServed verifies that GET /static/css/app.css returns 200.
func TestStaticFilesServed(t *testing.T) {
	staticFiles, err := fs.Sub(webfs.FS, "static")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}

	r := chi.NewRouter()
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))

	req := httptest.NewRequest(http.MethodGet, "/static/css/app.css", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}
