package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

// Publisher hands a delivery job for an already-saved notification off to a
// queue, so it can be picked up and processed by a worker. Satisfied by
// *queue.Producer.
type Publisher interface {
	Publish(ctx context.Context, id string) error
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

// WithPublisher makes Create hand off delivery of new notifications to a
// queue instead of leaving them pending. The API process wires this in; the
// worker process (which calls Dispatch directly, as jobs arrive) does not.
func WithPublisher(p Publisher) Option {
	return func(s *NotificationService) { s.publisher = p }
}

type NotificationService struct {
	repo      repository.Repository
	senders   SenderRegistry
	publisher Publisher
	retry     RetryPolicy
}

// NewNotificationService builds a service. senders is only used by Dispatch,
// so the API process (which only calls Create) may pass nil. Likewise the
// worker process, which only calls Dispatch, has no use for WithPublisher.
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

	if s.publisher != nil {
		// n is already durably saved as pending, so a publish failure here
		// does not lose it - but nothing will ever pick it up for delivery
		// either. Surfacing the error lets the caller retry the request;
		// closing that gap for good would need a transactional outbox, which
		// is more machinery than this service warrants today.
		if err := s.publisher.Publish(ctx, n.ID); err != nil {
			// n itself was saved successfully, so it is returned alongside
			// the error rather than dropped - the caller may still want its
			// ID, e.g. to log which notification is now stuck pending.
			return n, fmt.Errorf("publish delivery job for notification %s: %w", n.ID, err)
		}
	}

	return n, nil
}

func (s *NotificationService) Get(ctx context.Context, id string) (*notification.Notification, error) {
	return s.repo.Get(ctx, id)
}

// Dispatch delivers the notification identified by id through its channel's
// sender, retrying on failure with exponential backoff up to
// s.retry.MaxAttempts, and records the outcome. It is meant to be called by a
// queue consumer once per delivery job.
//
// Dispatch is idempotent: a notification that is no longer pending (already
// sent, or already given up on by an earlier run of this same job) is left
// untouched. That makes it safe to call again for a job redelivered after an
// at-least-once queue's consumer crashed before acknowledging it.
//
// A non-nil error means the job could not be processed at all - for example
// the notification's own record could not be read - and should be retried;
// a resolved delivery outcome, success or failure, is never reported as an
// error here.
func (s *NotificationService) Dispatch(ctx context.Context, id string) error {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("load notification %s: %w", id, err)
	}

	if n.Status != notification.StatusPending {
		return nil
	}

	sender, err := s.senders.Get(n.Channel)
	if err != nil {
		s.finish(n, notification.StatusFailed, 0, err)
		return nil
	}

	maxAttempts := s.retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = s.attempt(ctx, sender, n)
		if lastErr == nil {
			s.finish(n, notification.StatusSent, attempt, nil)
			return nil
		}

		log.Printf("send notification %s via %s (attempt %d/%d): %v", n.ID, n.Channel, attempt, maxAttempts, lastErr)
		if attempt < maxAttempts {
			time.Sleep(s.retry.backoff(attempt))
		}
	}

	s.finish(n, notification.StatusFailed, maxAttempts, lastErr)
	return nil
}

func (s *NotificationService) attempt(ctx context.Context, sender provider.Sender, n *notification.Notification) error {
	ctx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()

	return sender.Send(ctx, n)
}

// finish always runs to completion on its own timeout, detached from ctx, so
// a delivery outcome is recorded even if the caller (e.g. a worker shutting
// down) has already canceled its own context.
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
