// Package main is the entry point for the GO2shiny server.
// It wires together the router, embedded assets, and configuration,
// then starts listening for HTTP requests.
package main

import (
	"context"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/christiaanpauw/GO2shiny/internal/config"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
	webfs "github.com/christiaanpauw/GO2shiny/web"
)

func main() {
	cfg := config.Load()

	// Configure structured JSON logger.
	logLevel := new(slog.LevelVar)
	logLevel.Set(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// Parse HTML templates from the embedded filesystem.
	tmpl, err := template.ParseFS(webfs.FS, "templates/*.html")
	if err != nil {
		slog.Error("parse templates", "err", err)
		os.Exit(1)
	}

	// Obtain the static sub-tree for the file server.
	staticFiles, err := fs.Sub(webfs.FS, "static")
	if err != nil {
		slog.Error("static sub-fs", "err", err)
		os.Exit(1)
	}

	// Build the Chi router with standard middleware.
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Static assets: GET /static/*
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))

	// Application routes.
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/dashboard", http.StatusFound)
	})
	r.Get("/health", handlers.Health)
	r.Get("/dashboard", handlers.Dashboard(tmpl))

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background; block until OS signal arrives.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}

	slog.Info("server stopped")
}
