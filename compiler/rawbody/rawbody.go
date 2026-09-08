// Package rawbody decides whether an operation's HTTP handler captures the raw
// request body instead of decoding a JSON object into the request struct.
//
// It is a leaf package so that both the emitter (which generates the decoder)
// and the compiler (which reports on it) answer the question the same way.
package rawbody

import (
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// Attribute marks an operation whose HTTP body is captured verbatim into the
// "body" input field instead of being decoded as a JSON object.
const Attribute = "rawBody"

// FieldName is the input field the passthrough decoder writes into.
const FieldName = "body"

// HasAttribute reports whether the operation declares @rawBody().
func HasAttribute(m normalizer.Method) bool {
	for _, attr := range m.Attributes {
		if strings.EqualFold(strings.TrimSpace(attr.Name), Attribute) {
			return true
		}
	}
	return false
}

// HasField reports whether the input declares the raw-body field.
func HasField(input normalizer.Entity) bool {
	for _, f := range input.Fields {
		if strings.EqualFold(f.Name, FieldName) {
			return true
		}
	}
	return false
}

// Passthrough reports whether the handler captures the raw request body into
// req.Body instead of decoding a JSON object into req.
//
// Declaring @rawBody() on the operation is the explicit form. The implicit form
// — a "body" field with every other field bound from the path — stays supported
// for existing specs, but the compiler reports it (W_RAW_BODY_IMPLICIT): a field
// that happens to be named "body" silently switches the decoder, and a client
// that sends a JSON object then gets 400 with no hint of why.
func Passthrough(m normalizer.Method, path string) bool {
	if !HasField(m.Input) {
		return false
	}
	if HasAttribute(m) {
		return true
	}
	return boundByPathOnly(m.Input, path)
}

// Implicit reports whether passthrough is triggered by field naming alone,
// without @rawBody() to confirm the intent.
func Implicit(m normalizer.Method, path string) bool {
	return !HasAttribute(m) && HasField(m.Input) && boundByPathOnly(m.Input, path)
}

// IgnoredFields lists input fields that a passthrough handler never fills: not
// the raw body itself and not bound from the URL path.
func IgnoredFields(input normalizer.Entity, path string) []string {
	bound := pathParamSet(path)
	var out []string
	for _, f := range input.Fields {
		if strings.EqualFold(f.Name, FieldName) {
			continue
		}
		if bound[normalizeName(f.Name)] {
			continue
		}
		out = append(out, f.Name)
	}
	return out
}

// boundByPathOnly reports whether the input is a whole-body passthrough: it has
// a "body" field and every OTHER field is bound from the URL path. This lets a
// webhook capture the raw request body into req.Body while still taking a path
// param (e.g. POST /tg/webhook/{botId} with input {botId: string, body: _}). The
// plain single-"body" case is a subset, so this stays backward-compatible.
func boundByPathOnly(input normalizer.Entity, path string) bool {
	bound := pathParamSet(path)
	hasBody := false
	for _, f := range input.Fields {
		if strings.EqualFold(f.Name, FieldName) {
			hasBody = true
			continue
		}
		if !bound[normalizeName(f.Name)] {
			return false
		}
	}
	return hasBody
}

func pathParamSet(path string) map[string]bool {
	out := map[string]bool{}
	rest := path
	for {
		start := strings.Index(rest, "{")
		if start == -1 {
			return out
		}
		end := strings.Index(rest[start:], "}")
		if end == -1 {
			return out
		}
		out[normalizeName(rest[start+1:start+end])] = true
		rest = rest[start+end:]
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", ""))
}
