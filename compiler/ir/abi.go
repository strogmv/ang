package ir

import (
	"fmt"
	"math"
)

// ValidateABIV2 enforces the stable IR ABI contract for version 2.
// It must be called on boundaries between normalization, transforms and emitters.
func ValidateABIV2(schema *Schema) error {
	if schema == nil {
		return fmt.Errorf("nil schema")
	}
	if schema.IRVersion != IRVersionV2 {
		return fmt.Errorf("ir_version=%q, expected %q", schema.IRVersion, IRVersionV2)
	}
	if err := validateJSONValue(schema.Metadata, "schema.metadata"); err != nil {
		return err
	}
	for i := range schema.Entities {
		if err := validateJSONValue(schema.Entities[i].Metadata, fmt.Sprintf("entities[%d].metadata", i)); err != nil {
			return err
		}
		for j := range schema.Entities[i].Fields {
			if err := validateJSONValue(schema.Entities[i].Fields[j].Metadata, fmt.Sprintf("entities[%d].fields[%d].metadata", i, j)); err != nil {
				return err
			}
		}
	}
	for i := range schema.Services {
		if err := validateJSONValue(schema.Services[i].Metadata, fmt.Sprintf("services[%d].metadata", i)); err != nil {
			return err
		}
		for j := range schema.Services[i].Methods {
			if err := validateJSONValue(schema.Services[i].Methods[j].Metadata, fmt.Sprintf("services[%d].methods[%d].metadata", i, j)); err != nil {
				return err
			}
			for k := range schema.Services[i].Methods[j].Sources {
				if err := validateJSONValue(schema.Services[i].Methods[j].Sources[k].Metadata, fmt.Sprintf("services[%d].methods[%d].sources[%d].metadata", i, j, k)); err != nil {
					return err
				}
			}
		}
	}
	for i := range schema.Events {
		if err := validateJSONValue(schema.Events[i].Metadata, fmt.Sprintf("events[%d].metadata", i)); err != nil {
			return err
		}
	}
	for i := range schema.Endpoints {
		if err := validateJSONValue(schema.Endpoints[i].Metadata, fmt.Sprintf("endpoints[%d].metadata", i)); err != nil {
			return err
		}
	}
	return nil
}

func validateJSONValue(v any, path string) error {
	switch x := v.(type) {
	case nil, bool, string:
		return nil
	case int, int8, int16, int32, int64:
		return nil
	case uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return fmt.Errorf("%s contains non-finite float32", path)
		}
		return nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("%s contains non-finite float64", path)
		}
		return nil
	case []any:
		for i := range x {
			if err := validateJSONValue(x[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		for k, child := range x {
			if err := validateJSONValue(child, fmt.Sprintf("%s.%s", path, k)); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s contains non-ABI value of type %T", path, v)
	}
}
