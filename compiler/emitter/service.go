package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) EmitService(services []normalizer.Service) error {
	tmplPath := "templates/service.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	funcMap := e.getSharedFuncMap()
	funcMap["HasLogValue"] = func(fields []normalizer.Field) bool {
		for _, f := range fields {
			if f.IsSecret || f.IsPII {
				return true
			}
		}
		return false
	}

	t, err := template.New("service").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	for _, svc := range services {
		var buf bytes.Buffer
		if err := t.Execute(&buf, svc); err != nil {
			return err
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			formatted = buf.Bytes()
		}

		filename := strings.ToLower(svc.Name) + ".go"
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return err
		}
		fmt.Printf("Generated Service Port: %s\n", path)
	}

	return nil
}

func (e *Emitter) EmitServiceImpl(services []normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) error {
	tmplPath := "templates/service_impl.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	t, err := template.New("service_impl").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "service")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	for _, svc := range services {
		// Collect all imports for this service
		importMap := make(map[string]bool)

		// Base imports that every service needs
		baseImports := []string{
			"context",
			"encoding/json",
			"fmt",
			"log/slog",
			"net/http",
			"sort",
			"strings",
			"time",
			"github.com/google/uuid",
			"golang.org/x/crypto/bcrypt",
			e.GoModule + "/internal/config",
			e.GoModule + "/internal/domain",
			e.GoModule + "/internal/pkg/auth",
			e.GoModule + "/internal/pkg/errors",
			e.GoModule + "/internal/pkg/helpers",
			e.GoModule + "/internal/pkg/logger",
			e.GoModule + "/internal/pkg/presence",
			e.GoModule + "/internal/port",
		}
		for _, imp := range baseImports {
			importMap[imp] = true
		}

		// Add imports from methods
		for _, m := range svc.Methods {
			if m.Impl != nil {
				for _, imp := range m.Impl.Imports {
					imp = strings.Trim(imp, "\"")
					// Normalize some common names to full paths
					if imp == "http" {
						imp = "net/http"
					}
					if imp == "uuid" {
						imp = "github.com/google/uuid"
					}
					if imp != "" {
						importMap[imp] = true
					}
				}
			}
		}

		var allImports []string
		for imp := range importMap {
			allImports = append(allImports, imp)
		}
		sort.Strings(allImports)

		var buf bytes.Buffer
		a := auth
		if a == nil {
			a = &normalizer.AuthDef{}
		}
		data := struct {
			Service  normalizer.Service
			Entities []normalizer.Entity
			Auth     *normalizer.AuthDef
			Imports  []string
		}{
			Service:  svc,
			Entities: entities,
			Auth:     a,
			Imports:  allImports,
		}
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute template for %s: %w", svc.Name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Printf("Formatting failed for %s service impl. Writing raw. Error: %v\n", svc.Name, err)
			formatted = buf.Bytes()
		}

		filename := strings.ToLower(svc.Name) + ".go"
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return err
		}
		fmt.Printf("Generated Service Impl: %s\n", path)
	}

	return nil
}

func (e *Emitter) EmitCachedService(services []normalizer.Service) error {
	tmplPath := "templates/service_cached.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	t, err := template.New("service_cached").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "service")
	for _, svc := range services {
		var buf bytes.Buffer
		if err := t.Execute(&buf, svc); err != nil {
			return err
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			formatted = buf.Bytes()
		}

		filename := strings.ToLower(svc.Name) + "_cached.go"
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return err
		}
		fmt.Printf("Generated Cached Service: %s\n", path)
	}

	return nil
}
