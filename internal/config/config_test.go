package config

import (
	"reflect"
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
	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Fatalf("expected database url %q, got %q", defaultDatabaseURL, cfg.DatabaseURL)
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
	if want := []string{defaultKafkaBrokers}; !reflect.DeepEqual(cfg.KafkaBrokers, want) {
		t.Fatalf("expected kafka brokers %v, got %v", want, cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != defaultKafkaTopic {
		t.Fatalf("expected kafka topic %q, got %q", defaultKafkaTopic, cfg.KafkaTopic)
	}
	if cfg.KafkaConsumerGroup != defaultKafkaConsumerGroup {
		t.Fatalf("expected kafka consumer group %q, got %q", defaultKafkaConsumerGroup, cfg.KafkaConsumerGroup)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("NOTIHUB_HTTP_ADDR", ":9090")
	t.Setenv("NOTIHUB_DATABASE_URL", "postgres://user:pass@db:5432/notihub?sslmode=disable")
	t.Setenv("NOTIHUB_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("NOTIHUB_RATE_LIMIT_RPS", "20")
	t.Setenv("NOTIHUB_RATE_LIMIT_BURST", "40")
	t.Setenv("NOTIHUB_KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("NOTIHUB_KAFKA_TOPIC", "custom.deliveries")
	t.Setenv("NOTIHUB_KAFKA_CONSUMER_GROUP", "custom-group")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected addr %q, got %q", ":9090", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/notihub?sslmode=disable" {
		t.Fatalf("expected database url %q, got %q", "postgres://user:pass@db:5432/notihub?sslmode=disable", cfg.DatabaseURL)
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
	if want := []string{"kafka-1:9092", "kafka-2:9092"}; !reflect.DeepEqual(cfg.KafkaBrokers, want) {
		t.Fatalf("expected kafka brokers %v, got %v", want, cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "custom.deliveries" {
		t.Fatalf("expected kafka topic %q, got %q", "custom.deliveries", cfg.KafkaTopic)
	}
	if cfg.KafkaConsumerGroup != "custom-group" {
		t.Fatalf("expected kafka consumer group %q, got %q", "custom-group", cfg.KafkaConsumerGroup)
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
