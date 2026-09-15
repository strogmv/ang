package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func lspMessage(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func publishedDiagnostics(t *testing.T, stream string) map[string][]map[string]any {
	t.Helper()
	out := map[string][]map[string]any{}
	r := bufio.NewReader(strings.NewReader(stream))
	for {
		header, err := r.ReadString('\n')
		if err != nil {
			return out
		}
		length, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "Content-Length:")))
		if err != nil {
			continue
		}
		_, _ = r.ReadString('\n')
		body := make([]byte, length)
		if _, err := r.Read(body); err != nil {
			return out
		}
		var msg struct {
			Method string `json:"method"`
			Params struct {
				URI         string           `json:"uri"`
				Diagnostics []map[string]any `json:"diagnostics"`
			} `json:"params"`
		}
		if json.Unmarshal(body, &msg) == nil && msg.Method == "textDocument/publishDiagnostics" {
			out[msg.Params.URI] = msg.Params.Diagnostics
		}
	}
}

// After didSave the server type-checks in the background and publishes the
// errors on their CUE lines with source ang-types.
func TestLSPDidSavePublishesTypeErrorsOnCUELines(t *testing.T) {
	root := t.TempDir()
	cueFile := filepath.Join(root, "cue", "api", "impl_thing.cue")
	if err := os.MkdirAll(filepath.Dir(cueFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cueFile, []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &lockedBuffer{}
	s := &lspServer{
		out:           out,
		workspaceRoot: root,
		openDocs:      map[string]string{},
		lastDiagHash:  map[string]string{},
		debounce:      time.Millisecond,
		typeDone:      make(chan struct{}, 1),
		typeCheck: func(string) (typeCheckReport, error) {
			return typeCheckReport{Errors: []typeCheckError{{
				Message: "s.ThingRepo.FindMissing undefined (type port.ThingRepository has no field or method FindMissing)",
				CUEFile: "cue/api/impl_thing.cue", CUELine: 40, CUEColumn: 21, Match: typeCheckMatchExact,
			}}}, nil
		},
	}
	uri := pathToURI(cueFile)
	input := lspMessage(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didSave", "params": map[string]any{
		"textDocument": map[string]any{"uri": uri}, "text": "package api\n",
	}})
	s.in = bufio.NewReader(strings.NewReader(input))
	req, err := s.readMessage()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.handle(req); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.typeDone:
	case <-time.After(10 * time.Second):
		t.Fatal("type check did not finish")
	}

	diags := publishedDiagnostics(t, out.String())[uri]
	var found map[string]any
	for _, d := range diags {
		if d["source"] == lspTypesSource {
			found = d
		}
	}
	if found == nil {
		t.Fatalf("no ang-types diagnostic for %s in:\n%s", uri, out.String())
	}
	start := found["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(39) || start["character"] != float64(20) || !strings.Contains(found["message"].(string), "FindMissing") {
		t.Fatalf("diagnostic = %+v", found)
	}
}

func TestMergeDiagnosticsKeepsSemanticCacheIntact(t *testing.T) {
	semantic := map[string][]map[string]any{"file:///a.cue": {{"source": "ang"}}}
	types := map[string][]map[string]any{"file:///a.cue": {{"source": lspTypesSource}}, "file:///b.cue": {{"source": lspTypesSource}}}
	merged := mergeDiagnostics(semantic, types)
	if len(merged["file:///a.cue"]) != 2 || len(merged["file:///b.cue"]) != 1 {
		t.Fatalf("merged = %+v", merged)
	}
	if len(semantic["file:///a.cue"]) != 1 {
		t.Fatalf("semantic map was modified: %+v", semantic)
	}
}
