package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	mrand "math/rand/v2"
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

// attemptTimeout bounds how long a single delivery attempt may run,
// independent of the (already-finished) request that created the notification.
const attemptTimeout = 10 * time.Second

// SenderRegistry resolves the Sender responsible for a channel. Satisfied by
// *provider.Registry.
type SenderRegistry interface {
	Get(channel notification.Channel) (provider.Sender, error)
}

// RetryPolicy controls how a failed delivery is retried with exponential
// backoff before the notification is given up on and marked failed.
type RetryPolicy struct {
	// MaxAttempts is the total number of delivery attempts, including the
	// first one. A value <= 1 means no retries.
	MaxAttempts int
	// BaseDelay is the backoff before the second attempt; it doubles after
	// every subsequent failure, up to MaxDelay.
	BaseDelay time.Duration
	// MaxDelay caps the backoff between attempts.
	MaxDelay time.Duration
}

// DefaultRetryPolicy is used when no RetryPolicy option is given.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: 500 * time.Millisecond, MaxDelay: 10 * time.Second}
}

// backoff returns how long to wait after the given failed attempt (1-indexed)
// before retrying, with up to 20% jitter to avoid retry storms.
func (p RetryPolicy) backoff(attempt int) time.Duration {
	delay := p.BaseDelay << (attempt - 1)
	if delay <= 0 || delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	jitter := time.Duration(mrand.Int64N(int64(delay)/5 + 1))
	return delay + jitter
}

// Option configures a NotificationService.
type Option func(*NotificationService)

// WithRetryPolicy overrides the default retry behavior for failed deliveries.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(s *NotificationService) { s.retry = p }
}

type NotificationService struct {
	repo    repository.Repository
	senders SenderRegistry
	retry   RetryPolicy
}

// NewNotificationService builds a service. senders may be nil, in which case
// created notifications are stored but never actually delivered.
func NewNotificationService(repo repository.Repository, senders SenderRegistry, opts ...Option) *NotificationService {
	s := &NotificationService{repo: repo, senders: senders, retry: DefaultRetryPolicy()}
	for _, opt := range opts {
		opt(s)
	}
	return s
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

// dispatch delivers n through its channel's sender, retrying on failure with
// exponential backoff up to s.retry.MaxAttempts, and records the outcome. It
// runs in the background after Create has already returned the pending
// notification to the caller.
func (s *NotificationService) dispatch(n *notification.Notification) {
	sender, err := s.senders.Get(n.Channel)
	if err != nil {
		s.finish(n, notification.StatusFailed, 0, err)
		return
	}

	maxAttempts := s.retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = s.attempt(sender, n)
		if lastErr == nil {
			s.finish(n, notification.StatusSent, attempt, nil)
			return
		}

		log.Printf("send notification %s via %s (attempt %d/%d): %v", n.ID, n.Channel, attempt, maxAttempts, lastErr)
		if attempt < maxAttempts {
			time.Sleep(s.retry.backoff(attempt))
		}
	}

	s.finish(n, notification.StatusFailed, maxAttempts, lastErr)
}

func (s *NotificationService) attempt(sender provider.Sender, n *notification.Notification) error {
	ctx, cancel := context.WithTimeout(context.Background(), attemptTimeout)
	defer cancel()

	return sender.Send(ctx, n)
}

func (s *NotificationService) finish(n *notification.Notification, status notification.Status, attempts int, sendErr error) {
	lastErr := ""
	if sendErr != nil {
		lastErr = sendErr.Error()
	}

	ctx, cancel := context.WithTimeout(context.Background(), attemptTimeout)
	defer cancel()

	if err := s.repo.UpdateDeliveryResult(ctx, n.ID, status, attempts, lastErr); err != nil {
		log.Printf("update delivery result of notification %s to %s: %v", n.ID, status, err)
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
