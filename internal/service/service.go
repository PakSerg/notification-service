package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/PakSerg/NotiHub/internal/notification"
)

var (
	ErrInvalidChannel = errors.New("invalid channel")
	ErrEmptyRecipient = errors.New("recipient is required")
	ErrEmptyMessage   = errors.New("message is required")
)

type NotificationService struct{}

func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

func (s *NotificationService) Create(channel notification.Channel, recipient, message string) (*notification.Notification, error) {
	if !channel.Valid() {
		return nil, ErrInvalidChannel
	}
	if recipient == "" {
		return nil, ErrEmptyRecipient
	}
	if message == "" {
		return nil, ErrEmptyMessage
	}

	n := &notification.Notification{
		ID:        generateID(),
		Channel:   channel,
		Recipient: recipient,
		Message:   message,
		Status:    notification.StatusPending,
		CreatedAt: time.Now(),
	}

	return n, nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
