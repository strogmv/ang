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

type migrationStep struct {
	From  string
	To    string
	Apply func(*Schema)
}

var migrationRegistry = []migrationStep{
	{From: "0", To: IRVersionV1, Apply: migrateV0ToV1},
	{From: IRVersionV1, To: IRVersionV2, Apply: migrateV1ToV2},
}

// CurrentVersion returns the canonical IR version produced by the compiler.
func CurrentVersion() string {
	return IRVersionV2
}

// RegisteredMigrations returns available migration edges.
func RegisteredMigrations() []string {
	out := make([]string, 0, len(migrationRegistry))
	for _, step := range migrationRegistry {
		out = append(out, step.From+"->"+step.To)
	}
	return out
}

// MigrateToCurrent upgrades schema in-place to the current IR version.
// Empty version is treated as legacy v0.
func MigrateToCurrent(schema *Schema) error {
	return MigrateToVersion(schema, CurrentVersion())
}

// MigrateToVersion upgrades schema in-place to a target IR version.
func MigrateToVersion(schema *Schema, targetVersion string) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}
	target := normalizeVersion(targetVersion)
	if target != CurrentVersion() {
		return fmt.Errorf("unsupported target ir_version %q (current=%s)", targetVersion, CurrentVersion())
	}
	current := normalizeVersion(schema.IRVersion)

	switch current {
	case "0", IRVersionV1, IRVersionV2:
	default:
		return fmt.Errorf("unsupported ir_version %q (current=%s)", schema.IRVersion, CurrentVersion())
	}

	for current != target {
		step, ok := nextMigrationStep(current)
		if !ok {
			return fmt.Errorf("no migration path from ir_version %q to %q", current, target)
		}
		step.Apply(schema)
		current = normalizeVersion(schema.IRVersion)
	}

	switch target {
	case IRVersionV1:
		normalizeV1Invariants(schema)
	case IRVersionV2:
		normalizeV2Invariants(schema)
	default:
		return fmt.Errorf("unsupported target ir_version %q (current=%s)", target, CurrentVersion())
	}
	return nil
}

func nextMigrationStep(from string) (migrationStep, bool) {
	for _, step := range migrationRegistry {
		if step.From == from {
			return step, true
		}
	}
	return migrationStep{}, false
}

func normalizeVersion(raw string) string {
	switch strings.TrimSpace(raw) {
	case "", "0":
		return "0"
	default:
		return strings.TrimSpace(raw)
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
