package emitter

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	angir "github.com/strogmv/ang-ir"
	"github.com/strogmv/ang-ir/normalizer"
)

func TestEmitAssistantChatWithTools_MethodTemplateFormats(t *testing.T) {
	t.Parallel()

	root := filepath.Clean(filepath.Join("..", ".."))
	result, err := angir.Load(root)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	if result == nil || result.Schema == nil || result.Normalized == nil {
		t.Fatalf("expected schema and normalized model")
	}

	var assistant *normalizer.Service
	for i := range result.Normalized.Services {
		if strings.EqualFold(result.Normalized.Services[i].Name, "Assistant") {
			assistant = &result.Normalized.Services[i]
			break
		}
	}
	if assistant == nil {
		t.Fatal("assistant service not found")
	}

	var method *normalizer.Method
	for i := range assistant.Methods {
		if assistant.Methods[i].Name == "ChatWithAssistant" {
			method = &assistant.Methods[i]
			break
		}
	}
	if method == nil {
		t.Fatal("ChatWithAssistant method not found")
	}

	methodTmplContent, err := ReadTemplateByPath(serviceImplMethodTemplatePath)
	if err != nil {
		t.Fatalf("read method template: %v", err)
	}

	em := New(t.TempDir(), filepath.Join(t.TempDir(), "frontend"), "templates")
	em.GoModule = "github.com/strogmv/ang"

	nEntities := result.Normalized.Entities
	nEvents := result.Normalized.Events
	nServices := result.Normalized.Services

	funcMap := em.getSharedFuncMap()
	funcMap["ServiceImplMethodSignature"] = func(serviceName string, m normalizer.Method) (string, error) {
		return renderServiceImplMethodSignature(serviceName, m)
	}
	funcMap["CleanImplCode"] = cleanImplCode
	funcMap["FlowRenderable"] = flowRenderable
	funcMap["RenderFlow"] = func(args ...any) string {
		serviceName, _ := args[0].(string)
		methodName, _ := args[1].(string)
		isStreaming, _ := args[2].(bool)
		steps, _ := args[3].([]normalizer.FlowStep)
		infraValues := cloneInfraValues(em.InfraValues)
		infraValues[flowInfraKeyServicesCatalog] = nServices
		return renderFlowForServiceWithSchemaAndSinkModeWithInfra(serviceName, methodName, isStreaming, steps, nEntities, nEvents, em.WarningSink, infraValues)
	}
	funcMap["RenderImplSteps"] = func(svc normalizer.Service, steps []normalizer.ImplStep, serviceName, methodName string) string {
		return renderImplSteps(svc, steps, serviceName, methodName)
	}

	tmpl, err := template.New("service_impl_method").Funcs(funcMap).Parse(string(methodTmplContent))
	if err != nil {
		t.Fatalf("parse method template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, TemplateContext{
		Service: assistant,
		Method:  method,
		Imports: []string{`"context"`, `"encoding/json"`, `"fmt"`, `"net/http"`, `"os"`, `"strings"`, `"time"`, `"bytes"`, `"io"`},
		GoModule: "github.com/strogmv/ang",
	}); err != nil {
		t.Fatalf("execute method template: %v", err)
	}

	src := buf.Bytes()
	if _, err := formatGoStrict(src, "internal/service/assistant__chat_with_assistant.gen.go"); err != nil {
		t.Fatalf("generated method is invalid: %v\n\n%s", err, string(src))
	}
}
