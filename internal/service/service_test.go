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

func waitForStatus(t *testing.T, repo repository.Repository, id string, want notification.Status) {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		n, err := repo.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n.Status == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for status %q, last was %q", want, n.Status)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestCreateDispatchesAndMarksSent(t *testing.T) {
	sender := &stubSender{sent: make(chan *notification.Notification, 1)}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(3))

	n, err := svc.Create(context.Background(), notification.ChannelEmail, "user@example.com", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-sender.sent:
		if got.ID != n.ID {
			t.Fatalf("expected sender to receive notification %s, got %s", n.ID, got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dispatch")
	}

	waitForStatus(t, repo, n.ID, notification.StatusSent)

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", got.Attempts)
	}
}

func TestCreateDispatchFailureRetriesThenMarksFailed(t *testing.T) {
	const maxAttempts = 3
	sender := &stubSender{err: errors.New("boom"), sent: make(chan *notification.Notification, maxAttempts)}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(maxAttempts))

	n, err := svc.Create(context.Background(), notification.ChannelWebhook, "https://example.com/hook", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForStatus(t, repo, n.ID, notification.StatusFailed)

	if len(sender.sent) != maxAttempts {
		t.Fatalf("expected %d delivery attempts, got %d", maxAttempts, len(sender.sent))
	}

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Attempts != maxAttempts {
		t.Fatalf("expected %d attempts recorded, got %d", maxAttempts, got.Attempts)
	}
	if got.LastError != "boom" {
		t.Fatalf("expected last_error %q, got %q", "boom", got.LastError)
	}
}

func TestCreateDispatchSucceedsAfterTransientFailure(t *testing.T) {
	sender := &flakySender{failures: 1}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender}, fastRetryPolicy(3))

	n, err := svc.Create(context.Background(), notification.ChannelPush, "device-token", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForStatus(t, repo, n.ID, notification.StatusSent)

	got, err := repo.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", got.Attempts)
	}
}

func TestCreateWithoutSendersStaysPending(t *testing.T) {
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
