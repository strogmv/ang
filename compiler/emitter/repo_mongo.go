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

// EmitMongoRepo generates repository implementations for MongoDB-backed entities.
func (e *Emitter) EmitMongoRepo(repos []ir.Repository, entities []ir.Entity) error {
	reposNorm := IRReposToNormalizer(repos)
	entitiesNorm := IREntitiesToNormalizer(entities)

	tmplPath := filepath.Join(e.TemplatesDir, "mongo_repo.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/mongo_repo.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	entMap := make(map[string]normalizer.Entity)
	for _, ent := range entitiesNorm {
		entMap[ent.Name] = ent
	}

	funcMap := e.getSharedFuncMap()
	funcMap["MongoBsonName"] = func(name string) string {
		if strings.EqualFold(name, "id") {
			return "_id"
		}
		return JSONName(name)
	}
	funcMap["MongoValueExpr"] = func(f normalizer.Field) string {
		expr := "entity." + ExportName(f.Name)
		if f.Type == "string" && strings.Contains(strings.ToUpper(f.DB.Type), "TIMESTAMP") {
			return "normalizeTime(" + expr + ")"
		}
		return expr
	}
	funcMap["HasTime"] = func(fields []normalizer.Field) bool {
		for _, f := range fields {
			if strings.EqualFold(f.Type, "time.Time") {
				return true
			}
		}
		return false
	}

	t, err := template.New("mongo_repo").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "repository", "mongo")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	type finderOut struct {
		normalizer.RepositoryFinder
		ParamsSig    string
		ReturnType   string
		ReturnZero   string
		ReturnSlice  bool
		SelectEntity bool
		SelectFields []normalizer.Field
		OrderByField string
		OrderByDesc  bool
		HasLimit     bool
		IsCount      bool
	}

	for _, repo := range reposNorm {
		ent, ok := entMap[repo.Entity]
		if !ok {
			continue
		}
		// Skip DTO-only entities — they have no database table.
		if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
			continue
		}
		if v, ok := ent.Metadata["storage"].(string); !ok || v != "mongo" {
			continue
		}

		var finders []finderOut
		for _, f := range repo.Finders {
			fo := finderOut{
				RepositoryFinder: f,
				SelectEntity:     true,
			}
			sig := ComputeFinderSignature(repo.Entity, f, "")
			fo.Name = sig.Name
			fo.ReturnType = sig.ReturnType
			fo.ReturnZero = sig.ReturnZero
			fo.ReturnSlice = sig.ReturnSlice
			fo.ParamsSig = sig.ParamsSig

			switch {
			case fo.ReturnType == "int64":
				fo.SelectEntity = false
				fo.IsCount = (f.Returns == "count")
			case fo.ReturnType == "*domain."+repo.Entity:
				fo.SelectFields = ent.Fields
			case fo.ReturnType == "[]domain."+repo.Entity:
				fo.SelectFields = ent.Fields
			default:
				return fmt.Errorf("mongo repo unsupported return type: %s (%s.%s)", f.Returns, repo.Name, f.Name)
			}

			// Order By
			if f.OrderBy != "" {
				parts := strings.Fields(f.OrderBy)
				if len(parts) > 0 {
					field := parts[0]
					if strings.EqualFold(field, "id") {
						fo.OrderByField = "_id"
					} else {
						fo.OrderByField = JSONName(field)
					}
					if len(parts) > 1 && strings.EqualFold(parts[1], "DESC") {
						fo.OrderByDesc = true
					}
				}
			}

			if f.Limit > 0 {
				fo.HasLimit = true
			}

			finders = append(finders, fo)
		}

		data := struct {
			Name       string
			Entity     string
			Collection string
			Fields     []normalizer.Field
			Finders    []finderOut
		}{
			Name:       repo.Name,
			Entity:     repo.Entity,
			Collection: strings.ToLower(repo.Entity) + "s",
			Fields:     ent.Fields,
			Finders:    finders,
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		formatted, err := formatGoStrict(buf.Bytes(), "internal/adapter/repository/mongo/"+strings.ToLower(repo.Name)+".go")
		if err != nil {
			return err
		}

		filename := fmt.Sprintf("%s.go", strings.ToLower(repo.Name))
		path := filepath.Join(targetDir, filename)
		if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated Mongo Repo: %s\n", path)
	}
	return nil
}

// EmitMongoCommon generates shared utilities for Mongo repositories.
func (e *Emitter) EmitMongoCommon(entities []ir.Entity) error {
	entitiesNorm := IREntitiesToNormalizer(entities)

	hasMongo := false
	for _, ent := range entitiesNorm {
		if v, ok := ent.Metadata["storage"].(string); ok && v == "mongo" {
			hasMongo = true
			break
		}
	}
	if !hasMongo {
		return nil
	}

	tmplPath := filepath.Join(e.TemplatesDir, "mongo_helpers.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/mongo_helpers.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("mongo_helpers").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "repository", "mongo")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := formatGoStrict(buf.Bytes(), "internal/adapter/repository/mongo/helpers.go")
	if err != nil {
		return err
	}

	path := filepath.Join(targetDir, "helpers.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Mongo Helpers: %s\n", path)
	return nil
}
