package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("expected addr %q, got %q", defaultHTTPAddr, cfg.HTTPAddr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("expected db path %q, got %q", defaultDBPath, cfg.DBPath)
	}
	if cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("expected shutdown timeout %v, got %v", defaultShutdownTimeout, cfg.ShutdownTimeout)
	}
	if cfg.RateLimitRPS != defaultRateLimitRPS {
		t.Fatalf("expected rate limit rps %v, got %v", defaultRateLimitRPS, cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != defaultRateLimitBurst {
		t.Fatalf("expected rate limit burst %v, got %v", defaultRateLimitBurst, cfg.RateLimitBurst)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("NOTIHUB_HTTP_ADDR", ":9090")
	t.Setenv("NOTIHUB_DB_PATH", "/data/notihub.db")
	t.Setenv("NOTIHUB_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("NOTIHUB_RATE_LIMIT_RPS", "20")
	t.Setenv("NOTIHUB_RATE_LIMIT_BURST", "40")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected addr %q, got %q", ":9090", cfg.HTTPAddr)
	}
	if cfg.DBPath != "/data/notihub.db" {
		t.Fatalf("expected db path %q, got %q", "/data/notihub.db", cfg.DBPath)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("expected shutdown timeout %v, got %v", 30*time.Second, cfg.ShutdownTimeout)
	}
	if cfg.RateLimitRPS != 20 {
		t.Fatalf("expected rate limit rps %v, got %v", 20.0, cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 40 {
		t.Fatalf("expected rate limit burst %v, got %v", 40, cfg.RateLimitBurst)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	t.Setenv("NOTIHUB_SHUTDOWN_TIMEOUT", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}

func TestLoadInvalidRateLimitRPS(t *testing.T) {
	t.Setenv("NOTIHUB_RATE_LIMIT_RPS", "not-a-number")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid rate limit rps, got nil")
	}
}

func TestLoadInvalidRateLimitBurst(t *testing.T) {
	t.Setenv("NOTIHUB_RATE_LIMIT_BURST", "not-a-number")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid rate limit burst, got nil")
	}
}
