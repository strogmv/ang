package port

import (
	"context"
	"time"
)

// StateStore provides generic key-value storage for workflow and application state.
type StateStore interface {
	// Get retrieves a value by key. Returns nil if not found.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores a value with optional TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete removes a value by key.
	Delete(ctx context.Context, key string) error
}
