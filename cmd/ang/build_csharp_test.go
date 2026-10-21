package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/compiler/emitter"
)

func TestWriteCSharpFilesRemovesStaleOnly(t *testing.T) {
	out := t.TempDir()
	logf := func(string, ...any) {}
	if err := writeCSharpFiles(out, map[string][]byte{"Messages/A.cs": []byte("a"), "Grpc/B.cs": []byte("b")}, logf); err != nil {
		t.Fatal(err)
	}
	hand := filepath.Join(out, "Mine.cs") // hand-written neighbour, not in the manifest
	if err := os.WriteFile(hand, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCSharpFiles(out, map[string][]byte{"Messages/A.cs": []byte("a2")}, logf); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "Grpc", "B.cs")); !os.IsNotExist(err) {
		t.Fatal("stale generated file was not removed")
	}
	if b, _ := os.ReadFile(filepath.Join(out, "Messages", "A.cs")); string(b) != "a2" {
		t.Fatalf("changed file not rewritten: %q", b)
	}
	if _, err := os.Stat(hand); err != nil {
		t.Fatal("a file outside the manifest must never be removed")
	}
}

func TestRenderCSharpFilesWithProtoc(t *testing.T) {
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed")
	}
	root := t.TempDir()
	def := &ir.GRPC{
		Package: "game.v1", CSharpNamespace: "Game.V1", Listen: ":1",
		Messages: []ir.GRPCMessage{
			{Name: "In", Oneofs: []ir.GRPCOneof{{Name: "msg", Members: []ir.GRPCField{{Name: "a", Number: 1, Type: "A"}}}}},
			{Name: "Out", Oneofs: []ir.GRPCOneof{{Name: "msg", Members: []ir.GRPCField{{Name: "b", Number: 1, Type: "A"}}}}},
			{Name: "A", Fields: []ir.GRPCField{{Name: "x", Number: 1, Type: "string"}}},
		},
		Services: []ir.GRPCService{{Name: "S", Methods: []ir.GRPCMethod{{Name: "Run", Stream: "bidi", Request: "In", Response: "Out"}}}},
	}
	// The proto the emitter would have written into the staged backend.
	protoDir := filepath.Join(root, "game", "v1")
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := emitter.RenderGRPCProto(def, "example.com/game")
	if err := os.WriteFile(filepath.Join(protoDir, "game.proto"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := renderCSharpFiles(def, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Messages/Game.cs", "Grpc/SGrpc.cs", "Sessions/SRunSession.cs"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s; have %v", want, keys(files))
		}
	}
	if !strings.Contains(string(files["Messages/Game.cs"]), "namespace Game.V1") {
		t.Error("protoc output lacks the declared namespace")
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
