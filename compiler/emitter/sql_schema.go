package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/sqlmigrate"
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
		return inferSQLType(f)
	}

	t, err := template.New("schema_sql").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := e.outDir("db", "schema")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var fullSchema bytes.Buffer

	// System Tables
	fullSchema.WriteString(systemTablesDDL)
	fullSchema.WriteString("\n\n")

	names := newSQLRelationNames()
	for _, entity := range entitiesNorm {
		if hasSQLTable(entity) && entityHasID(entity) {
			if err := names.addTable(entity.TableName(), entity.Name, entity.Source); err != nil {
				return err
			}
		}
	}

	for _, entity := range entitiesNorm {
		entity = ensurePrimaryKey(entity)

		if !hasSQLTable(entity) {
			continue
		}
		if !entityHasID(entity) {
			continue
		}

		if err := t.Execute(&fullSchema, entity); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		tableName := entity.TableName()
		for _, f := range entity.Fields {
			if f.DB.Index {
				colName := DBName(f.Name)
				idxName := e.sqlIndexName(fmt.Sprintf("idx_%s_%s", tableName, colName), entity.Source)
				stmt := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s);", idxName, tableName, colName)
				if err := names.addIndex(idxName, tableName, stmt, entity.Source); err != nil {
					return err
				}
				fullSchema.WriteString("\n" + stmt)
			}
		}
		for _, index := range entity.Indexes {
			stmt, err := e.sqlIndexStatement(entity, tableName, index, names)
			if err != nil {
				return err
			}
			fullSchema.WriteString("\n" + stmt)
		}
		checkNames := map[string]bool{}
		for _, check := range entity.Checks {
			stmt, err := sqlCheckStatement(entity, tableName, check, checkNames)
			if err != nil {
				return err
			}
			fullSchema.WriteString("\n" + stmt)
		}
		if len(entity.DDL) > 0 {
			// The markers let ang migrate diff leave raw SQL out of what it
			// hands to Atlas (which neither inspects nor plans it).
			fullSchema.WriteString("\n" + sqlmigrate.DDLBeginMarker + " " + entity.Name)
			for _, ddl := range entity.DDL {
				fullSchema.WriteString("\n" + ddl)
			}
			fullSchema.WriteString("\n" + sqlmigrate.DDLEndMarker)
		}

		fullSchema.WriteString("\n\n")
	}

	path := filepath.Join(targetDir, "schema.sql")
	if err := WriteFileIfChanged(path, fullSchema.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	logGenerated("Generated SQL Schema: %s\n", path)
	return nil
}

func entityHasID(entity normalizer.Entity) bool {
	for _, f := range entity.Fields {
		if strings.ToLower(f.Name) == "id" {
			return true
		}
	}
	return false
}

// hasSQLTable says whether an entity is stored in PostgreSQL. It is not when it is
// declared @storage("mongo") (or has an ObjectId field), or when it is a DTO
// (@dto(only="true") / _dto: true): a shape of an answer or a projection that
// nothing saves. Such entities get no table, no SQL queries and no Postgres
// repository.
func hasSQLTable(entity normalizer.Entity) bool {
	if storage, ok := entity.Metadata["storage"].(string); ok && strings.EqualFold(storage, "mongo") {
		return false
	}
	if dto, ok := entity.Metadata["dto"].(bool); ok && dto {
		return false
	}
	for _, f := range entity.Fields {
		if strings.EqualFold(f.DB.Type, "ObjectId") {
			return false
		}
	}
	return true
}

func ensurePrimaryKey(entity normalizer.Entity) normalizer.Entity {
	hasPrimary := false
	for _, f := range entity.Fields {
		if f.DB.PrimaryKey {
			hasPrimary = true
			break
		}
	}
	if hasPrimary {
		return entity
	}
	for i := range entity.Fields {
		if strings.EqualFold(entity.Fields[i].Name, "id") {
			entity.Fields[i].DB.PrimaryKey = true
			return entity
		}
	}
	return entity
}

func inferSQLType(f normalizer.Field) string {
	if dbType := strings.TrimSpace(strings.ToUpper(f.DB.Type)); dbType != "" && !isFallbackTextDBType(dbType, f.Type) {
		return dbType
	}

	switch strings.TrimSpace(f.Type) {
	case "int", "int64":
		return "BIGINT"
	case "float32", "float64":
		return "DOUBLE PRECISION"
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

func isFallbackTextDBType(dbType, fieldType string) bool {
	if dbType != "TEXT" {
		return false
	}
	switch strings.TrimSpace(fieldType) {
	case "int", "int64", "float32", "float64", "bool", "time.Time", "*time.Time", "uuid":
		return true
	default:
		return false
	}
}

// EmitSQLQueries генерирует базовые CRUD запросы для SQLC (опционально)
func (e *Emitter) EmitSQLQueries(entities []ir.Entity) error {
	entitiesNorm := IREntitiesToNormalizer(entities)

	var buf bytes.Buffer

	for _, entity := range entitiesNorm {
		hasID := false
		for _, f := range entity.Fields {
			if strings.EqualFold(f.Name, "id") {
				hasID = true
			}
		}
		if !hasID || !hasSQLTable(entity) {
			continue
		}

		tableName := entity.TableName()

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

	path := e.outDir("db", "queries", "query.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	content := append([]byte("-- Code generated by ANG. DO NOT EDIT.\n\n"), buf.Bytes()...)
	if err := WriteFileIfChanged(path, content, 0644); err != nil {
		return err
	}
	logGenerated("Generated SQL Queries: %s\n", path)
	return nil
}
