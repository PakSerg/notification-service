package notification

import "time"

type Channel string

const (
	ChannelEmail   Channel = "email"
	ChannelWebhook Channel = "webhook"
	ChannelPush    Channel = "push"
)

func (c Channel) Valid() bool {
	switch c {
	case ChannelEmail, ChannelWebhook, ChannelPush:
		return true
	default:
		return false
	}
}

type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

type Notification struct {
	ID        string    `json:"id"`
	Channel   Channel   `json:"channel"`
	Recipient string    `json:"recipient"`
	Message   string    `json:"message"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// Attempts is how many delivery attempts have been made so far.
	Attempts int `json:"attempts"`
	// LastError is the error message of the most recent failed attempt.
	// Empty once the notification is sent, or before any attempt has failed.
	LastError string `json:"last_error,omitempty"`
}
