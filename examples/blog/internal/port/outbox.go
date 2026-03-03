package port

import "context"

// OutboxMessage represents a pending outbox event.
type OutboxMessage struct {
	ID      string
	Topic   string
	Payload []byte
}

// OutboxRepository stores and retrieves outbox messages for reliable event delivery.
type OutboxRepository interface {
	// SaveEvent persists an outbox event within the current transaction.
	SaveEvent(ctx context.Context, id, topic string, payload []byte) error
	// ListPending returns unprocessed messages up to the given limit.
	ListPending(ctx context.Context, limit int) ([]OutboxMessage, error)
	// MarkProcessed marks a message as delivered.
	MarkProcessed(ctx context.Context, id string) error
}
