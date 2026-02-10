package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

type lspRequest struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type lspResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *lspRespError   `json:"error,omitempty"`
}

type lspRespError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type lspServer struct {
	in            *bufio.Reader
	out           io.Writer
	workspaceRoot string
	openDocs      map[string]string
	lastDiagHash  map[string]string
	shutdown      bool
	debounce      time.Duration

	mu               sync.Mutex
	writeMu          sync.Mutex
	analyzeMu        sync.Mutex
	pendingTimer     *time.Timer
	cacheFingerprint string
	cacheByURI       map[string][]map[string]any
}

func runLSP(args []string) {
	fs := flag.NewFlagSet("lsp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	stdio := fs.Bool("stdio", false, "run Language Server Protocol over stdio")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("LSP FAILED: %v\n", err)
		os.Exit(1)
	}
	if !*stdio {
		fmt.Println("LSP FAILED: only --stdio mode is supported in MVP")
		os.Exit(1)
	}

	s := &lspServer{
		in:            bufio.NewReader(os.Stdin),
		out:           os.Stdout,
		workspaceRoot: ".",
		openDocs:      map[string]string{},
		lastDiagHash:  map[string]string{},
		debounce:      250 * time.Millisecond,
	}
	if err := s.serve(context.Background()); err != nil {
		fmt.Printf("LSP FAILED: %v\n", err)
		os.Exit(1)
	}
}

func (s *lspServer) serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		req, err := s.readMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if req.Method == "exit" {
			s.stopDebounce()
			if s.shutdown {
				return nil
			}
			return errors.New("received exit before shutdown")
		}
		if err := s.handle(req); err != nil {
			if len(req.ID) > 0 {
				_ = s.writeJSON(lspResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &lspRespError{
						Code:    -32603,
						Message: err.Error(),
					},
				})
			}
		}
	}
}

func (s *lspServer) handle(req lspRequest) error {
	switch req.Method {
	case "initialize":
		var p struct {
			RootURI  string `json:"rootUri"`
			RootPath string `json:"rootPath"`
		}
		_ = json.Unmarshal(req.Params, &p)
		root := strings.TrimSpace(uriToPath(p.RootURI))
		if root == "" {
			root = strings.TrimSpace(p.RootPath)
		}
		if root == "" {
			root = "."
		}
		s.workspaceRoot = filepath.Clean(root)
		s.invalidateCache()
		return s.writeJSON(lspResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"serverInfo": map[string]any{
					"name":    "ang-lsp",
					"version": compiler.Version,
				},
				"capabilities": map[string]any{
					"textDocumentSync": map[string]any{
						"openClose": true,
						"change":    1, // Full sync
						"save": map[string]any{
							"includeText": true,
						},
					},
					"completionProvider": map[string]any{
						"triggerCharacters": []string{"\"", "."},
					},
					"codeActionProvider": true,
					"executeCommandProvider": map[string]any{
						"commands": []string{"ang.openDoctor", "ang.showFixHint"},
					},
				},
			},
		})
	case "initialized":
		return nil
	case "shutdown":
		s.shutdown = true
		return s.writeJSON(lspResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		})
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.TextDocument.URI != "" {
			s.mu.Lock()
			s.openDocs[p.TextDocument.URI] = p.TextDocument.Text
			s.mu.Unlock()
			s.invalidateCache()
		}
		return s.publishAllDiagnostics()
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.TextDocument.URI != "" && len(p.ContentChanges) > 0 {
			s.mu.Lock()
			s.openDocs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			s.mu.Unlock()
			s.invalidateCache()
		}
		s.scheduleDebouncedPublish()
		return nil
	case "textDocument/didSave":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Text string `json:"text"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.TextDocument.URI != "" && p.Text != "" {
			s.mu.Lock()
			s.openDocs[p.TextDocument.URI] = p.Text
			s.mu.Unlock()
			s.invalidateCache()
		}
		return s.publishAllDiagnostics()
	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(req.Params, &p)
		s.mu.Lock()
		delete(s.openDocs, p.TextDocument.URI)
		s.mu.Unlock()
		s.invalidateCache()
		_ = s.publishDiagnosticsForURI(p.TextDocument.URI, nil)
		s.mu.Lock()
		delete(s.lastDiagHash, p.TextDocument.URI)
		s.mu.Unlock()
		return nil
	case "textDocument/completion":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(req.Params, &p)
		items := s.flowCompletionItems(p.TextDocument.URI)
		return s.writeJSON(lspResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"isIncomplete": false,
				"items":        items,
			},
		})
	case "textDocument/codeAction":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Context struct {
				Diagnostics []struct {
					Code    any    `json:"code"`
					Message string `json:"message"`
				} `json:"diagnostics"`
			} `json:"context"`
		}
		_ = json.Unmarshal(req.Params, &p)
		actions := buildCodeActions(p.TextDocument.URI, p.Context.Diagnostics)
		return s.writeJSON(lspResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  actions,
		})
	case "workspace/executeCommand":
		var p struct {
			Command   string `json:"command"`
			Arguments []any  `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		msg := "ANG command acknowledged"
		switch p.Command {
		case "ang.openDoctor":
			msg = "Run `ang doctor` in terminal to get concrete fix suggestions."
		case "ang.showFixHint":
			msg = "Review diagnostic hint and update CUE intent."
		}
		return s.writeJSON(lspResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"message": msg},
		})
	default:
		if len(req.ID) > 0 {
			return s.writeJSON(lspResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &lspRespError{
					Code:    -32601,
					Message: "method not found",
				},
			})
		}
		return nil
	}
}

func (s *lspServer) flowCompletionItems(uri string) []map[string]any {
	if !strings.HasSuffix(strings.ToLower(uriToPath(uri)), ".cue") {
		return []map[string]any{}
	}
	actions := []string{
		"repo.Find", "repo.List", "repo.Save", "repo.Delete",
		"mapping.Map", "mapping.Assign",
		"logic.Check", "logic.Call",
		"flow.If", "flow.For",
		"tx.Block",
		"fsm.Transition",
		"event.Publish",
		"cache.Get", "cache.Set",
		"rateLimit.Check",
		"storage.Upload",
		"mailer.Send",
	}
	items := make([]map[string]any, 0, len(actions))
	for _, a := range actions {
		items = append(items, map[string]any{
			"label":      a,
			"kind":       14, // keyword
			"detail":     "ANG flow action",
			"insertText": a,
		})
	}
	return items
}

func buildCodeActions(uri string, diagnostics []struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
}) []map[string]any {
	actions := []map[string]any{}
	seen := map[string]bool{}
	for _, d := range diagnostics {
		code := strings.TrimSpace(fmt.Sprintf("%v", d.Code))
		if code == "" || code == "<nil>" {
			code = "UNKNOWN"
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		title := fmt.Sprintf("ANG: suggest fix for %s", code)
		actions = append(actions, map[string]any{
			"title": title,
			"kind":  "quickfix",
			"command": map[string]any{
				"title":   title,
				"command": "ang.openDoctor",
				"arguments": []any{
					map[string]any{
						"uri":     uri,
						"code":    code,
						"message": d.Message,
					},
				},
			},
		})
	}
	if len(actions) == 0 {
		actions = append(actions, map[string]any{
			"title": "ANG: run doctor for this file",
			"kind":  "quickfix",
			"command": map[string]any{
				"title":   "ANG: run doctor for this file",
				"command": "ang.openDoctor",
				"arguments": []any{
					map[string]any{"uri": uri},
				},
			},
		})
	}
	return actions
}

func (s *lspServer) publishAllDiagnostics() error {
	s.analyzeMu.Lock()
	defer s.analyzeMu.Unlock()

	byURI, err := s.collectDiagnosticsByURI()
	if err != nil {
		return err
	}

	s.mu.Lock()
	prev := make(map[string]string, len(s.lastDiagHash))
	for k, v := range s.lastDiagHash {
		prev[k] = v
	}
	s.mu.Unlock()

	for uri, list := range byURI {
		hash := diagnosticsHash(list)
		if prev[uri] == hash {
			continue
		}
		if err := s.publishDiagnosticsForURI(uri, list); err != nil {
			return err
		}
		prev[uri] = hash
	}
	for uri := range prev {
		if _, ok := byURI[uri]; !ok {
			if err := s.publishDiagnosticsForURI(uri, []map[string]any{}); err != nil {
				return err
			}
			delete(prev, uri)
		}
	}

	s.mu.Lock()
	s.lastDiagHash = prev
	s.mu.Unlock()
	return nil
}

func (s *lspServer) collectDiagnosticsByURI() (map[string][]map[string]any, error) {
	s.mu.Lock()
	docs := make(map[string]string, len(s.openDocs))
	for k, v := range s.openDocs {
		docs[k] = v
	}
	workspaceRoot := s.workspaceRoot
	fingerprint := docsFingerprint(workspaceRoot, docs)
	if fingerprint == s.cacheFingerprint && s.cacheByURI != nil {
		cached := s.cacheByURI
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	tmpRoot, err := os.MkdirTemp("", "ang-lsp-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpRoot)

	projRoot := filepath.Join(tmpRoot, "proj")
	if err := os.MkdirAll(projRoot, 0o755); err != nil {
		return nil, err
	}
	if err := copyDir(filepath.Join(workspaceRoot, "cue"), filepath.Join(projRoot, "cue")); err != nil {
		return nil, err
	}
	if err := copyDir(filepath.Join(workspaceRoot, "cue.mod"), filepath.Join(projRoot, "cue.mod")); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for uri, text := range docs {
		srcPath := uriToPath(uri)
		if srcPath == "" {
			continue
		}
		if !strings.HasPrefix(filepath.Clean(srcPath), filepath.Clean(workspaceRoot)) {
			continue
		}
		rel, err := filepath.Rel(workspaceRoot, srcPath)
		if err != nil {
			continue
		}
		rel = filepath.Clean(rel)
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		dest := filepath.Join(projRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dest, []byte(text), 0o644); err != nil {
			return nil, err
		}
	}

	_, _, _, _, _, _, _, _, runErr := compiler.RunPipeline(projRoot)
	diags := compiler.LatestDiagnostics
	out := map[string][]map[string]any{}
	for _, d := range diags {
		if d.File == "" {
			continue
		}
		uri := pathToURI(filepath.Join(workspaceRoot, d.File))
		out[uri] = append(out[uri], toLSPDiagnostic(d))
	}

	if runErr != nil && len(diags) == 0 {
		if uri, diag := fallbackDiagnosticFromError(runErr, workspaceRoot); uri != "" {
			out[uri] = append(out[uri], diag)
		}
	}
	s.mu.Lock()
	s.cacheFingerprint = fingerprint
	s.cacheByURI = out
	s.mu.Unlock()
	return out, nil
}

func fallbackDiagnosticFromError(err error, workspaceRoot string) (string, map[string]any) {
	msg := err.Error()
	re := regexp.MustCompile(`(cue/[^:\s]+):(\d+):(\d+)`)
	m := re.FindStringSubmatch(msg)
	if len(m) == 4 {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		uri := pathToURI(filepath.Join(workspaceRoot, m[1]))
		return uri, map[string]any{
			"range": map[string]any{
				"start": map[string]int{"line": maxInt(line-1, 0), "character": maxInt(col-1, 0)},
				"end":   map[string]int{"line": maxInt(line-1, 0), "character": maxInt(col, 1)},
			},
			"severity": 1,
			"source":   "ang",
			"message":  msg,
		}
	}
	return "", map[string]any{}
}

func toLSPDiagnostic(d normalizer.Warning) map[string]any {
	line := maxInt(d.Line-1, 0)
	col := maxInt(d.Column-1, 0)
	endCol := col + 1
	severity := 2
	switch strings.ToLower(strings.TrimSpace(d.Severity)) {
	case "error":
		severity = 1
	case "info":
		severity = 3
	case "hint":
		severity = 4
	default:
		severity = 2
	}
	msg := strings.TrimSpace(d.Message)
	if d.Hint != "" {
		msg += "\nHint: " + strings.TrimSpace(d.Hint)
	}
	diag := map[string]any{
		"range": map[string]any{
			"start": map[string]int{"line": line, "character": col},
			"end":   map[string]int{"line": line, "character": endCol},
		},
		"severity": severity,
		"source":   "ang",
		"message":  msg,
	}
	if d.Code != "" {
		diag["code"] = d.Code
	}
	return diag
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *lspServer) publishDiagnosticsForURI(uri string, diagnostics []map[string]any) error {
	if uri == "" {
		return nil
	}
	return s.writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]any{
			"uri":         uri,
			"diagnostics": diagnostics,
		},
	})
}

func (s *lspServer) readMessage() (lspRequest, error) {
	headers := map[string]string{}
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			return lspRequest{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		headers[key] = val
	}

	clRaw := headers["content-length"]
	if clRaw == "" {
		return lspRequest{}, errors.New("missing Content-Length")
	}
	cl, err := strconv.Atoi(clRaw)
	if err != nil || cl < 0 {
		return lspRequest{}, errors.New("invalid Content-Length")
	}
	body := make([]byte, cl)
	if _, err := io.ReadFull(s.in, body); err != nil {
		return lspRequest{}, err
	}

	var req lspRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return lspRequest{}, err
	}
	return req, nil
}

func (s *lspServer) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(b))
	buf.Write(b)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.out.Write(buf.Bytes())
	return err
}

func (s *lspServer) scheduleDebouncedPublish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingTimer != nil {
		s.pendingTimer.Stop()
	}
	s.pendingTimer = time.AfterFunc(s.debounce, func() {
		_ = s.publishAllDiagnostics()
	})
}

func (s *lspServer) stopDebounce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingTimer != nil {
		s.pendingTimer.Stop()
		s.pendingTimer = nil
	}
}

func (s *lspServer) invalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheFingerprint = ""
	s.cacheByURI = nil
}

func docsFingerprint(workspaceRoot string, docs map[string]string) string {
	keys := make([]string, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	_, _ = h.Write([]byte(filepath.Clean(workspaceRoot)))
	for _, k := range keys {
		_, _ = h.Write([]byte("\nURI:" + k + "\n"))
		_, _ = h.Write([]byte(docs[k]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func diagnosticsHash(diags []map[string]any) string {
	b, _ := json.Marshal(diags)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func pathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	path = filepath.ToSlash(path)
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}

func uriToPath(uri string) string {
	if uri == "" {
		return ""
	}
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	if u.Scheme != "file" {
		return ""
	}
	return filepath.FromSlash(u.Path)
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
