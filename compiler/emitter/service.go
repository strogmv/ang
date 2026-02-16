package emitter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

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
				// Implementation Audit
				for _, m := range svc.Methods {
					hasFlow := len(m.Flow) > 0
					hasManual := overrides[m.Name]
					if !hasFlow && !hasManual {
						e.MissingImpls = append(e.MissingImpls, MissingImpl{
							Service: svc.Name,
							Method:  m.Name,
							Source:  m.Source,
						})
					}
				}

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
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return err
		}
		fmt.Printf("Generated Service Port: %s\n", path)
	}

	return nil
}

func (e *Emitter) EmitServiceImpl(services []ir.Service, entities []ir.Entity, auth *normalizer.AuthDef) error {
	tmplPath := "templates/service_impl.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}
	nServices := IRServicesToNormalizer(services)

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
	t, err := template.New("service_impl").Funcs(funcMapImpl).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "service")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	for _, svc := range nServices {
		// Collect all imports for this service
		importMap := make(map[string]bool)

		// Base imports that every service needs
		baseImports := []string{
			"context",
			"encoding/json",
			"fmt",
			"log/slog",
			"net/http",
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
			importMap[imp] = true
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
						importMap[imp] = true
					}
				}
			}
		}

		var allImports []string
		for imp := range importMap {
			allImports = append(allImports, imp)
		}
		sort.Strings(allImports)

		var buf bytes.Buffer
		a := auth
		if a == nil {
			a = &normalizer.AuthDef{}
		}
		
			overrides := e.getManualMethods(svc.Name)
				// Implementation Audit
				for _, m := range svc.Methods {
					hasFlow := len(m.Flow) > 0
					hasManual := overrides[m.Name]
					if !hasFlow && !hasManual {
						e.MissingImpls = append(e.MissingImpls, MissingImpl{
							Service: svc.Name,
							Method:  m.Name,
							Source:  m.Source,
						})
					}
				}

		if err := t.Execute(&buf, TemplateContext{
			Service:   &svc,
			GoModule:  e.GoModule,
			Overrides: overrides,
		}); err != nil {
			return fmt.Errorf("execute template for %s: %w", svc.Name, err)
		}

		formatted, err := formatGoStrict(buf.Bytes(), "internal/service/"+strings.ToLower(svc.Name)+".go")
		if err != nil {
			return err
		}

		filename := strings.ToLower(svc.Name) + ".go"
		path := filepath.Join(targetDir, filename)
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return err
		}
		fmt.Printf("Generated Service Impl: %s\n", path)
	}

	return nil
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
				// Implementation Audit
				for _, m := range svc.Methods {
					hasFlow := len(m.Flow) > 0
					hasManual := overrides[m.Name]
					if !hasFlow && !hasManual {
						e.MissingImpls = append(e.MissingImpls, MissingImpl{
							Service: svc.Name,
							Method:  m.Name,
							Source:  m.Source,
						})
					}
				}

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
		if err := os.WriteFile(path, formatted, 0644); err != nil {
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
