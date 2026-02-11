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

// EmitStubRepo генерирует memory implementation
func (e *Emitter) EmitStubRepo(repos []ir.Repository, entities []ir.Entity) error {
	reposNorm := IRReposToNormalizer(repos)
	entitiesNorm := IREntitiesToNormalizer(entities)

	tmplPath := filepath.Join(e.TemplatesDir, "stub_repo.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/stub_repo.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := template.FuncMap{
		"ExportName": ExportName,
		"Title":      ToTitle,
		"ToLower":    strings.ToLower,
		"GoModule":   func() string { return e.GoModule },
	}

	t, err := template.New("stub_repo").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "repository", "memory")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	entMap := make(map[string]normalizer.Entity)
	for _, ent := range entitiesNorm {
		entMap[ent.Name] = ent
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
			IsCustomType bool // True for custom types that can't be handled in memory
		}

		var finders []finderOut
		hasTime := false
		hasOrderBy := false

		for _, f := range repo.Finders {
			fo := finderOut{
				RepositoryFinder: f,
				SelectEntity:     true,
			}
			sig := ComputeFinderSignature(repo.Entity, f, "")
			fo.ReturnType = sig.ReturnType
			fo.ReturnZero = sig.ReturnZero
			fo.ReturnSlice = sig.ReturnSlice
			fo.ParamsSig = sig.ParamsSig
			hasTime = hasTime || sig.HasTime

			// Check for explicit ReturnType first (custom types like *domain.TenderReportInfo)
			if f.ReturnType != "" {
				fo.SelectEntity = false
				fo.IsCustomType = true // Custom types can't be handled in memory repos
			} else if f.Action == "delete" {
				fo.SelectEntity = false
			} else if f.Returns == "one" {
				fo.SelectFields = ent.Fields
			} else if f.Returns == "many" {
				fo.SelectFields = ent.Fields
			} else if f.Returns == "count" {
				fo.SelectEntity = false
				fo.SelectFields = []normalizer.Field{{Name: "count", Type: "int64"}}
			} else if f.Returns == "[]"+repo.Entity {
				fo.SelectFields = ent.Fields
			} else if f.Returns == repo.Entity || f.Returns == "*"+repo.Entity {
				fo.SelectFields = ent.Fields
			} else {
				fo.SelectEntity = false
				fo.IsCustomType = true // Custom types can't be handled in memory repos
			}

			// Order By
			if f.OrderBy != "" {
				hasOrderBy = true
				parts := strings.Fields(f.OrderBy)
				if len(parts) > 0 {
					fo.OrderByField = ExportName(parts[0])
					if len(parts) > 1 && strings.EqualFold(parts[1], "DESC") {
						fo.OrderByDesc = true
					}
				}
			}

			finders = append(finders, fo)
		}

		data := struct {
			Name       string
			Entity     string
			Finders    []finderOut
			HasTime    bool
			HasOrderBy bool
		}{
			Name:       repo.Name,
			Entity:     repo.Entity,
			Finders:    finders,
			HasTime:    hasTime,
			HasOrderBy: hasOrderBy,
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Printf("Formatting failed for %s stub. Writing raw.\n", repo.Name)
			formatted = buf.Bytes()
		}

		filename := fmt.Sprintf("%s.go", strings.ToLower(repo.Name))
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated Repo Stub: %s\n", path)
	}
	return nil
}
