package planner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/pkg/names"
)

func BuildFastAPIPlan(schema *ir.Schema, fallbackVersion string) FastAPIPlan {
	projectName := "ANG Service"
	version := strings.TrimSpace(fallbackVersion)
	if schema != nil {
		if n := strings.TrimSpace(schema.Project.Name); n != "" {
			projectName = n
		}
		if v := strings.TrimSpace(schema.Project.Version); v != "" {
			version = v
		}
	}
	if version == "" {
		version = "0.1.0"
	}

	plan := FastAPIPlan{
		ProjectName: projectName,
		Version:     version,
	}
	if schema == nil {
		return plan
	}

	plan.Models = BuildModelPlans(schema.Entities)
	routers, stubs := buildRoutesAndServicePlans(schema.Endpoints, schema.Services)
	plan.Routers = routers
	plan.ServiceStubs = stubs
	plan.RepoStubs = buildRepoPlans(schema.Repos)
	plan.RouterModules = make([]string, 0, len(routers))
	for _, r := range routers {
		plan.RouterModules = append(plan.RouterModules, r.ModuleName)
	}
	return plan
}

type pythonRPCSignature struct {
	InputModel  string
	OutputModel string
}

func buildRPCSignatures(services []ir.Service) map[string]pythonRPCSignature {
	sigMap := make(map[string]pythonRPCSignature, len(services))
	for _, svc := range services {
		for _, m := range svc.Methods {
			key := strings.ToLower(strings.TrimSpace(svc.Name)) + ":" + strings.ToLower(strings.TrimSpace(m.Name))
			in := ""
			if m.Input != nil {
				in = names.ToGoName(strings.TrimSpace(m.Input.Name))
			}
			outModel := ""
			if m.Output != nil {
				outModel = names.ToGoName(strings.TrimSpace(m.Output.Name))
			}
			sigMap[key] = pythonRPCSignature{
				InputModel:  in,
				OutputModel: outModel,
			}
		}
	}
	return sigMap
}

func buildRoutesAndServicePlans(endpoints []ir.Endpoint, services []ir.Service) ([]ServicePlan, []ServicePlan) {
	type groupedEndpoint struct {
		method    string
		path      string
		rpc       string
		needsAuth bool
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
	sigs := buildRPCSignatures(services)
	for _, ep := range endpoints {
		if strings.EqualFold(ep.Method, "WS") {
			continue
		}
		service := strings.TrimSpace(ep.Service)
		if service == "" {
			service = "Default"
		}
		method := strings.ToUpper(strings.TrimSpace(ep.Method))
		if method == "" {
			continue
		}
		group[service] = append(group[service], groupedEndpoint{
			method:    method,
			path:      ep.Path,
			rpc:       ep.RPC,
			needsAuth: ep.Auth != nil && !strings.EqualFold(strings.TrimSpace(ep.Auth.Type), "none"),
		})
	}

	serviceNames := make([]string, 0, len(group))
	for s := range group {
		serviceNames = append(serviceNames, s)
	}
	sort.Strings(serviceNames)

	routers := make([]ServicePlan, 0, len(serviceNames))
	serviceStubs := make([]ServicePlan, 0, len(serviceNames))
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
		className := names.ToGoName(serviceName) + "Service"
		getService := "get_" + module + "_service"

		used := map[string]bool{}
		routes := make([]RoutePlan, 0, len(eps))
		serviceMethods := make([]MethodPlan, 0, len(eps))
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
			inputType := "dict[str, Any]"
			outputType := "Any"
			sigKey := strings.ToLower(strings.TrimSpace(serviceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.rpc))
			if sig := sigs[sigKey]; sig.InputModel != "" && sig.InputModel != "Any" {
				inputType = "models." + sig.InputModel
			}
			if sig := sigs[sigKey]; sig.OutputModel != "" && sig.OutputModel != "Any" {
				outputType = "models." + sig.OutputModel
			}

			if hasBody {
				signatureParts = append(signatureParts, "payload: "+inputType)
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

			routes = append(routes, RoutePlan{
				ServiceName: serviceName,
				HandlerName: handler,
				Method:      ep.method,
				Path:        ep.path,
				InputType:   inputType,
				OutputType:  outputType,
				PathParams:  pathParams,
				NeedsAuth:   ep.needsAuth,
				Decorator:   decorator,
				Signature:   strings.Join(signatureParts, ", "),
				ReturnType:  outputType,
				CallExpr:    callExpr,
			})

			implKey := strings.ToLower(strings.TrimSpace(serviceName)) + ":" + strings.ToLower(strings.TrimSpace(ep.rpc))
			impl := implMap[implKey]
			for _, imp := range impl.imports {
				importSet[imp] = true
			}
			serviceMethods = append(serviceMethods, MethodPlan{
				Name:      handler,
				Body:      impl.body,
				CustomKey: serviceName + "." + handler,
			})
		}

		routers = append(routers, ServicePlan{
			ServiceName: serviceName,
			Name:        serviceName,
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
		serviceStubs = append(serviceStubs, ServicePlan{
			ServiceName: serviceName,
			Name:        serviceName,
			ModuleName:  module,
			ClassName:   className,
			GetService:  getService,
			Imports:     imports,
			Methods:     serviceMethods,
		})
	}
	return routers, serviceStubs
}

func buildRepoPlans(repos []ir.Repository) []RepoPlan {
	out := make([]RepoPlan, 0, len(repos))
	for _, r := range repos {
		baseName := names.ToGoName(strings.TrimSpace(r.Name))
		baseName = strings.TrimSuffix(baseName, "Repository")
		if baseName == "" {
			baseName = names.ToGoName(strings.TrimSpace(r.Name))
		}
		module := toSnake(baseName) + "_repository"
		if module == "" {
			module = "repository"
		}
		finders := make([]RepoFinderPlan, 0, len(r.Finders))
		for _, f := range r.Finders {
			name := toSnake(f.Name)
			if name == "" {
				continue
			}
			finders = append(finders, RepoFinderPlan{
				Name:      name,
				CustomKey: baseName + "Repository." + name,
			})
		}
		sort.Slice(finders, func(i, j int) bool { return finders[i].Name < finders[j].Name })
		out = append(out, RepoPlan{
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

var pathParamRe = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

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
