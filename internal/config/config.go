// Package config collects every knob the service has into one struct,
// so the rest of the code never reads the environment directly.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds the runtime settings of the service.
type Config struct {
	// HTTPAddr is the address the HTTP server listens on.
	HTTPAddr string
	// DBPath is the SQLite database file. Relative paths are resolved
	// against the working directory, which is the project root locally
	// and /data in the container.
	DBPath string
	// ShutdownTimeout bounds how long in-flight requests may finish
	// after a termination signal.
	ShutdownTimeout time.Duration
}

// Default values are chosen to make `go run ./cmd/notihub` work with no setup.
const (
	defaultHTTPAddr        = ":8080"
	defaultDBPath          = "notihub.db"
	defaultShutdownTimeout = 5 * time.Second
)

// Load reads the configuration from the environment, falling back to defaults.
func Load() (Config, error) {
	shutdownTimeout, err := durationEnv("NOTIHUB_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:        stringEnv("NOTIHUB_HTTP_ADDR", defaultHTTPAddr),
		DBPath:          stringEnv("NOTIHUB_DB_PATH", defaultDBPath),
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

func stringEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return value, nil
}
