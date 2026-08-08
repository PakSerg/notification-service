package provider

import (
	"context"
	"log"

	"github.com/PakSerg/NotiHub/internal/notification"
)

type PushSender struct{}

func NewPushSender() *PushSender {
	return &PushSender{}
}

func (s *PushSender) Send(ctx context.Context, n *notification.Notification) error {
	log.Printf("sending push to %s: %s", n.Recipient, n.Message)
	return nil
}
