package emitter

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// Method files rendered in parallel must be the same bytes, and their flow
// warnings must reach WarningSink in the same order, as one worker gives.
func TestServiceImplParallelMatchesSequential(t *testing.T) {
	schema := &ir.Schema{}
	for s := 0; s < 3; s++ {
		svc := ir.Service{Name: fmt.Sprintf("Svc%d", s), Source: fmt.Sprintf("cue/api/svc%d.cue:1", s)}
		for m := 0; m < 6; m++ {
			name := fmt.Sprintf("Op%d", m)
			svc.Methods = append(svc.Methods, ir.Method{
				Name:   name,
				Source: fmt.Sprintf("cue/api/svc%d.cue:%d", s, 10+m),
				Input:  &ir.Entity{Name: name + "Request"},
				Output: &ir.Entity{Name: name + "Response"},
				Flow: []ir.FlowStep{
					// ignoreErr without a reason is reported as FLOW_IGNORE_ERR.
					{Action: "logic.Call", Args: map[string]any{"func": "(func() error { return nil })", "ignoreErr": true}},
				},
			})
		}
		schema.Services = append(schema.Services, svc)
	}

	run := func(workers int) (string, []normalizer.Warning) {
		root := filepath.Join(t.TempDir(), "out")
		em := New(root, filepath.Join(root, "frontend"), "templates")
		em.GoModule = "example.com/test"
		em.ServiceImplWorkers = workers
		var warnings []normalizer.Warning
		em.WarningSink = func(w normalizer.Warning) { warnings = append(warnings, w) }
		if err := em.EmitServiceImplFromIR(schema, nil); err != nil {
			t.Fatalf("workers=%d: %v", workers, err)
		}
		hash, err := hashTree(root)
		if err != nil {
			t.Fatal(err)
		}
		return hash, warnings
	}

	sequentialHash, sequentialWarnings := run(1)
	if len(sequentialWarnings) == 0 {
		t.Fatal("the fixture must produce flow warnings to compare their order")
	}
	for i := 0; i < 3; i++ {
		parallelHash, parallelWarnings := run(8)
		if parallelHash != sequentialHash {
			t.Fatalf("parallel output differs from sequential")
		}
		if !reflect.DeepEqual(parallelWarnings, sequentialWarnings) {
			t.Fatalf("warnings differ:\nparallel:   %+v\nsequential: %+v", parallelWarnings, sequentialWarnings)
		}
	}
}
