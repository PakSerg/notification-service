package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/PakSerg/NotiHub/internal/notification"
	"github.com/PakSerg/NotiHub/internal/provider"
	"github.com/PakSerg/NotiHub/internal/repository"
)

var (
	ErrInvalidChannel = errors.New("invalid channel")
	ErrEmptyRecipient = errors.New("recipient is required")
	ErrEmptyMessage   = errors.New("message is required")
)

// dispatchTimeout bounds how long a background delivery attempt may run,
// independent of the (already-finished) request that created the notification.
const dispatchTimeout = 10 * time.Second

// SenderRegistry resolves the Sender responsible for a channel. Satisfied by
// *provider.Registry.
type SenderRegistry interface {
	Get(channel notification.Channel) (provider.Sender, error)
}

type NotificationService struct {
	repo    repository.Repository
	senders SenderRegistry
}

// NewNotificationService builds a service. senders may be nil, in which case
// created notifications are stored but never actually delivered.
func NewNotificationService(repo repository.Repository, senders SenderRegistry) *NotificationService {
	return &NotificationService{repo: repo, senders: senders}
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
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Save(ctx, n); err != nil {
		return nil, err
	}

	if s.senders != nil {
		go s.dispatch(n)
	}

	return n, nil
}

func (s *NotificationService) Get(ctx context.Context, id string) (*notification.Notification, error) {
	return s.repo.Get(ctx, id)
}

// dispatch delivers n through its channel's sender and records the outcome.
// It runs in the background after Create has already returned the pending
// notification to the caller, so it uses its own context rather than the
// (by then finished) request context.
func (s *NotificationService) dispatch(n *notification.Notification) {
	ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
	defer cancel()

	status := notification.StatusSent

	sender, err := s.senders.Get(n.Channel)
	if err != nil {
		status = notification.StatusFailed
	} else if err := sender.Send(ctx, n); err != nil {
		log.Printf("send notification %s via %s: %v", n.ID, n.Channel, err)
		status = notification.StatusFailed
	}

	if err := s.repo.UpdateStatus(ctx, n.ID, status); err != nil {
		log.Printf("update status of notification %s to %s: %v", n.ID, status, err)
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
