package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Minimal OpenAPI 3.x structs — only what's needed for fact extraction.

type openAPIDoc struct {
	OpenAPI    string                        `json:"openapi" yaml:"openapi"`
	Swagger    string                        `json:"swagger" yaml:"swagger"`
	Paths      map[string]openAPIPathItem    `json:"paths" yaml:"paths"`
	Components openAPIComponents             `json:"components" yaml:"components"`
}

type openAPIComponents struct {
	Schemas map[string]openAPISchema `json:"schemas" yaml:"schemas"`
}

type openAPIPathItem struct {
	Get     *openAPIPathOp `json:"get" yaml:"get"`
	Post    *openAPIPathOp `json:"post" yaml:"post"`
	Put     *openAPIPathOp `json:"put" yaml:"put"`
	Patch   *openAPIPathOp `json:"patch" yaml:"patch"`
	Delete  *openAPIPathOp `json:"delete" yaml:"delete"`
}

type openAPIPathOp struct {
	OperationID string                     `json:"operationId" yaml:"operationId"`
	Tags        []string                   `json:"tags" yaml:"tags"`
	Parameters  []openAPIParam             `json:"parameters" yaml:"parameters"`
	RequestBody *openAPIRequestBody        `json:"requestBody" yaml:"requestBody"`
	Responses   map[string]openAPIResponse `json:"responses" yaml:"responses"`
}

type openAPIParam struct {
	Name     string        `json:"name" yaml:"name"`
	In       string        `json:"in" yaml:"in"`
	Required bool          `json:"required" yaml:"required"`
	Schema   openAPISchema `json:"schema" yaml:"schema"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `json:"content" yaml:"content"`
}

type openAPIMediaType struct {
	Schema openAPISchema `json:"schema" yaml:"schema"`
}

type openAPIResponse struct {
	Content map[string]openAPIMediaType `json:"content" yaml:"content"`
}

type openAPISchema struct {
	Type       string                   `json:"type" yaml:"type"`
	Format     string                   `json:"format" yaml:"format"`
	Ref        string                   `json:"$ref" yaml:"$ref"`
	Properties map[string]openAPISchema `json:"properties" yaml:"properties"`
	Required   []string                 `json:"required" yaml:"required"`
	Items      *openAPISchema           `json:"items" yaml:"items"`
	MinLength  *int                     `json:"minLength" yaml:"minLength"`
	MaxLength  *int                     `json:"maxLength" yaml:"maxLength"`
	Pattern    string                   `json:"pattern" yaml:"pattern"`
	Minimum    *float64                 `json:"minimum" yaml:"minimum"`
	Maximum    *float64                 `json:"maximum" yaml:"maximum"`
}

func extractOpenAPIFacts(path string) (FactsEnvelope, error) {
	var env FactsEnvelope

	docPath, err := findOpenAPIFile(path)
	if err != nil {
		return env, err
	}

	doc, err := loadOpenAPIDoc(docPath)
	if err != nil {
		return env, fmt.Errorf("load openapi %s: %w", docPath, err)
	}

	// Extract entities from components.schemas
	for name, schema := range doc.Components.Schemas {
		schema = resolveOpenAPISchemaRef(doc, schema)
		if schema.Type != "object" && len(schema.Properties) == 0 {
			continue
		}
		entity := FactEntity{
			Name:      toPascalCase(name),
			TableHint: toSnakePlural(toPascalCase(name)),
			Source:    docPath,
			Fields:    openAPISchemaToFields(doc, schema),
		}
		env.Entities = append(env.Entities, entity)
	}

	// Extract operations from paths
	for httpPath, pathItem := range doc.Paths {
		methodOps := map[string]*openAPIPathOp{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"PATCH":  pathItem.Patch,
			"DELETE": pathItem.Delete,
		}
		for method, op := range methodOps {
			if op == nil {
				continue
			}
			opName := op.OperationID
			if opName == "" {
				opName = deriveOpName(method, httpPath)
			}
			serviceHint := ""
			if len(op.Tags) > 0 {
				serviceHint = strings.ToLower(op.Tags[0])
			}

			var inputFields []FactField
			// Path and query params
			for _, param := range op.Parameters {
				if param.In == "path" || param.In == "query" {
					schema := resolveOpenAPISchemaRef(doc, param.Schema)
					inputFields = append(inputFields, FactField{
						Name:        param.Name,
						CueTypeHint: openAPITypeToCUE(schema),
						Validate:    openAPIValidation(schema, param.Required),
						Required:    param.Required,
					})
				}
			}
			// Request body
			if op.RequestBody != nil {
				for _, mt := range op.RequestBody.Content {
					bodySchema := resolveOpenAPISchemaRef(doc, mt.Schema)
					reqSet := make(map[string]struct{}, len(bodySchema.Required))
					for _, r := range bodySchema.Required {
						reqSet[r] = struct{}{}
					}
					for fname, fschema := range bodySchema.Properties {
						fschema = resolveOpenAPISchemaRef(doc, fschema)
						_, req := reqSet[fname]
						inputFields = append(inputFields, FactField{
							Name:        fname,
							CueTypeHint: openAPITypeToCUE(fschema),
							Validate:    openAPIValidation(fschema, req),
							Required:    req,
						})
					}
					break // only first content type
				}
			}

			// Response 200
			var outputFields []FactField
			if resp, ok := op.Responses["200"]; ok {
				for _, mt := range resp.Content {
					respSchema := resolveOpenAPISchemaRef(doc, mt.Schema)
					reqSet := make(map[string]struct{}, len(respSchema.Required))
					for _, r := range respSchema.Required {
						reqSet[r] = struct{}{}
					}
					for fname, fschema := range respSchema.Properties {
						fschema = resolveOpenAPISchemaRef(doc, fschema)
						_, req := reqSet[fname]
						outputFields = append(outputFields, FactField{
							Name:        fname,
							CueTypeHint: openAPITypeToCUE(fschema),
							Validate:    openAPIValidation(fschema, req),
							Required:    req,
						})
					}
					break
				}
			}

			env.Operations = append(env.Operations, FactOp{
				Name:         toPascalCase(opName),
				ServiceHint:  serviceHint,
				Source:       docPath,
				HTTPMethod:   method,
				HTTPPath:     httpPath,
				InputFields:  inputFields,
				OutputFields: outputFields,
			})
		}
	}

	return env, nil
}

func findOpenAPIFile(path string) (string, error) {
	// path may be a direct file
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if !fi.IsDir() {
		return path, nil
	}

	candidates := []string{
		"openapi.json", "openapi.yaml", "openapi.yml",
		"swagger.json", "swagger.yaml", "swagger.yml",
	}
	for _, name := range candidates {
		p := filepath.Join(path, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no openapi/swagger file found in %s", path)
}

func loadOpenAPIDoc(path string) (*openAPIDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc openAPIDoc
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	}
	return &doc, nil
}

func openAPISchemaToFields(doc *openAPIDoc, schema openAPISchema) []FactField {
	schema = resolveOpenAPISchemaRef(doc, schema)
	reqSet := make(map[string]struct{}, len(schema.Required))
	for _, r := range schema.Required {
		reqSet[r] = struct{}{}
	}
	var fields []FactField
	for name, prop := range schema.Properties {
		prop = resolveOpenAPISchemaRef(doc, prop)
		_, req := reqSet[name]
		fields = append(fields, FactField{
			Name:        name,
			CueTypeHint: openAPITypeToCUE(prop),
			Validate:    openAPIValidation(prop, req),
			Required:    req,
		})
	}
	return fields
}

func resolveOpenAPISchemaRef(doc *openAPIDoc, schema openAPISchema) openAPISchema {
	if doc == nil || schema.Ref == "" {
		return schema
	}
	parts := strings.Split(schema.Ref, "/")
	if len(parts) == 0 {
		return schema
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return schema
	}
	resolved, ok := doc.Components.Schemas[name]
	if !ok {
		return schema
	}
	return resolved
}

func openAPIValidation(schema openAPISchema, required bool) string {
	var rules []string
	add := func(rule string) {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			return
		}
		for _, ex := range rules {
			if ex == rule {
				return
			}
		}
		rules = append(rules, rule)
	}

	if required {
		add("required")
	}
	if schema.MinLength != nil {
		add(fmt.Sprintf("min=%d", *schema.MinLength))
	}
	if schema.MaxLength != nil {
		add(fmt.Sprintf("max=%d", *schema.MaxLength))
	}
	if schema.Minimum != nil {
		add("min=" + formatNumericRule(*schema.Minimum))
	}
	if schema.Maximum != nil {
		add("max=" + formatNumericRule(*schema.Maximum))
	}
	if p := strings.TrimSpace(schema.Pattern); p != "" && !strings.Contains(p, ",") {
		add("regexp=" + p)
	}
	return strings.Join(rules, ",")
}

func formatNumericRule(v float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	if s == "" {
		return "0"
	}
	return s
}

// openAPITypeToCUE converts an OpenAPI schema to a CUE type hint.
func openAPITypeToCUE(s openAPISchema) string {
	if s.Ref != "" {
		// Extract schema name from $ref: "#/components/schemas/Foo"
		parts := strings.Split(s.Ref, "/")
		return parts[len(parts)-1]
	}
	switch s.Type {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	case "array":
		if s.Items != nil {
			return "[..." + openAPITypeToCUE(*s.Items) + "]"
		}
		return "[...]"
	case "object":
		return "{...}"
	default:
		if len(s.Properties) > 0 {
			return "{...}"
		}
		return "string"
	}
}

// deriveOpName generates an operation name from HTTP method + path.
// e.g. GET /users/{id} → GetUsersById
func deriveOpName(method, path string) string {
	lower := strings.ToLower(method)
	if len(lower) == 0 {
		method = ""
	} else {
		method = strings.ToUpper(lower[:1]) + lower[1:]
	}
	// clean path segments
	parts := strings.Split(path, "/")
	var b strings.Builder
	b.WriteString(method)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			inner := p[1 : len(p)-1]
			b.WriteString("By")
			b.WriteString(toPascalCase(inner))
		} else {
			b.WriteString(toPascalCase(p))
		}
	}
	return b.String()
}
