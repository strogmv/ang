package port

import (
	"context"
	"github.com/strogmv/ang/internal/domain"
)

type Publisher interface {
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
	Enqueue(ctx context.Context, subject string, payload []byte) error
}
