package main

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type openAPIContractDiff struct {
	AddedOperations   []string
	RemovedOperations []string
	// BreakingChanges are the messages of Breaking, sorted.
	BreakingChanges []string
	Breaking        []contractChange
}

// contractChange is one breaking change: At names what it touches (an
// operation, a schema, or schema.field), Message describes it.
type contractChange struct {
	At      string
	Message string
}

// diffOpenAPIContractsWithRecovery lets a fixed generator replace a malformed
// generated baseline. The current contract is still parsed strictly; only the
// unusable previous baseline is discarded.
func diffOpenAPIContractsWithRecovery(previous, current []byte) (openAPIContractDiff, bool, error) {
	diff, err := diffOpenAPIContracts(previous, current)
	if err == nil || !strings.HasPrefix(err.Error(), "parse previous OpenAPI") {
		return diff, false, err
	}
	diff, currentErr := diffOpenAPIContracts(nil, current)
	if currentErr != nil {
		return openAPIContractDiff{}, false, currentErr
	}
	return diff, true, nil
}

func diffOpenAPIContracts(previous, current []byte) (openAPIContractDiff, error) {
	before, err := openAPIOperations(previous)
	if err != nil {
		return openAPIContractDiff{}, fmt.Errorf("parse previous OpenAPI: %w", err)
	}
	after, err := openAPIOperations(current)
	if err != nil {
		return openAPIContractDiff{}, fmt.Errorf("parse generated OpenAPI: %w", err)
	}
	var diff openAPIContractDiff
	for operation := range after {
		if _, exists := before[operation]; !exists {
			diff.AddedOperations = append(diff.AddedOperations, operation)
		}
	}
	for operation := range before {
		if _, exists := after[operation]; !exists {
			diff.RemovedOperations = append(diff.RemovedOperations, operation)
			diff.Breaking = append(diff.Breaking, contractChange{At: operation, Message: "removed operation " + operation})
		}
	}
	beforeSchemas, err := openAPISchemas(previous)
	if err != nil {
		return openAPIContractDiff{}, fmt.Errorf("parse previous OpenAPI schemas: %w", err)
	}
	afterSchemas, err := openAPISchemas(current)
	if err != nil {
		return openAPIContractDiff{}, fmt.Errorf("parse generated OpenAPI schemas: %w", err)
	}
	uses, err := openAPISchemaUses(previous)
	if err != nil {
		return openAPIContractDiff{}, fmt.Errorf("parse previous OpenAPI schemas: %w", err)
	}
	afterUses, err := openAPISchemaUses(current)
	if err != nil {
		return openAPIContractDiff{}, fmt.Errorf("parse generated OpenAPI schemas: %w", err)
	}
	for name, use := range afterUses {
		uses[name] |= use
	}
	diff.Breaking = append(diff.Breaking, diffOpenAPISchemas(beforeSchemas, afterSchemas, uses)...)
	sort.Strings(diff.AddedOperations)
	sort.Strings(diff.RemovedOperations)
	sort.Slice(diff.Breaking, func(i, j int) bool { return diff.Breaking[i].Message < diff.Breaking[j].Message })
	for _, change := range diff.Breaking {
		diff.BreakingChanges = append(diff.BreakingChanges, change.Message)
	}
	return diff, nil
}

type openAPIContractSchema struct {
	Required   map[string]struct{}
	Properties map[string]string
}

func openAPISchemas(data []byte) (map[string]openAPIContractSchema, error) {
	out := map[string]openAPIContractSchema{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return out, nil
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string                  `yaml:"required"`
				Properties map[string]map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for name, raw := range doc.Components.Schemas {
		schema := openAPIContractSchema{Required: map[string]struct{}{}, Properties: map[string]string{}}
		for _, field := range raw.Required {
			schema.Required[field] = struct{}{}
		}
		for field, definition := range raw.Properties {
			typeName, _ := definition["type"].(string)
			ref, _ := definition["$ref"].(string)
			items := ""
			if itemMap, ok := definition["items"].(map[string]any); ok {
				itemType, _ := itemMap["type"].(string)
				itemRef, _ := itemMap["$ref"].(string)
				items = itemType + itemRef
			}
			schema.Properties[field] = typeName + ref + items
		}
		out[name] = schema
	}
	return out, nil
}

// schemaUse says where a schema travels: in requests (body, parameters), in
// responses, or both.
type schemaUse uint8

const (
	useRequest schemaUse = 1 << iota
	useResponse
)

// openAPISchemaUses finds, for every component schema an operation reaches
// (through $ref, at any depth), whether it is read from a request or written
// in a response.
func openAPISchemaUses(data []byte) (map[string]schemaUse, error) {
	uses := map[string]schemaUse{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return uses, nil
	}
	var doc struct {
		Paths      map[string]map[string]any `yaml:"paths"`
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var queue []string
	mark := func(name string, use schemaUse) {
		if uses[name]&use != use {
			uses[name] |= use
			queue = append(queue, name)
		}
	}
	for _, item := range doc.Paths {
		for key, operation := range item {
			if key == "parameters" {
				for _, ref := range openAPIRefs(operation) {
					mark(ref, useRequest)
				}
				continue
			}
			fields, ok := operation.(map[string]any)
			if !ok {
				continue
			}
			for _, ref := range openAPIRefs(fields["requestBody"]) {
				mark(ref, useRequest)
			}
			for _, ref := range openAPIRefs(fields["parameters"]) {
				mark(ref, useRequest)
			}
			for _, ref := range openAPIRefs(fields["responses"]) {
				mark(ref, useResponse)
			}
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, ref := range openAPIRefs(doc.Components.Schemas[name]) {
			mark(ref, uses[name])
		}
	}
	return uses, nil
}

// openAPIRefs lists the component schemas `$ref`erenced anywhere under node.
func openAPIRefs(node any) []string {
	var refs []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok {
				if name, found := strings.CutPrefix(ref, "#/components/schemas/"); found {
					refs = append(refs, name)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(node)
	return refs
}

// diffOpenAPISchemas lists the changes that break a client built against the
// previous contract. A field a response gains (required or not) breaks no
// client; one a request must now carry does. A schema no operation reaches
// is judged by its name (<Op>Request, <Op>Response), else both ways.
func diffOpenAPISchemas(before, after map[string]openAPIContractSchema, uses map[string]schemaUse) []contractChange {
	var breaking []contractChange
	add := func(at, format string, args ...any) {
		breaking = append(breaking, contractChange{At: at, Message: fmt.Sprintf(format, args...)})
	}
	for name, oldSchema := range before {
		newSchema, exists := after[name]
		if !exists {
			add(name, "removed schema %s", name)
			continue
		}
		use := uses[name]
		if use == 0 {
			// GET inputs are inline query parameters, so no operation
			// references their <Op>Request schema; ANG names both ends.
			switch {
			case strings.HasSuffix(name, "Request"):
				use = useRequest
			case strings.HasSuffix(name, "Response"):
				use = useResponse
			default:
				use = useRequest | useResponse
			}
		}
		for field, oldType := range oldSchema.Properties {
			newType, exists := newSchema.Properties[field]
			if !exists {
				add(name+"."+field, "removed field %s.%s", name, field)
				continue
			}
			if oldType != newType {
				add(name+"."+field, "changed field type %s.%s (%s -> %s)", name, field, oldType, newType)
			}
			_, wasRequired := oldSchema.Required[field]
			_, nowRequired := newSchema.Required[field]
			if !wasRequired && nowRequired && use&useRequest != 0 {
				add(name+"."+field, "field became required %s.%s", name, field)
			}
			if wasRequired && !nowRequired && use&useResponse != 0 {
				add(name+"."+field, "response field no longer required %s.%s", name, field)
			}
		}
		if use&useRequest != 0 {
			for field := range newSchema.Required {
				if _, existed := oldSchema.Properties[field]; !existed {
					add(name+"."+field, "added required field %s.%s", name, field)
				}
			}
		}
	}
	return breaking
}

func openAPIOperations(data []byte) (map[string]struct{}, error) {
	operations := map[string]struct{}{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return operations, nil
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	methods := map[string]struct{}{"get": {}, "put": {}, "post": {}, "delete": {}, "options": {}, "head": {}, "patch": {}, "trace": {}}
	for path, item := range doc.Paths {
		for method := range item {
			method = strings.ToLower(strings.TrimSpace(method))
			if _, ok := methods[method]; ok {
				operations[strings.ToUpper(method)+" "+path] = struct{}{}
			}
		}
	}
	return operations, nil
}
