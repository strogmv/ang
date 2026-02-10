package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

func runAPIDiff(args []string) {
	fs := flag.NewFlagSet("api-diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	basePath := fs.String("base", "api/openapi.base.yaml", "baseline OpenAPI spec")
	currentPath := fs.String("current", "api/openapi.yaml", "current OpenAPI spec")
	writeBase := fs.Bool("write-base", false, "overwrite base with current")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("API diff FAILED: %v\n", err)
		os.Exit(1)
	}

	if *writeBase {
		data, err := os.ReadFile(*currentPath)
		if err != nil {
			fmt.Printf("API diff FAILED: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*basePath, data, 0644); err != nil {
			fmt.Printf("API diff FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Baseline written to %s\n", *basePath)
		return
	}

	base, err := parseOpenAPI(*basePath)
	if err != nil {
		fmt.Printf("API diff FAILED: %v\n", err)
		os.Exit(1)
	}
	current, err := parseOpenAPI(*currentPath)
	if err != nil {
		fmt.Printf("API diff FAILED: %v\n", err)
		os.Exit(1)
	}

	report := diffOpenAPI(base, current)
	printAPIDiff(report)
}

type apiDoc struct {
	Endpoints map[string]struct{}
	Schemas   map[string]apiSchema
}

type apiSchema struct{ Fields map[string]apiField }
type apiField struct {
	Type   string
	Format string
}
type apiDiffReport struct {
	Breaking  []string
	Additions []string
	Semver    string
}

func parseOpenAPI(path string) (apiDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return apiDoc{}, err
	}
	lines := strings.Split(string(data), "\n")
	doc := apiDoc{Endpoints: make(map[string]struct{}), Schemas: make(map[string]apiSchema)}
	section := ""
	var currentPath, currentMethod, currentSchema string
	_ = currentMethod
	var inSchemas, inProperties bool
	var currentField, currentFieldType, currentFieldFormat string
	var currentFieldIsArray bool
	var currentFieldArrayType string

	finalizeField := func() {
		if currentSchema == "" || currentField == "" {
			return
		}
		schema := doc.Schemas[currentSchema]
		if schema.Fields == nil {
			schema.Fields = make(map[string]apiField)
		}
		fieldType := currentFieldType
		if currentFieldIsArray {
			if currentFieldArrayType != "" {
				fieldType = "array:" + currentFieldArrayType
			} else {
				fieldType = "array"
			}
		}
		schema.Fields[currentField] = apiField{Type: fieldType, Format: currentFieldFormat}
		doc.Schemas[currentSchema] = schema
		currentField = ""
		currentFieldType = ""
		currentFieldFormat = ""
		currentFieldIsArray = false
		currentFieldArrayType = ""
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "paths:") {
			section = "paths"
			continue
		}
		if strings.HasPrefix(line, "components:") {
			section = "components"
			inSchemas = false
			continue
		}
		if section == "components" && strings.HasPrefix(line, "  schemas:") {
			inSchemas = true
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if section == "paths" {
			if strings.HasPrefix(line, "  ") && strings.HasSuffix(trimmed, ":") && indent == 2 {
				currentPath = strings.TrimSuffix(trimmed, ":")
				continue
			}
			if strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") && indent == 4 {
				method := strings.TrimSuffix(trimmed, ":")
				switch method {
				case "get", "post", "put", "patch", "delete":
					currentMethod = method
					if currentPath != "" {
						doc.Endpoints[method+" "+currentPath] = struct{}{}
					}
				default:
					currentMethod = ""
				}
				continue
			}
			continue
		}
		if section == "components" && inSchemas {
			if indent == 4 && strings.HasSuffix(trimmed, ":") && trimmed != "schemas:" {
				finalizeField()
				currentSchema = strings.TrimSuffix(trimmed, ":")
				doc.Schemas[currentSchema] = apiSchema{Fields: make(map[string]apiField)}
				inProperties = false
				continue
			}
			if indent == 6 && trimmed == "properties:" {
				inProperties = true
				continue
			}
			if inProperties && indent == 8 && strings.HasSuffix(trimmed, ":") {
				finalizeField()
				currentField = strings.TrimSuffix(trimmed, ":")
				continue
			}
			if inProperties && indent == 10 && strings.HasPrefix(trimmed, "type:") {
				currentFieldType = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
				currentFieldIsArray = currentFieldType == "array"
				continue
			}
			if inProperties && indent == 10 && strings.HasPrefix(trimmed, "format:") {
				currentFieldFormat = strings.TrimSpace(strings.TrimPrefix(trimmed, "format:"))
				continue
			}
			if inProperties && indent == 10 && trimmed == "items:" {
				continue
			}
			if inProperties && indent == 12 && strings.HasPrefix(trimmed, "type:") && currentFieldIsArray {
				currentFieldArrayType = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
				continue
			}
			if indent <= 6 && inProperties {
				finalizeField()
				inProperties = false
			}
		}
	}
	finalizeField()
	return doc, nil
}

func diffOpenAPI(base, current apiDoc) apiDiffReport {
	report := apiDiffReport{}
	for ep := range base.Endpoints {
		if _, ok := current.Endpoints[ep]; !ok {
			report.Breaking = append(report.Breaking, "Removed endpoint: "+ep)
		}
	}
	for ep := range current.Endpoints {
		if _, ok := base.Endpoints[ep]; !ok {
			report.Additions = append(report.Additions, "Added endpoint: "+ep)
		}
	}
	for name, schema := range base.Schemas {
		currSchema, ok := current.Schemas[name]
		if !ok {
			report.Breaking = append(report.Breaking, "Removed schema: "+name)
			continue
		}
		for field, def := range schema.Fields {
			currField, ok := currSchema.Fields[field]
			if !ok {
				report.Breaking = append(report.Breaking, "Removed field: "+name+"."+field)
				continue
			}
			baseSig := def.Type + "|" + def.Format
			currSig := currField.Type + "|" + currField.Format
			if baseSig != currSig {
				report.Breaking = append(report.Breaking, "Changed field type: "+name+"."+field+" ("+baseSig+" -> "+currSig+")")
			}
		}
		for field := range currSchema.Fields {
			if _, ok := schema.Fields[field]; !ok {
				report.Additions = append(report.Additions, "Added field: "+name+"."+field)
			}
		}
	}
	for name := range current.Schemas {
		if _, ok := base.Schemas[name]; !ok {
			report.Additions = append(report.Additions, "Added schema: "+name)
		}
	}
	if len(report.Breaking) > 0 {
		report.Semver = "major"
	} else if len(report.Additions) > 0 {
		report.Semver = "minor"
	} else {
		report.Semver = "patch"
	}
	sort.Strings(report.Breaking)
	sort.Strings(report.Additions)
	return report
}

func printAPIDiff(report apiDiffReport) {
	fmt.Println("API Diff Report\n--------------")
	if len(report.Breaking) == 0 {
		fmt.Println("Breaking changes: none")
	} else {
		fmt.Println("Breaking changes:")
		for _, item := range report.Breaking {
			fmt.Println("  - " + item)
		}
	}
	if len(report.Additions) == 0 {
		fmt.Println("Additions: none")
	} else {
		fmt.Println("Additions:")
		for _, item := range report.Additions {
			fmt.Println("  - " + item)
		}
	}
	fmt.Printf("Recommended semver bump: %s\n", report.Semver)
}
