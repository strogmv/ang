package emitter

import (
	"bytes"
	"database/sql"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitPostgresRepo генерирует реализацию репозитория для Postgres
func (e *Emitter) EmitPostgresRepo(repos []normalizer.Repository, entities []normalizer.Entity) error {
	tmplPath := filepath.Join(e.TemplatesDir, "postgres_repo.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/postgres_repo.tmpl" // Fallback
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	entMap := make(map[string]normalizer.Entity)
	for _, ent := range entities {
		entMap[ent.Name] = ent
	}

	funcMap := e.getSharedFuncMap()
	funcMap["IsTimestampString"] = func(f normalizer.Field) bool {
		return f.Type == "string" && strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP")
	}
	funcMap["NeedsScanVar"] = func(f normalizer.Field) bool {
		return f.IsOptional || (f.Type == "string" && strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP"))
	}
	funcMap["ScanVarName"] = func(f normalizer.Field) string {
		return strings.ToLower(f.Name) + "Val"
	}
	funcMap["ScanVarType"] = func(f normalizer.Field) string {
		if f.IsOptional {
			return funcMap["ScanType"].(func(normalizer.Field) string)(f)
		}
		if f.Type == "string" && strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP") {
			return "string"
		}
		return funcMap["ScanType"].(func(normalizer.Field) string)(f)
	}
	funcMap["ScanType"] = func(f normalizer.Field) string {
		switch f.Type {
		case "string":
			return "sql.NullString"
		case "int", "int64":
			return "sql.NullInt64"
		case "float64":
			return "sql.NullFloat64"
		case "bool":
			return "sql.NullBool"
		case "time.Time":
			return "sql.NullTime"
		default:
			return "sql.NullString"
		}
	}
	funcMap["ScanArg"] = func(f normalizer.Field) string {
		if f.IsOptional {
			return "&" + strings.ToLower(f.Name) + "Val"
		}
		if f.Type == "string" && strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP") {
			return "&" + strings.ToLower(f.Name) + "Val"
		}
		return "&entity." + ExportName(f.Name)
	}
	funcMap["ScanAssign"] = func(f normalizer.Field, varName string) string {
		if strings.HasPrefix(f.Type, "map[") || f.Type == "any" || f.Type == "interface{}" {
			return "unmarshalJSON(" + varName + ".String, &entity." + ExportName(f.Name) + ")"
		}
		switch f.Type {
		case "string":
			if strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP") {
				if f.IsOptional {
					return "entity." + ExportName(f.Name) + " = normalizeTimeString(" + varName + ".String)"
				}
				return "entity." + ExportName(f.Name) + " = normalizeTimeString(" + varName + ")"
			}
			return "entity." + ExportName(f.Name) + " = " + varName + ".String"
		case "int":
			return "entity." + ExportName(f.Name) + " = int(" + varName + ".Int64)"
		case "int64":
			return "entity." + ExportName(f.Name) + " = " + varName + ".Int64"
		case "float64":
			return "entity." + ExportName(f.Name) + " = " + varName + ".Float64"
		case "bool":
			return "entity." + ExportName(f.Name) + " = " + varName + ".Bool"
		case "time.Time":
			return "entity." + ExportName(f.Name) + " = " + varName + ".Time"
		default:
			return "entity." + ExportName(f.Name) + " = " + varName + ".String"
		}
	}

	t, err := template.New("postgres_repo").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "repository", "postgres")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	for _, repo := range repos {
		ent, ok := entMap[repo.Entity]
		if !ok {
			continue
		}

		var cols []string
		var placeholders []string
		var updateSets []string
		var insertArgs []string
		var selectCols []string
		hasTime := false

		for _, f := range ent.Fields {
			if f.SkipDomain {
				continue
			}
			colName := DBName(f.Name)
			cols = append(cols, colName)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(cols)))

			if colName != "id" {
				updateSets = append(updateSets, fmt.Sprintf("%s = EXCLUDED.%s", colName, colName))
			}

			arg := "entity." + ExportName(f.Name)
			if f.IsOptional {
				if f.Type == "time.Time" || f.Type == "*time.Time" {
					arg = "nullTime(" + arg + ")"
				} else if f.Type == "int" || f.Type == "int64" {
					arg = "nullInt(" + arg + ")"
				} else if strings.HasPrefix(f.Type, "map[") || f.Type == "any" || f.Type == "interface{}" {
					arg = "nullJSON(" + arg + ")"
				} else {
					arg = "nullString(" + arg + ")"
				}
			}
			insertArgs = append(insertArgs, arg)

			sCol := colName
			if f.Type == "string" && f.DB.Type != "TEXT" && f.DB.Type != "" {
				sCol = colName + "::text"
			} else if f.Type == "time.Time" || f.Type == "*time.Time" || strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP") {
				sCol = colName + "::text"
			}
			selectCols = append(selectCols, sCol)
		}

		type finderOut struct {
			normalizer.RepositoryFinder
			ParamsSig          string
			Args               string
			ReturnType         string
			ReturnZero         string
			ReturnSlice        bool
			SelectCols         string
			WhereClause        string
			OrderBySQL         string
			SelectEntity       bool
			SelectCustomEntity bool   // For custom domain types like TenderReportInfo
			CustomEntityName   string // The entity name for custom types
			SelectFields       []normalizer.Field
		}

		var finders []finderOut
		seenMethods := map[string]bool{
			"Save":     true,
			"FindByID": true,
			"Delete":   true,
		}

		for _, f := range repo.Finders {
			if seenMethods[f.Name] {
				continue
			}
			seenMethods[f.Name] = true

			fo := finderOut{
				RepositoryFinder: f,
				SelectEntity:     true,
			}

			// Always default SelectCols to all columns if we are selecting an entity
			fo.SelectCols = strings.Join(selectCols, ", ")

			// If explicit ReturnType is set, use it directly
			if f.ReturnType != "" {
				fo.ReturnType = f.ReturnType
				fo.ReturnZero = "nil"
				fo.SelectEntity = false

				// Parse return type to determine if slice and entity name
				retType := f.ReturnType
				isSlice := strings.HasPrefix(retType, "[]")
				if isSlice {
					retType = strings.TrimPrefix(retType, "[]")
					fo.ReturnSlice = true
				}
				retType = strings.TrimPrefix(retType, "*")
				retType = strings.TrimPrefix(retType, "domain.")

				// Try to find matching entity for custom struct types
				if customEnt, ok := entMap[retType]; ok {
					fo.SelectCustomEntity = true
					fo.CustomEntityName = retType

					// If scan_fields is specified, use that order
					if len(f.ScanFields) > 0 {
						entFieldsMap := make(map[string]normalizer.Field)
						for _, field := range customEnt.Fields {
							entFieldsMap[strings.ToLower(field.Name)] = field
						}
						for _, sf := range f.ScanFields {
							if field, ok := entFieldsMap[strings.ToLower(sf)]; ok {
								fo.SelectFields = append(fo.SelectFields, field)
							}
						}
					} else {
						// Use all non-complex fields by default
						for _, field := range customEnt.Fields {
							if field.Name == "DTO" || strings.HasPrefix(field.Type, "[]") {
								continue
							}
							fo.SelectFields = append(fo.SelectFields, field)
						}
					}
				} else if len(f.ScanFields) > 0 {
					// Entity not in entMap (likely a DTO), create fields from scan_fields
					fo.SelectCustomEntity = true
					fo.CustomEntityName = retType
					for _, sf := range f.ScanFields {
						// Infer type from field name pattern or default to string
						fieldType := "string"
						sfLower := strings.ToLower(sf)
						if strings.HasSuffix(sfLower, "id") || strings.HasSuffix(sfLower, "amount") ||
							strings.HasSuffix(sfLower, "price") || strings.HasSuffix(sfLower, "number") ||
							strings.HasSuffix(sfLower, "bids") || strings.HasSuffix(sfLower, "drops") {
							fieldType = "int"
						} else if strings.HasSuffix(sfLower, "savings") || strings.HasSuffix(sfLower, "rating") {
							fieldType = "float64"
						}
						fo.SelectFields = append(fo.SelectFields, normalizer.Field{
							Name: sf,
							Type: fieldType,
						})
					}
				}
			} else if f.Action == "delete" {
				fo.ReturnType = "int64"
				fo.ReturnZero = "0"
				fo.SelectEntity = false
			} else if f.Returns == "one" {
				fo.ReturnType = "*domain." + repo.Entity
				fo.ReturnZero = "nil"
				fo.SelectFields = ent.Fields
			} else if f.Returns == "many" {
				fo.ReturnType = "[]domain." + repo.Entity
				fo.ReturnZero = "nil"
				fo.ReturnSlice = true
				fo.SelectFields = ent.Fields
			} else if f.Returns == "count" {
				fo.ReturnType = "int64"
				fo.ReturnZero = "0"
				fo.SelectEntity = false
				fo.SelectCols = "COUNT(*)"
				fo.SelectFields = []normalizer.Field{{Name: "count", Type: "int64"}}
			} else if f.Returns == "[]"+repo.Entity {
				fo.ReturnType = "[]domain." + repo.Entity
				fo.ReturnZero = "nil"
				fo.ReturnSlice = true
				fo.SelectFields = ent.Fields
			} else if f.Returns == repo.Entity || f.Returns == "*"+repo.Entity {
				fo.ReturnType = "*domain." + repo.Entity
				fo.ReturnZero = "nil"
				fo.SelectFields = ent.Fields
			} else {
				fo.ReturnType = f.Returns
				fo.ReturnZero = "nil" // Default
				fo.SelectEntity = false
				fo.SelectCols = strings.Join(f.Select, ", ")
				if fo.SelectCols == "" {
					fo.SelectCols = "*" // Fallback
				}

				// Check if return type is a custom domain entity (e.g., *domain.TenderReportInfo or []domain.TenderBidHistoryItem)
				retType := f.Returns
				isSlice := strings.HasPrefix(retType, "[]")
				if isSlice {
					retType = strings.TrimPrefix(retType, "[]")
					fo.ReturnSlice = true
				}
				retType = strings.TrimPrefix(retType, "*")
				retType = strings.TrimPrefix(retType, "domain.")

				// Try to find matching entity for custom struct types
				if customEnt, ok := entMap[retType]; ok {
					fo.SelectCustomEntity = true
					fo.CustomEntityName = retType

					// If scan_fields is specified, use that order and filter entity fields
					if len(f.ScanFields) > 0 {
						// Build a map of entity fields by lowercase name
						entFieldsMap := make(map[string]normalizer.Field)
						for _, field := range customEnt.Fields {
							entFieldsMap[strings.ToLower(field.Name)] = field
						}
						// Iterate in scan_fields order
						for _, sf := range f.ScanFields {
							if field, ok := entFieldsMap[strings.ToLower(sf)]; ok {
								fo.SelectFields = append(fo.SelectFields, field)
							}
						}
					} else {
						// Use all non-complex fields by default
						for _, field := range customEnt.Fields {
							// Skip DTO field, slice fields, and nested struct types
							if field.Name == "DTO" || strings.HasPrefix(field.Type, "[]") {
								continue
							}
							fo.SelectFields = append(fo.SelectFields, field)
						}
					}
				} else {
					// Map select column names back to Field objects
					for _, col := range f.Select {
						for _, field := range ent.Fields {
							if strings.EqualFold(field.Name, col) {
								fo.SelectFields = append(fo.SelectFields, field)
								break
							}
						}
					}
					// If still empty but we have a return type, create a dummy field for the template
					if len(fo.SelectFields) == 0 && fo.ReturnType != "" {
						fo.SelectFields = []normalizer.Field{
							{Name: "v", Type: fo.ReturnType},
						}
					}
				}
			}

			var params []string
			var args []string
			var wheres []string
			for _, w := range f.Where {
				pType := w.ParamType
				if pType == "time" || pType == "time.Time" {
					pType = "time.Time"
					hasTime = true
				}
				params = append(params, fmt.Sprintf("%s %s", w.Param, pType))
				args = append(args, w.Param)
				wheres = append(wheres, fmt.Sprintf("%s %s $%s", strings.ToLower(w.Field), w.Op, fmt.Sprint(len(args))))
			}
			fo.ParamsSig = strings.Join(params, ", ")
			fo.Args = strings.Join(args, ", ")
			fo.WhereClause = strings.Join(wheres, " AND ")
			fo.OrderBySQL = f.OrderBy

			finders = append(finders, fo)
		}

		data := struct {
			Name          string
			Entity        string
			Table         string
			Columns       string
			Placeholders  string
			UpdateSet     string
			InsertArgs    string
			SelectColumns string
			Fields        []normalizer.Field
			Finders       []finderOut
			HasTime       bool
		}{
			Name:          repo.Name,
			Entity:        repo.Entity,
			Table:         strings.ToLower(repo.Entity) + "s",
			Columns:       strings.Join(cols, ", "),
			Placeholders:  strings.Join(placeholders, ", "),
			UpdateSet:     strings.Join(updateSets, ", "),
			InsertArgs:    strings.Join(insertArgs, ", "),
			SelectColumns: strings.Join(selectCols, ", "),
			Fields:        ent.Fields,
			Finders:       finders,
			HasTime:       hasTime,
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Printf("Formatting failed for %s postgres repo. Writing raw.\n", repo.Name)
			formatted = buf.Bytes()
		}

		filename := fmt.Sprintf("%s.go", strings.ToLower(repo.Name))
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated Postgres Repo: %s\n", path)
	}

	return nil
}

// EmitPostgresCommon generates shared utils for Postgres repos
func (e *Emitter) EmitPostgresCommon() error {
	tmplPath := filepath.Join(e.TemplatesDir, "postgres_common.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/postgres_common.tmpl" // Fallback
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("postgres_common").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "repository", "postgres")
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

	path := filepath.Join(targetDir, "common.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Postgres Common: %s\n", path)
	return nil
}

// Ensure optional field scanners are referenced.
var _ = sql.NullString{}
