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
	RPC        string
	MethodName string
	MethodBase string
	Method     string
	Path       string
	PathParams []string
	HasBody    bool
}

type pythonSDKData struct {
	Version   string
	Endpoints []pythonEndpoint
}

// EmitPythonSDK generates a minimal Python client SDK from normalized endpoints.
func (e *Emitter) EmitPythonSDK(endpoints []normalizer.Endpoint) error {
	data := pythonSDKData{
		Version:   e.Version,
		Endpoints: buildPythonEndpoints(endpoints),
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
		{"templates/python-sdk/pyproject.toml.tmpl", "pyproject.toml", 0644},
		{"templates/python-sdk/README.md.tmpl", "README.md", 0644},
		{"templates/python-sdk/__init__.py.tmpl", "ang_sdk/__init__.py", 0644},
		{"templates/python-sdk/client.py.tmpl", "ang_sdk/client.py", 0644},
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

var pathParamRe = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func buildPythonEndpoints(endpoints []normalizer.Endpoint) []pythonEndpoint {
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
		out = append(out, pythonEndpoint{
			RPC:        ep.RPC,
			MethodBase: toSnake(ep.RPC),
			Method:     method,
			Path:       ep.Path,
			PathParams: params,
			HasBody:    method != "GET" && method != "DELETE",
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
	if err := os.WriteFile(path, buf.Bytes(), mode); err != nil {
		return fmt.Errorf("write python sdk file %s: %w", outName, err)
	}
	fmt.Printf("Generated Python SDK: %s\n", path)
	return nil
}
