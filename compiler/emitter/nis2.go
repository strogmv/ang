package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// NIS2TemplateData is the top-level context for nis2.tmpl.
type NIS2TemplateData struct {
	Services     []normalizer.Service
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

// EmitNIS2 generates internal/compliance/nis2.gen.go with NIS2 Directive compliance helpers.
func (e *Emitter) EmitNIS2(services []ir.Service) error {
	norm := IRServicesToNormalizer(services)

	tmplContent, err := ReadTemplateByPath("templates/nis2.tmpl")
	if err != nil {
		return fmt.Errorf("nis2: read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	t, err := template.New("nis2").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("nis2: parse template: %w", err)
	}

	ctx := NIS2TemplateData{
		Services:     norm,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("nis2: execute: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "compliance")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("nis2: mkdir: %w", err)
	}

	outPath := filepath.Join(targetDir, "nis2.gen.go")
	if err := writeFileAtomic(outPath, formatted, 0644); err != nil {
		return fmt.Errorf("nis2: write: %w", err)
	}
	fmt.Printf("Generated: %s\n", outPath)
	return nil
}
