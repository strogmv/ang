package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/strogmv/ang/internal/port"
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

// Compile-time interface checks.
var _ port.IdempotencyStore = (*SystemRepository)(nil)
var _ port.OutboxRepository = (*SystemRepository)(nil)
