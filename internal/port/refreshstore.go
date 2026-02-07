package port

import (
	"context"
	"time"
)

type RefreshTokenRecord struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
}

// RefreshTokenStore handles refresh token persistence and rotation.
type RefreshTokenStore interface {
	Save(ctx context.Context, token, userID string, expiresAt time.Time) error
	Rotate(ctx context.Context, oldToken, newToken, userID string, expiresAt time.Time) error
	Revoke(ctx context.Context, token string) error
	RevokeAll(ctx context.Context, userID string) error
	Find(ctx context.Context, token string) (*RefreshTokenRecord, error)
}
