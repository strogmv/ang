package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/strogmv/ang/compiler"
)

type cueHistoryEntry struct {
	ID        int64          `json:"id"`
	Tool      string         `json:"tool"`
	Path      string         `json:"path"`
	Timestamp string         `json:"timestamp"`
	Changed   bool           `json:"changed"`
	BytesFrom int            `json:"bytes_before"`
	BytesTo   int            `json:"bytes_after"`
	Meta      map[string]any `json:"meta,omitempty"`
	Before    []byte         `json:"-"`
	After     []byte         `json:"-"`
}

type cueHistoryDiskEntry struct {
	ID        int64          `json:"id"`
	Tool      string         `json:"tool"`
	Path      string         `json:"path"`
	Timestamp string         `json:"timestamp"`
	Changed   bool           `json:"changed"`
	BytesFrom int            `json:"bytes_before"`
	BytesTo   int            `json:"bytes_after"`
	Meta      map[string]any `json:"meta,omitempty"`
	BeforeB64 string         `json:"before_b64"`
	AfterB64  string         `json:"after_b64"`
}

var cueHistoryState = struct {
	sync.Mutex
	NextID  int64
	Entries []cueHistoryEntry
}{
	NextID: 1,
}

const cueHistoryMaxEntries = 100

func registerCUETools(addTool toolAdder) {
	addTool("cue_apply_patch", mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update CUE intent with atomic validation"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
		mcp.WithString("selector", mcp.Description("Target node path")),
		mcp.WithBoolean("forced_merge", mcp.Description("Overwrite instead of deep merge")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, content := mcp.ParseString(request, "path", ""), mcp.ParseString(request, "content", "")
		selector := mcp.ParseString(request, "selector", "")
		force := mcp.ParseBoolean(request, "forced_merge", false)
		if err := validateCuePath(path); err != nil {
			return mcp.NewToolResultText("Denied"), nil
		}
		newContent, err := GetMergedContent(path, selector, content, force)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Merge error: %v", err)), nil
		}
		orig, _ := os.ReadFile(path)
		os.WriteFile(path, newContent, 0o644)
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./"+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Syntax validation FAILED:\n%s", string(out))), nil
		}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Architecture validation FAILED: %v", err)), nil
		}
		changed := !bytes.Equal(orig, newContent)
		diffText := unifiedDiff(orig, newContent)
		resp := map[string]any{
			"status":         "ok",
			"path":           path,
			"selector":       selector,
			"forced_merge":   force,
			"changed":        changed,
			"bytes_before":   len(orig),
			"bytes_after":    len(newContent),
			"before_preview": previewText(string(orig), 1200),
			"after_preview":  previewText(string(newContent), 1200),
			"diff_unified":   previewText(diffText, 6000),
		}
		if changed {
			entry := recordCueHistory("cue_apply_patch", path, orig, newContent, map[string]any{
				"selector":     selector,
				"forced_merge": force,
			})
			resp["history_id"] = entry.ID
			resp["generated_impact"] = collectGeneratedImpact()
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("cue_set_field", mcp.NewTool("cue_set_field",
		mcp.WithDescription("Atomically add/update one entity field in CUE with predictable behavior."),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("entity", mcp.Required()),
		mcp.WithString("field", mcp.Required()),
		mcp.WithString("type", mcp.Required()),
		mcp.WithBoolean("optional", mcp.Description("Mark field as optional label (field?).")),
		mcp.WithBoolean("overwrite", mcp.Description("Replace field when already exists. Default false.")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "")
		entityName := strings.TrimSpace(mcp.ParseString(request, "entity", ""))
		fieldName := strings.TrimSpace(mcp.ParseString(request, "field", ""))
		fieldType := strings.TrimSpace(mcp.ParseString(request, "type", ""))
		optional := mcp.ParseBoolean(request, "optional", false)
		overwrite := mcp.ParseBoolean(request, "overwrite", false)
		if err := validateCuePath(path); err != nil {
			return mcp.NewToolResultText("Denied"), nil
		}
		if entityName == "" || fieldName == "" || fieldType == "" {
			return mcp.NewToolResultText("entity, field, type are required"), nil
		}

		orig, err := os.ReadFile(path)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("read error: %v", err)), nil
		}
		next, changed, err := applySetFieldPatch(orig, entityName, fieldName, fieldType, optional, overwrite)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("cue_set_field error: %v", err)), nil
		}
		if !changed {
			resp := map[string]any{
				"status":   "ok",
				"path":     path,
				"entity":   entityName,
				"field":    fieldName,
				"changed":  false,
				"optional": optional,
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		}

		if err := os.WriteFile(path, next, 0o644); err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("write error: %v", err)), nil
		}
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./"+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Syntax validation FAILED:\n%s", string(out))), nil
		}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			_ = os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Architecture validation FAILED: %v", err)), nil
		}

		diffText := unifiedDiff(orig, next)
		resp := map[string]any{
			"status":         "ok",
			"path":           path,
			"entity":         entityName,
			"field":          fieldName,
			"type":           fieldType,
			"optional":       optional,
			"overwrite":      overwrite,
			"changed":        true,
			"before_preview": previewText(string(orig), 1200),
			"after_preview":  previewText(string(next), 1200),
			"diff_unified":   previewText(diffText, 6000),
		}
		if changed {
			entry := recordCueHistory("cue_set_field", path, orig, next, map[string]any{
				"entity":    entityName,
				"field":     fieldName,
				"type":      fieldType,
				"optional":  optional,
				"overwrite": overwrite,
			})
			resp["history_id"] = entry.ID
			resp["generated_impact"] = collectGeneratedImpact()
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("cue_history", mcp.NewTool("cue_history",
		mcp.WithDescription("Show recent successful CUE changes in current MCP session."),
		mcp.WithNumber("limit", mcp.Description("Max entries to return (default 10, max 50).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := int(mcp.ParseFloat64(request, "limit", 10))
		if limit <= 0 {
			limit = 10
		}
		if limit > 50 {
			limit = 50
		}
		items, total := listCueHistory(limit)
		resp := map[string]any{
			"status":          "ok",
			"total":           total,
			"returned":        len(items),
			"session_history": items,
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("cue_undo", mcp.NewTool("cue_undo",
		mcp.WithDescription("Undo last successful CUE change from session history."),
		mcp.WithNumber("id", mcp.Description("Optional history id to undo; default is latest entry.")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		undoID := int64(mcp.ParseFloat64(request, "id", 0))
		entry, ok := popCueHistory(undoID)
		if !ok {
			return mcp.NewToolResultText(`{"status":"empty","message":"No CUE changes to undo in this MCP session"}`), nil
		}
		if err := validateCuePath(entry.Path); err != nil {
			pushCueHistory(entry)
			return mcp.NewToolResultText("Denied"), nil
		}
		current, err := os.ReadFile(entry.Path)
		if err != nil {
			pushCueHistory(entry)
			return mcp.NewToolResultText(fmt.Sprintf("undo read error: %v", err)), nil
		}
		if err := os.WriteFile(entry.Path, entry.Before, 0o644); err != nil {
			pushCueHistory(entry)
			return mcp.NewToolResultText(fmt.Sprintf("undo write error: %v", err)), nil
		}
		dir := filepath.Dir(entry.Path)
		cmd := exec.Command("cue", "vet", "./"+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.WriteFile(entry.Path, current, 0o644)
			pushCueHistory(entry)
			return mcp.NewToolResultText(fmt.Sprintf("Undo validation FAILED:\n%s", string(out))), nil
		}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			_ = os.WriteFile(entry.Path, current, 0o644)
			pushCueHistory(entry)
			return mcp.NewToolResultText(fmt.Sprintf("Undo architecture validation FAILED: %v", err)), nil
		}

		resp := map[string]any{
			"status":          "ok",
			"undone_history":  entry.ID,
			"path":            entry.Path,
			"source_tool":     entry.Tool,
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"before_preview":  previewText(string(current), 1200),
			"after_preview":   previewText(string(entry.Before), 1200),
			"diff_unified":    previewText(unifiedDiff(current, entry.Before), 6000),
			"remaining_count": cueHistoryCount(),
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("run_preset", mcp.NewTool("run_preset",
		mcp.WithDescription("Run build, unit, lint"),
		mcp.WithString("name", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		var cmd *exec.Cmd
		switch name {
		case "build":
			cmd = exec.Command(resolveANGExecutable(), "build")
		case "unit":
			cmd = exec.Command("go", "test", "-v", "./...")
		default:
			return mcp.NewToolResultText("Unknown preset"), nil
		}
		logFile := "ang-build.log"
		out, err := cmd.CombinedOutput()
		logText := string(out)
		if strings.TrimSpace(logText) == "" {
			if err != nil {
				logText = fmt.Sprintf("preset %q failed with no output: %v\n", name, err)
			} else {
				logText = fmt.Sprintf("preset %q completed with no output\n", name)
			}
		}
		_ = os.WriteFile(logFile, []byte(logText), 0o644)
		status := "SUCCESS"
		if err != nil {
			status = "FAILED"
		}
		resp := map[string]any{
			"status": status,
			"preset": name,
		}
		if status == "FAILED" {
			resp["log_tail"] = tailLines(logText, 30)
			resp["doctor"] = buildDoctorResponse(logText)
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("cue_add_endpoint", mcp.NewTool("cue_add_endpoint",
		mcp.WithDescription("Atomically add HTTP endpoint to a CUE endpoint list."),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("method", mcp.Required()),
		mcp.WithString("endpoint_path", mcp.Required()),
		mcp.WithString("service", mcp.Required()),
		mcp.WithString("rpc", mcp.Required()),
		mcp.WithString("auth", mcp.Description("Auth mode, optional (e.g. jwt, none).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "")
		method := strings.ToUpper(strings.TrimSpace(mcp.ParseString(request, "method", "")))
		endpointPath := strings.TrimSpace(mcp.ParseString(request, "endpoint_path", ""))
		service := strings.TrimSpace(mcp.ParseString(request, "service", ""))
		rpc := strings.TrimSpace(mcp.ParseString(request, "rpc", ""))
		auth := strings.TrimSpace(mcp.ParseString(request, "auth", ""))
		if err := validateCuePath(path); err != nil {
			return mcp.NewToolResultText("Denied"), nil
		}
		if method == "" || endpointPath == "" || service == "" || rpc == "" {
			return mcp.NewToolResultText("method, endpoint_path, service, rpc are required"), nil
		}
		orig, err := os.ReadFile(path)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("read error: %v", err)), nil
		}
		next, changed, err := applyAddEndpointPatch(orig, method, endpointPath, service, rpc, auth)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("cue_add_endpoint error: %v", err)), nil
		}
		if !changed {
			resp := map[string]any{
				"status":        "ok",
				"path":          path,
				"method":        method,
				"endpoint_path": endpointPath,
				"service":       service,
				"rpc":           rpc,
				"changed":       false,
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		}
		if err := os.WriteFile(path, next, 0o644); err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("write error: %v", err)), nil
		}
		dir := filepath.Dir(path)
		cmd := exec.Command("cue", "vet", "./"+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Syntax validation FAILED:\n%s", string(out))), nil
		}
		if _, _, _, _, _, _, _, _, err := compiler.RunPipeline("."); err != nil {
			_ = os.WriteFile(path, orig, 0o644)
			return mcp.NewToolResultText(fmt.Sprintf("Architecture validation FAILED: %v", err)), nil
		}
		diffText := unifiedDiff(orig, next)
		resp := map[string]any{
			"status":         "ok",
			"path":           path,
			"method":         method,
			"endpoint_path":  endpointPath,
			"service":        service,
			"rpc":            rpc,
			"auth":           auth,
			"changed":        true,
			"before_preview": previewText(string(orig), 1200),
			"after_preview":  previewText(string(next), 1200),
			"diff_unified":   previewText(diffText, 6000),
		}
		entry := recordCueHistory("cue_add_endpoint", path, orig, next, map[string]any{
			"method":        method,
			"endpoint_path": endpointPath,
			"service":       service,
			"rpc":           rpc,
			"auth":          auth,
		})
		resp["history_id"] = entry.ID
		resp["generated_impact"] = collectGeneratedImpact()
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func previewText(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...<truncated>"
}

func unifiedDiff(before, after []byte) string {
	tmpDir, err := os.MkdirTemp("", "ang-cue-diff-*")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(tmpDir)

	beforePath := filepath.Join(tmpDir, "before.cue")
	afterPath := filepath.Join(tmpDir, "after.cue")
	_ = os.WriteFile(beforePath, before, 0o644)
	_ = os.WriteFile(afterPath, after, 0o644)

	cmd := exec.Command("git", "--no-pager", "diff", "--no-index", "--", beforePath, afterPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	// git diff exits with code 1 when differences exist; this is expected.
	if len(out) > 0 {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func tailLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) <= n {
		return strings.TrimRight(strings.Join(lines, "\n"), "\n")
	}
	start := len(lines) - n
	return strings.TrimRight(strings.Join(lines[start:], "\n"), "\n")
}

func applySetFieldPatch(src []byte, entityName, fieldName, fieldType string, optional, overwrite bool) ([]byte, bool, error) {
	f, err := parser.ParseFile("target.cue", src, parser.ParseComments)
	if err != nil {
		return nil, false, fmt.Errorf("parse file: %w", err)
	}
	entityField := findEntityField(f, entityName)
	if entityField == nil {
		return nil, false, fmt.Errorf("entity %q not found", entityName)
	}
	fieldsNode := findFieldInExpr(entityField.Value, "fields")
	if fieldsNode == nil {
		return nil, false, fmt.Errorf("entity %q has no fields block", entityName)
	}
	fieldsStruct, ok := fieldsNode.Value.(*ast.StructLit)
	if !ok {
		return nil, false, fmt.Errorf("entity %q fields is not a struct", entityName)
	}

	newField, err := buildCueField(fieldName, fieldType, optional)
	if err != nil {
		return nil, false, err
	}
	for idx, decl := range fieldsStruct.Elts {
		existing, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		if cueLabelName(existing.Label) != fieldName {
			continue
		}
		if !overwrite {
			return nil, false, fmt.Errorf("field %q already exists in %q (set overwrite=true to replace)", fieldName, entityName)
		}
		fieldsStruct.Elts[idx] = newField
		out, err := format.Node(f)
		if err != nil {
			return nil, false, fmt.Errorf("format result: %w", err)
		}
		return out, !bytes.Equal(src, out), nil
	}

	fieldsStruct.Elts = append(fieldsStruct.Elts, newField)
	out, err := format.Node(f)
	if err != nil {
		return nil, false, fmt.Errorf("format result: %w", err)
	}
	return out, !bytes.Equal(src, out), nil
}

func findEntityField(file *ast.File, entityName string) *ast.Field {
	candidates := []string{entityName}
	if strings.HasPrefix(entityName, "#") {
		candidates = append(candidates, strings.TrimPrefix(entityName, "#"))
	} else {
		candidates = append(candidates, "#"+entityName)
	}
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label := cueLabelName(f.Label)
		for _, c := range candidates {
			if label == c {
				return f
			}
		}
	}
	return nil
}

func findFieldInExpr(expr ast.Expr, fieldName string) *ast.Field {
	switch v := expr.(type) {
	case *ast.StructLit:
		for _, elt := range v.Elts {
			f, ok := elt.(*ast.Field)
			if !ok {
				continue
			}
			if cueLabelName(f.Label) == fieldName {
				return f
			}
		}
	case *ast.BinaryExpr:
		if left := findFieldInExpr(v.X, fieldName); left != nil {
			return left
		}
		return findFieldInExpr(v.Y, fieldName)
	case *ast.ParenExpr:
		return findFieldInExpr(v.X, fieldName)
	}
	return nil
}

func cueLabelName(label ast.Label) string {
	switch l := label.(type) {
	case *ast.Ident:
		return l.Name
	case *ast.BasicLit:
		return strings.Trim(l.Value, "\"`")
	default:
		return fmt.Sprint(label)
	}
}

func buildCueField(fieldName, fieldType string, optional bool) (*ast.Field, error) {
	label := fieldName
	if optional {
		label += "?"
	}
	snippet := "package p\n" + label + ": {\n\ttype: " + strconv.Quote(fieldType) + "\n}\n"
	sf, err := parser.ParseFile("snippet.cue", snippet)
	if err != nil {
		return nil, fmt.Errorf("build field snippet: %w", err)
	}
	for _, decl := range sf.Decls {
		if f, ok := decl.(*ast.Field); ok {
			if optional {
				f.Optional = token.Blank.Pos()
			}
			return f, nil
		}
	}
	return nil, fmt.Errorf("failed to build field")
}

func applyAddEndpointPatch(src []byte, method, endpointPath, service, rpc, auth string) ([]byte, bool, error) {
	f, err := parser.ParseFile("target.cue", src, parser.ParseComments)
	if err != nil {
		return nil, false, fmt.Errorf("parse file: %w", err)
	}
	endpoints := findOrCreateListField(f, "endpoints")
	if endpoints == nil {
		return nil, false, fmt.Errorf("unable to locate or create endpoints list")
	}
	list, ok := endpoints.Value.(*ast.ListLit)
	if !ok {
		return nil, false, fmt.Errorf("endpoints is not a list")
	}
	for _, elt := range list.Elts {
		obj, ok := elt.(*ast.StructLit)
		if !ok {
			continue
		}
		if endpointStructMatches(obj, method, endpointPath, service, rpc) {
			return src, false, nil
		}
	}

	entry, err := parseEndpointEntry(method, endpointPath, service, rpc, auth)
	if err != nil {
		return nil, false, err
	}
	list.Elts = append(list.Elts, entry)
	out, err := format.Node(f)
	if err != nil {
		return nil, false, fmt.Errorf("format result: %w", err)
	}
	return out, !bytes.Equal(src, out), nil
}

func findOrCreateListField(f *ast.File, fieldName string) *ast.Field {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		if cueLabelName(fd.Label) == fieldName {
			return fd
		}
	}
	newField := &ast.Field{
		Label: ast.NewIdent(fieldName),
		Value: &ast.ListLit{},
	}
	f.Decls = append(f.Decls, newField)
	return newField
}

func parseEndpointEntry(method, endpointPath, service, rpc, auth string) (*ast.StructLit, error) {
	buf := "package p\nx: [{\n" +
		"  method: " + strconv.Quote(method) + "\n" +
		"  path: " + strconv.Quote(endpointPath) + "\n" +
		"  service: " + strconv.Quote(service) + "\n" +
		"  rpc: " + strconv.Quote(rpc) + "\n"
	if auth != "" {
		buf += "  auth: " + strconv.Quote(auth) + "\n"
	}
	buf += "}]\n"
	pf, err := parser.ParseFile("endpoint_snippet.cue", buf)
	if err != nil {
		return nil, fmt.Errorf("build endpoint snippet: %w", err)
	}
	for _, d := range pf.Decls {
		fd, ok := d.(*ast.Field)
		if !ok || cueLabelName(fd.Label) != "x" {
			continue
		}
		lst, ok := fd.Value.(*ast.ListLit)
		if !ok || len(lst.Elts) == 0 {
			continue
		}
		st, ok := lst.Elts[0].(*ast.StructLit)
		if ok {
			return st, nil
		}
	}
	return nil, fmt.Errorf("cannot build endpoint struct")
}

func endpointStructMatches(st *ast.StructLit, method, endpointPath, service, rpc string) bool {
	var m, p, s, r string
	for _, elt := range st.Elts {
		fd, ok := elt.(*ast.Field)
		if !ok {
			continue
		}
		val := ""
		if lit, ok := fd.Value.(*ast.BasicLit); ok {
			val = strings.Trim(lit.Value, "\"`")
		}
		switch cueLabelName(fd.Label) {
		case "method":
			m = strings.ToUpper(val)
		case "path":
			p = val
		case "service":
			s = val
		case "rpc":
			r = val
		}
	}
	return strings.ToUpper(method) == m && endpointPath == p && service == s && rpc == r
}

func recordCueHistory(tool, path string, before, after []byte, meta map[string]any) cueHistoryEntry {
	cueHistoryState.Lock()
	defer cueHistoryState.Unlock()
	existing := readPersistentHistoryLocked()
	if len(existing) > 0 {
		cueHistoryState.NextID = existing[len(existing)-1].ID + 1
	}
	entry := cueHistoryEntry{
		ID:        cueHistoryState.NextID,
		Tool:      tool,
		Path:      path,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Changed:   !bytes.Equal(before, after),
		BytesFrom: len(before),
		BytesTo:   len(after),
		Meta:      meta,
		Before:    append([]byte(nil), before...),
		After:     append([]byte(nil), after...),
	}
	cueHistoryState.NextID++
	existing = append(existing, entry)
	if len(existing) > cueHistoryMaxEntries {
		existing = existing[len(existing)-cueHistoryMaxEntries:]
	}
	cueHistoryState.Entries = existing
	_ = writePersistentHistoryLocked(existing)
	return entry
}

func listCueHistory(limit int) ([]map[string]any, int) {
	cueHistoryState.Lock()
	defer cueHistoryState.Unlock()
	entries := readPersistentHistoryLocked()
	cueHistoryState.Entries = entries
	total := len(entries)
	start := total - limit
	if start < 0 {
		start = 0
	}
	out := make([]map[string]any, 0, total-start)
	for i := total - 1; i >= start; i-- {
		e := entries[i]
		out = append(out, map[string]any{
			"id":           e.ID,
			"tool":         e.Tool,
			"path":         e.Path,
			"timestamp":    e.Timestamp,
			"changed":      e.Changed,
			"bytes_before": e.BytesFrom,
			"bytes_after":  e.BytesTo,
			"meta":         e.Meta,
		})
	}
	return out, total
}

func popCueHistory(targetID int64) (cueHistoryEntry, bool) {
	cueHistoryState.Lock()
	defer cueHistoryState.Unlock()
	entries := readPersistentHistoryLocked()
	n := len(entries)
	if n == 0 {
		return cueHistoryEntry{}, false
	}
	idx := n - 1
	if targetID > 0 {
		idx = -1
		for i := n - 1; i >= 0; i-- {
			if entries[i].ID == targetID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return cueHistoryEntry{}, false
		}
	}
	target := entries[idx]
	entries = append(entries[:idx], entries[idx+1:]...)
	cueHistoryState.Entries = entries
	_ = writePersistentHistoryLocked(entries)
	return target, true
}

func pushCueHistory(entry cueHistoryEntry) {
	cueHistoryState.Lock()
	defer cueHistoryState.Unlock()
	entries := readPersistentHistoryLocked()
	entries = append(entries, entry)
	if len(entries) > cueHistoryMaxEntries {
		entries = entries[len(entries)-cueHistoryMaxEntries:]
	}
	cueHistoryState.Entries = entries
	_ = writePersistentHistoryLocked(entries)
}

func cueHistoryCount() int {
	cueHistoryState.Lock()
	defer cueHistoryState.Unlock()
	entries := readPersistentHistoryLocked()
	cueHistoryState.Entries = entries
	return len(entries)
}

func persistentHistoryPath() string {
	return filepath.Join(".ang", "history.jsonl")
}

func readPersistentHistoryLocked() []cueHistoryEntry {
	f, err := os.Open(persistentHistoryPath())
	if err != nil {
		return []cueHistoryEntry{}
	}
	defer f.Close()
	entries := []cueHistoryEntry{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var raw cueHistoryDiskEntry
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		before, _ := base64.StdEncoding.DecodeString(raw.BeforeB64)
		after, _ := base64.StdEncoding.DecodeString(raw.AfterB64)
		entries = append(entries, cueHistoryEntry{
			ID:        raw.ID,
			Tool:      raw.Tool,
			Path:      raw.Path,
			Timestamp: raw.Timestamp,
			Changed:   raw.Changed,
			BytesFrom: raw.BytesFrom,
			BytesTo:   raw.BytesTo,
			Meta:      raw.Meta,
			Before:    before,
			After:     after,
		})
	}
	return entries
}

func writePersistentHistoryLocked(entries []cueHistoryEntry) error {
	if err := os.MkdirAll(".ang", 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(persistentHistoryPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, e := range entries {
		raw := cueHistoryDiskEntry{
			ID:        e.ID,
			Tool:      e.Tool,
			Path:      e.Path,
			Timestamp: e.Timestamp,
			Changed:   e.Changed,
			BytesFrom: e.BytesFrom,
			BytesTo:   e.BytesTo,
			Meta:      e.Meta,
			BeforeB64: base64.StdEncoding.EncodeToString(e.Before),
			AfterB64:  base64.StdEncoding.EncodeToString(e.After),
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func collectGeneratedImpact() map[string]any {
	result := map[string]any{
		"status":          "unknown",
		"generated_roots": []string{"internal/", "sdk/", "api/"},
	}

	buildCmd := exec.Command(resolveANGExecutable(), "build")
	buildOut, buildErr := buildCmd.CombinedOutput()
	buildText := string(buildOut)
	buildStatus := "success"
	if buildErr != nil || strings.Contains(buildText, "Build FAILED") {
		buildStatus = "failed"
	}
	result["build_status"] = buildStatus
	result["build_log_excerpt"] = truncate(buildText, 4000)
	if buildStatus == "failed" {
		result["status"] = "build_failed"
		result["doctor"] = buildDoctorResponse(buildText)
		return result
	}

	numstatCmd := exec.Command("git", "diff", "--numstat", "--", "internal/", "sdk/", "api/")
	numstatOut, numstatErr := numstatCmd.CombinedOutput()
	if numstatErr != nil && len(numstatOut) == 0 {
		result["status"] = "diff_error"
		result["error"] = numstatErr.Error()
		return result
	}

	entries := parseNumstat(string(numstatOut))
	shortstatCmd := exec.Command("git", "diff", "--shortstat", "--", "internal/", "sdk/", "api/")
	shortstatOut, _ := shortstatCmd.CombinedOutput()

	result["status"] = "ok"
	result["summary"] = strings.TrimSpace(string(shortstatOut))
	result["changed_files"] = entries
	result["changed_files_count"] = len(entries)
	return result
}

func parseNumstat(s string) []map[string]any {
	out := []map[string]any{}
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		path := parts[2]
		out = append(out, map[string]any{
			"path":    path,
			"added":   added,
			"deleted": deleted,
		})
	}
	return out
}
