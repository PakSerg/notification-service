package provider

import (
	"context"
	"log"

	"github.com/PakSerg/NotiHub/internal/notification"
)

type EmailSender struct{}

func NewEmailSender() *EmailSender {
	return &EmailSender{}
}

func (s *EmailSender) Send(ctx context.Context, n *notification.Notification) error {
	log.Printf("sending email to %s: %s", n.Recipient, n.Message)
	return nil
}
