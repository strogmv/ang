package emitter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/rawgo"
)

var (
	// Packages that embedded Go bodies reach for but no flow action implies, so
	// nothing else in the scan would add them. moduleRelative paths are joined
	// to the project's own module path.
	logicCallImportHints = []struct {
		needle         string
		path           string
		moduleRelative bool
	}{
		{needle: "excelize.", path: "github.com/xuri/excelize/v2"},
		{needle: "reqctx.", path: "internal/pkg/reqctx", moduleRelative: true},
	}
)

// methodImports is the import list of one method file: the service's list,
// plus what the method's logic.Call steps name in `imports:`. A step import
// replaces a service import of the same package name (crypto/rand over
// math/rand), so it stays local to its method: another file of the service
// that means math/rand is not forced onto crypto/rand.
func methodImports(serviceMap map[string]string, serviceList []string, flow []normalizer.FlowStep) []string {
	var own [][2]string // alias, path
	var visit func([]normalizer.FlowStep)
	visit = func(steps []normalizer.FlowStep) {
		for _, step := range steps {
			if step.Action == "logic.Call" {
				var entries []string
				switch v := step.Args["imports"].(type) {
				case []string:
					entries = v
				case string:
					entries = []string{v}
				}
				for _, entry := range entries {
					if alias, path := rawgo.NormalizeImport(entry); path != "" {
						own = append(own, [2]string{alias, path})
					}
				}
			}
			for _, key := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if sub, ok := step.Args[key].([]normalizer.FlowStep); ok {
					visit(sub)
				}
			}
			for _, key := range []string{"_cases", "_branches"} {
				if groups, ok := step.Args[key].(map[string][]normalizer.FlowStep); ok {
					for _, branch := range groups {
						visit(branch)
					}
				}
			}
		}
	}
	visit(flow)
	if len(own) == 0 {
		return serviceList
	}
	merged := make(map[string]string, len(serviceMap)+len(own))
	for path, alias := range serviceMap {
		merged[path] = alias
	}
	for _, imp := range own {
		alias, path := imp[0], imp[1]
		name := alias
		if name == "" {
			name = rawgo.PackageName(path)
		}
		for other, otherAlias := range merged {
			otherName := otherAlias
			if otherName == "" {
				otherName = rawgo.PackageName(other)
			}
			if other != path && otherName == name {
				delete(merged, other)
			}
		}
		merged[path] = alias
	}
	out := make([]string, 0, len(merged))
	for path, alias := range merged {
		if alias == "" {
			out = append(out, fmt.Sprintf("%q", path))
		} else {
			out = append(out, fmt.Sprintf("%s %q", alias, path))
		}
	}
	sort.Strings(out)
	return out
}

func addLogicCallImports(fn string, goModule string, importMap map[string]string) {
	fn = strings.TrimSpace(fn)
	if fn == "" {
		return
	}
	for _, hint := range logicCallImportHints {
		if !strings.Contains(fn, hint.needle) {
			continue
		}
		path := hint.path
		if hint.moduleRelative {
			if goModule == "" {
				continue
			}
			path = goModule + "/" + path
		}
		importMap[path] = ""
	}
}

const serviceImplTemplatePath = "templates/service_impl.tmpl"

const (
	serviceImplScaffoldTemplatePath = "templates/service_impl_scaffold.tmpl"
	serviceImplMethodTemplatePath   = "templates/service_impl_method.tmpl"
)

func hasMethodImplementation(m normalizer.Method, overrides map[string]bool) bool {
	if len(m.Flow) > 0 {
		return true
	}
	if m.Impl != nil && strings.TrimSpace(m.Impl.Code) != "" {
		return true
	}
	return overrides[m.Name]
}

func (e *Emitter) addMissingImpl(service, method, source string) {
	if service == "" || method == "" {
		return
	}
	if e.missingImplIndex == nil {
		e.missingImplIndex = make(map[string]struct{})
	}
	key := service + "." + method
	if source != "" {
		key += "|" + source
	}
	if _, exists := e.missingImplIndex[key]; exists {
		return
	}
	e.missingImplIndex[key] = struct{}{}
	e.MissingImpls = append(e.MissingImpls, MissingImpl{
		Service: service,
		Method:  method,
		Source:  source,
	})
}

func (e *Emitter) auditMissingImplementations(svc normalizer.Service, overrides map[string]bool) {
	for _, m := range svc.Methods {
		if hasMethodImplementation(m, overrides) {
			continue
		}
		e.addMissingImpl(svc.Name, m.Name, m.Source)
	}
}

func (e *Emitter) EmitService(services []ir.Service) error {
	tmplPath := "templates/service.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}
	nServices := IRServicesToNormalizer(services)

	funcMap := e.getSharedFuncMap()
	funcMap["HasLogValue"] = func(fields []normalizer.Field) bool {
		for _, f := range fields {
			if f.IsSecret || f.IsPII {
				return true
			}
		}
		return false
	}
	funcMap["HasConstraints"] = func(svc normalizer.Service) bool {
		for _, m := range svc.Methods {
			for _, f := range m.Input.Fields {
				if f.Constraints != nil {
					return true
				}
			}
			if m.Output.Name != "" {
				for _, f := range m.Output.Fields {
					if f.Constraints != nil {
						return true
					}
				}
			}
		}
		return false
	}
	funcMap["FileOutput"] = func(m normalizer.Method) *normalizer.FileOutputSpec {
		return normalizer.MethodFileOutput(m.Metadata)
	}
	funcMap["ServiceHasFileOutput"] = func(svc *normalizer.Service) bool {
		for _, m := range svc.Methods {
			if normalizer.MethodFileOutput(m.Metadata) != nil {
				return true
			}
		}
		return false
	}
	funcMap["ServiceInterfaceDecl"] = func(svc normalizer.Service) (string, error) {
		return renderServiceInterfaceDecl(svc)
	}
	funcMap["ServiceImplTypeDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplTypeDecl(svc, entities, auth, e.serviceImplOptions())
	}
	funcMap["ServiceImplConstructorDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplConstructorDecl(svc, entities, auth, e.serviceImplOptions())
	}

	t, err := template.New("service").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := e.outDir("internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	for _, svc := range nServices {
		var buf bytes.Buffer
		overrides := e.getManualMethods(svc.Name)
		e.auditMissingImplementations(svc, overrides)

		if err := t.Execute(&buf, TemplateContext{
			Service:   &svc,
			GoModule:  e.GoModule,
			Overrides: overrides,
		}); err != nil {
			return err
		}

		formatted, err := formatGoStrict(buf.Bytes(), "internal/port/"+strings.ToLower(svc.Name)+".go")
		if err != nil {
			return err
		}

		filename := strings.ToLower(svc.Name) + ".go"
		path := filepath.Join(targetDir, filename)
		if err := writeFileAtomic(path, formatted, 0644); err != nil {
			return err
		}
		logGenerated("Generated Service Port: %s\n", path)
	}

	return nil
}

func (e *Emitter) EmitServiceImpl(services []ir.Service, entities []ir.Entity, events []ir.Event, auth *normalizer.AuthDef) error {
	return e.emitServiceImpl(services, entities, events, auth, nil)
}

// emitServiceImpl writes the service implementations and the project's
// shared helpers (emitHelpers).
func (e *Emitter) emitServiceImpl(services []ir.Service, entities []ir.Entity, events []ir.Event, auth *normalizer.AuthDef, helpers []ir.Helper) error {
	scaffoldTmplContent, err := ReadTemplateByPath(serviceImplScaffoldTemplatePath)
	if err != nil {
		return err
	}
	methodTmplContent, err := ReadTemplateByPath(serviceImplMethodTemplatePath)
	if err != nil {
		return err
	}
	nServices := IRServicesToNormalizer(services)
	nEntities := IREntitiesToNormalizer(entities)
	nEvents := IREventsToNormalizer(events)
	if err := wrapTransactionalOperations(nServices); err != nil {
		return err
	}

	funcMapImpl := e.getSharedFuncMap()
	funcMapImpl["ServiceImplTypeDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplTypeDecl(svc, entities, auth, e.serviceImplOptions())
	}
	funcMapImpl["ServiceImplConstructorDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplConstructorDecl(svc, entities, auth, e.serviceImplOptions())
	}
	funcMapImpl["ServiceImplMethodSignature"] = func(serviceName string, m normalizer.Method) (string, error) {
		return renderServiceImplMethodSignature(serviceName, m)
	}
	funcMapImpl["ImplCode"] = implCode
	funcMapImpl["FlowRenderable"] = flowRenderable
	// renderFlowWith renders a method's flow, sending its warnings to sink; each
	// parallel method job binds its own sink (see runServiceImplMethodJobs).
	flowInfra := e.flowInfraValues()
	renderFlowWith := func(sink func(normalizer.Warning)) func(args ...any) string {
		return func(args ...any) string {
			return renderFlowTemplateArgs(args, flowInfra, nServices, nEntities, nEvents, sink)
		}
	}
	funcMapImpl["RenderFlow"] = renderFlowWith(e.WarningSink)
	funcMapImpl["RenderImplSteps"] = func(svc normalizer.Service, steps []normalizer.ImplStep, serviceName, methodName string) string {
		return renderImplSteps(svc, steps, serviceName, methodName)
	}
	scaffoldT, err := template.New("service_impl_scaffold").Funcs(funcMapImpl).Parse(string(scaffoldTmplContent))
	if err != nil {
		return err
	}
	methodT, err := template.New("service_impl_method").Funcs(funcMapImpl).Parse(string(methodTmplContent))
	if err != nil {
		return err
	}

	targetDir := e.outDir("internal", "service")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	var methodJobs []serviceImplMethodJob
	var prunes []func() error
	for _, svc := range nServices {
		// Collect all imports for this service
		importMap := make(map[string]string) // path → alias (empty = no alias)

		// Base imports that every service needs (raw Go may use them
		// without listing them; compiler.validateRawGoImports reads the same list).
		baseImports := rawgo.ServiceBaseImports(e.GoModule)
		for _, imp := range baseImports {
			importMap[imp] = ""
		}

		// Add imports from methods
		for _, m := range svc.Methods {
			if m.Impl != nil {
				for _, imp := range m.Impl.Imports {
					imp = strings.Trim(imp, "\"")
					// Normalize some common names to full paths
					if imp == "http" {
						imp = "net/http"
					}
					if imp == "uuid" {
						imp = "github.com/google/uuid"
					}
					if imp != "" {
						// Don't overwrite a non-empty alias (e.g. cryptorand for crypto/rand)
						if cur, ok := importMap[imp]; !ok || cur == "" {
							importMap[imp] = ""
						}
					}
				}
			}
			// Scan flow steps for actions that need extra imports
			var scanFlowImports func(steps []normalizer.FlowStep)
			scanFlowImports = func(steps []normalizer.FlowStep) {
				for _, step := range steps {
					switch step.Action {
					case "exec.Run":
						importMap["os/exec"] = ""
					case "exec.Stream":
						importMap["os/exec"] = ""
						importMap["bufio"] = ""
						importMap["io"] = ""
					case "fs.WriteFile":
						importMap["path/filepath"] = ""
					case "cache.Get", "cache.Set", "cache.Del":
						importMap["github.com/redis/go-redis/v9"] = ""
					case "http.Call":
						importMap["net/http"] = ""
						importMap["io"] = ""
					case "http.Request":
						importMap["net/http"] = ""
						importMap["io"] = ""
						importMap["context"] = ""
						importMap["encoding/json"] = ""
						importMap["strings"] = ""
					case "http.RetryPolicy":
						importMap["net/http"] = ""
						importMap["io"] = ""
						importMap["context"] = ""
						importMap["strings"] = ""
						importMap["time"] = ""
					case "http.Paginate":
						importMap["net/http"] = ""
						importMap["io"] = ""
						importMap["net/url"] = ""
						importMap["encoding/json"] = ""
					case "idem.DeriveKey", "idempotency.DeriveKey":
						importMap["crypto/sha256"] = ""
						importMap["encoding/hex"] = ""
						importMap["fmt"] = ""
					case "idem.Check", "idem.SaveResult", "idempotency.Check", "idempotency.SaveResult":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
					case "dedupe.Once":
						importMap["fmt"] = ""
					case "ratelimit.Check", "ratelimit.Limit":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["time"] = ""
						importMap["net/http"] = ""
					case "quota.Check":
						importMap["fmt"] = ""
						importMap["net/http"] = ""
						importMap["strconv"] = ""
						importMap["time"] = ""
					case "budget.Check", "budget.Consume":
						importMap["fmt"] = ""
						importMap["net/http"] = ""
						importMap["strconv"] = ""
					case "context.Trim":
						importMap["strings"] = ""
					case "profile.Require":
						importMap["net/http"] = ""
					case "concurrency.Limit", "concurrency.Run":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["time"] = ""
						importMap["net/http"] = ""
					case "circuit.Check":
						importMap["fmt"] = ""
						importMap["net/http"] = ""
					case "circuit.RecordSuccess":
						// no extra imports
					case "circuit.RecordFailure", "circuit.Breaker":
						importMap["encoding/json"] = ""
						importMap["time"] = ""
						importMap["fmt"] = ""
						importMap["net/http"] = ""
					case "bulkhead.Acquire", "bulkhead.Run":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["time"] = ""
						importMap["net/http"] = ""
					case "log.Emit", "metric.Emit":
						importMap["log/slog"] = ""
					case "trace.Span":
						importMap["go.opentelemetry.io/otel"] = ""
						importMap["go.opentelemetry.io/otel/attribute"] = ""
						importMap["fmt"] = ""
					case "slo.Budget":
						importMap["context"] = ""
						importMap["time"] = ""
						importMap["log/slog"] = ""
					case "rand.Code":
						importMap["crypto/rand"] = "cryptorand"
						importMap["encoding/binary"] = ""
					case "rand.Token":
						importMap["crypto/rand"] = "cryptorand"
						importMap["encoding/hex"] = ""
					case "regex.Match", "regex.Replace":
						importMap["regexp"] = ""
					case "base64.Encode", "base64.Decode":
						importMap["encoding/base64"] = ""
					case "url.Parse", "query.Encode", "query.Decode":
						importMap["net/url"] = ""
					case "path.Base":
						importMap["path"] = ""
						importMap["strings"] = ""
					case "str.ReplaceAll", "str.TrimSpace":
						importMap["strings"] = ""
					case "url.Build":
						importMap["net/url"] = ""
						importMap["path"] = ""
					case "map.Set", "map.Merge":
						importMap["maps"] = ""
					case "hash.Sum":
						importMap["crypto/sha1"] = ""
						importMap["crypto/sha256"] = ""
						importMap["crypto/md5"] = ""
						importMap["encoding/hex"] = ""
						importMap["strings"] = ""
					case "hash.HMAC":
						importMap["crypto/hmac"] = ""
						importMap["crypto/sha1"] = ""
						importMap["crypto/sha256"] = ""
						importMap["crypto/md5"] = ""
						importMap["encoding/hex"] = ""
						importMap["hash"] = ""
						importMap["strings"] = ""
					case "ulid.New":
						importMap["encoding/base32"] = ""
						importMap["crypto/rand"] = "cryptorand"
						importMap["time"] = ""
					case "math.Op":
						importMap["math"] = ""
						importMap["strings"] = ""
					case "jsonpath.Get", "jsonpath.Set":
						importMap["strconv"] = ""
						importMap["strings"] = ""
					case "template.Render":
						importMap["bytes"] = ""
						importMap["text/template"] = ""
					case "jwt.Sign", "jwt.Verify", "token.Generate", "token.Verify":
						importMap["crypto/hmac"] = ""
						importMap["crypto/sha256"] = ""
						importMap["encoding/base64"] = ""
					case "oauth2.Token", "oauth2.Refresh":
						importMap["io"] = ""
						importMap["net/url"] = ""
					case "oauth.Google.GetURL", "oauth.Google.Exchange", "oauth.Google.UserInfo":
						importMap["golang.org/x/oauth2"] = ""
						importMap["golang.org/x/oauth2/google"] = ""
						importMap["encoding/json"] = ""
						importMap["strings"] = ""
					case "http.SOAP":
						importMap["net/http"] = ""
						importMap["io"] = ""
						importMap["context"] = ""
						importMap["encoding/xml"] = ""
						importMap["strings"] = ""
						importMap["fmt"] = ""
					case "crypto.Encrypt", "crypto.Decrypt":
						importMap["crypto/aes"] = ""
						importMap["crypto/cipher"] = ""
						importMap["crypto/sha256"] = ""
						importMap["crypto/rand"] = "cryptorand"
						importMap["encoding/base64"] = ""
					case "rbac.CheckPermission":
						importMap[e.GoModule+"/internal/pkg/rbac"] = ""
					case "parallel.Run":
						importMap["sync"] = ""
					case "flow.Parallel", "flow.Join":
						importMap["sync"] = ""
						importMap["context"] = ""
					case "flow.Switch":
						if match, ok := step.Args["match"].(string); ok && strings.EqualFold(strings.TrimSpace(match), "glob") {
							importMap["path"] = ""
						}
					case "flow.Race":
						importMap["sync"] = ""
						importMap["context"] = ""
						importMap["fmt"] = ""
					case "pdf.Render":
						importMap[e.GoModule+"/internal/pkg/report"] = ""
					case "webhook.Send":
						importMap["bytes"] = ""
					case "webhook.VerifySignature":
						importMap["crypto/hmac"] = ""
						importMap["crypto/sha256"] = ""
						importMap["encoding/hex"] = ""
					case "queue.Dequeue":
						importMap["crypto/rand"] = "cryptorand"
						importMap["math/big"] = ""
					case "storage.Upload":
						importMap["bytes"] = ""
					case "storage.Download":
						importMap["io"] = ""
					case "archive.ZipDir":
						importMap["archive/zip"] = ""
						importMap["bytes"] = ""
						importMap["io"] = ""
						importMap["path/filepath"] = ""
					case "session.Get", "locale.Resolve":
						importMap[e.GoModule+"/internal/pkg/reqctx"] = ""
					case "claude.Chat":
						importMap["bytes"] = ""
						importMap["context"] = ""
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["io"] = ""
						importMap["net/http"] = ""
						importMap["os"] = ""
						importMap["time"] = ""
					case "openai.Chat", "openai.Embed":
						importMap["bytes"] = ""
						importMap["context"] = ""
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["io"] = ""
						importMap["net/http"] = ""
						importMap["os"] = ""
						importMap["time"] = ""
					case "openai.Stream":
						importMap["bufio"] = ""
						importMap["bytes"] = ""
						importMap["context"] = ""
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["net/http"] = ""
						importMap["os"] = ""
						importMap["strings"] = ""
						importMap["time"] = ""
					case "plan.BuildAutomata", "plan.BuildMicroPlan":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["strings"] = ""
					case "cue.EmitProject":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["sort"] = ""
						importMap["strings"] = ""
					case "cue.ValidateProject":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["os"] = ""
						importMap["os/exec"] = ""
						importMap["path/filepath"] = ""
						importMap["strings"] = ""
					case "cue.WriteProjectFiles":
						importMap["encoding/json"] = ""
						importMap["fmt"] = ""
						importMap["os"] = ""
						importMap["path/filepath"] = ""
						importMap["sort"] = ""
						importMap["strings"] = ""
					case "logic.Call":
						if fn, ok := step.Args["func"].(string); ok {
							addLogicCallImports(fn, e.GoModule, importMap)
						}
						if imports, ok := step.Args["_funcRefImports"].([]string); ok {
							for _, path := range imports {
								importMap[path] = ""
							}
						}
					}
					// Recurse into child steps
					for _, childKey := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
						if sub, ok := step.Args[childKey].([]normalizer.FlowStep); ok {
							scanFlowImports(sub)
						}
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							scanFlowImports(branch)
						}
					}
					if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range branches {
							scanFlowImports(branch)
						}
					}
				}
			}
			scanFlowImports(m.Flow)
		}

		var allImports []string
		for path, alias := range importMap {
			if alias == "" {
				allImports = append(allImports, fmt.Sprintf("%q", path))
			} else {
				allImports = append(allImports, fmt.Sprintf("%s %q", alias, path))
			}
		}
		sort.Strings(allImports)

		a := auth
		if a == nil {
			a = &normalizer.AuthDef{}
		}

		overrides := e.getManualMethods(svc.Name)
		e.auditMissingImplementations(svc, overrides)

		serviceLower := strings.ToLower(svc.Name)
		legacyMonolith := filepath.Join(targetDir, serviceLower+".go")
		_ = os.Remove(legacyMonolith)

		scaffoldFile := serviceLower + "_impl.gen.go"
		keep := map[string]struct{}{
			scaffoldFile: {},
		}

		serviceSources := collectServiceImplSources(svc)
		serviceHeader := e.renderGeneratedHeader(serviceSources)

		var scaffoldBuf bytes.Buffer
		if err := scaffoldT.Execute(&scaffoldBuf, TemplateContext{
			Service:          &svc,
			Entities:         nEntities,
			Auth:             a,
			Imports:          allImports,
			GoModule:         e.GoModule,
			Overrides:        overrides,
			ProvenanceHeader: serviceHeader,
		}); err != nil {
			return fmt.Errorf("execute scaffold template for %s: %w", svc.Name, err)
		}

		scaffoldUnit := "internal/service/" + scaffoldFile
		scaffoldFormatted, err := formatGoStrict(scaffoldBuf.Bytes(), scaffoldUnit)
		if err != nil {
			return err
		}
		scaffoldPath := filepath.Join(targetDir, scaffoldFile)
		if err := writeFileAtomic(scaffoldPath, scaffoldFormatted, 0644); err != nil {
			return err
		}
		logGenerated("Generated Service Impl Scaffold: %s\n", scaffoldPath)

		for _, m := range svc.Methods {
			if overrides[m.Name] {
				continue
			}
			methodFile := serviceLower + "__" + ToSnakeCase(m.Name) + ".gen.go"
			keep[methodFile] = struct{}{}

			method := m
			methodSources := []string{method.Source}
			if strings.TrimSpace(method.Source) == "" {
				methodSources = serviceSources
			}
			methodJobs = append(methodJobs, serviceImplMethodJob{
				name: svc.Name + "." + m.Name,
				ctx: TemplateContext{
					Service:          &svc,
					Method:           &method,
					Entities:         nEntities,
					Auth:             a,
					Imports:          methodImports(importMap, allImports, method.Flow),
					GoModule:         e.GoModule,
					Overrides:        overrides,
					ProvenanceHeader: e.renderGeneratedHeader(methodSources),
				},
				unit:       "internal/service/" + methodFile,
				path:       filepath.Join(targetDir, methodFile),
				sourcePath: e.sourceBackendPath("internal", "service", methodFile),
			})
		}

		prunes = append(prunes, func() error {
			return pruneGeneratedFiles(
				targetDir,
				keep,
				func(name string) bool {
					if name == scaffoldFile {
						return true
					}
					return strings.HasPrefix(name, serviceLower+"__") && strings.HasSuffix(name, ".gen.go")
				},
				func(path string) bool {
					return fileContainsAny(path, "Code generated by ANG", "DO NOT EDIT")
				},
			)
		})
	}

	if err := e.emitOperationMetrics(); err != nil {
		return err
	}

	// Method files are independent: render, format and write them in parallel.
	// Pruning waits for them, since it deletes what no method wrote.
	if err := e.runServiceImplMethodJobs(methodT, renderFlowWith, methodJobs); err != nil {
		return err
	}
	for _, prune := range prunes {
		if err := prune(); err != nil {
			return err
		}
	}
	return e.emitHelpers(targetDir, helpers, nServices)
}

func cloneInfraValues(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src)+1)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

type provenanceSource struct {
	Path string
	SHA  string
}

func collectServiceImplSources(svc normalizer.Service) []string {
	out := make([]string, 0, len(svc.Methods)+1)
	out = append(out, svc.Source)
	for _, m := range svc.Methods {
		out = append(out, m.Source)
	}
	return out
}

func sourceRefToFile(source string) string {
	s := strings.TrimSpace(source)
	if s == "" {
		return ""
	}
	last := strings.LastIndex(s, ":")
	if last <= 0 || last+1 >= len(s) {
		return filepath.Clean(s)
	}
	if _, err := strconv.Atoi(s[last+1:]); err != nil {
		return filepath.Clean(s)
	}
	return filepath.Clean(s[:last])
}

func (e *Emitter) resolveSourcePath(file string) string {
	if file == "" {
		return ""
	}
	if filepath.IsAbs(file) {
		return filepath.Clean(file)
	}
	candidates := []string{
		e.outDir(file),
		file,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return filepath.Clean(c)
		}
	}
	return filepath.Clean(file)
}

func (e *Emitter) displaySourcePath(path string) string {
	if path == "" {
		return "<unknown>"
	}
	if rel, err := filepath.Rel(e.OutputDir, path); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func (e *Emitter) hashSourceFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:8]
}

func (e *Emitter) renderGeneratedHeader(sourceRefs []string) string {
	seen := make(map[string]struct{})
	items := make([]provenanceSource, 0, len(sourceRefs))

	for _, ref := range sourceRefs {
		file := sourceRefToFile(ref)
		if file == "" {
			continue
		}
		resolved := e.resolveSourcePath(file)
		display := e.displaySourcePath(resolved)
		if _, ok := seen[display]; ok {
			continue
		}
		seen[display] = struct{}{}
		items = append(items, provenanceSource{
			Path: display,
			SHA:  e.hashSourceFile(resolved),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })

	version := strings.TrimSpace(e.Version)
	if version == "" {
		version = "dev"
	}

	var b strings.Builder
	b.WriteString("// Code generated by ANG v")
	b.WriteString(version)
	b.WriteString(" from:\n")
	if len(items) == 0 {
		b.WriteString("//   <unknown source>\n")
	} else {
		for _, it := range items {
			b.WriteString("//   ")
			b.WriteString(it.Path)
			b.WriteString(" (sha: ")
			b.WriteString(it.SHA)
			b.WriteString(")\n")
		}
	}
	b.WriteString("// DO NOT EDIT.\n\n")
	return b.String()
}

func (e *Emitter) EmitCachedService(services []ir.Service) error {
	tmplPath := "templates/service_cached.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}
	nServices := IRServicesToNormalizer(services)

	t, err := template.New("service_cached").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := e.outDir("internal", "service")
	for _, svc := range nServices {
		var buf bytes.Buffer
		overrides := e.getManualMethods(svc.Name)
		e.auditMissingImplementations(svc, overrides)

		if err := t.Execute(&buf, TemplateContext{
			Service:   &svc,
			GoModule:  e.GoModule,
			Overrides: overrides,
		}); err != nil {
			return err
		}

		formatted, err := formatGoStrict(buf.Bytes(), "internal/service/"+strings.ToLower(svc.Name)+"_cached.go")
		if err != nil {
			return err
		}

		filename := strings.ToLower(svc.Name) + "_cached.go"
		path := filepath.Join(targetDir, filename)
		if err := writeFileAtomic(path, formatted, 0644); err != nil {
			return err
		}
		logGenerated("Generated Cached Service: %s\n", path)
	}

	return nil
}

func (e *Emitter) getManualMethods(serviceName string) map[string]bool {
	overrides := make(map[string]bool)
	// Hand-written overrides live in the project; the output directory of a
	// build starts empty.
	manualFile := e.sourceBackendPath("internal", "service", strings.ToLower(serviceName)+".manual.go")

	if _, err := os.Stat(manualFile); os.IsNotExist(err) {
		return overrides
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, manualFile, nil, 0)
	if err != nil {
		return overrides
	}

	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				// Ищем методы вида (s *ServiceNameImpl) MethodName
				overrides[fn.Name.Name] = true
			}
		}
	}
	return overrides
}

// flowInfraValues is e.InfraValues plus what flow rendering needs from target
// options: under target.service_config, the string config fields by env var.
func (e *Emitter) flowInfraValues() map[string]any {
	if !e.ServiceConfig {
		return e.InfraValues
	}
	cfg := &normalizer.ConfigDef{}
	if e.IRSchema != nil {
		cfg.Fields = IRFieldsToNormalizer(e.IRSchema.Config.Fields)
	}
	cfg = ensureRuntimeConfigFields(cfg)
	fields := map[string]string{}
	for _, f := range cfg.Fields {
		if f.EnvVar == "" || (f.Type != "" && f.Type != "string") {
			continue
		}
		name := f.Name
		if name == "" {
			continue
		}
		if name[0] < 'A' || name[0] > 'Z' {
			name = ExportName(name)
		}
		fields[f.EnvVar] = name
	}
	out := cloneInfraValues(e.InfraValues)
	out[flowInfraKeyServiceConfig] = fields
	return out
}
