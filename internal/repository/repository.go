package repository

import (
	"context"
	"errors"

	"github.com/PakSerg/NotiHub/internal/notification"
)

var ErrNotFound = errors.New("notification not found")

type Repository interface {
	Save(ctx context.Context, n *notification.Notification) error
	Get(ctx context.Context, id string) (*notification.Notification, error)
	UpdateStatus(ctx context.Context, id string, status notification.Status) error
}

// Compile-time checks that every implementation satisfies the interface.
var (
	_ Repository = (*MemoryRepository)(nil)
	_ Repository = (*SQLiteRepository)(nil)
)
