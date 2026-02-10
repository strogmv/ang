package emitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type contractEndpoint struct {
	Endpoint    normalizer.Endpoint
	Input       normalizer.Entity
	HasInput    bool
	HasRequired bool
	BodyJSON    string
	QueryParams string
}

type authEndpoints struct {
	RegisterMethod string
	RegisterPath   string
	LoginMethod    string
	LoginPath      string
	RefreshMethod  string
	RefreshPath    string
}

// EmitContractTests generates HTTP/WS contract tests from CUE definitions.
func (e *Emitter) EmitContractTests(irEndpoints []ir.Endpoint, irServices []ir.Service) error {
	tmplPath := "templates/contract_tests.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	endpoints := IREndpointsToNormalizer(irEndpoints)
	services := IRServicesToNormalizer(irServices)

	methodsByService := make(map[string]map[string]normalizer.Method)
	for _, svc := range services {
		methods := make(map[string]normalizer.Method)
		for _, m := range svc.Methods {
			methods[m.Name] = m
		}
		methodsByService[svc.Name] = methods
	}

	type validationRules struct {
		Required bool
		Email    bool
		URL      bool
		Min      *float64
		Gte      *float64
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
			case "gte":
				rules.Gte = &num
			}
		}
		return rules
	}
	pathParams := func(path string) map[string]struct{} {
		re := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
		out := make(map[string]struct{})
		matches := re.FindAllStringSubmatch(path, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			out[strings.ToLower(m[1])] = struct{}{}
		}
		return out
	}
	hasRequired := func(fields []normalizer.Field, exclude map[string]struct{}) bool {
		for _, f := range fields {
			if _, ok := exclude[strings.ToLower(f.Name)]; ok {
				continue
			}
			rules := parseValidateTag(f.ValidateTag)
			if !f.IsOptional || rules.Required {
				return true
			}
		}
		return false
	}
	minRequiredBody := func(entity normalizer.Entity, exclude map[string]struct{}) (string, error) {
		body := make(map[string]any)
		var bodyFields []normalizer.Field
		for _, f := range entity.Fields {
			if _, ok := exclude[strings.ToLower(f.Name)]; ok {
				continue
			}
			rules := parseValidateTag(f.ValidateTag)
			nameLower := strings.ToLower(f.Name)
			force := nameLower == "companyname" || nameLower == "password"
			if f.IsOptional && !rules.Required && !force {
				continue
			}
			bodyFields = append(bodyFields, f)
			if rules.Email {
				body[nameLower] = "user@example.com"
				continue
			}
			if rules.URL {
				body[nameLower] = "https://example.com"
				continue
			}
			if strings.HasSuffix(nameLower, "at") || nameLower == "startsat" || nameLower == "endsat" {
				body[nameLower] = "2026-01-01T00:00:00Z"
				continue
			}
			switch f.Type {
			case "string":
				if nameLower == "password" {
					body[nameLower] = "test1234"
				} else if nameLower == "companyname" {
					body[nameLower] = "Test Company"
				} else {
					body[nameLower] = "test"
				}
			case "int", "int64":
				val := int64(1)
				if rules.Min != nil && *rules.Min > float64(val) {
					val = int64(*rules.Min)
				}
				if rules.Gte != nil && *rules.Gte > float64(val) {
					val = int64(*rules.Gte)
				}
				body[nameLower] = val
			case "float", "float64":
				val := 1.0
				if rules.Min != nil && *rules.Min > val {
					val = *rules.Min
				}
				if rules.Gte != nil && *rules.Gte > val {
					val = *rules.Gte
				}
				body[nameLower] = val
			case "bool":
				body[nameLower] = true
			case "time.Time":
				body[nameLower] = "2026-01-01T00:00:00Z"
			case "map[string]any":
				body[nameLower] = map[string]any{}
			case "[]any":
				body[nameLower] = []any{}
			default:
				if strings.HasPrefix(f.Type, "[]") {
					body[nameLower] = []any{}
				} else {
					body[nameLower] = map[string]any{}
				}
			}
		}
		if len(bodyFields) == 1 {
			nameLower := strings.ToLower(bodyFields[0].Name)
			if val, ok := body[nameLower]; ok {
				if data, err := json.Marshal(val); err == nil {
					return strconv.Quote(string(data)), nil
				}
			}
		}
		data, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		return strconv.Quote(string(data)), nil
	}
	minQueryParams := func(entity normalizer.Entity, pagination *normalizer.PaginationDef, exclude map[string]struct{}) string {
		var parts []string
		for _, f := range entity.Fields {
			if _, ok := exclude[strings.ToLower(f.Name)]; ok {
				continue
			}
			rules := parseValidateTag(f.ValidateTag)
			if f.IsOptional && !rules.Required {
				continue
			}
			lower := strings.ToLower(f.Name)
			if rules.Email {
				parts = append(parts, lower+"=user@example.com")
				continue
			}
			if rules.URL {
				parts = append(parts, lower+"=https%3A%2F%2Fexample.com")
				continue
			}
			switch f.Type {
			case "string":
				parts = append(parts, lower+"=test")
			case "int", "int64":
				val := int64(1)
				if rules.Min != nil && *rules.Min > float64(val) {
					val = int64(*rules.Min)
				}
				if rules.Gte != nil && *rules.Gte > float64(val) {
					val = int64(*rules.Gte)
				}
				parts = append(parts, lower+"="+strconv.FormatInt(val, 10))
			case "float", "float64":
				val := 1.0
				if rules.Min != nil && *rules.Min > val {
					val = *rules.Min
				}
				if rules.Gte != nil && *rules.Gte > val {
					val = *rules.Gte
				}
				parts = append(parts, lower+"="+strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", val), "0"), "."))
			case "bool":
				parts = append(parts, lower+"=true")
			}
		}
		if pagination != nil {
			switch pagination.Type {
			case "offset":
				if pagination.DefaultLimit > 0 {
					parts = append(parts, "limit="+strconv.Itoa(pagination.DefaultLimit))
				} else {
					parts = append(parts, "limit=10")
				}
				parts = append(parts, "offset=0")
			case "cursor":
				parts = append(parts, "limit=10")
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return "?" + strings.Join(parts, "&")
	}

	var out []contractEndpoint
	auth := authEndpoints{}
	for _, ep := range endpoints {
		methods := methodsByService[ep.ServiceName]
		m, ok := methods[ep.RPC]
		ce := contractEndpoint{Endpoint: ep}
		if ok {
			exclude := pathParams(ep.Path)
			for _, inject := range ep.AuthInject {
				exclude[strings.ToLower(inject)] = struct{}{}
			}
			ce.Input = m.Input
			ce.HasInput = m.Input.Name != ""
			ce.HasRequired = hasRequired(m.Input.Fields, exclude)
			if ce.HasInput {
				if body, err := minRequiredBody(m.Input, exclude); err == nil {
					ce.BodyJSON = body
				}
			}
			if ce.HasInput {
				ce.QueryParams = minQueryParams(m.Input, ep.Pagination, exclude)
			}
		}
		switch ep.RPC {
		case "RegisterUser":
			auth.RegisterMethod = ep.Method
			auth.RegisterPath = ep.Path
		case "LoginUser":
			auth.LoginMethod = ep.Method
			auth.LoginPath = ep.Path
		case "RefreshToken":
			auth.RefreshMethod = ep.Method
			auth.RefreshPath = ep.Path
		}
		out = append(out, ce)
	}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
		"HasAuth": func(ep normalizer.Endpoint) bool {
			return strings.EqualFold(ep.AuthType, "jwt")
		},
		"HasBody": func(ce contractEndpoint) bool {
			method := strings.ToLower(ce.Endpoint.Method)
			return method != "get" && method != "ws" && ce.HasInput
		},
		"HasRequired": func(ce contractEndpoint) bool {
			return ce.HasRequired
		},
	}

	t, err := template.New("contract_tests").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "tests", "contract")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, struct {
		Endpoints []contractEndpoint
		Auth      authEndpoints
	}{Endpoints: out, Auth: auth}); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	path := filepath.Join(targetDir, "contract_test.go")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Contract Tests: %s\n", path)
	return nil
}
