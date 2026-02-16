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

// EmitRedisClient генерирует Redis клиент
func (e *Emitter) EmitRedisClient() error {
	tmplPath := filepath.Join(e.TemplatesDir, "redis_client.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/redis_client.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("redis_client").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "cache", "redis")
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

	path := filepath.Join(targetDir, "client.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Redis Client: %s\n", path)
	return nil
}

// EmitMongoSchema генерирует JSON Schema для Mongo валидации
func (e *Emitter) EmitMongoSchema(entities []ir.Entity) error {
	entitiesNorm := IREntitiesToNormalizer(entities)

	tmplContent := `{ 
  "bsonType": "object",
  "required": [
    {{- range $i, $f := .Fields -}}
      {{- if not .IsOptional }}"{{ $f.Name | ToLower }}"{{- if not (last $i $.Fields) -}},{{- end -}}{{ end }}
    {{- end }}
  ],
  "properties": {
    {{- range $i, $f := .Fields }}
    "{{ $f.Name | ToLower }}": {
      "bsonType": "{{ MongoType $f.Type $f.DB.Type }}"
    }{{ if not (last $i $.Fields) }},{{ end }}
    {{- end }}
  }
}`

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
		"last": func(x int, a interface{}) bool {
			return x == len(a.([]normalizer.Field))-1
		},
		"MongoType": func(goType, dbType string) string {
			if strings.EqualFold(dbType, "ObjectId") {
				return "objectId"
			}
			if strings.EqualFold(dbType, "UUID") {
				return "string"
			}
			switch goType {
			case "int":
				return "int"
			case "float64", "float32":
				return "double"
			case "bool":
				return "bool"
			case "[]any", "[]interface{}":
				return "array"
			case "map[string]any", "struct":
				return "object"
			default:
				return "string"
			}
		},
	}

	t, err := template.New("mongo_schema").Funcs(funcMap).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "db", "mongo", "schemas")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	for _, entity := range entitiesNorm {
		isMongo := false
		for _, f := range entity.Fields {
			if strings.EqualFold(f.DB.Type, "ObjectId") {
				isMongo = true
				break
			}
		}
		if entity.Name == "Bet" {
			isMongo = true
		}

		if !isMongo {
			continue
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, entity); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		jsonStr := strings.ReplaceAll(buf.String(), ",  ]\n", "  ]\n")

		filename := fmt.Sprintf("%s.json", strings.ToLower(entity.Name))
		path := filepath.Join(targetDir, filename)
		if err := WriteFileIfChanged(path, []byte(jsonStr), 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated Mongo Schema: %s\n", path)
	}

	return nil
}

func collectPublishedEventsFromIR(services []ir.Service, schedules []ir.Schedule) []string {
	uniqueEvents := make(map[string]bool)
	var eventList []string

	addEvent := func(evt string) {
		if evt == "" || uniqueEvents[evt] {
			return
		}
		uniqueEvents[evt] = true
		eventList = append(eventList, evt)
	}

	var scanForEvents func([]ir.FlowStep)
	scanForEvents = func(steps []ir.FlowStep) {
		for _, step := range steps {
			if step.Action == "event.Publish" || step.Action == "event.Broadcast" {
				addEvent(fmt.Sprint(step.Args["name"]))
			}
			if len(step.Steps) > 0 {
				scanForEvents(step.Steps)
			}
			if len(step.Then) > 0 {
				scanForEvents(step.Then)
			}
			if len(step.Else) > 0 {
				scanForEvents(step.Else)
			}
		}
	}

	for _, s := range services {
		for _, evt := range s.Publishes {
			addEvent(evt)
		}
		for _, m := range s.Methods {
			scanForEvents(m.Flow)
		}
	}
	for _, s := range schedules {
		addEvent(s.Publish)
	}
	sort.Strings(eventList)
	return eventList
}

// EmitPublisherInterface генерирует интерфейс для публикации
func (e *Emitter) EmitPublisherInterface(services []ir.Service, schedules []ir.Schedule) error {
	tmplPath := filepath.Join(e.TemplatesDir, "publisher_interface.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/publisher_interface.tmpl"
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

	t, err := template.New("publisher_interface").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	eventList := collectPublishedEventsFromIR(services, schedules)

	var buf bytes.Buffer
	if err := t.Execute(&buf, eventList); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "publisher.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Publisher Interface: %s\n", path)
	return nil
}

// EmitNatsAdapter генерирует реализацию NATS клиента
func (e *Emitter) EmitNatsAdapter(services []ir.Service, schedules []ir.Schedule) error {
	tmplPath := filepath.Join(e.TemplatesDir, "nats_client_v2.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/nats_client_v2.tmpl"
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

	t, err := template.New("nats_client").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "events", "nats")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	eventList := collectPublishedEventsFromIR(services, schedules)

	var buf bytes.Buffer
	if err := t.Execute(&buf, eventList); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "client.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated NATS Adapter: %s\n", path)
	return nil
}
