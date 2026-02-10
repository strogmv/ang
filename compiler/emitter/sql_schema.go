package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitSQL генерирует SQL схему
func (e *Emitter) EmitSQL(entities []ir.Entity) error {
	entitiesNorm := IREntitiesToNormalizer(entities)

	tmplPath := filepath.Join(e.TemplatesDir, "schema_sql.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/schema_sql.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["SQLDefault"] = func(val string) string {
		if val == "" {
			return ""
		}
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			return fmt.Sprintf("'%s'", strings.Trim(val, "\""))
		}
		return val
	}
	funcMap["SQLType"] = func(f normalizer.Field) string {
		if f.DB.Type != "" {
			return f.DB.Type
		}
		switch f.Type {
		case "string":
			return "TEXT"
		case "int", "int64":
			return "BIGINT"
		case "bool":
			return "BOOLEAN"
		case "time.Time", "*time.Time":
			return "TIMESTAMPTZ"
		case "uuid":
			return "UUID"
		default:
			return "TEXT"
		}
	}

	t, err := template.New("schema_sql").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "db", "schema")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var fullSchema bytes.Buffer

	// System Tables
	fullSchema.WriteString("-- System Tables for Outbox and Idempotency\n")
	fullSchema.WriteString(`CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox_events (created_at) WHERE processed_at IS NULL;

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    response_payload JSONB,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ
);
`)
	fullSchema.WriteString("\n\n")

	for _, entity := range entitiesNorm {
		isMongo := false
		for _, f := range entity.Fields {
			if strings.EqualFold(f.DB.Type, "ObjectId") {
				isMongo = true
				break
			}
		}
		if isMongo {
			continue
		}

		hasID := false
		for _, f := range entity.Fields {
			if strings.ToLower(f.Name) == "id" {
				hasID = true
				break
			}
		}
		if !hasID {
			continue
		}

		if err := t.Execute(&fullSchema, entity); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		tableName := strings.ToLower(entity.Name) + "s"
		for _, f := range entity.Fields {
			if f.DB.Index {
				colName := DBName(f.Name)
				idxName := fmt.Sprintf("idx_%s_%s", tableName, colName)
				fullSchema.WriteString(fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS %s ON %s (%s);", idxName, tableName, colName))
			}
		}

		fullSchema.WriteString("\n\n")
	}

	path := filepath.Join(targetDir, "schema.sql")
	if err := os.WriteFile(path, fullSchema.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated SQL Schema: %s\n", path)
	return nil
}

// EmitSQLQueries генерирует базовые CRUD запросы для SQLC (опционально)
func (e *Emitter) EmitSQLQueries(entities []ir.Entity) error {
	entitiesNorm := IREntitiesToNormalizer(entities)

	var buf bytes.Buffer

	for _, entity := range entitiesNorm {
		isSQL := false
		hasID := false
		for _, f := range entity.Fields {
			if strings.EqualFold(f.Name, "id") {
				hasID = true
				if f.DB.Type != "ObjectId" {
					isSQL = true
				}
			}
		}

		if !hasID || !isSQL {
			continue
		}

		tableName := strings.ToLower(entity.Name) + "s"

		// Create
		buf.WriteString(fmt.Sprintf("-- name: Create%s :one\n", entity.Name))
		buf.WriteString(fmt.Sprintf("INSERT INTO %s (", tableName))

		var cols []string
		var vals []string
		for _, f := range entity.Fields {
			if f.SkipDomain {
				continue
			}
			cols = append(cols, DBName(f.Name))
			vals = append(vals, fmt.Sprintf("$%d", len(cols)))
		}
		buf.WriteString(strings.Join(cols, ", "))
		buf.WriteString(") VALUES (")
		buf.WriteString(strings.Join(vals, ", "))
		buf.WriteString(") RETURNING *;\n\n")

		// Get
		buf.WriteString(fmt.Sprintf("-- name: Get%s :one\n", entity.Name))
		buf.WriteString(fmt.Sprintf("SELECT * FROM %s WHERE id = $1 LIMIT 1;\n\n", tableName))

		// List
		buf.WriteString(fmt.Sprintf("-- name: List%s :many\n", entity.Name))
		buf.WriteString(fmt.Sprintf("SELECT * FROM %s ORDER BY id LIMIT $1 OFFSET $2;\n\n", tableName))
	}

	path := filepath.Join(e.OutputDir, "db", "queries", "query.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated SQL Queries: %s\n", path)
	return nil
}
