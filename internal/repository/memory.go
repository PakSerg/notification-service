package repository

import (
	"context"
	"sync"

	"github.com/PakSerg/NotiHub/internal/notification"
)

type MemoryRepository struct {
	mu            sync.Mutex
	notifications map[string]*notification.Notification
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		notifications: make(map[string]*notification.Notification),
	}
}

func (r *MemoryRepository) Save(ctx context.Context, n *notification.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.notifications[n.ID] = n
	return nil
}

func (r *MemoryRepository) Get(ctx context.Context, id string) (*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		return nil, ErrNotFound
	}
	return n, nil
}

func (r *MemoryRepository) UpdateDeliveryResult(ctx context.Context, id string, status notification.Status, attempts int, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		return ErrNotFound
	}
	n.Status = status
	n.Attempts = attempts
	n.LastError = lastErr
	return nil
}
