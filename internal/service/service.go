package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/PakSerg/NotiHub/internal/notification"
	"github.com/PakSerg/NotiHub/internal/repository"
)

var (
	ErrInvalidChannel = errors.New("invalid channel")
	ErrEmptyRecipient = errors.New("recipient is required")
	ErrEmptyMessage   = errors.New("message is required")
)

type NotificationService struct {
	repo repository.Repository
}

func NewNotificationService(repo repository.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) Create(ctx context.Context, channel notification.Channel, recipient, message string) (*notification.Notification, error) {
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

	if err := s.repo.Save(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

func (s *NotificationService) Get(ctx context.Context, id string) (*notification.Notification, error) {
	return s.repo.Get(ctx, id)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
