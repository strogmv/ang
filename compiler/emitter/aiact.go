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

// AIOperationMeta holds the EU AI Act metadata for a single AI-augmented operation.
type AIOperationMeta struct {
	OperationID string
	RiskLevel   string
	UseCase     string
	Oversight   string
}

// AIActTemplateData is the top-level context for aiact.tmpl.
type AIActTemplateData struct {
	AIOperations []AIOperationMeta
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

func collectAIOperations(services []normalizer.Service) []AIOperationMeta {
	var ops []AIOperationMeta
	seen := map[string]bool{}
	for _, svc := range services {
		for _, m := range svc.Methods {
			hasAI := false
			for _, step := range m.Flow {
				if strings.HasPrefix(step.Action, "claude.") {
					hasAI = true
					break
				}
			}
			if !hasAI {
				continue
			}
			key := svc.Name + "." + m.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			op := AIOperationMeta{
				OperationID: key,
				RiskLevel:   "limited",
				UseCase:     "",
				Oversight:   "human_review",
			}
			if m.AIActPolicy != nil {
				if m.AIActPolicy.Risk != "" {
					op.RiskLevel = m.AIActPolicy.Risk
				}
				op.UseCase = m.AIActPolicy.UseCase
				op.Oversight = m.AIActPolicy.Oversight
			}
			ops = append(ops, op)
		}
	}
	return ops
}

// EmitAIAct generates internal/compliance/aiact.gen.go with EU AI Act compliance helpers.
func (e *Emitter) EmitAIAct(services []ir.Service) error {
	norm := IRServicesToNormalizer(services)
	ops := collectAIOperations(norm)
	if len(ops) == 0 {
		// Remove stale file if no AI operations present.
		outPath := e.outDir("internal", "compliance", "aiact.gen.go")
		if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	tmplContent, err := e.ReadTemplate("aiact.tmpl")
	if err != nil {
		return fmt.Errorf("aiact: read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["Title"] = func(s string) string {
		if len(s) == 0 {
			return ""
		}
		return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}

	t, err := template.New("aiact").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("aiact: parse template: %w", err)
	}

	ctx := AIActTemplateData{
		AIOperations: ops,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("aiact: execute: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Use unformatted output rather than failing; compilation will catch real errors.
		formatted = buf.Bytes()
	}

	targetDir := e.outDir("internal", "compliance")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("aiact: mkdir: %w", err)
	}

	outPath := filepath.Join(targetDir, "aiact.gen.go")
	if err := writeFileAtomic(outPath, formatted, 0644); err != nil {
		return fmt.Errorf("aiact: write: %w", err)
	}
	fmt.Printf("Generated: %s\n", outPath)
	return nil
}
