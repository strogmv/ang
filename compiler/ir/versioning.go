package ir

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// IRVersionV1 is the legacy canonical IR schema version.
	IRVersionV1 = "1"
	// IRVersionV2 is the current canonical IR schema version.
	IRVersionV2 = "2"
)

// MigrateToCurrent upgrades schema in-place to the current IR version.
// Empty version is treated as legacy v0.
func MigrateToCurrent(schema *Schema) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}

	switch strings.TrimSpace(schema.IRVersion) {
	case "", "0":
		migrateV0ToV1(schema)
		migrateV1ToV2(schema)
		return nil
	case IRVersionV1:
		migrateV1ToV2(schema)
		return nil
	case IRVersionV2:
		normalizeV2Invariants(schema)
		return nil
	default:
		return fmt.Errorf("unsupported ir_version %q (current=%s)", schema.IRVersion, IRVersionV2)
	}
}

func migrateV0ToV1(schema *Schema) {
	schema.IRVersion = IRVersionV1
	normalizeV1Invariants(schema)
}

func migrateV1ToV2(schema *Schema) {
	normalizeV1Invariants(schema)
	schema.IRVersion = IRVersionV2
	normalizeV2Invariants(schema)
}

func normalizeV1Invariants(schema *Schema) {
	ensureMap(&schema.Metadata)
	for i := range schema.Entities {
		ensureMap(&schema.Entities[i].Metadata)
		for j := range schema.Entities[i].Fields {
			ensureMap(&schema.Entities[i].Fields[j].Metadata)
		}
	}
	for i := range schema.Services {
		ensureMap(&schema.Services[i].Metadata)
		for j := range schema.Services[i].Methods {
			ensureMap(&schema.Services[i].Methods[j].Metadata)
			for k := range schema.Services[i].Methods[j].Sources {
				ensureMap(&schema.Services[i].Methods[j].Sources[k].Metadata)
			}
		}
	}
	for i := range schema.Events {
		ensureMap(&schema.Events[i].Metadata)
	}
	for i := range schema.Endpoints {
		ensureMap(&schema.Endpoints[i].Metadata)
	}
}

func normalizeV2Invariants(schema *Schema) {
	normalizeV1Invariants(schema)
}

func ensureMap(dst *map[string]any) {
	if *dst == nil {
		*dst = map[string]any{}
	}
}

// ToCanonicalJSON normalizes schema version and returns stable indented JSON.
func ToCanonicalJSON(schema *Schema) ([]byte, error) {
	if err := MigrateToCurrent(schema); err != nil {
		return nil, err
	}
	return json.MarshalIndent(schema, "", "  ")
}
