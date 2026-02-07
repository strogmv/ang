package authstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/strogmv/ang/internal/port"
)

type MemoryStore struct {
	mu     sync.Mutex
	tokens map[string]port.RefreshTokenRecord
	byUser map[string]map[string]struct{}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tokens: make(map[string]port.RefreshTokenRecord),
		byUser: make(map[string]map[string]struct{}),
	}
}

func (s *MemoryStore) Save(ctx context.Context, token, userID string, expiresAt time.Time) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = port.RefreshTokenRecord{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	if s.byUser[userID] == nil {
		s.byUser[userID] = make(map[string]struct{})
	}
	s.byUser[userID][token] = struct{}{}
	return nil
}

func (s *MemoryStore) Rotate(ctx context.Context, oldToken, newToken, userID string, expiresAt time.Time) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tokens[oldToken]
	if !ok {
		return fmt.Errorf("refresh token not found")
	}
	if rec.Revoked {
		// token replay detected, revoke all for this user
		for tok := range s.byUser[rec.UserID] {
			r := s.tokens[tok]
			r.Revoked = true
			s.tokens[tok] = r
		}
		return fmt.Errorf("refresh token replay detected")
	}
	rec.Revoked = true
	s.tokens[oldToken] = rec
	delete(s.byUser[rec.UserID], oldToken)
	s.tokens[newToken] = port.RefreshTokenRecord{
		Token:     newToken,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	if s.byUser[userID] == nil {
		s.byUser[userID] = make(map[string]struct{})
	}
	s.byUser[userID][newToken] = struct{}{}
	return nil
}

func (s *MemoryStore) Revoke(ctx context.Context, token string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tokens[token]
	if !ok {
		return nil
	}
	rec.Revoked = true
	s.tokens[token] = rec
	delete(s.byUser[rec.UserID], token)
	return nil
}

func (s *MemoryStore) RevokeAll(ctx context.Context, userID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for tok := range s.byUser[userID] {
		rec := s.tokens[tok]
		rec.Revoked = true
		s.tokens[tok] = rec
	}
	delete(s.byUser, userID)
	return nil
}

func (s *MemoryStore) Find(ctx context.Context, token string) (*port.RefreshTokenRecord, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tokens[token]
	if !ok {
		return nil, nil
	}
	return &rec, nil
}
