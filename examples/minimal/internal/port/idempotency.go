package port

import "context"

// IdempotencyStore checks and records operation idempotency keys.
type IdempotencyStore interface {
	// Check returns true if the key was already processed, along with the cached response.
	Check(ctx context.Context, key string) (bool, []byte, error)
	// Save records a processed key with its response payload.
	Save(ctx context.Context, key string, data []byte) error
}
