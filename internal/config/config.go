// Package config reads application configuration from environment variables.
package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds all runtime configuration values.
type Config struct {
	DatabaseURL     string
	Port            string
	LogLevel        slog.Level
	MaxDBConns      int
	CacheTTLSeconds int
}

// Load reads configuration from environment variables, applying defaults where needed.
func Load() Config {
	c := Config{
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		Port:            getEnv("PORT", "8080"),
		MaxDBConns:      getEnvInt("MAX_DB_CONNS", 10),
		CacheTTLSeconds: getEnvInt("CACHE_TTL_SECONDS", 300),
		LogLevel:        parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}
	return c
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
