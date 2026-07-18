package flowir

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestDecodeTimeFormatZeroModes(t *testing.T) {
	defaultMode, err := DecodeAs[TimeFormat](normalizer.FlowStep{
		Action: "time.Format",
		Args:   map[string]any{"input": "createdAt", "output": "formatted"},
	})
	if err != nil {
		t.Fatalf("decode default mode: %v", err)
	}
	if defaultMode.Zero != "format" {
		t.Fatalf("default zero mode = %q, want format", defaultMode.Zero)
	}

	emptyMode, err := DecodeAs[TimeFormat](normalizer.FlowStep{
		Action: "time.Format",
		Args:   map[string]any{"input": "createdAt", "output": "formatted", "zero": "empty"},
	})
	if err != nil {
		t.Fatalf("decode empty mode: %v", err)
	}
	if emptyMode.Zero != "empty" {
		t.Fatalf("empty zero mode = %q, want empty", emptyMode.Zero)
	}

	_, err = Decode(normalizer.FlowStep{
		Action: "time.Format",
		Args:   map[string]any{"input": "createdAt", "output": "formatted", "zero": "null"},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported zero mode") {
		t.Fatalf("invalid zero mode error = %v", err)
	}
}

func TestDecodeMappingMapEntityConstruction(t *testing.T) {
	constructed, err := DecodeAs[MappingMap](normalizer.FlowStep{
		Action: "mapping.Map",
		Args:   map[string]any{"output": "newUser", "entity": "User"},
	})
	if err != nil {
		t.Fatalf("decode entity construction: %v", err)
	}
	if constructed.Input.Source != "" || constructed.Output != "newUser" || constructed.Entity != "User" {
		t.Fatalf("unexpected decoded construction: %#v", constructed)
	}

	_, err = Decode(normalizer.FlowStep{
		Action: "mapping.Map",
		Args:   map[string]any{"output": "target"},
	})
	if err == nil || !strings.Contains(err.Error(), "requires both input/from and output/to") {
		t.Fatalf("invalid map error = %v", err)
	}
}
