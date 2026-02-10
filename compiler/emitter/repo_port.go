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
