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

type pythonModelField struct {
	Name       string
	Type       string
	IsOptional bool
}

type pythonModel struct {
	Name   string
	Fields []pythonModelField
}

type pythonRoute struct {
	Method      string
	Decorator   string
	Path        string
	HandlerName string
	Signature   string
	ReturnType  string
	CallExpr    string
}

type pythonRouter struct {
	ServiceName string
	ModuleName  string
	ClassName   string
	GetService  string
	Routes      []pythonRoute
}

type pythonServiceMethod struct {
	Name      string
	Body      string
	CustomKey string
}

type pythonServiceStub struct {
	ServiceName string
	ModuleName  string
	ClassName   string
	GetService  string
	Imports     []string
	Methods     []pythonServiceMethod
}

type pythonRepoFinder struct {
	Name      string
	CustomKey string
}

type pythonRepoStub struct {
	RepoName      string
	ModuleName    string
	PortClassName string
	PGClassName   string
	Finders       []pythonRepoFinder
}

type pythonFastAPIData struct {
	ProjectName   string
	Version       string
	Models        []planner.ModelPlan
	Routers       []planner.ServicePlan
	ServiceStubs  []planner.ServicePlan
	RepoStubs     []planner.RepoPlan
	RouterModules []string
}

// EmitPythonFastAPIBackend generates a minimal FastAPI backend scaffold (M3 MVP).
func (e *Emitter) EmitPythonFastAPIBackend(
	entities []normalizer.Entity,
	services []normalizer.Service,
	endpoints []normalizer.Endpoint,
	repos []normalizer.Repository,
	project *normalizer.ProjectDef,
) error {
	var projectDef normalizer.ProjectDef
	if project != nil {
		projectDef = *project
	}
	schema := ir.ConvertFromNormalizer(
		entities, services, nil, nil, endpoints, repos,
		normalizer.ConfigDef{}, nil, nil, nil, nil, projectDef,
	)
	return e.EmitPythonFastAPIBackendFromIR(schema, e.Version)
}

func (e *Emitter) EmitPythonFastAPIBackendFromIR(schema *ir.Schema, fallbackVersion string) error {
	if err := ir.MigrateToCurrent(schema); err != nil {
		return fmt.Errorf("migrate ir schema: %w", err)
	}
	plan := planner.BuildFastAPIPlan(schema, fallbackVersion)
	return e.EmitPythonFastAPIBackendFromPlan(plan)
}

func (e *Emitter) EmitPythonFastAPIBackendFromPlan(plan planner.FastAPIPlan) error {
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitFastAPIBackend(e.OutputDir, rt, plan)
}

func buildPythonFastAPIData(
	entities []normalizer.Entity,
	services []normalizer.Service,
	endpoints []normalizer.Endpoint,
	repos []normalizer.Repository,
	project *normalizer.ProjectDef,
	fallbackVersion string,
) pythonFastAPIData {
	var projectDef normalizer.ProjectDef
	if project != nil {
		projectDef = *project
	}
	schema := ir.ConvertFromNormalizer(
		entities, services, nil, nil, endpoints, repos,
		normalizer.ConfigDef{}, nil, nil, nil, nil, projectDef,
	)
	plan := planner.BuildFastAPIPlan(schema, fallbackVersion)
	return pythonFastAPIData{
		ProjectName:   plan.ProjectName,
		Version:       plan.Version,
		Models:        plan.Models,
		Routers:       plan.Routers,
		ServiceStubs:  plan.ServiceStubs,
		RepoStubs:     plan.RepoStubs,
		RouterModules: plan.RouterModules,
	}
}

func buildPythonModels(entities []normalizer.Entity) []pythonModel {
	entityNames := pyemitter.BuildEntityNameSet(entities)
	out := make([]pythonModel, 0, len(entities))
	for _, ent := range entities {
		m := pythonModel{Name: ExportName(ent.Name)}
		for _, f := range ent.Fields {
			if f.SkipDomain {
				continue
			}
			m.Fields = append(m.Fields, pythonModelField{
				Name:       f.Name,
				Type:       pyemitter.FieldTypeWithEntities(f, entityNames),
				IsOptional: f.IsOptional,
			})
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func buildPythonRoutersAndServices(endpoints []normalizer.Endpoint, services []normalizer.Service) ([]pythonRouter, []pythonServiceStub) {
	type groupedEndpoint struct {
		method string
		path   string
		rpc    string
	}
	type implSnippet struct {
		body    string
		imports []string
	}
	implMap := make(map[string]implSnippet, len(services))
	for _, svc := range services {
		for _, m := range svc.Methods {
			if m.Impl == nil {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(m.Impl.Lang), "python") {
				continue
			}
			body := strings.TrimSpace(m.Impl.Code)
			if body == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(svc.Name)) + ":" + strings.ToLower(strings.TrimSpace(m.Name))
			imports := make([]string, 0, len(m.Impl.Imports))
			for _, imp := range m.Impl.Imports {
				if s := strings.TrimSpace(imp); s != "" {
					imports = append(imports, s)
				}
			}
			implMap[key] = implSnippet{
				body:    indentPythonBlock(body, 8),
				imports: imports,
			}
		}
	}

	group := map[string][]groupedEndpoint{}
	sigs := buildPythonRPCSignatures(services)
	for _, ep := range endpoints {
		if strings.EqualFold(ep.Method, "WS") {
			continue
		}
		service := strings.TrimSpace(ep.ServiceName)
		if service == "" {
			service = "Default"
		}
		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" {
			continue
		}
		group[service] = append(group[service], groupedEndpoint{
			method: method,
			path:   ep.Path,
			rpc:    ep.RPC,
		})
	}

	serviceNames := make([]string, 0, len(group))
	for s := range group {
		serviceNames = append(serviceNames, s)
	}
	sort.Strings(serviceNames)

	routers := make([]pythonRouter, 0, len(serviceNames))
	serviceStubs := make([]pythonServiceStub, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		eps := group[serviceName]
		sort.Slice(eps, func(i, j int) bool {
			if eps[i].method != eps[j].method {
				return eps[i].method < eps[j].method
			}
			if eps[i].path != eps[j].path {
				return eps[i].path < eps[j].path
			}
			return eps[i].rpc < eps[j].rpc
		})

		module := toSnake(serviceName)
		if module == "" {
			module = "service"
		}
		className := ExportName(serviceName) + "Service"
		getService := "get_" + module + "_service"

		used := map[string]bool{}
		routes := make([]pythonRoute, 0, len(eps))
		serviceMethods := make([]pythonServiceMethod, 0, len(eps))
		importSet := map[string]bool{}
		for _, ep := range eps {
			base := toSnake(ep.rpc)
			if base == "" {
				base = toSnake(ep.method) + "_handler"
			}
			handler := base
			if used[handler] {
				handler = handler + "_" + strings.ToLower(ep.method)
			}
			baseHandler := handler
			for n := 2; used[handler]; n++ {
				handler = fmt.Sprintf("%s_%d", baseHandler, n)
			}
			used[handler] = true

			pathParams := pathParamNames(ep.path)
			hasBody := ep.method != "GET" && ep.method != "DELETE"

			signatureParts := make([]string, 0, len(pathParams)+2)
			callArgs := make([]string, 0, len(pathParams)+1)
			for _, p := range pathParams {
				signatureParts = append(signatureParts, p+": str")
				callArgs = append(callArgs, p)
			}
			if hasBody {
				sigKey := strings.ToLower(strings.TrimSpace(serviceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.rpc))
				payloadType := "dict[str, Any]"
				if sig := sigs[sigKey]; sig.InputModel != "" && sig.InputModel != "Any" {
					payloadType = "models." + sig.InputModel
				}
				signatureParts = append(signatureParts, "payload: "+payloadType)
				callArgs = append(callArgs, "payload")
			}
			signatureParts = append(signatureParts, fmt.Sprintf("svc: %s = Depends(%s)", className, getService))
			callExpr := "await svc." + handler + "()"
			if len(callArgs) > 0 {
				callExpr = "await svc." + handler + "(" + strings.Join(callArgs, ", ") + ")"
			}

			decorator := strings.ToLower(ep.method)
			switch decorator {
			case "get", "post", "put", "patch", "delete", "head", "options":
			default:
				decorator = "api_route"
			}

			returnType := "Any"
			sigKey := strings.ToLower(strings.TrimSpace(serviceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.rpc))
			if sig := sigs[sigKey]; sig.OutputModel != "" && sig.OutputModel != "Any" {
				returnType = "models." + sig.OutputModel
			}

			routes = append(routes, pythonRoute{
				Method:      ep.method,
				Decorator:   decorator,
				Path:        ep.path,
				HandlerName: handler,
				Signature:   strings.Join(signatureParts, ", "),
				ReturnType:  returnType,
				CallExpr:    callExpr,
			})
			implKey := strings.ToLower(strings.TrimSpace(serviceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.rpc))
			impl := implMap[implKey]
			for _, imp := range impl.imports {
				importSet[imp] = true
			}
			serviceMethods = append(serviceMethods, pythonServiceMethod{
				Name:      handler,
				Body:      impl.body,
				CustomKey: serviceName + "." + handler,
			})
		}

		routers = append(routers, pythonRouter{
			ServiceName: serviceName,
			ModuleName:  module,
			ClassName:   className,
			GetService:  getService,
			Routes:      routes,
		})
		imports := make([]string, 0, len(importSet))
		for imp := range importSet {
			imports = append(imports, imp)
		}
		sort.Strings(imports)
		serviceStubs = append(serviceStubs, pythonServiceStub{
			ServiceName: serviceName,
			ModuleName:  module,
			ClassName:   className,
			GetService:  getService,
			Imports:     imports,
			Methods:     serviceMethods,
		})
	}

	return routers, serviceStubs
}

func indentPythonBlock(code string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(code, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

func buildPythonRepoStubs(repos []normalizer.Repository) []pythonRepoStub {
	out := make([]pythonRepoStub, 0, len(repos))
	for _, r := range repos {
		baseName := ExportName(strings.TrimSpace(r.Name))
		baseName = strings.TrimSuffix(baseName, "Repository")
		if baseName == "" {
			baseName = ExportName(strings.TrimSpace(r.Name))
		}
		module := toSnake(baseName) + "_repository"
		if module == "" {
			module = "repository"
		}
		finders := make([]pythonRepoFinder, 0, len(r.Finders))
		for _, f := range r.Finders {
			name := toSnake(f.Name)
			if name == "" {
				continue
			}
			finders = append(finders, pythonRepoFinder{
				Name:      name,
				CustomKey: baseName + "Repository." + name,
			})
		}
		sort.Slice(finders, func(i, j int) bool { return finders[i].Name < finders[j].Name })

		out = append(out, pythonRepoStub{
			RepoName:      baseName + "Repository",
			ModuleName:    module,
			PortClassName: baseName + "Repository",
			PGClassName:   "Postgres" + baseName + "Repository",
			Finders:       finders,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RepoName < out[j].RepoName })
	return out
}
