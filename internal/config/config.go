// Package config collects every knob the service has into one struct,
// so the rest of the code never reads the environment directly.
package config

import (
	"fmt"
	"os"
	"strconv"
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
	// RateLimitRPS is the sustained number of requests per second allowed
	// per client IP. A value <= 0 disables rate limiting entirely.
	RateLimitRPS float64
	// RateLimitBurst is the number of requests a client may burst above
	// RateLimitRPS before being throttled.
	RateLimitBurst int
	// RetryMaxAttempts is the total number of delivery attempts made for a
	// notification, including the first one, before it is marked failed.
	RetryMaxAttempts int
	// RetryBaseDelay is the backoff before the second delivery attempt; it
	// doubles after every subsequent failure, up to RetryMaxDelay.
	RetryBaseDelay time.Duration
	// RetryMaxDelay caps the backoff between delivery attempts.
	RetryMaxDelay time.Duration
}

// Default values are chosen to make `go run ./cmd/notihub` work with no setup.
const (
	defaultHTTPAddr         = ":8080"
	defaultDBPath           = "notihub.db"
	defaultShutdownTimeout  = 5 * time.Second
	defaultRateLimitRPS     = 5.0
	defaultRateLimitBurst   = 10
	defaultRetryMaxAttempts = 3
	defaultRetryBaseDelay   = 500 * time.Millisecond
	defaultRetryMaxDelay    = 10 * time.Second
)

// Load reads the configuration from the environment, falling back to defaults.
func Load() (Config, error) {
	shutdownTimeout, err := durationEnv("NOTIHUB_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	rateLimitRPS, err := floatEnv("NOTIHUB_RATE_LIMIT_RPS", defaultRateLimitRPS)
	if err != nil {
		return Config{}, err
	}

	rateLimitBurst, err := intEnv("NOTIHUB_RATE_LIMIT_BURST", defaultRateLimitBurst)
	if err != nil {
		return Config{}, err
	}

	retryMaxAttempts, err := intEnv("NOTIHUB_RETRY_MAX_ATTEMPTS", defaultRetryMaxAttempts)
	if err != nil {
		return Config{}, err
	}

	retryBaseDelay, err := durationEnv("NOTIHUB_RETRY_BASE_DELAY", defaultRetryBaseDelay)
	if err != nil {
		return Config{}, err
	}

	retryMaxDelay, err := durationEnv("NOTIHUB_RETRY_MAX_DELAY", defaultRetryMaxDelay)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:         stringEnv("NOTIHUB_HTTP_ADDR", defaultHTTPAddr),
		DBPath:           stringEnv("NOTIHUB_DB_PATH", defaultDBPath),
		ShutdownTimeout:  shutdownTimeout,
		RateLimitRPS:     rateLimitRPS,
		RateLimitBurst:   rateLimitBurst,
		RetryMaxAttempts: retryMaxAttempts,
		RetryBaseDelay:   retryBaseDelay,
		RetryMaxDelay:    retryMaxDelay,
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

func floatEnv(key string, fallback float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return value, nil
}

func intEnv(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return value, nil
}
