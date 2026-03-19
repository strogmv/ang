package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/minimal/internal/port"
)

// SystemRepository implements IdempotencyStore and OutboxRepository via Postgres.
type SystemRepository struct {
	DB *pgxpool.Pool
}

func NewSystemRepository(pool *pgxpool.Pool) *SystemRepository {
	return &SystemRepository{DB: pool}
}

// ---------- IdempotencyStore ----------

func (r *SystemRepository) Check(ctx context.Context, key string) (bool, []byte, error) {
	exec := getExecutor(ctx, r.DB)
	var data []byte
	err := exec.QueryRow(ctx, "SELECT response FROM idempotency_keys WHERE key = $1", key).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, data, nil
}

func (r *SystemRepository) Save(ctx context.Context, key string, data []byte) error {
	exec := getExecutor(ctx, r.DB)
	_, err := exec.Exec(ctx,
		"INSERT INTO idempotency_keys (key, response) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET response = $2",
		key, data)
	return err
}

// ---------- OutboxRepository ----------

func (r *SystemRepository) SaveEvent(ctx context.Context, id, topic string, payload []byte) error {
	exec := getExecutor(ctx, r.DB)
	_, err := exec.Exec(ctx,
		"INSERT INTO outbox_events (id, topic, payload) VALUES ($1, $2, $3)",
		id, topic, payload)
	return err
}

func (r *SystemRepository) ListPending(ctx context.Context, limit int) ([]port.OutboxMessage, error) {
	exec := getExecutor(ctx, r.DB)
	rows, err := exec.Query(ctx,
		"SELECT id, topic, payload FROM outbox_events WHERE processed_at IS NULL ORDER BY id LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []port.OutboxMessage
	for rows.Next() {
		var m port.OutboxMessage
		if err := rows.Scan(&m.ID, &m.Topic, &m.Payload); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *SystemRepository) MarkProcessed(ctx context.Context, id string) error {
	exec := getExecutor(ctx, r.DB)
	_, err := exec.Exec(ctx, "UPDATE outbox_events SET processed_at = NOW() WHERE id = $1", id)
	return err
}

// ---------- StateStore ----------

func (r *SystemRepository) Get(ctx context.Context, key string) ([]byte, error) {
	exec := getExecutor(ctx, r.DB)
	var data []byte
	err := exec.QueryRow(ctx, "SELECT value FROM kv_store WHERE key = $1", key).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (r *SystemRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	exec := getExecutor(ctx, r.DB)
	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}
	_, err := exec.Exec(ctx,
		"INSERT INTO kv_store (key, value, expires_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO UPDATE SET value = $2, expires_at = $3",
		key, value, expiresAt)
	return err
}

func (r *SystemRepository) Delete(ctx context.Context, key string) error {
	exec := getExecutor(ctx, r.DB)
	_, err := exec.Exec(ctx, "DELETE FROM kv_store WHERE key = $1", key)
	return err
}

// Compile-time interface checks.
var _ port.IdempotencyStore = (*SystemRepository)(nil)
var _ port.OutboxRepository = (*SystemRepository)(nil)
var _ port.StateStore = (*SystemRepository)(nil)
