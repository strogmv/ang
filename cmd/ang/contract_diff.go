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
	BreakingChanges   []string
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
			diff.BreakingChanges = append(diff.BreakingChanges, "removed operation "+operation)
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
	diff.BreakingChanges = append(diff.BreakingChanges, diffOpenAPISchemas(beforeSchemas, afterSchemas)...)
	sort.Strings(diff.AddedOperations)
	sort.Strings(diff.RemovedOperations)
	sort.Strings(diff.BreakingChanges)
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

func diffOpenAPISchemas(before, after map[string]openAPIContractSchema) []string {
	var breaking []string
	for name, oldSchema := range before {
		newSchema, exists := after[name]
		if !exists {
			breaking = append(breaking, "removed schema "+name)
			continue
		}
		for field, oldType := range oldSchema.Properties {
			newType, exists := newSchema.Properties[field]
			if !exists {
				breaking = append(breaking, fmt.Sprintf("removed field %s.%s", name, field))
				continue
			}
			if oldType != newType {
				breaking = append(breaking, fmt.Sprintf("changed field type %s.%s (%s -> %s)", name, field, oldType, newType))
			}
			if _, wasRequired := oldSchema.Required[field]; !wasRequired {
				if _, nowRequired := newSchema.Required[field]; nowRequired {
					breaking = append(breaking, fmt.Sprintf("field became required %s.%s", name, field))
				}
			}
		}
		for field := range newSchema.Required {
			if _, existed := oldSchema.Properties[field]; !existed {
				breaking = append(breaking, fmt.Sprintf("added required field %s.%s", name, field))
			}
		}
	}
	sort.Strings(breaking)
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
