package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/PakSerg/NotiHub/internal/notification"
	"github.com/PakSerg/NotiHub/internal/provider"
	"github.com/PakSerg/NotiHub/internal/repository"
)

// stubSender records the notifications it was asked to send and returns err
// for every call. sent must be large enough to buffer every attempt a test
// expects, since nothing drains it until the test asserts on it.
type stubSender struct {
	err  error
	sent chan *notification.Notification
}

func (s *stubSender) Send(ctx context.Context, n *notification.Notification) error {
	s.sent <- n
	return s.err
}

// fastRetryPolicy keeps retry-driven tests quick and deterministic.
func fastRetryPolicy(maxAttempts int) Option {
	return WithRetryPolicy(RetryPolicy{MaxAttempts: maxAttempts, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond})
}

// flakySender fails the first `failures` calls, then succeeds.
type flakySender struct {
	mu       sync.Mutex
	failures int
}

func (s *flakySender) Send(ctx context.Context, n *notification.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failures > 0 {
		s.failures--
		return errors.New("transient failure")
	}
	return nil
}

// stubRegistry always returns sender, regardless of channel.
type stubRegistry struct {
	sender provider.Sender
}

func (r *stubRegistry) Get(notification.Channel) (provider.Sender, error) {
	return r.sender, nil
}

// stubPublisher records every ID it was asked to publish.
type stubPublisher struct {
	mu  sync.Mutex
	err error
	ids []string
}

func (p *stubPublisher) Publish(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ids = append(p.ids, id)
	return p.err
}

func (p *stubPublisher) published() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.ids...)
}

func TestCreatePublishesDeliveryJob(t *testing.T) {
	repo := repository.NewMemoryRepository()
	publisher := &stubPublisher{}
	svc := NewNotificationService(repo, nil, WithPublisher(publisher))

	n, err := svc.Create(context.Background(), notification.ChannelEmail, "user@example.com", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n.Status != notification.StatusPending {
		t.Fatalf("expected status %q, got %q", notification.StatusPending, n.Status)
	}
	if got := publisher.published(); len(got) != 1 || got[0] != n.ID {
		t.Fatalf("expected publish of [%s], got %v", n.ID, got)
	}
}

func TestCreatePublishFailurePropagates(t *testing.T) {
	repo := repository.NewMemoryRepository()
	publisher := &stubPublisher{err: errors.New("broker unavailable")}
	svc := NewNotificationService(repo, nil, WithPublisher(publisher))

	n, err := svc.Create(context.Background(), notification.ChannelEmail, "user@example.com", "hi")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The notification is still saved as pending even though nothing will
	// ever pick it up - see the comment in Create about the dual-write gap.
	got, getErr := repo.Get(context.Background(), n.ID)
	if getErr != nil {
		t.Fatalf("unexpected error on get: %v", getErr)
	}
	if got.Status != notification.StatusPending {
		t.Fatalf("expected status %q, got %q", notification.StatusPending, got.Status)
	}
}

func TestCreateWithoutPublisherStaysPending(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, nil)

	n, err := svc.Create(context.Background(), notification.ChannelPush, "device-token", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Status != notification.StatusPending {
		t.Fatalf("expected status %q, got %q", notification.StatusPending, n.Status)
	}
}

func TestCreateValidation(t *testing.T) {
	svc := NewNotificationService(repository.NewMemoryRepository(), nil)
	ctx := context.Background()

	cases := []struct {
		name      string
		channel   notification.Channel
		recipient string
		message   string
		wantErr   error
	}{
		{"invalid channel", notification.Channel("sms"), "a", "b", ErrInvalidChannel},
		{"empty recipient", notification.ChannelEmail, "", "b", ErrEmptyRecipient},
		{"empty message", notification.ChannelEmail, "a", "", ErrEmptyMessage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, tc.channel, tc.recipient, tc.message); !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func seed(t *testing.T, repo repository.Repository, n *notification.Notification) {
	t.Helper()
	if err := repo.Save(context.Background(), n); err != nil {
		t.Fatalf("unexpected error seeding notification: %v", err)
	}
}

func TestDispatchSendsAndMarksSent(t *testing.T) {
	sender := &stubSender{sent: make(chan *notification.Notification, 1)}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(3))

	n := &notification.Notification{ID: "abc123", Channel: notification.ChannelEmail, Status: notification.StatusPending}
	seed(t, repo, n)

	if err := svc.Dispatch(context.Background(), n.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-sender.sent:
		if got.ID != n.ID {
			t.Fatalf("expected sender to receive notification %s, got %s", n.ID, got.ID)
		}
	default:
		t.Fatal("expected sender to have been called")
	}

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != notification.StatusSent {
		t.Fatalf("expected status %q, got %q", notification.StatusSent, got.Status)
	}
	if got.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", got.Attempts)
	}
}

func TestDispatchFailureRetriesThenMarksFailed(t *testing.T) {
	const maxAttempts = 3
	sender := &stubSender{err: errors.New("boom"), sent: make(chan *notification.Notification, maxAttempts)}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(maxAttempts))

	n := &notification.Notification{ID: "abc123", Channel: notification.ChannelWebhook, Status: notification.StatusPending}
	seed(t, repo, n)

	if err := svc.Dispatch(context.Background(), n.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != maxAttempts {
		t.Fatalf("expected %d delivery attempts, got %d", maxAttempts, len(sender.sent))
	}

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != notification.StatusFailed {
		t.Fatalf("expected status %q, got %q", notification.StatusFailed, got.Status)
	}
	if got.Attempts != maxAttempts {
		t.Fatalf("expected %d attempts recorded, got %d", maxAttempts, got.Attempts)
	}
	if got.LastError != "boom" {
		t.Fatalf("expected last_error %q, got %q", "boom", got.LastError)
	}
}

func TestDispatchSucceedsAfterTransientFailure(t *testing.T) {
	sender := &flakySender{failures: 1}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(3))

	n := &notification.Notification{ID: "abc123", Channel: notification.ChannelPush, Status: notification.StatusPending}
	seed(t, repo, n)

	if err := svc.Dispatch(context.Background(), n.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != notification.StatusSent {
		t.Fatalf("expected status %q, got %q", notification.StatusSent, got.Status)
	}
	if got.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", got.Attempts)
	}
}

func TestDispatchUnknownChannelMarksFailedImmediately(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &provider.Registry{}, fastRetryPolicy(3))

	n := &notification.Notification{ID: "abc123", Channel: notification.Channel("sms"), Status: notification.StatusPending}
	seed(t, repo, n)

	if err := svc.Dispatch(context.Background(), n.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != notification.StatusFailed {
		t.Fatalf("expected status %q, got %q", notification.StatusFailed, got.Status)
	}
	if got.Attempts != 0 {
		t.Fatalf("expected 0 attempts, got %d", got.Attempts)
	}
}

// A redelivered job for a notification that a previous run already resolved
// must not be sent again.
func TestDispatchSkipsAlreadyResolvedNotification(t *testing.T) {
	sender := &stubSender{sent: make(chan *notification.Notification, 1)}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(3))

	n := &notification.Notification{ID: "abc123", Channel: notification.ChannelEmail, Status: notification.StatusSent, Attempts: 1}
	seed(t, repo, n)

	if err := svc.Dispatch(context.Background(), n.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-sender.sent:
		t.Fatal("expected sender not to be called for an already-resolved notification")
	default:
	}
}

func TestDispatchNotFoundReturnsError(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &provider.Registry{})

	if err := svc.Dispatch(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
