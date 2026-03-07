package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
)

// EmitTransactionPort генерирует интерфейс менеджера транзакций
func (e *Emitter) EmitTransactionPort() error {
	tmplPath := filepath.Join(e.TemplatesDir, "tx_port.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/tx_port.tmpl" // Fallback
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("tx_port").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := formatGoStrict(buf.Bytes(), "internal/port/tx.go")
	if err != nil {
		return err
	}

	path := filepath.Join(targetDir, "tx.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Transaction Port: %s\n", path)
	return nil
}

// EmitIdempotencyPort генерирует интерфейс IdempotencyStore
func (e *Emitter) EmitIdempotencyPort() error {
	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	src := []byte(`package port

import "context"

// IdempotencyStore checks and records operation idempotency keys.
type IdempotencyStore interface {
	// Check returns true if the key was already processed, along with the cached response.
	Check(ctx context.Context, key string) (bool, []byte, error)
	// Save records a processed key with its response payload.
	Save(ctx context.Context, key string, data []byte) error
}
`)
	formatted, err := formatGoStrict(src, "internal/port/idempotency.go")
	if err != nil {
		return err
	}
	path := filepath.Join(targetDir, "idempotency.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Idempotency Port: %s\n", path)
	return nil
}

// EmitStateStorePort generates port.StateStore interface.
func (e *Emitter) EmitStateStorePort() error {
	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	src := []byte(`package port

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
`)
	formatted, err := formatGoStrict(src, "internal/port/statestore.go")
	if err != nil {
		return err
	}
	path := filepath.Join(targetDir, "statestore.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated State Store Port: %s\n", path)
	return nil
}

// EmitPolicyPort generates port.PolicyEngine interface and policy decision contracts.
func (e *Emitter) EmitPolicyPort() error {
	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	src := []byte(`package port

import "context"

// PolicyInput is the canonical payload used by policy.Evaluate/Require/Decide flow actions.
type PolicyInput struct {
	PolicyKey string
	Subject   string
	Resource  string
	Operation string
	Tenant    string
	Attrs     any
	Context   any
}

// PolicyDecision describes resolved policy verdict and optional side effects.
type PolicyDecision struct {
	Decision string
	Reason   string
	Effects  map[string]any
}

// PolicyEngine evaluates policy inputs into deterministic decisions.
type PolicyEngine interface {
	Evaluate(ctx context.Context, input PolicyInput) (PolicyDecision, error)
}
`)
	formatted, err := formatGoStrict(src, "internal/port/policy.go")
	if err != nil {
		return err
	}
	path := filepath.Join(targetDir, "policy.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Policy Port: %s\n", path)
	return nil
}

// EmitOutboxPort генерирует интерфейс OutboxRepository
func (e *Emitter) EmitOutboxPort() error {
	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	src := []byte(`package port

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
`)
	formatted, err := formatGoStrict(src, "internal/port/outbox.go")
	if err != nil {
		return err
	}
	path := filepath.Join(targetDir, "outbox.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Outbox Port: %s\n", path)
	return nil
}

// EmitSystemRepository generates a Postgres adapter that satisfies both
// port.IdempotencyStore and port.OutboxRepository using two simple tables.
func (e *Emitter) EmitSystemRepository() error {
	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "repository", "postgres")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	src := []byte(`package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"` + e.GoModule + `/internal/port"
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
`)

	goFmt, err := formatGoStrict(src, "internal/adapter/repository/postgres/systemrepository.go")
	if err != nil {
		return err
	}

	path := filepath.Join(targetDir, "systemrepository.go")
	if err := WriteFileIfChanged(path, goFmt, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated System Repository: %s\n", path)
	return nil
}

// EmitRepository генерирует интерфейсы репозиториев
func (e *Emitter) EmitRepository(repos []ir.Repository, entities []ir.Entity) error {
	nRepos := IRReposToNormalizer(repos)
	nEntities := IREntitiesToNormalizer(entities)

	// Build entity-name → hasID map
	entityHasID := make(map[string]bool)
	for _, ent := range nEntities {
		for _, f := range ent.Fields {
			if strings.ToLower(f.Name) == "id" {
				entityHasID[ent.Name] = true
				break
			}
		}
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	keep := make(map[string]struct{})
	for _, repo := range nRepos {
		rendered, err := e.renderRepositoryPortAST(repo, entityHasID[repo.Entity])
		if err != nil {
			return fmt.Errorf("render repository %s: %w", repo.Name, err)
		}
		filename := fmt.Sprintf("%s.go", strings.ToLower(repo.Name))
		path := filepath.Join(targetDir, filename)
		keep[filename] = struct{}{}
		if err := WriteFileIfChanged(path, rendered, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated Repository: %s\n", path)
	}

	if err := pruneGeneratedFiles(
		targetDir,
		keep,
		func(name string) bool {
			return strings.HasSuffix(name, "repository.go")
		},
		func(path string) bool {
			return fileContainsAny(path, "defines storage operations for")
		},
	); err != nil {
		return fmt.Errorf("prune stale repository ports: %w", err)
	}
	return nil
}
