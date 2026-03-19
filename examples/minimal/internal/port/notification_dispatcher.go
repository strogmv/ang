package port

import "context"

// NotificationMessage is a transport-agnostic envelope for multi-channel delivery.
type NotificationMessage struct {
	Event    string
	Type     string
	UserID   string
	Audience string
	EntityID string
	Payload  any
	Channels []string
	Template string
	MuteKey  string
	Metadata map[string]any
}

// NotificationDispatcher routes notification message to configured channel sinks.
type NotificationDispatcher interface {
	Dispatch(ctx context.Context, msg NotificationMessage) error
}

// NotificationInAppSink delivers notifications via "in_app" channel.
type NotificationInAppSink interface {
	Send(ctx context.Context, msg NotificationMessage) error
}
