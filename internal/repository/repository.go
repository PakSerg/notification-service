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
	// UpdateDeliveryResult records the outcome of a delivery attempt: the
	// resulting status, the total number of attempts made so far, and the
	// error message of the last failed attempt (empty on success).
	UpdateDeliveryResult(ctx context.Context, id string, status notification.Status, attempts int, lastErr string) error
}

// Compile-time checks that every implementation satisfies the interface.
var (
	_ Repository = (*MemoryRepository)(nil)
	_ Repository = (*PostgresRepository)(nil)
)
