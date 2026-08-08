package provider

import (
	"context"

	"github.com/PakSerg/NotiHub/internal/notification"
)

type Sender interface {
	Send(ctx context.Context, n *notification.Notification) error
}
