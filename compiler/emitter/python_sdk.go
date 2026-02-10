package emitter

import (
	"fmt"
	"sort"
	"strings"

	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/planner"
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
	PayloadAnnot     string
	ReturnType       string
	ReturnAnnot      string
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
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitSDK(e.OutputDir, version, rt, endpoints, services, entities)
}

func (e *Emitter) EmitPythonSDKFromIR(schema *ir.Schema, fallbackVersion string) error {
	if err := ir.MigrateToCurrent(schema); err != nil {
		return fmt.Errorf("migrate ir schema: %w", err)
	}
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitSDKFromIR(e.OutputDir, fallbackVersion, rt, schema)
}

func buildPythonSDKModels(entities []normalizer.Entity) []pythonSDKModel {
	irEntities := make([]ir.Entity, 0, len(entities))
	for _, ent := range entities {
		irEntities = append(irEntities, ir.ConvertEntity(ent))
	}
	modelPlans := planner.BuildModelPlans(irEntities)
	out := make([]pythonSDKModel, 0, len(modelPlans))
	for _, m := range modelPlans {
		model := pythonSDKModel{Name: m.Name}
		for _, f := range m.Fields {
			model.Fields = append(model.Fields, pythonSDKModelField{
				Name:       f.Name,
				Type:       f.Type,
				IsOptional: f.IsOptional,
			})
		}
		out = append(out, model)
	}
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
		params := pyemitter.PathParamNames(ep.Path)
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
		out = append(out, pythonEndpoint{
			RPC:              ep.RPC,
			MethodBase:       pyemitter.ToSnake(ep.RPC),
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
