package provider

import (
	"fmt"

	"github.com/PakSerg/NotiHub/internal/notification"
)

type Registry struct {
	senders map[notification.Channel]Sender
}

func NewRegistry() *Registry {
	return &Registry{
		senders: map[notification.Channel]Sender{
			notification.ChannelEmail:   NewEmailSender(),
			notification.ChannelWebhook: NewWebhookSender(),
			notification.ChannelPush:    NewPushSender(),
		},
	}
}

func (r *Registry) Get(channel notification.Channel) (Sender, error) {
	sender, ok := r.senders[channel]
	if !ok {
		return nil, fmt.Errorf("no sender registered for channel %q", channel)
	}
	return sender, nil
}
