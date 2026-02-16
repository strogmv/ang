package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestPipelineMissingImplAudit_NoFalsePositivesForFlowImplManual(t *testing.T) {
	basePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(basePath, "cue", "api"), 0o755); err != nil {
		t.Fatalf("mkdir cue/api: %v", err)
	}

	apiCue := `package api

FlowMethod: {
	service: "auth"
	input: {}
	flow: [{
		action: "logic.Call"
		func:   "Noop"
		args:   []
	}]
}

ImplMethod: {
	service: "auth"
	input: {}
	impls: go: {
		code: "return nil, nil"
	}
}

ManualMethod: {
	service: "auth"
	input: {}
}

MissingMethod: {
	service: "auth"
	input: {}
}
`
	if err := os.WriteFile(filepath.Join(basePath, "cue", "api", "api.cue"), []byte(apiCue), 0o644); err != nil {
		t.Fatalf("write cue/api/api.cue: %v", err)
	}

	entities, services, endpoints, repos, events, bizErrors, schedules, _, err := compiler.RunPipeline(basePath)
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}
	schema, err := compiler.ConvertAndTransform(
		entities, services, events, bizErrors, endpoints, repos,
		normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{},
	)
	if err != nil {
		t.Fatalf("ConvertAndTransform failed: %v", err)
	}

	outDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outDir, "internal", "service"), 0o755); err != nil {
		t.Fatalf("mkdir internal/service: %v", err)
	}
	manual := `package service

func (s *AuthImpl) ManualMethod() {}
`
	if err := os.WriteFile(filepath.Join(outDir, "internal", "service", "auth.manual.go"), []byte(manual), 0o644); err != nil {
		t.Fatalf("write auth.manual.go: %v", err)
	}

	em := emitter.New(outDir, filepath.Join(outDir, "frontend"), filepath.Join("templates"))
	em.GoModule = "example.com/test"

	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatalf("EmitServiceFromIR failed: %v", err)
	}
	if err := em.EmitServiceImplFromIR(schema, nil); err != nil {
		t.Fatalf("EmitServiceImplFromIR failed: %v", err)
	}
	if err := em.EmitCachedServiceFromIR(schema); err != nil {
		t.Fatalf("EmitCachedServiceFromIR failed: %v", err)
	}

	if len(em.MissingImpls) != 1 {
		t.Fatalf("expected exactly 1 missing implementation, got %d (%+v)", len(em.MissingImpls), em.MissingImpls)
	}
	got := em.MissingImpls[0]
	if got.Service != "Auth" || got.Method != "MissingMethod" {
		t.Fatalf("unexpected missing implementation: %+v", got)
	}
}
