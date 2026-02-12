package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/strogmv/ang/internal/port"
)

// Dispatcher routes notification messages to configured channel sinks.
type Dispatcher struct {
	InAppSink port.NotificationInAppSink
}

// NewDispatcher builds a runtime dispatcher with channel-specific sinks.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

// Dispatch delivers message to requested channels, or to default channels when omitted.
func (d *Dispatcher) Dispatch(ctx context.Context, msg port.NotificationMessage) error {
	channels := msg.Channels
	if len(channels) == 0 {
		channels = []string{
			"in_app",
		}
	}
	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		switch channel {
		case "in_app":
			if d.InAppSink == nil {
				return fmt.Errorf("notification sink %q is not configured", channel)
			}
			if err := d.InAppSink.Send(ctx, msg); err != nil {
				return fmt.Errorf("send via %s: %w", channel, err)
			}
		default:
			return fmt.Errorf("notification channel %q is not supported", channel)
		}
	}
	return nil
}
