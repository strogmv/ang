package generator

import (
	"strings"
	"testing"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
)

func TestExecute_SkipsMissingCapabilities(t *testing.T) {
	td := normalizer.TargetDef{Name: "go"}
	caps := compiler.CapabilitySet{
		compiler.CapabilityHTTP: true,
	}

	called := false
	err := Execute(td, caps, []Step{
		{
			Name:     "Needs SQL",
			Requires: []compiler.Capability{compiler.CapabilitySQLRepo},
			Run: func() error {
				called = true
				return nil
			},
		},
	}, func(string, ...interface{}) {}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatalf("step should be skipped when capabilities are missing")
	}
}

func TestExecuteRunsAdjacentParallelSafeStepsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	steps := []Step{
		{Name: "one", ParallelSafe: true, Run: func() error { started <- "one"; <-release; return nil }},
		{Name: "two", ParallelSafe: true, Run: func() error { started <- "two"; <-release; return nil }},
	}
	done := make(chan error, 1)
	go func() {
		done <- Execute(normalizer.TargetDef{Name: "go"}, compiler.CapabilitySet{}, steps, nil, nil)
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("parallel-safe steps did not start concurrently")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestStepRegistry_DuplicateStepNameFailsFast(t *testing.T) {
	t.Parallel()

	reg := NewStepRegistry()
	reg.Register(Step{Name: "Service Impls", Run: func() error { return nil }})
	reg.Register(Step{Name: "Service Impls", Run: func() error { return nil }})

	if err := reg.Err(); err == nil || !strings.Contains(err.Error(), "duplicate step name") {
		t.Fatalf("expected duplicate step name error, got: %v", err)
	}
	if got := len(reg.Steps()); got != 1 {
		t.Fatalf("expected duplicate step to be ignored, got %d steps", got)
	}
}

func TestStepRegistry_DuplicateArtifactKeyFailsFast(t *testing.T) {
	t.Parallel()

	reg := NewStepRegistry()
	reg.Register(Step{Name: "Service Impls", ArtifactKey: "go:service_impl", Run: func() error { return nil }})
	reg.Register(Step{Name: "Service Implementations Alt", ArtifactKey: "go:service_impl", Run: func() error { return nil }})

	if err := reg.Err(); err == nil || !strings.Contains(err.Error(), "duplicate artifact key") {
		t.Fatalf("expected duplicate artifact key error, got: %v", err)
	}
	if got := len(reg.Steps()); got != 1 {
		t.Fatalf("expected duplicate artifact key step to be ignored, got %d steps", got)
	}
}

func TestStepRegistry_Execute(t *testing.T) {
	td := normalizer.TargetDef{Name: "python"}
	caps := compiler.CapabilitySet{
		compiler.CapabilityHTTP:                 true,
		compiler.CapabilityProfileGoLegacy:      false,
		compiler.CapabilityProfilePythonFastAPI: true,
	}

	reg := NewStepRegistry()
	called := false
	reg.Register(Step{
		Name:     "OpenAPI",
		Requires: []compiler.Capability{compiler.CapabilityHTTP},
		Run: func() error {
			called = true
			return nil
		},
	})

	if err := reg.Execute(td, caps, func(string, ...interface{}) {}, nil); err != nil {
		t.Fatalf("registry execute: %v", err)
	}
	if !called {
		t.Fatalf("expected registered step to run")
	}
}
