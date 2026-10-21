package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/compiler/emitter"
)

// csharpManifest lists what the last build wrote under csharp_out, so a file
// that the contract no longer produces is removed and nothing else is touched.
const csharpManifest = "ang-generated.txt"

// emitCSharp writes the C# client of a `GRPC:` declaration to its csharp_out:
// Messages/ (protoc --csharp_out), Grpc/ (stubs), Sessions/ (typed bidi sessions).
// Like copy_proto_to it runs after the staged build is promoted, resolved against the
// project's real backend directory, because the destination may lie outside it.
func (r *buildRun) emitCSharp() error {
	def := r.irSchema.GRPC
	if def == nil || def.CSharpOut == "" {
		return nil
	}
	for _, s := range r.summaries {
		if !strings.EqualFold(strings.TrimSpace(s.Lang), "go") {
			continue
		}
		staged := filepath.FromSlash(s.StagedBackend)
		out := filepath.Join(filepath.FromSlash(s.Backend), filepath.FromSlash(def.CSharpOut))
		files, err := renderCSharpFiles(def, filepath.Join(staged, "api", "grpc"))
		if err != nil {
			return err
		}
		return writeCSharpFiles(out, files, r.logText)
	}
	return nil
}

// renderCSharpFiles returns relative path -> content for everything the C# client needs.
func renderCSharpFiles(def *ir.GRPC, protoRoot string) (map[string][]byte, error) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		return nil, fmt.Errorf("csharp_out needs protoc on PATH to generate the message classes: %w", err)
	}
	tmp, err := os.MkdirTemp("", "ang-csharp-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	var stderr bytes.Buffer
	cmd := exec.Command(protoc, "-I", protoRoot, "--csharp_out="+tmp, filepath.FromSlash(emitter.GRPCProtoPath(def)))
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("protoc --csharp_out: %w\n%s", err, stderr.String())
	}
	files := map[string][]byte{}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cs") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tmp, e.Name()))
		if err != nil {
			return nil, err
		}
		files[filepath.ToSlash(filepath.Join("Messages", e.Name()))] = data
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("protoc --csharp_out produced no .cs files")
	}
	for _, svc := range def.Services {
		files["Grpc/"+svc.Name+"Grpc.cs"] = []byte(emitter.RenderCSharpStubs(def, svc))
	}
	for _, sm := range emitter.SessionMethods(def) {
		files["Sessions/"+sm.Service.Name+sm.Method.Name+"Session.cs"] = []byte(emitter.RenderCSharpSession(def, sm.Service, sm.Method))
	}
	return files, nil
}

// writeCSharpFiles writes files under out (only the ones that changed) and removes
// files the previous build listed in the manifest but this one no longer produces.
func writeCSharpFiles(out string, files map[string][]byte, logf func(string, ...any)) error {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		dst := filepath.Join(out, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if old, err := os.ReadFile(dst); err == nil && bytes.Equal(old, files[n]) {
			continue
		}
		if err := os.WriteFile(dst, files[n], 0o644); err != nil {
			return err
		}
		logf("Generated C#: %s", dst)
	}
	mpath := filepath.Join(out, csharpManifest)
	if prev, err := os.ReadFile(mpath); err == nil {
		for _, line := range strings.Split(string(prev), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if _, keep := files[line]; !keep {
				_ = os.Remove(filepath.Join(out, filepath.FromSlash(line)))
				logf("Removed stale C#: %s", line)
			}
		}
	}
	manifest := "# Files ANG generated here; ang build rewrites the list and removes a listed file the contract no longer produces.\n" + strings.Join(names, "\n") + "\n"
	return os.WriteFile(mpath, []byte(manifest), 0o644)
}
