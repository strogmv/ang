package generator

import (
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
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
	}, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatalf("step should be skipped when capabilities are missing")
	}
}
