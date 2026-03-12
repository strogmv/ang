package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// EmitOpenAPI generates the Swagger specification.
type OpenAPIEndpoint struct {
	Endpoint  normalizer.Endpoint
	ErrorDefs []normalizer.ErrorDef
	Input     normalizer.Entity
	Output    normalizer.Entity
}

type OpenAPIContext struct {
	Endpoints    []OpenAPIEndpoint
	Schemas      []normalizer.Entity
	Title        string
	Version      string
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

func (e *Emitter) EmitOpenAPI(irEndpoints []ir.Endpoint, irServices []ir.Service, irErrors []ir.Error, project *normalizer.ProjectDef) error {
	endpoints := IREndpointsToNormalizer(irEndpoints)
	services := IRServicesToNormalizer(irServices)
	errors := IRErrorsToNormalizer(irErrors)

	tmplPath := "templates/openapi.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	type validationRules struct {
		Required bool
		Email    bool
		URL      bool
		Min      *float64
		Max      *float64
		Len      *float64
		Gte      *float64
		Lte      *float64
	}
	parseValidateTag := func(tag string) validationRules {
		var rules validationRules
		parts := strings.Split(tag, ",")
		for _, raw := range parts {
			part := strings.TrimSpace(raw)
			if part == "" {
				continue
			}
			switch part {
			case "required":
				rules.Required = true
				continue
			case "email":
				rules.Email = true
				continue
			case "url":
				rules.URL = true
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			if val == "" {
				continue
			}
			num, err := strconv.ParseFloat(val, 64)
			if err != nil {
				continue
			}
			switch key {
			case "min":
				rules.Min = &num
			case "max":
				rules.Max = &num
			case "len":
				rules.Len = &num
			case "gte":
				rules.Gte = &num
			case "lte":
				rules.Lte = &num
			}
		}
		return rules
	}
	requiredField := func(f normalizer.Field) bool {
		rules := parseValidateTag(f.ValidateTag)
		return !f.IsOptional || rules.Required
	}
	openAPIRules := func(f normalizer.Field) []string {
		rules := parseValidateTag(f.ValidateTag)
		isString := f.Type == "string"
		isNumber := f.Type == "int" || f.Type == "int64" || f.Type == "float64" || f.Type == "float32" || f.Type == "float"
		var minVal *float64
		var maxVal *float64
		if rules.Min != nil {
			minVal = rules.Min
		}
		if rules.Gte != nil && (minVal == nil || *rules.Gte > *minVal) {
			minVal = rules.Gte
		}
		if rules.Max != nil {
			maxVal = rules.Max
		}
		if rules.Lte != nil && (maxVal == nil || *rules.Lte < *maxVal) {
			maxVal = rules.Lte
		}
		var out []string
		if isString {
			if rules.Len != nil {
				out = append(out, fmt.Sprintf("minLength: %d", int(*rules.Len)))
				out = append(out, fmt.Sprintf("maxLength: %d", int(*rules.Len)))
			} else {
				if minVal != nil {
					out = append(out, fmt.Sprintf("minLength: %d", int(*minVal)))
				}
				if maxVal != nil {
					out = append(out, fmt.Sprintf("maxLength: %d", int(*maxVal)))
				}
			}
		}
		if isNumber {
			if minVal != nil {
				out = append(out, fmt.Sprintf("minimum: %s", strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", *minVal), "0"), ".")))
			}
			if maxVal != nil {
				out = append(out, fmt.Sprintf("maximum: %s", strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", *maxVal), "0"), ".")))
			}
		}
		return out
	}
	openAPIFieldFormat := func(f normalizer.Field) string {
		rules := parseValidateTag(f.ValidateTag)
		if f.Type == "string" {
			if rules.Email {
				return "email"
			}
			if rules.URL {
				return "uri"
			}
		}
		return ""
	}
	openAPIRequiredFields := func(entity normalizer.Entity) []string {
		var fields []string
		for _, f := range entity.Fields {
			if requiredField(f) {
				fields = append(fields, strings.ToLower(f.Name))
			}
		}
		return fields
	}

	funcMap := e.getSharedFuncMap()
	funcMap["Lower"] = strings.ToLower
	funcMap["IsArray"] = func(goType string) bool {
		return strings.HasPrefix(goType, "[]")
	}
	funcMap["OpenAPIType"] = func(goType string) string {
		if strings.HasPrefix(goType, "[]") {
			return "array"
		}
		switch goType {
		case "int", "int64":
			return "integer"
		case "float64", "float32":
			return "number"
		case "bool":
			return "boolean"
		case "time.Time":
			return "string"
		default:
			return "string"
		}
	}
	funcMap["OpenAPIFormat"] = func(goType string) string {
		switch goType {
		case "int", "int64":
			return "int64"
		case "float64", "float32":
			return "double"
		case "time.Time":
			return "date-time"
		default:
			return ""
		}
	}
	funcMap["OpenAPIFieldFormat"] = func(f normalizer.Field) string {
		if format := openAPIFieldFormat(f); format != "" {
			return format
		}
		return ""
	}
	funcMap["OpenAPIRules"] = func(f normalizer.Field) []string {
		return openAPIRules(f)
	}
	funcMap["OpenAPIRequiredFields"] = func(entity normalizer.Entity) []string {
		return openAPIRequiredFields(entity)
	}
	funcMap["IsRequiredField"] = func(f normalizer.Field) bool {
		return requiredField(f)
	}
	funcMap["OpenAPIItemsType"] = func(goType string) string {
		if strings.HasPrefix(goType, "[]") {
			return strings.TrimPrefix(goType, "[]")
		}
		return ""
	}

	t, err := template.New("openapi").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "api")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	errMap := make(map[string]normalizer.ErrorDef)
	for _, e := range errors {
		errMap[e.Name] = e
	}

	methodsByService := make(map[string]map[string]normalizer.Method)
	for _, svc := range services {
		methods := make(map[string]normalizer.Method)
		for _, m := range svc.Methods {
			methods[m.Name] = m
		}
		methodsByService[svc.Name] = methods
	}

	var apiEndpoints []OpenAPIEndpoint
	schemaMap := make(map[string]normalizer.Entity)
	nestedMap := make(map[string]normalizer.Entity)
	for _, ep := range endpoints {
		methods := methodsByService[ep.ServiceName]
		method, ok := methods[ep.RPC]
		if ok {
			ep.Errors = method.Throws
			ep.Pagination = method.Pagination
			if method.Input.Name != "" {
				schemaMap[method.Input.Name] = method.Input
				for _, nested := range nestedEntitiesFromEntity(method.Input) {
					nestedMap[nested.Name] = nested
				}
			}
			if method.Output.Name != "" {
				schemaMap[method.Output.Name] = method.Output
				for _, nested := range nestedEntitiesFromEntity(method.Output) {
					nestedMap[nested.Name] = nested
				}
			}
		}

		var defs []normalizer.ErrorDef
		for _, name := range ep.Errors {
			if def, ok := errMap[name]; ok {
				defs = append(defs, def)
			}
		}
		oe := OpenAPIEndpoint{
			Endpoint:  ep,
			ErrorDefs: defs,
		}
		if ok {
			oe.Input = method.Input
			oe.Output = method.Output
		}
		apiEndpoints = append(apiEndpoints, oe)
	}

	var schemas []normalizer.Entity
	schemaNames := make([]string, 0, len(schemaMap))
	for name := range schemaMap {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)
	for _, name := range schemaNames {
		schemas = append(schemas, schemaMap[name])
	}
	nestedNames := make([]string, 0, len(nestedMap))
	for name := range nestedMap {
		nestedNames = append(nestedNames, name)
	}
	sort.Strings(nestedNames)
	for _, name := range nestedNames {
		schemas = append(schemas, nestedMap[name])
	}

	var buf bytes.Buffer
	title := "ANG API"
	version := "0.1.0"
	if project != nil {
		if strings.TrimSpace(project.Name) != "" {
			title = project.Name + " API"
		}
		if strings.TrimSpace(project.Version) != "" {
			version = project.Version
		}
	}
	ctx := OpenAPIContext{
		Endpoints:    apiEndpoints,
		Schemas:      schemas,
		Title:        title,
		Version:      version,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}
	if err := t.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	path := filepath.Join(targetDir, "openapi.yaml")
	if err := WriteFileIfChanged(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated OpenAPI Spec: %s\n", path)
	return nil
}

// EmitAsyncAPI generates the AsyncAPI specification for events.
func (e *Emitter) EmitAsyncAPI(irEvents []ir.Event, project *normalizer.ProjectDef) error {
	events := IREventsToNormalizer(irEvents)

	tmplPath := "templates/asyncapi.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["AsyncApiType"] = func(goType string) string {
		switch goType {
		case "int", "int64":
			return "integer"
		case "float64", "float":
			return "number"
		case "bool":
			return "boolean"
		default:
			return "string"
		}
	}

	t, err := template.New("asyncapi").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "api")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	title := "ANG Events"
	version := "0.1.0"
	if project != nil {
		if strings.TrimSpace(project.Name) != "" {
			title = project.Name + " Events"
		}
		if strings.TrimSpace(project.Version) != "" {
			version = project.Version
		}
	}
	data := struct {
		Events       []normalizer.EventDef
		Title        string
		Version      string
		ANGVersion   string
		InputHash    string
		CompilerHash string
	}{
		Events:       events,
		Title:        title,
		Version:      version,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	path := filepath.Join(targetDir, "asyncapi.yaml")
	if err := WriteFileIfChanged(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated AsyncAPI Spec: %s\n", path)
	return nil
}

// rateLimitWindowSeconds converts a duration string (e.g. "1h", "30m") to seconds.
// Returns 0 if the string is empty or invalid.
func rateLimitWindowSeconds(window string) int {
	if window == "" {
		return 0
	}
	d, err := time.ParseDuration(window)
	if err != nil || d <= 0 {
		return 0
	}
	return int(d.Seconds())
}
