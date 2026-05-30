package notification

import (
	"context"
	"fmt"
	"log"
	"time"
)

// EventStore is the subset of Store methods used by Service.
type EventStore interface {
	ListEnabled(ctx context.Context, offline, online bool) ([]Channel, error)
}

// Service fans out offline/online notifications to all relevant channels.
type Service struct {
	store EventStore
}

// NewService creates a Service backed by the given store.
func NewService(s EventStore) *Service {
	return &Service{store: s}
}

// NotifyPingChange sends a notification when a switch goes offline or comes back online.
// switchID is reserved for future per-switch filtering; name and ip are used in the message.
// Sending is best-effort: errors are logged and never returned.
func (svc *Service) NotifyPingChange(switchID, name, ip string, online bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	channels, err := svc.store.ListEnabled(ctx, !online, online)
	if err != nil {
		log.Printf("notifications: list channels: %v", err)
		return
	}
	if len(channels) == 0 {
		return
	}

	msg := buildMessage(name, ip, online)
	for _, ch := range channels {
		ch := ch
		sender, err := NewSender(&ch)
		if err != nil {
			log.Printf("notifications[%s]: build sender: %v", ch.Name, err)
			continue
		}
		if err := sender.Send(ctx, msg); err != nil {
			log.Printf("notifications[%s]: send: %v", ch.Name, err)
		}
	}
}

func buildMessage(name, ip string, online bool) string {
	if online {
		return fmt.Sprintf("🟢 Switch Online: %s (%s)\nSwitch is reachable again.", name, ip)
	}
	return fmt.Sprintf("🔴 Switch Offline: %s (%s)\nTwo consecutive ping failures — switch may be unreachable.", name, ip)
}
