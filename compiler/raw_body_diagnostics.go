package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/rawbody"
)

const (
	codeRawBodyImplicit         = "W_RAW_BODY_IMPLICIT"
	codeRawBodyAttributeIgnored = "W_RAW_BODY_ATTRIBUTE_IGNORED"
	codeRawBodyFieldsIgnored    = "W_RAW_BODY_FIELDS_IGNORED"
)

// emitRawBodyDiagnostics reports operations whose request decoding is decided by
// a field name rather than by a declared intent.
//
// An input field named "body" makes the generated handler read the raw HTTP body
// into that string. Clients built from the same spec send a JSON object, so the
// call fails with 400 "cannot unmarshal object into Go value of type string" and
// nothing in the spec explains why. @rawBody() states the intent; these
// diagnostics point at every operation that relies on the naming instead.
func emitRawBodyDiagnostics(services []normalizer.Service, endpoints []normalizer.Endpoint, opts PipelineOptions) {
	for _, diag := range collectRawBodyDiagnostics(services, endpoints) {
		recordPipelineDiagnostic(diag, opts)
	}
}

// CollectRawBodyDiagnostics exposes the raw-body checks to tooling.
func CollectRawBodyDiagnostics(services []normalizer.Service, endpoints []normalizer.Endpoint) []normalizer.Warning {
	return collectRawBodyDiagnostics(services, endpoints)
}

func collectRawBodyDiagnostics(services []normalizer.Service, endpoints []normalizer.Endpoint) []normalizer.Warning {
	methodByOp := make(map[string]normalizer.Method)
	for _, svc := range services {
		for _, method := range svc.Methods {
			methodByOp[rawBodyOpKey(svc.Name, method.Name)] = method
		}
	}

	var out []normalizer.Warning
	for _, ep := range endpoints {
		// GET handlers decode query parameters; the body field never applies.
		if strings.EqualFold(strings.TrimSpace(ep.Method), "GET") {
			continue
		}
		method, ok := methodByOp[rawBodyOpKey(ep.ServiceName, ep.RPC)]
		if !ok {
			continue
		}
		op := strings.TrimSpace(ep.ServiceName + "." + ep.RPC)
		file, line := parseSourcePos(method.Source)
		hasAttribute := rawbody.HasAttribute(method)
		hasField := rawbody.HasField(method.Input)

		if hasAttribute && !hasField {
			out = append(out, normalizer.Warning{
				Kind:     "raw-body",
				Code:     codeRawBodyAttributeIgnored,
				Severity: "warn",
				Message:  fmt.Sprintf("%s declares @rawBody() but its input has no \"body\" field, so the attribute has no effect", op),
				Op:       op,
				File:     file,
				Line:     line,
				Hint:     "Add a `body: string` input field to capture the raw request body, or drop @rawBody() to keep normal JSON decoding.",
			})
			continue
		}

		if rawbody.Implicit(method, ep.Path) {
			out = append(out, normalizer.Warning{
				Kind:     "raw-body",
				Code:     codeRawBodyImplicit,
				Severity: "warn",
				Message: fmt.Sprintf(
					"%s has an input field named \"body\", so its handler reads the raw request body as a string instead of decoding a JSON object; SDK clients send an object and get 400",
					op),
				Op:      op,
				File:    file,
				Line:    line,
				CUEPath: op,
				Hint:    "Declare the intent with @rawBody() on the operation (webhooks, signature verification), or rename the field (for example to `text` or `message`) to decode the body as JSON.",
				SuggestedFix: []normalizer.Fix{{
					Op:        "insert",
					File:      file,
					CUEPath:   op,
					Value:     map[string]any{"attribute": "@rawBody()"},
					After:     "@rawBody()",
					Rationale: "raw-body passthrough should be declared, not inferred from a field name",
				}},
			})
			continue
		}

		if hasAttribute {
			if ignored := rawbody.IgnoredFields(method.Input, ep.Path); len(ignored) > 0 {
				out = append(out, normalizer.Warning{
					Kind:     "raw-body",
					Code:     codeRawBodyFieldsIgnored,
					Severity: "warn",
					Message: fmt.Sprintf(
						"%s captures the raw body, so its input field(s) %s are never populated: they are neither the raw body nor path parameters",
						op, strings.Join(ignored, ", ")),
					Op:   op,
					File: file,
					Line: line,
					Hint: "Move those values into the URL path, read them from the raw body inside the implementation, or drop them from the input.",
				})
			}
		}
	}
	return out
}

func rawBodyOpKey(service, rpc string) string {
	return strings.ToLower(strings.TrimSpace(service) + "." + strings.TrimSpace(rpc))
}
