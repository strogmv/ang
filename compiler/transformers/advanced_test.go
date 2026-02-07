package transformers

import (
	"testing"
	"github.com/strogmv/ang/compiler/ir"
)

func TestProfilingTransformer(t *testing.T) {
	schema := &ir.Schema{
		Metadata: make(map[string]any),
	}
	
	tr := &ProfilingTransformer{}
	err := tr.Transform(schema)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	
	if schema.Metadata["profiling_enabled"] != true {
		t.Errorf("Expected profiling_enabled to be true")
	}
	if schema.Metadata["pprof_endpoint"] != "/debug/pprof" {
		t.Errorf("Expected pprof_endpoint to be /debug/pprof, got %v", schema.Metadata["pprof_endpoint"])
	}
}
