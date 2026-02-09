package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

type pythonEndpoint struct {
	RPC              string
	MethodName       string
	MethodBase       string
	Method           string
	Path             string
	PathParams       []string
	HasBody          bool
	PayloadType      string
	ReturnType       string
	PayloadModelName string
	ReturnModelName  string
}

type pythonSDKData struct {
	Version   string
	Endpoints []pythonEndpoint
	Models    []pythonSDKModel
}

type pythonSDKModel struct {
	Name   string
	Fields []pythonSDKModelField
}

type pythonSDKModelField struct {
	Name       string
	Type       string
	IsOptional bool
}

// EmitPythonSDK generates a minimal Python client SDK from normalized endpoints.
func (e *Emitter) EmitPythonSDK(endpoints []normalizer.Endpoint, services []normalizer.Service, entities []normalizer.Entity, project *normalizer.ProjectDef) error {
	version := strings.TrimSpace(e.Version)
	if project != nil {
		if v := strings.TrimSpace(project.Version); v != "" {
			version = v
		}
	}
	if version == "" {
		version = "0.1.0"
	}

	data := pythonSDKData{
		Version:   version,
		Endpoints: buildPythonEndpoints(endpoints, buildPythonRPCSignatures(services)),
		Models:    buildPythonSDKModels(entities),
	}

	funcs := e.getSharedFuncMap()
	funcs["JoinParams"] = func(params []string) string {
		return strings.Join(params, ", ")
	}
	funcs["JoinTypedParams"] = func(params []string) string {
		typed := make([]string, 0, len(params))
		for _, p := range params {
			typed = append(typed, p+": str")
		}
		return strings.Join(typed, ", ")
	}
	funcs["PathWithFormat"] = func(path string) string {
		return pathParamRe.ReplaceAllString(path, "{$1}")
	}

	files := []struct {
		template string
		out      string
		mode     os.FileMode
	}{
		{"templates/python/sdk/pyproject.toml.tmpl", "pyproject.toml", 0644},
		{"templates/python/sdk/README.md.tmpl", "README.md", 0644},
		{"templates/python/sdk/__init__.py.tmpl", "ang_sdk/__init__.py", 0644},
		{"templates/python/sdk/client.py.tmpl", "ang_sdk/client.py", 0644},
		{"templates/python/sdk/models.py.tmpl", "ang_sdk/models.py", 0644},
		{"templates/python/sdk/errors.py.tmpl", "ang_sdk/errors.py", 0644},
	}

	targetRoot := filepath.Join(e.OutputDir, "sdk", "python")
	if err := os.MkdirAll(filepath.Join(targetRoot, "ang_sdk"), 0755); err != nil {
		return fmt.Errorf("mkdir python sdk: %w", err)
	}

	for _, f := range files {
		if err := e.emitPythonTemplate(targetRoot, f.template, data, funcs, f.out, f.mode); err != nil {
			return err
		}
	}
	return nil
}

func buildPythonSDKModels(entities []normalizer.Entity) []pythonSDKModel {
	entityNames := buildPythonEntityNameSet(entities)
	out := make([]pythonSDKModel, 0, len(entities))
	for _, ent := range entities {
		model := pythonSDKModel{Name: ExportName(ent.Name)}
		for _, f := range ent.Fields {
			if f.SkipDomain {
				continue
			}
			model.Fields = append(model.Fields, pythonSDKModelField{
				Name:       f.Name,
				Type:       pythonFieldTypeWithEntities(f, entityNames),
				IsOptional: f.IsOptional,
			})
		}
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type pythonRPCSignature struct {
	InputModel  string
	OutputModel string
}

func buildPythonRPCSignatures(services []normalizer.Service) map[string]pythonRPCSignature {
	out := make(map[string]pythonRPCSignature, len(services))
	for _, svc := range services {
		for _, m := range svc.Methods {
			key := strings.ToLower(strings.TrimSpace(svc.Name)) + ":" + strings.ToLower(strings.TrimSpace(m.Name))
			out[key] = pythonRPCSignature{
				InputModel:  ExportName(strings.TrimSpace(m.Input.Name)),
				OutputModel: ExportName(strings.TrimSpace(m.Output.Name)),
			}
		}
	}
	return out
}

var pathParamRe = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func buildPythonEndpoints(endpoints []normalizer.Endpoint, sigs map[string]pythonRPCSignature) []pythonEndpoint {
	out := make([]pythonEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if strings.EqualFold(ep.Method, "WS") {
			continue
		}
		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" {
			continue
		}
		params := pathParamNames(ep.Path)
		sigKey := strings.ToLower(strings.TrimSpace(ep.ServiceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.RPC))
		sig := sigs[sigKey]
		payloadType := "dict[str, Any]"
		payloadModelName := ""
		if sig.InputModel != "" && sig.InputModel != "Any" {
			payloadType = "models." + sig.InputModel
			payloadModelName = sig.InputModel
		}
		returnType := "Any"
		returnModelName := ""
		if sig.OutputModel != "" && sig.OutputModel != "Any" {
			returnType = "models." + sig.OutputModel
			returnModelName = sig.OutputModel
		}
		out = append(out, pythonEndpoint{
			RPC:              ep.RPC,
			MethodBase:       toSnake(ep.RPC),
			Method:           method,
			Path:             ep.Path,
			PathParams:       params,
			HasBody:          method != "GET" && method != "DELETE",
			PayloadType:      payloadType,
			ReturnType:       returnType,
			PayloadModelName: payloadModelName,
			ReturnModelName:  returnModelName,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].MethodName != out[j].MethodName {
			return out[i].MethodName < out[j].MethodName
		}
		if out[i].Method != out[j].Method {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})

	used := make(map[string]bool, len(out))
	for i := range out {
		base := out[i].MethodBase
		if base == "" {
			base = "call"
		}
		name := base
		if used[name] {
			name = fmt.Sprintf("%s_%s", base, strings.ToLower(out[i].Method))
		}
		nameBase := name
		for n := 2; used[name]; n++ {
			name = fmt.Sprintf("%s_%d", nameBase, n)
		}
		used[name] = true
		out[i].MethodName = name
	}

	return out
}

func pathParamNames(path string) []string {
	matches := pathParamRe.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func toSnake(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' && runes[i-1] >= 'a' && runes[i-1] <= 'z' {
			b.WriteRune('_')
		}
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (e *Emitter) emitPythonTemplate(root, tmplPath string, data interface{}, funcs template.FuncMap, outName string, mode os.FileMode) error {
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read python template %s: %w", tmplPath, err)
	}

	t, err := template.New(filepath.Base(tmplPath)).Funcs(funcs).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse python template %s: %w", tmplPath, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("render python template %s: %w", tmplPath, err)
	}

	path := filepath.Join(root, outName)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", outName, err)
	}
	content := buf.Bytes()
	if shouldPreservePythonCustomBlocks(outName) {
		if prev, err := os.ReadFile(path); err == nil {
			content = []byte(mergePythonCustomBlocks(string(content), string(prev)))
		}
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		return fmt.Errorf("write python sdk file %s: %w", outName, err)
	}
	fmt.Printf("Generated Python SDK: %s\n", path)
	return nil
}

func shouldPreservePythonCustomBlocks(outName string) bool {
	normalized := filepath.ToSlash(strings.TrimSpace(outName))
	return strings.HasPrefix(normalized, "services/") || strings.HasPrefix(normalized, "repositories/postgres/")
}

func mergePythonCustomBlocks(generated, existing string) string {
	existingBlocks := extractPythonCustomBlocks(existing)
	if len(existingBlocks) == 0 {
		return generated
	}
	lines := strings.SplitAfter(generated, "\n")
	var out strings.Builder

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		beginKey, isBegin := parseCustomMarkerLine(line, "BEGIN")
		if !isBegin {
			out.WriteString(line)
			continue
		}

		out.WriteString(line)
		j := i + 1
		for ; j < len(lines); j++ {
			endKey, isEnd := parseCustomMarkerLine(lines[j], "END")
			if isEnd && endKey == beginKey {
				break
			}
		}
		if j >= len(lines) {
			// Malformed block in generated content.
			for k := i + 1; k < len(lines); k++ {
				out.WriteString(lines[k])
			}
			break
		}

		if prevBody, ok := existingBlocks[beginKey]; ok {
			out.WriteString(prevBody)
		} else {
			for k := i + 1; k < j; k++ {
				out.WriteString(lines[k])
			}
		}
		out.WriteString(lines[j])
		i = j
	}
	return out.String()
}

func extractPythonCustomBlocks(content string) map[string]string {
	lines := strings.SplitAfter(content, "\n")
	out := map[string]string{}

	for i := 0; i < len(lines); i++ {
		key, isBegin := parseCustomMarkerLine(lines[i], "BEGIN")
		if !isBegin {
			continue
		}
		j := i + 1
		for ; j < len(lines); j++ {
			endKey, isEnd := parseCustomMarkerLine(lines[j], "END")
			if isEnd && endKey == key {
				break
			}
		}
		if j >= len(lines) {
			continue
		}
		var body strings.Builder
		for k := i + 1; k < j; k++ {
			body.WriteString(lines[k])
		}
		out[key] = body.String()
		i = j
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseCustomMarkerLine(line, kind string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	prefix := "# ANG:" + kind + "_CUSTOM "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if key == "" {
		return "", false
	}
	return key, true
}
