package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler/emitter"
)

// copyGRPCProtos copies the generated proto of a `GRPC:` declaration to the
// directories its copy_proto_to lists (a Unity client's Assets/Proto, say).
// The build generates into a staging directory and promotes it afterwards, so
// a destination outside the project can only be written from here, resolved
// against the project's real backend directory.
func (r *buildRun) copyGRPCProtos() error {
	def := r.irSchema.GRPC
	if def == nil || len(def.CopyProtoTo) == 0 {
		return nil
	}
	rel := filepath.FromSlash(emitter.GRPCProtoPath(def))
	for _, s := range r.summaries {
		if !strings.EqualFold(strings.TrimSpace(s.Lang), "go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(filepath.FromSlash(s.StagedBackend), "api", "grpc", rel))
		if err != nil {
			return fmt.Errorf("read generated proto: %w", err)
		}
		for _, dir := range def.CopyProtoTo {
			dst := filepath.Join(filepath.FromSlash(s.Backend), filepath.FromSlash(dir), rel)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return fmt.Errorf("copy proto to %s: %w", dir, err)
			}
			if old, err := os.ReadFile(dst); err == nil && string(old) == string(src) {
				continue
			}
			if err := os.WriteFile(dst, src, 0o644); err != nil {
				return fmt.Errorf("copy proto to %s: %w", dir, err)
			}
			r.logText("Copied gRPC Proto: %s", dst)
		}
		break // one backend carries the contract
	}
	return nil
}
