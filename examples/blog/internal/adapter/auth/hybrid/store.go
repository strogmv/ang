package authhybrid

import (
	"context"
	"time"

	authpg "github.com/strogmv/ang/internal/adapter/auth/postgres"
	authredis "github.com/strogmv/ang/internal/adapter/auth/redis"
	"github.com/strogmv/ang/internal/port"
)

type Store struct {
	pg    *authpg.Store
	redis *authredis.Store
}

func NewStore(pg *authpg.Store, redis *authredis.Store) *Store {
	return &Store{pg: pg, redis: redis}
}

func (s *Store) Save(ctx context.Context, token, userID string, expiresAt time.Time) error {
	if s.pg != nil {
		if err := s.pg.Save(ctx, token, userID, expiresAt); err != nil {
			return err
		}
	}
	if s.redis != nil {
		_ = s.redis.Save(ctx, token, userID, expiresAt)
	}
	return nil
}

func (s *Store) Rotate(ctx context.Context, oldToken, newToken, userID string, expiresAt time.Time) error {
	if s.pg != nil {
		if err := s.pg.Rotate(ctx, oldToken, newToken, userID, expiresAt); err != nil {
			return err
		}
	}
	if s.redis != nil {
		_ = s.redis.Rotate(ctx, oldToken, newToken, userID, expiresAt)
	}
	return nil
}

func (s *Store) Revoke(ctx context.Context, token string) error {
	if s.pg != nil {
		if err := s.pg.Revoke(ctx, token); err != nil {
			return err
		}
	}
	if s.redis != nil {
		_ = s.redis.Revoke(ctx, token)
	}
	return nil
}

func (s *Store) RevokeAll(ctx context.Context, userID string) error {
	if s.pg != nil {
		if err := s.pg.RevokeAll(ctx, userID); err != nil {
			return err
		}
	}
	if s.redis != nil {
		_ = s.redis.RevokeAll(ctx, userID)
	}
	return nil
}

func (s *Store) Find(ctx context.Context, token string) (*port.RefreshTokenRecord, error) {
	if s.redis != nil {
		rec, err := s.redis.Find(ctx, token)
		if err == nil && rec != nil && !rec.Revoked {
			return rec, nil
		}
	}
	if s.pg != nil {
		rec, err := s.pg.Find(ctx, token)
		if err != nil || rec == nil {
			return rec, err
		}
		if s.redis != nil && !rec.Revoked {
			_ = s.redis.Save(ctx, token, rec.UserID, rec.ExpiresAt)
		}
		return rec, nil
	}
	return nil, nil
}
