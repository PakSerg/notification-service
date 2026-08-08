package provider

import (
	"testing"

	"github.com/PakSerg/NotiHub/internal/notification"
)

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	sender, err := r.Get(notification.ChannelEmail)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender == nil {
		t.Fatal("expected sender, got nil")
	}
}

func TestRegistryGetUnknownChannel(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get(notification.Channel("sms"))
	if err == nil {
		t.Fatal("expected error for unknown channel, got nil")
	}
}
