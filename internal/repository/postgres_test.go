package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/PakSerg/NotiHub/internal/notification"
)

// testDatabaseURL returns the DSN of a real PostgreSQL instance to run
// against, skipping the test when none is configured (e.g. running `go test`
// locally without a database). CI provides one via a service container.
func testDatabaseURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("NOTIHUB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NOTIHUB_TEST_DATABASE_URL not set, skipping PostgreSQL integration test")
	}
	return url
}

func newTestRepository(t *testing.T) *PostgresRepository {
	t.Helper()

	r, err := NewPostgresRepository(context.Background(), testDatabaseURL(t))
	if err != nil {
		t.Fatalf("unexpected error on open: %v", err)
	}
	t.Cleanup(func() {
		if _, err := r.db.Exec("TRUNCATE notifications"); err != nil {
			t.Errorf("unexpected error on truncate: %v", err)
		}
		if err := r.Close(); err != nil {
			t.Errorf("unexpected error on close: %v", err)
		}
	})

	return r
}

func TestPostgresRepositorySaveAndGet(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	n := &notification.Notification{
		ID:        "abc123",
		Channel:   notification.ChannelEmail,
		Recipient: "user@example.com",
		Message:   "hello",
		Status:    notification.StatusPending,
		CreatedAt: time.Now().Round(time.Microsecond),
	}

	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	got, err := r.Get(ctx, n.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}

	if got.ID != n.ID || got.Channel != n.Channel || got.Recipient != n.Recipient ||
		got.Message != n.Message || got.Status != n.Status {
		t.Fatalf("expected %+v, got %+v", n, got)
	}
	if !got.CreatedAt.Equal(n.CreatedAt) {
		t.Fatalf("expected created_at %v, got %v", n.CreatedAt, got.CreatedAt)
	}
}

func TestPostgresRepositorySaveOverwrites(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	n := &notification.Notification{ID: "abc123", Message: "first", Status: notification.StatusPending}
	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	n.Message = "second"
	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("unexpected error on second save: %v", err)
	}

	got, err := r.Get(ctx, n.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.Message != "second" {
		t.Fatalf("expected message %q, got %q", "second", got.Message)
	}
}

func TestPostgresRepositoryGetNotFound(t *testing.T) {
	r := newTestRepository(t)

	_, err := r.Get(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresRepositoryUpdateDeliveryResult(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	n := &notification.Notification{ID: "abc123", Status: notification.StatusPending}
	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	if err := r.UpdateDeliveryResult(ctx, n.ID, notification.Status("failed"), 3, "boom"); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	got, err := r.Get(ctx, n.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.Status != notification.Status("failed") {
		t.Fatalf("expected status %q, got %q", "failed", got.Status)
	}
	if got.Attempts != 3 {
		t.Fatalf("expected attempts %d, got %d", 3, got.Attempts)
	}
	if got.LastError != "boom" {
		t.Fatalf("expected last_error %q, got %q", "boom", got.LastError)
	}
}

func TestPostgresRepositoryUpdateDeliveryResultNotFound(t *testing.T) {
	r := newTestRepository(t)

	err := r.UpdateDeliveryResult(context.Background(), "does-not-exist", notification.Status("sent"), 1, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// Data must survive an app restart: open a second, independent connection
// pool against the same database and confirm it sees what the first wrote.
func TestPostgresRepositoryVisibleAcrossConnections(t *testing.T) {
	ctx := context.Background()
	dsn := testDatabaseURL(t)

	first, err := NewPostgresRepository(ctx, dsn)
	if err != nil {
		t.Fatalf("unexpected error on open: %v", err)
	}
	if err := first.Save(ctx, &notification.Notification{ID: "abc123", Status: notification.StatusPending}); err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	second, err := NewPostgresRepository(ctx, dsn)
	if err != nil {
		t.Fatalf("unexpected error on reopen: %v", err)
	}
	t.Cleanup(func() {
		if _, err := second.db.Exec("TRUNCATE notifications"); err != nil {
			t.Errorf("unexpected error on truncate: %v", err)
		}
		second.Close()
	})

	if _, err := second.Get(ctx, "abc123"); err != nil {
		t.Fatalf("unexpected error on get from second connection: %v", err)
	}
}
