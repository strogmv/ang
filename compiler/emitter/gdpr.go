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

type GDPREntityData struct {
	Entity             normalizer.Entity
	Receiver           string
	RepoField          string
	OwnerField         string
	OwnerFieldGo       string
	OwnerFieldType     string
	GDPRFields         []normalizer.Field
	SupportsErase      bool
	SupportsExport     bool
	SupportsRetention  bool
	EraseSkipReason    string
	ExportSkipReason   string
	RetentionSkipReason string
}

// GDPRTemplateData is the top-level context for gdpr.tmpl.
type GDPRTemplateData struct {
	Entities     []GDPREntityData
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

func HasAnyGDPRPolicy(entities []normalizer.Entity) bool {
	for _, e := range entities {
		if e.GDPRPolicy != nil {
			return true
		}
	}
	return false
}

func buildGDPREntityData(entities []normalizer.Entity) []GDPREntityData {
	out := make([]GDPREntityData, 0, len(entities))
	for _, e := range entities {
		if e.GDPRPolicy == nil {
			continue
		}

		item := GDPREntityData{
			Entity:    e,
			Receiver:  ExportName(firstNonEmpty(e.Owner, e.BoundedContext, e.Name)) + "Impl",
			RepoField: ExportName(e.Name) + "Repo",
		}

		for _, f := range e.Fields {
			if isGDPRManagedField(f) {
				item.GDPRFields = append(item.GDPRFields, f)
			}
		}

		ownerField, ok := findFieldByName(e.Fields, e.GDPRPolicy.OwnerField)
		if !ok {
			item.EraseSkipReason = fmt.Sprintf("owner field %q not found on entity", e.GDPRPolicy.OwnerField)
			item.ExportSkipReason = item.EraseSkipReason
		} else {
			item.OwnerField = ownerField.Name
			item.OwnerFieldGo = ExportName(ownerField.Name)
			item.OwnerFieldType = ownerField.Type
			if ownerField.Type != "string" {
				reason := fmt.Sprintf("owner field %q has unsupported type %q; only string owner IDs are supported", ownerField.Name, ownerField.Type)
				item.EraseSkipReason = reason
				item.ExportSkipReason = reason
			} else {
				item.SupportsErase = e.GDPRPolicy.Erasable
				item.SupportsExport = e.GDPRPolicy.Exportable
			}
		}

		if e.GDPRPolicy.Retention != "" {
			idField, hasID := findFieldByName(e.Fields, "id")
			createdAtField, hasCreatedAt := findFieldByName(e.Fields, "createdAt")
			switch {
			case !hasID:
				item.RetentionSkipReason = `required field "id" not found on entity`
			case idField.Type != "string":
				item.RetentionSkipReason = fmt.Sprintf(`field %q has unsupported type %q; retention purge requires string IDs`, idField.Name, idField.Type)
			case !hasCreatedAt:
				item.RetentionSkipReason = `required field "createdAt" not found on entity`
			case createdAtField.Type != "time.Time":
				item.RetentionSkipReason = fmt.Sprintf(`field %q has unsupported type %q; retention purge requires time.Time`, createdAtField.Name, createdAtField.Type)
			default:
				item.SupportsRetention = true
			}
		}

		out = append(out, item)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func findFieldByName(fields []normalizer.Field, want string) (normalizer.Field, bool) {
	want = strings.TrimSpace(want)
	if want == "" {
		return normalizer.Field{}, false
	}
	wantExported := ExportName(want)
	for _, f := range fields {
		if strings.EqualFold(f.Name, want) || strings.EqualFold(ExportName(f.Name), wantExported) {
			return f, true
		}
	}
	return normalizer.Field{}, false
}

func isGDPRManagedField(f normalizer.Field) bool {
	if f.IsPII || f.IsSecret {
		return true
	}
	if f.Metadata == nil {
		return false
	}
	_, ok := f.Metadata["gdpr"]
	return ok
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// EmitGDPR generates internal/service/gdpr.gen.go for entities with @gdpr annotations.
// The file is skipped entirely when no entity has a GDPR policy.
func (e *Emitter) EmitGDPR(entities []ir.Entity) error {
	targetDir := e.outDir("internal", "service")
	outPath := filepath.Join(targetDir, "gdpr.gen.go")

	norm := IREntitiesToNormalizer(entities)
	if !HasAnyGDPRPolicy(norm) {
		return removeFileIfExists(outPath)
	}

	tmplContent, err := ReadTemplateByPath("templates/gdpr.tmpl")
	if err != nil {
		return fmt.Errorf("gdpr: read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["ZeroLiteral"] = func(goType string) string {
		switch {
		case goType == "string":
			return `""`
		case goType == "bool":
			return "false"
		case goType == "any" || goType == "interface{}":
			return "nil"
		case strings.HasPrefix(goType, "int"), strings.HasPrefix(goType, "float"), strings.HasPrefix(goType, "uint"):
			return "0"
		case goType == "time.Time":
			return "time.Time{}"
		case strings.HasPrefix(goType, "*"), strings.HasPrefix(goType, "[]"), strings.HasPrefix(goType, "map["):
			return "nil"
		case strings.Contains(goType, "."):
			return goType + "{}"
		default:
			return goType + "{}"
		}
	}
	funcMap["ToLower"] = strings.ToLower

	t, err := template.New("gdpr").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("gdpr: parse template: %w", err)
	}

	ctx := GDPRTemplateData{
		Entities:     buildGDPREntityData(norm),
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
		formatted = buf.Bytes()
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("gdpr: mkdir: %w", err)
	}

	if err := writeFileAtomic(outPath, formatted, 0644); err != nil {
		return fmt.Errorf("gdpr: write: %w", err)
	}
	fmt.Printf("Generated: %s\n", outPath)
	return nil
}
