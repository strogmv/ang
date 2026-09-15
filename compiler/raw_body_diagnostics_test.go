package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/rawbody"
)

func rawBodyFixture(attrs []string, fields ...string) ([]normalizer.Service, normalizer.Method) {
	m := normalizer.Method{Name: "PostGuestMessage", Source: "cue/api/chat.cue:42"}
	for _, name := range attrs {
		m.Attributes = append(m.Attributes, normalizer.Attribute{Name: name})
	}
	for _, name := range fields {
		m.Input.Fields = append(m.Input.Fields, normalizer.Field{Name: name, Type: "string"})
	}
	return []normalizer.Service{{Name: "company", Methods: []normalizer.Method{m}}}, m
}

func endpoints(verb, path string) []normalizer.Endpoint {
	return []normalizer.Endpoint{{Method: verb, Path: path, ServiceName: "company", RPC: "PostGuestMessage"}}
}

func codes(warnings []normalizer.Warning) []string {
	var out []string
	for _, w := range warnings {
		out = append(out, w.Code)
	}
	return out
}

func TestRawBodyDiagnosticsReportsImplicitPassthrough(t *testing.T) {
	services, _ := rawBodyFixture(nil, "token", "body")
	warnings := collectRawBodyDiagnostics(services, endpoints("POST", "/links/{token}/messages"))
	if len(warnings) != 1 {
		t.Fatalf("expected one diagnostic, got %v", codes(warnings))
	}
	w := warnings[0]
	if w.Code != codeRawBodyImplicit {
		t.Fatalf("expected %s, got %s", codeRawBodyImplicit, w.Code)
	}
	if w.Severity != "warn" {
		t.Fatalf("expected a warning severity, got %q", w.Severity)
	}
	if w.File != "cue/api/chat.cue" || w.Line != 42 {
		t.Fatalf("expected the CUE position to be carried, got %s:%d", w.File, w.Line)
	}
	if !strings.Contains(w.Hint, "@rawBody()") {
		t.Fatalf("expected the hint to name the attribute, got %q", w.Hint)
	}
	if len(w.SuggestedFix) != 1 {
		t.Fatalf("expected a suggested fix, got %d", len(w.SuggestedFix))
	}
}

func TestRawBodyDiagnosticsSilentWhenDeclared(t *testing.T) {
	services, _ := rawBodyFixture([]string{rawbody.Attribute}, "token", "body")
	if warnings := collectRawBodyDiagnostics(services, endpoints("POST", "/links/{token}/messages")); len(warnings) != 0 {
		t.Fatalf("expected no diagnostics for a declared passthrough, got %v", codes(warnings))
	}
}

func TestRawBodyDiagnosticsSilentForPlainJSONInput(t *testing.T) {
	services, _ := rawBodyFixture(nil, "token", "text")
	if warnings := collectRawBodyDiagnostics(services, endpoints("POST", "/links/{token}/messages")); len(warnings) != 0 {
		t.Fatalf("expected no diagnostics for a JSON input, got %v", codes(warnings))
	}
}

func TestRawBodyDiagnosticsSkipsGET(t *testing.T) {
	services, _ := rawBodyFixture(nil, "token", "body")
	if warnings := collectRawBodyDiagnostics(services, endpoints("GET", "/links/{token}/messages")); len(warnings) != 0 {
		t.Fatalf("GET handlers decode query parameters, got %v", codes(warnings))
	}
}

func TestRawBodyDiagnosticsReportsAttributeWithoutField(t *testing.T) {
	services, _ := rawBodyFixture([]string{rawbody.Attribute}, "token", "text")
	warnings := collectRawBodyDiagnostics(services, endpoints("POST", "/links/{token}/messages"))
	if len(warnings) != 1 || warnings[0].Code != codeRawBodyAttributeIgnored {
		t.Fatalf("expected %s, got %v", codeRawBodyAttributeIgnored, codes(warnings))
	}
}

func TestRawBodyDiagnosticsReportsUnfillableFields(t *testing.T) {
	services, _ := rawBodyFixture([]string{rawbody.Attribute}, "token", "body", "locale")
	warnings := collectRawBodyDiagnostics(services, endpoints("POST", "/links/{token}/messages"))
	if len(warnings) != 1 || warnings[0].Code != codeRawBodyFieldsIgnored {
		t.Fatalf("expected %s, got %v", codeRawBodyFieldsIgnored, codes(warnings))
	}
	if !strings.Contains(warnings[0].Message, "locale") {
		t.Fatalf("expected the message to name the field, got %q", warnings[0].Message)
	}
}

func TestRawBodyDiagnosticsIgnoresUnknownOperation(t *testing.T) {
	if warnings := collectRawBodyDiagnostics(nil, endpoints("POST", "/links/{token}/messages")); len(warnings) != 0 {
		t.Fatalf("expected no diagnostics without a matching method, got %v", codes(warnings))
	}
}
