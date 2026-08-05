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
)

type Notification struct {
	ID        string    `json:"id"`
	Channel   Channel   `json:"channel"`
	Recipient string    `json:"recipient"`
	Message   string    `json:"message"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
