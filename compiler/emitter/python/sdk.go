package python

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/planner"
)

type SDKModelField struct {
	Name       string
	Type       string
	IsOptional bool
}

type SDKModel struct {
	Name   string
	Fields []SDKModelField
}

type Endpoint struct {
	RPC              string
	MethodName       string
	MethodBase       string
	Method           string
	Path             string
	PathParams       []string
	HasBody          bool
	PayloadType      string
	PayloadAnnot     string
	ReturnType       string
	ReturnAnnot      string
	PayloadModelName string
	ReturnModelName  string
}

type sdkData struct {
	Version   string
	Endpoints []Endpoint
	Models    []SDKModel
}

type rpcSignature struct {
	InputModel  string
	OutputModel string
}

func EmitSDK(outputDir, version string, rt Runtime, endpoints []normalizer.Endpoint, services []normalizer.Service, entities []normalizer.Entity) error {
	data := sdkData{
		Version:   version,
		Endpoints: BuildEndpoints(endpoints, buildRPCSignatures(services)),
		Models:    BuildSDKModels(entities),
	}

	rt.Funcs["JoinParams"] = func(params []string) string {
		return strings.Join(params, ", ")
	}
	rt.Funcs["JoinTypedParams"] = func(params []string) string {
		typed := make([]string, 0, len(params))
		for _, p := range params {
			typed = append(typed, p+": str")
		}
		return strings.Join(typed, ", ")
	}
	rt.Funcs["PathWithFormat"] = func(path string) string {
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

	targetRoot := filepath.Join(outputDir, "sdk", "python")
	if err := os.MkdirAll(filepath.Join(targetRoot, "ang_sdk"), 0755); err != nil {
		return fmt.Errorf("mkdir python sdk: %w", err)
	}

	for _, f := range files {
		if err := rt.RenderTemplate(targetRoot, f.template, data, f.out, f.mode); err != nil {
			return err
		}
	}
	return nil
}

func BuildSDKModels(entities []normalizer.Entity) []SDKModel {
	irEntities := make([]ir.Entity, 0, len(entities))
	for _, ent := range entities {
		irEntities = append(irEntities, ir.ConvertEntity(ent))
	}
	modelPlans := planner.BuildModelPlans(irEntities)
	out := make([]SDKModel, 0, len(modelPlans))
	for _, m := range modelPlans {
		model := SDKModel{Name: m.Name}
		for _, f := range m.Fields {
			model.Fields = append(model.Fields, SDKModelField{
				Name:       f.Name,
				Type:       f.Type,
				IsOptional: f.IsOptional,
			})
		}
		out = append(out, model)
	}
	return out
}

func buildRPCSignatures(services []normalizer.Service) map[string]rpcSignature {
	out := make(map[string]rpcSignature, len(services))
	for _, svc := range services {
		for _, m := range svc.Methods {
			key := strings.ToLower(strings.TrimSpace(svc.Name)) + ":" + strings.ToLower(strings.TrimSpace(m.Name))
			out[key] = rpcSignature{
				InputModel:  exportName(strings.TrimSpace(m.Input.Name)),
				OutputModel: exportName(strings.TrimSpace(m.Output.Name)),
			}
		}
	}
	return out
}

var pathParamRe = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func BuildEndpoints(endpoints []normalizer.Endpoint, sigs map[string]rpcSignature) []Endpoint {
	out := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if strings.EqualFold(ep.Method, "WS") {
			continue
		}
		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" {
			continue
		}
		params := PathParamNames(ep.Path)
		sigKey := strings.ToLower(strings.TrimSpace(ep.ServiceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.RPC))
		sig := sigs[sigKey]
		payloadType := "dict[str, Any]"
		payloadAnnot := "dict[str, Any] | None"
		payloadModelName := ""
		if sig.InputModel != "" && sig.InputModel != "Any" {
			payloadType = "models." + sig.InputModel
			payloadAnnot = payloadType + " | dict[str, Any] | None"
			payloadModelName = sig.InputModel
		}
		returnType := "Any"
		returnAnnot := "Any | None"
		returnModelName := ""
		if sig.OutputModel != "" && sig.OutputModel != "Any" {
			returnType = "models." + sig.OutputModel
			returnAnnot = returnType + " | None"
			returnModelName = sig.OutputModel
		}
		out = append(out, Endpoint{
			RPC:              ep.RPC,
			MethodBase:       ToSnake(ep.RPC),
			Method:           method,
			Path:             ep.Path,
			PathParams:       params,
			HasBody:          method != "GET" && method != "DELETE",
			PayloadType:      payloadType,
			PayloadAnnot:     payloadAnnot,
			ReturnType:       returnType,
			ReturnAnnot:      returnAnnot,
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

func PathParamNames(path string) []string {
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

func ToSnake(s string) string {
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

func exportName(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(s)
	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
