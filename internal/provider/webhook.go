package provider

import (
	"context"
	"log"

	"github.com/PakSerg/NotiHub/internal/notification"
)

type WebhookSender struct{}

func NewWebhookSender() *WebhookSender {
	return &WebhookSender{}
}

func (s *WebhookSender) Send(ctx context.Context, n *notification.Notification) error {
	log.Printf("calling webhook %s: %s", n.Recipient, n.Message)
	return nil
}
