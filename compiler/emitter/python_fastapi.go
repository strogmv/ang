package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
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
	Name string
	Body string
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
	Name string
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
	Models        []pythonModel
	Routers       []pythonRouter
	ServiceStubs  []pythonServiceStub
	RepoStubs     []pythonRepoStub
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
	data := buildPythonFastAPIData(entities, services, endpoints, repos, project, e.Version)
	funcs := e.getSharedFuncMap()

	root := filepath.Join(e.OutputDir, "app")
	for _, dir := range []string{
		root,
		filepath.Join(root, "routers"),
		filepath.Join(root, "services"),
		filepath.Join(root, "repositories"),
		filepath.Join(root, "repositories", "postgres"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	baseFiles := []struct {
		template string
		out      string
	}{
		{"templates/python/fastapi/main.py.tmpl", "main.py"},
		{"templates/python/fastapi/models.py.tmpl", "models.py"},
		{"templates/python/fastapi/pyproject.toml.tmpl", "pyproject.toml"},
		{"templates/python/fastapi/README.md.tmpl", "README.md"},
		{"templates/python/fastapi/routers_init.py.tmpl", filepath.Join("routers", "__init__.py")},
		{"templates/python/fastapi/services_init.py.tmpl", filepath.Join("services", "__init__.py")},
		{"templates/python/fastapi/repositories_init.py.tmpl", filepath.Join("repositories", "__init__.py")},
		{"templates/python/fastapi/repositories_postgres_init.py.tmpl", filepath.Join("repositories", "postgres", "__init__.py")},
	}
	for _, f := range baseFiles {
		if err := e.emitPythonTemplate(root, f.template, data, funcs, f.out, 0644); err != nil {
			return err
		}
	}

	for _, r := range data.Routers {
		if err := e.emitPythonTemplate(root, "templates/python/fastapi/router.py.tmpl", r, funcs, filepath.Join("routers", r.ModuleName+".py"), 0644); err != nil {
			return err
		}
	}
	for _, s := range data.ServiceStubs {
		if err := e.emitPythonTemplate(root, "templates/python/fastapi/service.py.tmpl", s, funcs, filepath.Join("services", s.ModuleName+".py"), 0644); err != nil {
			return err
		}
	}
	for _, r := range data.RepoStubs {
		if err := e.emitPythonTemplate(root, "templates/python/fastapi/repository_port.py.tmpl", r, funcs, filepath.Join("repositories", r.ModuleName+".py"), 0644); err != nil {
			return err
		}
		if err := e.emitPythonTemplate(root, "templates/python/fastapi/repository_postgres.py.tmpl", r, funcs, filepath.Join("repositories", "postgres", r.ModuleName+".py"), 0644); err != nil {
			return err
		}
	}

	return nil
}

func buildPythonFastAPIData(
	entities []normalizer.Entity,
	services []normalizer.Service,
	endpoints []normalizer.Endpoint,
	repos []normalizer.Repository,
	project *normalizer.ProjectDef,
	fallbackVersion string,
) pythonFastAPIData {
	projectName := "ANG Service"
	version := strings.TrimSpace(fallbackVersion)
	if project != nil {
		if n := strings.TrimSpace(project.Name); n != "" {
			projectName = n
		}
		if v := strings.TrimSpace(project.Version); v != "" {
			version = v
		}
	}
	if version == "" {
		version = "0.1.0"
	}

	models := buildPythonModels(entities)
	routers, serviceStubs := buildPythonRoutersAndServices(endpoints, services)
	repoStubs := buildPythonRepoStubs(repos)

	routerModules := make([]string, 0, len(routers))
	for _, r := range routers {
		routerModules = append(routerModules, r.ModuleName)
	}

	return pythonFastAPIData{
		ProjectName:   projectName,
		Version:       version,
		Models:        models,
		Routers:       routers,
		ServiceStubs:  serviceStubs,
		RepoStubs:     repoStubs,
		RouterModules: routerModules,
	}
}

func buildPythonModels(entities []normalizer.Entity) []pythonModel {
	entityNames := buildPythonEntityNameSet(entities)
	out := make([]pythonModel, 0, len(entities))
	for _, ent := range entities {
		m := pythonModel{Name: ExportName(ent.Name)}
		for _, f := range ent.Fields {
			if f.SkipDomain {
				continue
			}
			m.Fields = append(m.Fields, pythonModelField{
				Name:       f.Name,
				Type:       pythonFieldTypeWithEntities(f, entityNames),
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
				signatureParts = append(signatureParts, "payload: dict[str, Any]")
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

			routes = append(routes, pythonRoute{
				Method:      ep.method,
				Decorator:   decorator,
				Path:        ep.path,
				HandlerName: handler,
				Signature:   strings.Join(signatureParts, ", "),
				CallExpr:    callExpr,
			})
			implKey := strings.ToLower(strings.TrimSpace(serviceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.rpc))
			impl := implMap[implKey]
			for _, imp := range impl.imports {
				importSet[imp] = true
			}
			serviceMethods = append(serviceMethods, pythonServiceMethod{Name: handler, Body: impl.body})
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
			finders = append(finders, pythonRepoFinder{Name: name})
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
