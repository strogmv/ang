package authredis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/strogmv/ang/internal/port"
)

const (
	refreshPrefix = "refresh:"
	userPrefix    = "refresh_user:"
)

type Store struct {
	client *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

func (s *Store) Save(ctx context.Context, token, userID string, expiresAt time.Time) error {
	rec := port.RefreshTokenRecord{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	b, _ := json.Marshal(rec)
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = time.Hour
	}
	if err := s.client.Set(ctx, refreshPrefix+token, b, ttl).Err(); err != nil {
		return err
	}
	return s.client.SAdd(ctx, userPrefix+userID, token).Err()
}

func (s *Store) Rotate(ctx context.Context, oldToken, newToken, userID string, expiresAt time.Time) error {
	rec, err := s.Find(ctx, oldToken)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("refresh token not found")
	}
	if rec.Revoked {
		_ = s.RevokeAll(ctx, rec.UserID)
		return fmt.Errorf("refresh token replay detected")
	}
	rec.Revoked = true
	b, _ := json.Marshal(rec)
	ttl := time.Until(rec.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Hour
	}
	if err := s.client.Set(ctx, refreshPrefix+oldToken, b, ttl).Err(); err != nil {
		return err
	}
	_ = s.client.SRem(ctx, userPrefix+rec.UserID, oldToken).Err()
	return s.Save(ctx, newToken, userID, expiresAt)
}

func (s *Store) Revoke(ctx context.Context, token string) error {
	rec, err := s.Find(ctx, token)
	if err != nil || rec == nil {
		return err
	}
	rec.Revoked = true
	b, _ := json.Marshal(rec)
	ttl := time.Until(rec.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Hour
	}
	if err := s.client.Set(ctx, refreshPrefix+token, b, ttl).Err(); err != nil {
		return err
	}
	_ = s.client.SRem(ctx, userPrefix+rec.UserID, token).Err()
	return nil
}

func (s *Store) RevokeAll(ctx context.Context, userID string) error {
	tokens, err := s.client.SMembers(ctx, userPrefix+userID).Result()
	if err != nil {
		return err
	}
	for _, tok := range tokens {
		_ = s.Revoke(ctx, tok)
	}
	return s.client.Del(ctx, userPrefix+userID).Err()
}

func (s *Store) Find(ctx context.Context, token string) (*port.RefreshTokenRecord, error) {
	raw, err := s.client.Get(ctx, refreshPrefix+token).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec port.RefreshTokenRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
