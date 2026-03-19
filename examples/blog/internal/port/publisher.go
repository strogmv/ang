package port

import (
	"context"
	"github.com/example/blog/internal/domain"
	"time"
)

type Publisher interface {
	PublishCommentCreated(ctx context.Context, event domain.CommentCreated) error
	BroadcastCommentCreated(ctx context.Context, event domain.CommentCreated) error
	PublishPostCreated(ctx context.Context, event domain.PostCreated) error
	BroadcastPostCreated(ctx context.Context, event domain.PostCreated) error
	PublishPostPublished(ctx context.Context, event domain.PostPublished) error
	BroadcastPostPublished(ctx context.Context, event domain.PostPublished) error
	PublishPostUpdated(ctx context.Context, event domain.PostUpdated) error
	BroadcastPostUpdated(ctx context.Context, event domain.PostUpdated) error
	PublishUserLoggedIn(ctx context.Context, event domain.UserLoggedIn) error
	BroadcastUserLoggedIn(ctx context.Context, event domain.UserLoggedIn) error
	PublishUserRegistered(ctx context.Context, event domain.UserRegistered) error
	BroadcastUserRegistered(ctx context.Context, event domain.UserRegistered) error
	// Wait blocks until an event with matching correlation arrives or timeout occurs.
	Wait(ctx context.Context, name string, match string) (any, error)
	// Subscribe registers an async handler for events with matching correlation.
	Subscribe(ctx context.Context, name string, match string, handler func(context.Context, any)) error
}

// QueuePublisher sends arbitrary messages to a named subject/queue.
type QueuePublisher interface {
	// Enqueue publishes payload to a subject.
	Enqueue(ctx context.Context, subject string, payload []byte) error
	// Dequeue blocks until a message is available or timeout/context expires.
	Dequeue(ctx context.Context, subject string, timeout time.Duration) (messageID string, payload []byte, err error)
	// Ack acknowledges message processing for brokers that require explicit ack.
	Ack(ctx context.Context, subject string, messageID string) error
	// Nack signals failed processing for brokers that support negative acknowledgement.
	Nack(ctx context.Context, subject string, messageID string, reason string) error
	// PublishDLQ sends failed payloads to dead-letter stream/topic.
	PublishDLQ(ctx context.Context, subject string, payload []byte, reason string) error
}
