package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// GDPRTemplateData is the top-level context for gdpr.tmpl.
type GDPRTemplateData struct {
	Entities     []normalizer.Entity
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

// HasAnyGDPRPolicy returns true when at least one entity has a GDPR policy.
func HasAnyGDPRPolicy(entities []normalizer.Entity) bool {
	for _, e := range entities {
		if e.GDPRPolicy != nil {
			return true
		}
	}
	return false
}

// EmitGDPR generates internal/service/gdpr.gen.go for entities with @gdpr annotations.
// The file is skipped entirely when no entity has a GDPR policy.
func (e *Emitter) EmitGDPR(entities []ir.Entity) error {
	norm := IREntitiesToNormalizer(entities)
	if !HasAnyGDPRPolicy(norm) {
		return nil
	}

	tmplContent, err := ReadTemplateByPath("templates/gdpr.tmpl")
	if err != nil {
		return fmt.Errorf("gdpr: read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["PIIFields"] = func(fields []normalizer.Field) []normalizer.Field {
		var pii []normalizer.Field
		for _, f := range fields {
			if f.IsPII {
				pii = append(pii, f)
			}
		}
		return pii
	}
	funcMap["ZeroLiteral"] = func(goType string) string {
		switch {
		case goType == "string":
			return `""`
		case goType == "bool":
			return "false"
		case strings.HasPrefix(goType, "int"), strings.HasPrefix(goType, "float"), goType == "uint64":
			return "0"
		case goType == "time.Time":
			return "time.Time{}"
		case strings.HasPrefix(goType, "*"):
			return "nil"
		default:
			return `""`
		}
	}
	funcMap["HasAuditLog"] = func() bool { return false }
	funcMap["ToLower"] = strings.ToLower

	t, err := template.New("gdpr").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("gdpr: parse template: %w", err)
	}

	ctx := GDPRTemplateData{
		Entities:     norm,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("gdpr: execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return raw source on format error so the user can diagnose
		formatted = buf.Bytes()
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "service")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("gdpr: mkdir: %w", err)
	}

	outPath := filepath.Join(targetDir, "gdpr.gen.go")
	if err := writeFileAtomic(outPath, formatted, 0644); err != nil {
		return fmt.Errorf("gdpr: write: %w", err)
	}
	fmt.Printf("Generated: %s\n", outPath)
	return nil
}
