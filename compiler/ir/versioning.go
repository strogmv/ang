package ir

import (
	"fmt"
	"strings"
)

const (
	// IRVersionV1 is the current canonical IR schema version.
	IRVersionV1 = "1"
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
		return nil
	case IRVersionV1:
		return nil
	default:
		return fmt.Errorf("unsupported ir_version %q (current=%s)", schema.IRVersion, IRVersionV1)
	}
}

func migrateV0ToV1(schema *Schema) {
	schema.IRVersion = IRVersionV1
	if schema.Metadata == nil {
		schema.Metadata = make(map[string]any)
	}
}
