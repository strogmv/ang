package authpg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/strogmv/ang/internal/port"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Save(ctx context.Context, token, userID string, expiresAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("db not configured")
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO refreshtokens (id, userid, expiresat, revoked)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (id) DO UPDATE
		SET userid = EXCLUDED.userid, expiresat = EXCLUDED.expiresat, revoked = false
	`, token, userID, expiresAt)
	return err
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
	if err := s.Revoke(ctx, oldToken); err != nil {
		return err
	}
	return s.Save(ctx, newToken, userID, expiresAt)
}

func (s *Store) Revoke(ctx context.Context, token string) error {
	if s.db == nil {
		return fmt.Errorf("db not configured")
	}
	_, err := s.db.Exec(ctx, `UPDATE refreshtokens SET revoked = true WHERE id = $1`, token)
	return err
}

func (s *Store) RevokeAll(ctx context.Context, userID string) error {
	if s.db == nil {
		return fmt.Errorf("db not configured")
	}
	_, err := s.db.Exec(ctx, `UPDATE refreshtokens SET revoked = true WHERE userid = $1`, userID)
	return err
}

func (s *Store) Find(ctx context.Context, token string) (*port.RefreshTokenRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	row := s.db.QueryRow(ctx, `SELECT id, userid, expiresat, revoked FROM refreshtokens WHERE id = $1`, token)
	var rec port.RefreshTokenRecord
	if err := row.Scan(&rec.Token, &rec.UserID, &rec.ExpiresAt, &rec.Revoked); err != nil {
		return nil, nil
	}
	return &rec, nil
}
