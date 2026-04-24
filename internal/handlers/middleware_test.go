package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// TestSecurityHeaders verifies that the SecurityHeaders middleware sets all
// required HTTP security headers on every response (NF-13).
func TestSecurityHeaders(t *testing.T) {
	r := chi.NewRouter()
	r.Use(handlers.SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, value := range want {
		if got := w.Header().Get(header); got != value {
			t.Errorf("header %s: want %q, got %q", header, value, got)
		}
	}
}

// TestSecurityHeadersOnEveryRoute verifies that security headers are present
// on routes returning different status codes, including 404.
func TestSecurityHeadersOnEveryRoute(t *testing.T) {
	r := chi.NewRouter()
	r.Use(handlers.SecurityHeaders)
	r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/error", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "oops", http.StatusInternalServerError)
	})

	requiredHeaders := []struct {
		name  string
		value string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	paths := []string{"/ok", "/error", "/not-found"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			for _, h := range requiredHeaders {
				if got := w.Header().Get(h.name); got != h.value {
					t.Errorf("%s %s: want %q, got %q", path, h.name, h.value, got)
				}
			}
		})
	}
}

// TestRateLimitExceeded verifies that more requests than the concurrency
// limit returns 429 Too Many Requests (NF-14). The chi Throttle middleware
// limits the number of concurrent in-flight requests; excess requests are
// rejected immediately with 429.
func TestRateLimitExceeded(t *testing.T) {
	const limit = 3

	// Slow handler that holds the connection open long enough for all
	// goroutines to pile up simultaneously.
	slow := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(middleware.Throttle(limit))
	r.Get("/api/test", slow)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Send more requests than the limit concurrently.
	const total = limit + 5
	codes := make([]int, total)
	var wg sync.WaitGroup

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := http.Get(srv.URL + "/api/test") //nolint:noctx
			if err != nil {
				t.Logf("request %d error: %v", idx, err)
				return
			}
			defer resp.Body.Close()
			codes[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	got429 := 0
	for _, code := range codes {
		if code == http.StatusTooManyRequests {
			got429++
		}
	}

	if got429 == 0 {
		t.Error("want at least one 429 Too Many Requests when concurrency limit exceeded, got none")
	}
}
