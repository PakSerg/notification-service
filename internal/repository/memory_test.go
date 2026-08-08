package repository

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/PakSerg/NotiHub/internal/notification"
)

func TestMemoryRepositorySaveAndGet(t *testing.T) {
	r := NewMemoryRepository()
	ctx := context.Background()

	n := &notification.Notification{
		ID:      "abc123",
		Channel: notification.ChannelEmail,
		Status:  notification.StatusPending,
	}

	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	got, err := r.Get(ctx, "abc123")
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.ID != n.ID {
		t.Fatalf("expected id %q, got %q", n.ID, got.ID)
	}
}

func TestMemoryRepositoryGetNotFound(t *testing.T) {
	r := NewMemoryRepository()

	_, err := r.Get(context.Background(), "does-not-exist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryRepositoryUpdateStatus(t *testing.T) {
	r := NewMemoryRepository()
	ctx := context.Background()

	n := &notification.Notification{ID: "abc123", Status: notification.StatusPending}
	if err := r.Save(ctx, n); err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	if err := r.UpdateStatus(ctx, "abc123", notification.Status("sent")); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	got, err := r.Get(ctx, "abc123")
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.Status != notification.Status("sent") {
		t.Fatalf("expected status %q, got %q", "sent", got.Status)
	}
}

func TestMemoryRepositoryConcurrentAccess(t *testing.T) {
	r := NewMemoryRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := strconv.Itoa(i)
			r.Save(ctx, &notification.Notification{ID: id, Status: notification.StatusPending})
			r.Get(ctx, id)
		}(i)
	}
	wg.Wait()
}
