package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func TestCopyGRPCProtosWritesOutsideTheProject(t *testing.T) {
	root := t.TempDir()
	staged := filepath.Join(root, "stage")
	backend := filepath.Join(root, "server")
	proto := filepath.Join(staged, "api", "grpc", "game", "v1", "game.proto")
	if err := os.MkdirAll(filepath.Dir(proto), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proto, []byte("syntax = \"proto3\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &buildRun{
		irSchema:  &ir.Schema{GRPC: &ir.GRPC{Package: "game.v1", CopyProtoTo: []string{"../client/Assets/Proto"}}},
		summaries: []buildTargetSummary{{Lang: "go", Backend: filepath.ToSlash(backend), StagedBackend: filepath.ToSlash(staged)}},
		logText:   func(string, ...any) {},
	}
	if err := r.copyGRPCProtos(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "client", "Assets", "Proto", "game", "v1", "game.proto"))
	if err != nil || string(got) != "syntax = \"proto3\";\n" {
		t.Fatalf("copy missing or wrong: %q %v", got, err)
	}
	// A second run changes nothing and does not fail.
	if err := r.copyGRPCProtos(); err != nil {
		t.Fatal(err)
	}
}

func TestCopyGRPCProtosNoopWithoutDeclaration(t *testing.T) {
	r := &buildRun{irSchema: &ir.Schema{}, logText: func(string, ...any) {}}
	if err := r.copyGRPCProtos(); err != nil {
		t.Fatal(err)
	}
}
