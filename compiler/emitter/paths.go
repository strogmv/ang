package emitter

import "path/filepath"

// outDir builds an output path relative to e.OutputDir.
// Use this instead of repeating filepath.Join(e.OutputDir, ...) throughout the emitter.
//
//	e.outDir("internal", "domain")   →  <out>/internal/domain
//	e.outDir("cmd", "server")         →  <out>/cmd/server
func (e *Emitter) outDir(parts ...string) string {
	return filepath.Join(append([]string{e.OutputDir}, parts...)...)
}
