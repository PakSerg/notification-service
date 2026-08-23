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
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("NOTIHUB_HTTP_ADDR", ":9090")
	t.Setenv("NOTIHUB_DB_PATH", "/data/notihub.db")
	t.Setenv("NOTIHUB_SHUTDOWN_TIMEOUT", "30s")

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
}

func TestLoadInvalidDuration(t *testing.T) {
	t.Setenv("NOTIHUB_SHUTDOWN_TIMEOUT", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}
