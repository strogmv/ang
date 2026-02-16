package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/strogmv/ang/internal/port"
)

// Dispatcher routes notification messages to configured channel sinks.
type Dispatcher struct {
	InAppSink       port.NotificationInAppSink
	UserMuteChecker func(ctx context.Context, userID string, msg port.NotificationMessage) (bool, error)
}

type dispatchPolicy struct {
	Event    string
	Type     string
	Audience string
	Channels []string
	Template string
	MuteKey  string
}

var dispatchPolicies = []dispatchPolicy{}

// NewDispatcher builds a runtime dispatcher with channel-specific sinks.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{}
	return d
}

// Dispatch delivers message to requested channels, or to default channels when omitted.
func (d *Dispatcher) Dispatch(ctx context.Context, msg port.NotificationMessage) error {
	msg = applyDispatchPolicy(msg)
	if d.UserMuteChecker != nil && strings.TrimSpace(msg.UserID) != "" {
		muted, err := d.UserMuteChecker(ctx, msg.UserID, msg)
		if err != nil {
			return fmt.Errorf("resolve notification mute: %w", err)
		}
		if muted {
			return nil
		}
	}
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

func applyDispatchPolicy(msg port.NotificationMessage) port.NotificationMessage {
	for _, rule := range dispatchPolicies {
		if strings.TrimSpace(rule.Event) != "" && !strings.EqualFold(strings.TrimSpace(rule.Event), strings.TrimSpace(msg.Event)) {
			continue
		}
		if strings.TrimSpace(rule.Type) != "" && !strings.EqualFold(strings.TrimSpace(rule.Type), strings.TrimSpace(msg.Type)) {
			continue
		}
		if strings.TrimSpace(rule.Audience) != "" && !strings.EqualFold(strings.TrimSpace(rule.Audience), strings.TrimSpace(msg.Audience)) {
			continue
		}
		if strings.TrimSpace(msg.Type) == "" && strings.TrimSpace(rule.Type) != "" {
			msg.Type = strings.TrimSpace(rule.Type)
		}
		if len(msg.Channels) == 0 && len(rule.Channels) > 0 {
			msg.Channels = append([]string(nil), rule.Channels...)
		}
		if strings.TrimSpace(msg.Template) == "" && strings.TrimSpace(rule.Template) != "" {
			msg.Template = strings.TrimSpace(rule.Template)
		}
		if strings.TrimSpace(msg.MuteKey) == "" && strings.TrimSpace(rule.MuteKey) != "" {
			msg.MuteKey = strings.TrimSpace(rule.MuteKey)
		}
		return msg
	}
	return msg
}
