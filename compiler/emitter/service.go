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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

var (
	reSelectorID = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9]*)Id\b`)
)

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
	funcMap["ServiceInterfaceDecl"] = func(svc normalizer.Service) (string, error) {
		return renderServiceInterfaceDecl(svc)
	}
	funcMap["ServiceImplTypeDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplTypeDecl(svc, entities, auth)
	}
	funcMap["ServiceImplConstructorDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplConstructorDecl(svc, entities, auth)
	}

	t, err := template.New("service").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
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
		fmt.Printf("Generated Service Port: %s\n", path)
	}

	return nil
}

func (e *Emitter) EmitServiceImpl(services []ir.Service, entities []ir.Entity, events []ir.Event, auth *normalizer.AuthDef) error {
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

	funcMapImpl := e.getSharedFuncMap()
	funcMapImpl["ServiceImplTypeDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplTypeDecl(svc, entities, auth)
	}
	funcMapImpl["ServiceImplConstructorDecl"] = func(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
		return renderServiceImplConstructorDecl(svc, entities, auth)
	}
	funcMapImpl["ServiceImplMethodSignature"] = func(serviceName string, m normalizer.Method) (string, error) {
		return renderServiceImplMethodSignature(serviceName, m)
	}
	funcMapImpl["CleanImplCode"] = cleanImplCode
	funcMapImpl["FlowRenderable"] = flowRenderable
	funcMapImpl["RenderFlow"] = func(args ...any) string {
		if len(args) < 2 {
			return ""
		}
		serviceName, _ := args[0].(string)
		methodName := ""
		var steps []normalizer.FlowStep
		switch len(args) {
		case 2:
			steps, _ = args[1].([]normalizer.FlowStep)
		default:
			methodName, _ = args[1].(string)
			steps, _ = args[2].([]normalizer.FlowStep)
		}
		return renderFlowForServiceWithSchemaAndSink(serviceName, methodName, steps, nEntities, nEvents, e.WarningSink)
	}
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

	targetDir := filepath.Join(e.OutputDir, "internal", "service")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	for _, svc := range nServices {
		// Collect all imports for this service
		importMap := make(map[string]string) // path → alias (empty = no alias)

		// Base imports that every service needs
		baseImports := []string{
			"context",
			"encoding/json",
			"fmt",
			"log/slog",
			"net/http",
			"os",
			"sort",
			"strings",
			"time",
			"github.com/google/uuid",
			"golang.org/x/crypto/bcrypt",
			e.GoModule + "/internal/config",
			e.GoModule + "/internal/domain",
			e.GoModule + "/internal/pkg/auth",
			e.GoModule + "/internal/pkg/errors",
			e.GoModule + "/internal/pkg/helpers",
			e.GoModule + "/internal/pkg/logger",
			e.GoModule + "/internal/pkg/presence",
			e.GoModule + "/internal/port",
		}
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
					case "url.Parse", "url.Build", "query.Encode", "query.Decode":
						importMap["net/url"] = ""
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
					case "jwt.Sign", "jwt.Verify":
						importMap["crypto/hmac"] = ""
						importMap["crypto/sha256"] = ""
						importMap["encoding/base64"] = ""
					case "oauth2.Token", "oauth2.Refresh":
						importMap["io"] = ""
						importMap["net/url"] = ""
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
					case "session.Get":
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
		fmt.Printf("Generated Service Impl Scaffold: %s\n", scaffoldPath)

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
			methodHeader := e.renderGeneratedHeader(methodSources)

			var methodBuf bytes.Buffer
			if err := methodT.Execute(&methodBuf, TemplateContext{
				Service:          &svc,
				Method:           &method,
				Entities:         nEntities,
				Auth:             a,
				Imports:          allImports,
				GoModule:         e.GoModule,
				Overrides:        overrides,
				ProvenanceHeader: methodHeader,
			}); err != nil {
				return fmt.Errorf("execute method template for %s.%s: %w", svc.Name, m.Name, err)
			}

			methodUnit := "internal/service/" + methodFile
			methodFormatted, err := formatGoStrict(methodBuf.Bytes(), methodUnit)
			if err != nil {
				return err
			}
			methodPath := filepath.Join(targetDir, methodFile)
			if err := writeFileAtomic(methodPath, methodFormatted, 0644); err != nil {
				return err
			}
			fmt.Printf("Generated Service Impl Method: %s\n", methodPath)
		}

		if err := pruneGeneratedFiles(
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
		); err != nil {
			return err
		}
	}

	return nil
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
		filepath.Join(e.OutputDir, file),
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

func cleanImplCode(code, outputName string) string {
	cleaned := strings.TrimLeft(code, "\n")
	out := strings.TrimSpace(outputName)
	if cleaned == "" {
		return cleaned
	}

	lines := strings.Split(cleaned, "\n")
	filtered := make([]string, 0, len(lines))
	removedRespDecl := false
	respDecl := ""
	if out != "" {
		respDecl = "var resp port." + out
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if respDecl != "" && !removedRespDecl && trimmed == respDecl {
			removedRespDecl = true
			continue
		}
		if trimmed == "var err error" {
			continue
		}
		if strings.HasPrefix(trimmed, "err := ") {
			line = strings.Replace(line, "err :=", "err =", 1)
		}
		if out != "" && strings.HasPrefix(trimmed, "resp := port.") {
			line = strings.Replace(line, "resp :=", "resp =", 1)
		}
		// Generic selector normalization for common Go initialisms.
		// Example: req.UserId -> req.UserID, item.CompanyId -> item.CompanyID.
		line = reSelectorID.ReplaceAllString(line, ".$1ID")
		line = strings.ReplaceAll(line, "l.", "slog.")
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
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

	targetDir := filepath.Join(e.OutputDir, "internal", "service")
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
		fmt.Printf("Generated Cached Service: %s\n", path)
	}

	return nil
}

func (e *Emitter) getManualMethods(serviceName string) map[string]bool {
	overrides := make(map[string]bool)
	manualFile := filepath.Join(e.OutputDir, "internal/service", strings.ToLower(serviceName)+".manual.go")

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
