package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
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

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "tx.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
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
	formatted, err := format.Source(src)
	if err != nil {
		formatted = src
	}
	path := filepath.Join(targetDir, "idempotency.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Idempotency Port: %s\n", path)
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
	formatted, err := format.Source(src)
	if err != nil {
		formatted = src
	}
	path := filepath.Join(targetDir, "outbox.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
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

// Compile-time interface checks.
var _ port.IdempotencyStore = (*SystemRepository)(nil)
var _ port.OutboxRepository = (*SystemRepository)(nil)
`)

	goFmt, err := format.Source(src)
	if err != nil {
		fmt.Printf("SystemRepository format error: %v, writing raw\n", err)
		goFmt = src
	}

	path := filepath.Join(targetDir, "systemrepository.go")
	if err := os.WriteFile(path, goFmt, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated System Repository: %s\n", path)
	return nil
}

// EmitRepository генерирует интерфейсы репозиториев
func (e *Emitter) EmitRepository(repos []ir.Repository, entities []ir.Entity) error {
	nRepos := IRReposToNormalizer(repos)
	_ = entities

	tmplPath := filepath.Join(e.TemplatesDir, "repository.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/repository.tmpl" // Fallback
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := template.FuncMap{
		"ExportName": ExportName,
		"GoModule":   func() string { return e.GoModule },
	}

	t, err := template.New("repository").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	for _, repo := range nRepos {
		fmt.Printf("DEBUG repo_port: Processing repo %s with %d finders\n", repo.Entity, len(repo.Finders))
		for _, f := range repo.Finders {
			fmt.Printf("DEBUG repo_port: Finder %s ReturnType='%s' Returns='%s' CustomSQL len=%d\n", f.Name, f.ReturnType, f.Returns, len(f.CustomSQL))
		}
		var buf bytes.Buffer

		type finderOut struct {
			normalizer.RepositoryFinder
			ParamsSig  string
			ReturnType string
		}

		var finders []finderOut
		hasTime := false
		for _, f := range repo.Finders {
			fo := finderOut{RepositoryFinder: f}

			// Debug: print ReturnType for finders that have it
			if f.ReturnType != "" {
				fmt.Printf("DEBUG: Finder %s.%s has ReturnType: %s\n", repo.Entity, f.Name, f.ReturnType)
			}

			// Compute return type - use explicit ReturnType if provided
			if f.ReturnType != "" {
				fo.ReturnType = f.ReturnType
			} else if f.Action == "delete" {
				fo.ReturnType = "int64"
			} else if f.Returns == "one" {
				fo.ReturnType = "*domain." + repo.Entity
			} else if f.Returns == "many" {
				fo.ReturnType = "[]domain." + repo.Entity
			} else if f.Returns == "count" {
				fo.ReturnType = "int64"
			} else if f.Returns == "[]"+repo.Entity {
				fo.ReturnType = "[]domain." + repo.Entity
			} else if f.Returns == repo.Entity || f.Returns == "*"+repo.Entity {
				fo.ReturnType = "*domain." + repo.Entity
			} else {
				fo.ReturnType = f.Returns
			}

			// Compute params signature
			var params []string
			for _, w := range f.Where {
				pType := w.ParamType
				if pType == "time" || pType == "time.Time" {
					pType = "time.Time"
					hasTime = true
				}
				params = append(params, fmt.Sprintf("%s %s", w.Param, pType))
			}
			fo.ParamsSig = strings.Join(params, ", ")

			finders = append(finders, fo)
		}

		data := struct {
			normalizer.Repository
			Finders []finderOut
			HasTime bool
		}{
			Repository: repo,
			Finders:    finders,
			HasTime:    hasTime,
		}

		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Printf("Formatting failed for %s. Writing raw.\n", repo.Name)
			formatted = buf.Bytes()
		}

		filename := fmt.Sprintf("%s.go", strings.ToLower(repo.Name))
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated Repository: %s\n", path)
	}
	return nil
}
