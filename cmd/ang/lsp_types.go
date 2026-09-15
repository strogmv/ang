package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const lspTypesSource = "ang-types"

// lspTypeCheck type-checks embedded Go of the saved workspace the way
// `ang vet logic --types` does: a dry run into a temporary directory and
// go build -overlay, so the project is not touched. The editor's ang is often
// not the pinned revision, so a lock mismatch does not stop the check.
func lspTypeCheck(workspaceRoot string) (typeCheckReport, error) {
	return typeCheckEmbeddedGo(workspaceRoot, []string{"./internal/service/..."}, "", true)
}

// scheduleTypeCheck starts a type check of the saved files in the background.
// Only one runs at a time; a save during a run asks for one more run after it,
// so the last result always reflects the last save.
func (s *lspServer) scheduleTypeCheck() {
	if s.typeCheck == nil {
		return
	}
	s.mu.Lock()
	s.typeGeneration++
	if s.typeRunning {
		s.typeRerun = true
		s.mu.Unlock()
		return
	}
	s.typeRunning = true
	s.mu.Unlock()
	go s.runTypeChecks()
}

func (s *lspServer) runTypeChecks() {
	for {
		s.mu.Lock()
		generation := s.typeGeneration
		root := s.workspaceRoot
		s.typeRerun = false
		s.mu.Unlock()

		// The check runs the compiler in this process, like the semantic
		// diagnostics do; both use its global state, so they take turns.
		s.analyzeMu.Lock()
		report, err := s.typeCheck(root)
		s.analyzeMu.Unlock()

		s.mu.Lock()
		stale := generation != s.typeGeneration
		if !stale {
			if err != nil {
				// A project that does not generate is reported by the semantic
				// diagnostics; keep the editor free of a second copy.
				fmt.Fprintf(os.Stderr, "ang lsp: type check: %v\n", err)
				s.typeDiags = nil
			} else {
				s.typeDiags = typeCheckDiagnostics(root, report)
			}
		}
		rerun := s.typeRerun
		if !rerun {
			s.typeRunning = false
		}
		done := s.typeDone
		s.mu.Unlock()

		if !stale {
			_ = s.publishAllDiagnostics()
		}
		if done != nil {
			select {
			case done <- struct{}{}:
			default:
			}
		}
		if !rerun {
			return
		}
	}
}

// typeCheckDiagnostics places each compiler error on its CUE line; an error
// that maps to no CUE line stays on the generated file.
func typeCheckDiagnostics(workspaceRoot string, report typeCheckReport) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, e := range report.Errors {
		file, line, column := e.CUEFile, e.CUELine, e.CUEColumn
		if file == "" {
			file, line, column = e.GeneratedFile, e.GeneratedLine, e.GeneratedColumn
		}
		if file == "" || line <= 0 {
			continue
		}
		path := filepath.FromSlash(file)
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspaceRoot, path)
		}
		start := maxInt(column-1, 0)
		uri := pathToURI(path)
		out[uri] = append(out[uri], map[string]any{
			"range": map[string]any{
				"start": map[string]int{"line": line - 1, "character": start},
				"end":   map[string]int{"line": line - 1, "character": start + 1},
			},
			"severity": 1,
			"source":   lspTypesSource,
			"message":  e.Message,
		})
	}
	return out
}

// mergeDiagnostics returns semantic diagnostics with type errors added; the
// cached semantic map is not modified.
func mergeDiagnostics(semantic, types map[string][]map[string]any) map[string][]map[string]any {
	if len(types) == 0 {
		return semantic
	}
	out := make(map[string][]map[string]any, len(semantic)+len(types))
	for uri, list := range semantic {
		out[uri] = list
	}
	for uri, list := range types {
		merged := make([]map[string]any, 0, len(out[uri])+len(list))
		merged = append(merged, out[uri]...)
		merged = append(merged, list...)
		out[uri] = merged
	}
	return out
}
