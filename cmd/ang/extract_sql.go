package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[` + "`" + `"]?(\w+)[` + "`" + `"]?\s*\(([^;]*)\)`)
	columnRe      = regexp.MustCompile(`(?i)^\s*(\w+)\s+([\w\s\(\)]+?)(?:\s+(?:NOT\s+NULL|NULL|DEFAULT|PRIMARY|UNIQUE|REFERENCES|CHECK|ON)[^\n,]*)?(?:,|\s*$)`)
)

// extractSQLFacts reads SQL files and extracts table/column definitions.
func extractSQLFacts(path string) (FactsEnvelope, error) {
	var env FactsEnvelope

	files, err := collectSQLFiles(path)
	if err != nil {
		return env, err
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		entities := parseSQLEntities(string(data), f)
		env.Entities = append(env.Entities, entities...)
	}

	return env, nil
}

func collectSQLFiles(path string) ([]string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(p)
			if base == "vendor" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".sql") {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

func parseSQLEntities(content, source string) []FactEntity {
	var entities []FactEntity
	matches := createTableRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		tableName := m[1]
		body := m[2]
		entityName := singularPascal(tableName)
		fields := parseSQLColumns(body)
		entities = append(entities, FactEntity{
			Name:      entityName,
			TableHint: strings.ToLower(tableName),
			Source:    source,
			Fields:    fields,
		})
	}
	return entities
}

func parseSQLColumns(body string) []FactField {
	var fields []FactField
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		// Skip constraint/index lines
		if strings.HasPrefix(upper, "CONSTRAINT") ||
			strings.HasPrefix(upper, "PRIMARY KEY") ||
			strings.HasPrefix(upper, "UNIQUE") ||
			strings.HasPrefix(upper, "FOREIGN KEY") ||
			strings.HasPrefix(upper, "CHECK") ||
			strings.HasPrefix(upper, "INDEX") ||
			strings.HasPrefix(upper, "KEY ") {
			continue
		}

		m := columnRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		colName := m[1]
		colType := strings.TrimSpace(m[2])

		// Skip SQL keywords that appear as first token
		lowerName := strings.ToLower(colName)
		if lowerName == "constraint" || lowerName == "primary" || lowerName == "unique" ||
			lowerName == "foreign" || lowerName == "check" || lowerName == "index" {
			continue
		}

		cueHint := sqlTypeToCUE(colType)
		fields = append(fields, FactField{
			Name:        toPascalCase(colName),
			CueTypeHint: cueHint,
			DBTag:       strings.ToLower(colName),
		})
	}
	return fields
}

// sqlTypeToCUE maps SQL column type to CUE type hint.
func sqlTypeToCUE(sqlType string) string {
	upper := strings.ToUpper(strings.TrimSpace(sqlType))
	// strip size specifiers
	if idx := strings.Index(upper, "("); idx >= 0 {
		upper = upper[:idx]
	}
	upper = strings.TrimSpace(upper)

	switch {
	case upper == "VARCHAR" || upper == "TEXT" || upper == "CHAR" ||
		upper == "NVARCHAR" || upper == "CLOB" || upper == "TINYTEXT" ||
		upper == "MEDIUMTEXT" || upper == "LONGTEXT" || upper == "STRING":
		return "string"

	case upper == "INT" || upper == "INTEGER" || upper == "BIGINT" ||
		upper == "SMALLINT" || upper == "TINYINT" || upper == "MEDIUMINT" ||
		upper == "SERIAL" || upper == "BIGSERIAL" || upper == "SMALLSERIAL":
		return "int"

	case upper == "BOOLEAN" || upper == "BOOL":
		return "bool"

	case upper == "DECIMAL" || upper == "NUMERIC" || upper == "FLOAT" ||
		upper == "DOUBLE" || upper == "REAL" || upper == "FLOAT4" || upper == "FLOAT8":
		return "float"

	case upper == "TIMESTAMP" || upper == "TIMESTAMPTZ" ||
		upper == "TIMESTAMP WITH TIME ZONE" || upper == "TIMESTAMP WITHOUT TIME ZONE" ||
		upper == "DATE" || upper == "TIME" || upper == "TIMETZ":
		return "string"

	case upper == "UUID":
		return "string"

	case upper == "JSONB" || upper == "JSON":
		return "{...}"

	case upper == "BYTEA" || upper == "BLOB" || upper == "BINARY" || upper == "VARBINARY":
		return "bytes"

	default:
		return "string"
	}
}
