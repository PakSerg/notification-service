package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PakSerg/NotiHub/internal/notification"
	"github.com/PakSerg/NotiHub/internal/provider"
	"github.com/PakSerg/NotiHub/internal/repository"
)

// stubSender records the notifications it was asked to send and returns err
// for every call.
type stubSender struct {
	err  error
	sent chan *notification.Notification
}

func (s *stubSender) Send(ctx context.Context, n *notification.Notification) error {
	s.sent <- n
	return s.err
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
	svc := NewNotificationService(repo, &stubRegistry{sender: sender})

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
}

func TestCreateDispatchFailureMarksFailed(t *testing.T) {
	sender := &stubSender{err: errors.New("boom"), sent: make(chan *notification.Notification, 1)}
	repo := repository.NewMemoryRepository()
	svc := NewNotificationService(repo, &stubRegistry{sender: sender})

	n, err := svc.Create(context.Background(), notification.ChannelWebhook, "https://example.com/hook", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForStatus(t, repo, n.ID, notification.StatusFailed)
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
