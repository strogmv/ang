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

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitConfig generates typed configuration.
func (e *Emitter) EmitConfig(config *normalizer.ConfigDef) error {
	if config == nil {
		config = &normalizer.ConfigDef{Fields: []normalizer.Field{}}
	}
	tmplPath := "templates/config.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := template.FuncMap{
		"FieldName": func(f normalizer.Field) string {
			if f.Name == "" {
				return ""
			}
			// If already starts with uppercase, assume it's already exported/correct
			if len(f.Name) > 0 && f.Name[0] >= 'A' && f.Name[0] <= 'Z' {
				return f.Name
			}
			return ExportName(f.Name)
		},
		"ConfigType": func(f normalizer.Field) string {
			if f.Type == "" {
				return "string"
			}
			return f.Type
		},
		"ConfigDefault": func(f normalizer.Field) string {
			if f.Default == "" {
				switch f.Type {
				case "int":
					return "0"
				case "bool":
					return "false"
				default:
					return "\"\""
				}
			}
			if f.Type == "string" {
				return fmt.Sprintf("%q", strings.Trim(f.Default, "\""))
			}
			return f.Default
		},
		"ConfigLoadExpr": func(f normalizer.Field) string {
			def := func() string {
				switch f.Type {
				case "int":
					return "0"
				case "bool":
					return "false"
				default:
					return "\"\""
				}
			}()
			if f.Default != "" {
				if f.Type == "string" {
					def = fmt.Sprintf("%q", strings.Trim(f.Default, "\""))
				} else {
					def = f.Default
				}
			}
			if f.EnvVar == "" {
				return def
			}
			switch f.Type {
			case "int":
				return fmt.Sprintf("getEnvInt(%q, %s)", f.EnvVar, def)
			case "bool":
				return fmt.Sprintf("getEnvBool(%q, %s)", f.EnvVar, def)
			default:
				return fmt.Sprintf("getEnv(%q, %s)", f.EnvVar, def)
			}
		},
		"HasType": func(fields []normalizer.Field, typ string) bool {
			for _, f := range fields {
				if strings.EqualFold(f.Type, typ) {
					return true
				}
			}
			return false
		},
	}

	t, err := template.New("config").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "config")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, config); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Formatting failed for config. Writing raw.\n")
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "config.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Config: %s\n", path)
	return nil
}

// EmitLogger generates the logging package.
func (e *Emitter) EmitLogger() error {
	tmplPath := "templates/logger.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("logger").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "logger")
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

	path := filepath.Join(targetDir, "logger.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Logger: %s\n", path)
	return nil
}

// EmitTracing generates the tracing package.
func (e *Emitter) EmitTracing() error {
	tmplPath := "templates/tracing.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("tracing").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "tracing")
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

	path := filepath.Join(targetDir, "tracing.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Tracing: %s\n", path)
	return nil
}

// EmitErrors generates the error handling package.
func (e *Emitter) EmitErrors(errors []ir.Error) error {
	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "errors")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// 1. Generate errors.go (the logic)
	tmplPath := "templates/errors.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("errors").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, errors); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "errors.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Errors: %s\n", path)

	// 2. Generate codes.go (the registry)
	tmplPathCodes := "templates/error_codes.tmpl"
	tmplContentCodes, err := ReadTemplateByPath(tmplPathCodes)
	if err != nil {
		return fmt.Errorf("read codes template: %w", err)
	}

	tCodes, err := template.New("codes").Funcs(template.FuncMap{
		"ExportName": ExportName,
	}).Parse(string(tmplContentCodes))
	if err != nil {
		return fmt.Errorf("parse codes template: %w", err)
	}

	var bufCodes bytes.Buffer
	if err := tCodes.Execute(&bufCodes, errors); err != nil {
		return fmt.Errorf("execute codes template: %w", err)
	}

	formattedCodes, err := format.Source(bufCodes.Bytes())
	if err != nil {
		formattedCodes = bufCodes.Bytes()
	}

	pathCodes := filepath.Join(targetDir, "codes.go")
	if err := os.WriteFile(pathCodes, formattedCodes, 0644); err != nil {
		return fmt.Errorf("write codes file: %w", err)
	}
	fmt.Printf("Generated Error Codes: %s\n", pathCodes)

	return nil
}

// EmitViews generates view definitions for field-level security.
func (e *Emitter) EmitViews(views []ir.View) error {
	if len(views) == 0 {
		return nil
	}

	tmplPath := "templates/views.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("views").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "views")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, views); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "views.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Views: %s\n", path)
	return nil
}

// EmitScheduler generates a simple time-based scheduler.
func (e *Emitter) EmitScheduler(schedules []ir.Schedule) error {
	if len(schedules) == 0 {
		return nil
	}

	tmplPath := "templates/scheduler.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("scheduler").Funcs(template.FuncMap{
		"Title":      ToTitle,
		"ExportName": ExportName,
		"GoModule":   func() string { return e.GoModule },
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "scheduler")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	type scheduleOut struct {
		Name    string
		Service string
		Action  string
		At      string
		Publish string
		Every   string
		Payload []normalizer.SchedulePayloadField
	}
	var out []scheduleOut
	for _, s := range schedules {
		payload := make([]normalizer.SchedulePayloadField, 0, len(s.Payload))
		for _, p := range s.Payload {
			value := ""
			switch v := p.Default.(type) {
			case string:
				value = v
			case bool:
				if v {
					value = "true"
				} else {
					value = "false"
				}
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
				value = fmt.Sprint(v)
			}
			payload = append(payload, normalizer.SchedulePayloadField{
				Name:  p.Name,
				Type:  IRTypeRefToGoType(p.Type),
				Value: value,
			})
		}
		out = append(out, scheduleOut{
			Name:    s.Name,
			Service: s.Service,
			Action:  s.Action,
			At:      s.At,
			Publish: s.Publish,
			Every:   s.Every,
			Payload: payload,
		})
	}

	hasPublish := false
	for _, s := range out {
		if s.Publish != "" {
			hasPublish = true
			break
		}
	}
	var buf bytes.Buffer
	data := struct {
		Schedules  []scheduleOut
		HasPublish bool
	}{
		Schedules:  out,
		HasPublish: hasPublish,
	}
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "scheduler.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Scheduler: %s\n", path)
	return nil
}

// EmitInfraConfigs generates configs for Atlas and SQLC.
func (e *Emitter) EmitInfraConfigs() error {
	// Atlas
	atlasContent := `env "local" {
  src = "file://db/schema/schema.sql"
  dev = "docker://postgres/15/dev"
  migration {
    dir = "file://db/migrations"
  }
}
`
	if err := os.WriteFile(filepath.Join(e.OutputDir, "atlas.hcl"), []byte(atlasContent), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated Atlas Config: atlas.hcl\n")

	// SQLC
	sqlcContent := `version: "2"
sql:
  - schema: "db/schema/schema.sql"
    queries: "db/queries"
    engine: "postgresql"
    gen:
      go:
        package: "db"
        out: "internal/adapter/repository/sql/db"
        sql_package: "pgx/v5"
`
	if err := os.WriteFile(filepath.Join(e.OutputDir, "sqlc.yaml"), []byte(sqlcContent), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated SQLC Config: sqlc.yaml\n")

	if err := os.MkdirAll(filepath.Join(e.OutputDir, "db", "queries"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(e.OutputDir, "db", "migrations"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(e.OutputDir, "scripts"), 0755); err != nil {
		return err
	}

	// Migration helper scripts
	diffScript := `#!/usr/bin/env bash
set -euo pipefail

NAME="${1:-}"
if [[ -z "${NAME}" ]]; then
  echo "Usage: ./scripts/atlas-diff.sh <migration_name>"
  exit 1
fi

atlas migrate diff "${NAME}" --env local --to file://db/schema/schema.sql --dir file://db/migrations

LATEST="$(ls -t db/migrations/*.sql 2>/dev/null | head -n 1 || true)"
if [[ -z "${LATEST}" ]]; then
  exit 0
fi

if grep -nE "DROP TABLE|DROP COLUMN" "${LATEST}" >/dev/null; then
  echo "Detected destructive statements in ${LATEST}."
  echo "Review the migration. Re-run with ALLOW_DROP=1 to accept."
  if [[ "${ALLOW_DROP:-}" != "1" ]]; then
    exit 1
  fi
fi
`

	applyScript := `#!/usr/bin/env bash
set -euo pipefail

DB_URL="${DB_URL:-}"
if [[ -z "${DB_URL}" ]]; then
  echo "Set DB_URL, e.g. postgres://user:pass@localhost:5432/db?sslmode=disable"
  exit 1
fi

atlas migrate apply --dir file://db/migrations --url "${DB_URL}"
`

	if err := writeExecutable(filepath.Join(e.OutputDir, "scripts", "atlas-diff.sh"), []byte(diffScript)); err != nil {
		return err
	}
	if err := writeExecutable(filepath.Join(e.OutputDir, "scripts", "atlas-apply.sh"), []byte(applyScript)); err != nil {
		return err
	}

	return nil
}

// EmitHealth generates health and readiness probes.
func (e *Emitter) EmitHealth() error {
	tmplPath := filepath.Join(e.TemplatesDir, "health.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/health.tmpl" // Fallback
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	data := map[string]any{
		"ANGVersion": "0.1.0", // Hardcoded for now
	}

	t, err := template.New("health").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "health.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Health Probes: %s\n", path)
	return nil
}

// EmitHelpers generates the helpers utility package.
func (e *Emitter) EmitHelpers() error {
	tmplPath := "templates/helpers.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "helpers")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	formatted, err := format.Source(tmplContent)
	if err != nil {
		formatted = tmplContent
	}

	path := filepath.Join(targetDir, "helpers.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Helpers: %s\n", path)
	return nil
}

// EmitCircuitBreaker generates the circuit breaker package.
func (e *Emitter) EmitCircuitBreaker() error {
	tmplPath := "templates/circuitbreaker.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "circuitbreaker")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	formatted, err := format.Source(tmplContent)
	if err != nil {
		formatted = tmplContent
	}

	path := filepath.Join(targetDir, "breaker.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Circuit Breaker: %s\n", path)
	return nil
}

// EmitPresence generates the presence store package.
func (e *Emitter) EmitPresence() error {
	tmplPath := "templates/presence.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "presence")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	formatted, err := format.Source(tmplContent)
	if err != nil {
		formatted = tmplContent
	}

	path := filepath.Join(targetDir, "store.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Presence: %s\n", path)
	return nil
}

// EmitReportPDF generates the PDF report generator package.
func (e *Emitter) EmitReportPDF() error {
	tmplPath := "templates/report_pdf.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("report_pdf").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "report")
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

	path := filepath.Join(targetDir, "pdf.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Report PDF: %s\n", path)
	return nil
}

// EmitNotificationMuting generates the notification muting repository decorator.
func (e *Emitter) EmitNotificationMuting(def *normalizer.NotificationMutingDef, schema *ir.Schema) error {
	if def == nil || !def.Enabled {
		return nil
	}

	tmplPath := "templates/notification_muting.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	// Find the Notification repository to get its finders
	type finderDelegation struct {
		Name       string
		ParamsSig  string
		ReturnType string
		ArgNames   string
	}

	var extraFinders []finderDelegation
	if schema != nil {
		for _, repo := range schema.Repos {
			if repo.Entity != "Notification" {
				continue
			}
			for _, f := range repo.Finders {
				fd := finderDelegation{Name: ExportName(f.Name)}

				// Compute return type
				if f.ReturnType != "" {
					fd.ReturnType = f.ReturnType
				} else if f.Action == "delete" {
					fd.ReturnType = "int64"
				} else if f.Returns == "one" {
					fd.ReturnType = "*domain.Notification"
				} else if f.Returns == "many" {
					fd.ReturnType = "[]domain.Notification"
				} else if f.Returns == "count" {
					fd.ReturnType = "int64"
				} else {
					fd.ReturnType = "[]domain.Notification"
				}

				// Compute params and arg names
				var params []string
				var argNames []string
				for _, w := range f.Where {
					pType := w.ParamType
					if pType == "time" || pType == "time.Time" {
						pType = "time.Time"
					}
					params = append(params, fmt.Sprintf("%s %s", w.Param, pType))
					argNames = append(argNames, w.Param)
				}
				fd.ParamsSig = strings.Join(params, ", ")
				fd.ArgNames = strings.Join(argNames, ", ")

				extraFinders = append(extraFinders, fd)
			}
			break
		}
	}

	t, err := template.New("notification_muting").Funcs(template.FuncMap{
		"GoModule":        func() string { return e.GoModule },
		"MuteAllField":    func() string { return ExportName(def.MuteAllField) },
		"MutedTypesField": func() string { return ExportName(def.MutedTypesField) },
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	data := struct {
		ExtraFinders []finderDelegation
	}{
		ExtraFinders: extraFinders,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "notification_muting.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Notification Muting Decorator: %s\n", path)
	return nil
}

func writeExecutable(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0755); err != nil {
		return err
	}
	fmt.Printf("Generated Script: %s\n", path)
	return nil
}

// EmitMermaid generates the architecture diagram.
func (e *Emitter) EmitMermaid(ctx MainContext) error {
	var buf bytes.Buffer
	buf.WriteString("# Architecture Overview\n\nGenerated by ANG.\n\n")
	buf.WriteString("```mermaid\n")
	buf.WriteString("graph TD\n")

	// Infrastructure
	if ctx.HasSQL {
		buf.WriteString("  Postgres[(Postgres)]\n")
		buf.WriteString("  style Postgres fill:#336791,stroke:#333,stroke-width:2px,color:#fff\n")
	}
	if ctx.HasMongo {
		buf.WriteString("  Mongo[(MongoDB)]\n")
		buf.WriteString("  style Mongo fill:#47A248,stroke:#333,stroke-width:2px,color:#fff\n")
	}
	if ctx.HasNats {
		buf.WriteString("  NATS{{NATS Bus}}\n")
		buf.WriteString("  style NATS fill:#27AAE1,stroke:#333,stroke-width:2px,color:#fff\n")
	}
	if ctx.HasCache {
		buf.WriteString("  Redis[(Redis)]\n")
		buf.WriteString("  style Redis fill:#D82C20,stroke:#333,stroke-width:2px,color:#fff\n")
	}
	if ctx.HasS3 {
		buf.WriteString("  S3[(S3 Storage)]\n")
		buf.WriteString("  style S3 fill:#FF9900,stroke:#333,stroke-width:2px,color:#fff\n")
	}

	buf.WriteString("\n")

	// Services
	for _, s := range ctx.Services {
		sName := s.Name
		buf.WriteString(fmt.Sprintf("  %s[%s]\n", sName, sName))

		// DB Dependencies
		if ctx.HasSQL {
			buf.WriteString(fmt.Sprintf("  %s -.-> Postgres\n", sName))
		}
		if ctx.HasMongo {
			buf.WriteString(fmt.Sprintf("  %s -.-> Mongo\n", sName))
		}
		if ctx.HasCache {
			buf.WriteString(fmt.Sprintf("  %s -.-> Redis\n", sName))
		}
		if s.RequiresS3 {
			buf.WriteString(fmt.Sprintf("  %s -.-> S3\n", sName))
		}

		// Events (Pub)
		for _, pub := range s.Publishes {
			buf.WriteString(fmt.Sprintf("  %s -- Pub: %s --> NATS\n", sName, pub))
		}

		// Events (Sub)
		subscribeKeys := make([]string, 0, len(s.Subscribes))
		for evtName := range s.Subscribes {
			subscribeKeys = append(subscribeKeys, evtName)
		}
		sort.Strings(subscribeKeys)
		for _, evtName := range subscribeKeys {
			buf.WriteString(fmt.Sprintf("  NATS -- Sub: %s --> %s\n", evtName, sName))
		}

		// Middlewares (Circuit Breakers)
		for _, ep := range ctx.Endpoints {
			if strings.EqualFold(ep.ServiceName, sName) && ep.CircuitBreaker != nil {
				buf.WriteString(fmt.Sprintf("  %s -- CircuitBreaker: %s --> %s\n", sName, ep.RPC, sName))
				buf.WriteString(fmt.Sprintf("  style %s stroke:#f00,stroke-width:4px\n", sName))
			}
		}
	}

	buf.WriteString("```\n")

	path := filepath.Join(e.OutputDir, "ARCHITECTURE.md")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated Architecture Map: %s\n", path)
	return nil
}

type K8sContext struct {
	Name     string
	HasCache bool
	HasSQL   bool
	HasMongo bool
	HasNats  bool
	HasS3    bool
}

// EmitK8s generates Kubernetes manifests.
func (e *Emitter) EmitK8s(services []ir.Service, isMicroservice bool) error {
	targetDir := filepath.Join(e.OutputDir, "deploy", "k8s")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	if isMicroservice {
		for _, svc := range services {
			ctx := K8sContext{
				Name:     svc.Name,
				HasCache: svc.RequiresRedis,
				HasSQL:   svc.RequiresSQL,
				HasMongo: svc.RequiresMongo,
				HasNats:  svc.RequiresNats,
				HasS3:    svc.RequiresS3,
			}
			if err := e.emitK8sFiles(targetDir, ctx); err != nil {
				return err
			}
		}
	} else {
		// Monolith
		ctx := K8sContext{Name: "server"}
		for _, svc := range services {
			if svc.RequiresRedis {
				ctx.HasCache = true
			}
			if svc.RequiresSQL {
				ctx.HasSQL = true
			}
			if svc.RequiresMongo {
				ctx.HasMongo = true
			}
			if svc.RequiresNats {
				ctx.HasNats = true
			}
			if svc.RequiresS3 {
				ctx.HasS3 = true
			}
		}
		return e.emitK8sFiles(targetDir, ctx)
	}
	return nil
}

func (e *Emitter) emitK8sFiles(baseDir string, ctx K8sContext) error {
	files := []string{"deployment", "service", "configmap"}

	for _, file := range files {
		tmplPath := fmt.Sprintf("templates/k8s/%s.tmpl", file)
		tmplContent, err := ReadTemplateByPath(tmplPath)
		if err != nil {
			return fmt.Errorf("read template %s: %w", file, err)
		}

		t, err := template.New(file).Funcs(template.FuncMap{
			"ToLower": strings.ToLower,
		}).Parse(string(tmplContent))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", file, err)
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, ctx); err != nil {
			return fmt.Errorf("execute template %s: %w", file, err)
		}

		// Save as <service-name>-<kind>.yaml
		filename := fmt.Sprintf("%s-%s.yaml", strings.ToLower(ctx.Name), file)
		path := filepath.Join(baseDir, filename)
		if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
			return err
		}
		fmt.Printf("Generated K8s Manifest: %s\n", path)
	}
	return nil
}
