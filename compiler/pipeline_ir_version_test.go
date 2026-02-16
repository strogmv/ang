package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestConvertAndTransform_UsesCurrentIRVersion(t *testing.T) {
	t.Parallel()

	schema, err := ConvertAndTransform(
		nil, nil, nil, nil, nil, nil,
		normalizer.ConfigDef{},
		nil, nil,
		nil, nil,
		normalizer.ProjectDef{Name: "pipeline-ir-version"},
	)
	if err != nil {
		t.Fatalf("ConvertAndTransform failed: %v", err)
	}
	if schema == nil {
		t.Fatalf("expected non-nil schema")
	}
	if schema.IRVersion != ir.IRVersionV2 {
		t.Fatalf("expected ir_version=%s, got %s", ir.IRVersionV2, schema.IRVersion)
	}
}
