package compiler

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// Authorization default of a target (target.auth_default).
const (
	// AuthDefaultAllow keeps an endpoint without an auth block public, as
	// ANG always did. A public endpoint that changes state is reported.
	AuthDefaultAllow = "allow"
	// AuthDefaultDeny makes every endpoint without an auth block require a
	// signed-in caller; a public endpoint says so with auth: {public: true}.
	AuthDefaultDeny = "deny"
)

// Diagnostic codes of the endpoint authorization check.
const (
	// CodeAuthPublicMutation: an endpoint without auth changes state and does
	// not say it is public on purpose (auth_default "allow").
	CodeAuthPublicMutation = "AUTH_PUBLIC_MUTATION"
	// CodeAuthPublicNoReason: auth: {public: true} on an endpoint that changes
	// state, without a reason. An error under auth_default "deny".
	CodeAuthPublicNoReason = "AUTH_PUBLIC_NO_REASON"
	// CodeAuthDefaultInvalid: target.auth_default is neither "allow" nor "deny".
	CodeAuthDefaultInvalid = "AUTH_DEFAULT_INVALID"
)

// ValidAuthDefault reports whether mode is a known target.auth_default value
// ("" means "allow").
func ValidAuthDefault(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "", AuthDefaultAllow, AuthDefaultDeny:
		return true
	}
	return false
}

// AuthCheckResult is what ApplyAuthDefault found.
type AuthCheckResult struct {
	Warnings []normalizer.Warning
	// Errors are the endpoints a "deny" target refuses to build, one line each.
	Errors []string
}

// Err joins Errors into one error, nil when there are none.
func (r AuthCheckResult) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %d public endpoint(s) change state without a reason (auth: {public: true, reason: \"...\"}):\n  %s",
		CodeAuthPublicNoReason, len(r.Errors), strings.Join(r.Errors, "\n  "))
}

// ApplyAuthDefault returns the schema a target emits under its auth default,
// and what it found about public endpoints that change state.
//
// "allow" (and ""): the schema is returned as is; an endpoint without auth
// that changes state gets AUTH_PUBLIC_MUTATION unless it says
// auth: {public: true, reason: "..."}; public: true without a reason on such
// an endpoint gets AUTH_PUBLIC_NO_REASON (a warning).
//
// "deny": every endpoint without auth and without public: true requires a
// signed-in caller (auth type jwt), in a copy of the schema; public: true
// without a reason on an endpoint that changes state is an error.
func ApplyAuthDefault(schema *ir.Schema, mode string) (*ir.Schema, AuthCheckResult, error) {
	var res AuthCheckResult
	if schema == nil {
		return nil, res, nil
	}
	mode = strings.TrimSpace(mode)
	if !ValidAuthDefault(mode) {
		return nil, res, fmt.Errorf("%s: auth_default must be %q or %q, got %q", CodeAuthDefaultInvalid, AuthDefaultAllow, AuthDefaultDeny, mode)
	}
	deny := mode == AuthDefaultDeny
	methods := methodIndex(schema)

	out := schema
	if deny {
		cp := *schema
		cp.Endpoints = make([]ir.Endpoint, len(schema.Endpoints))
		copy(cp.Endpoints, schema.Endpoints)
		out = &cp
	}
	for i := range out.Endpoints {
		ep := &out.Endpoints[i]
		explicit := normalizer.EndpointIsExplicitlyPublic(ep.Metadata)
		if !explicit && !endpointIsPublic(*ep) {
			continue
		}
		if deny && !explicit {
			ep.Auth = &ir.EndpointAuth{Type: "jwt"}
			continue
		}
		mutates, why := EndpointMutates(*ep, methods[ep.Service+"."+ep.RPC])
		if !mutates {
			continue
		}
		where := endpointLabel(*ep)
		if explicit {
			if normalizer.EndpointPublicReason(ep.Metadata) != "" {
				continue
			}
			if deny {
				res.Errors = append(res.Errors, fmt.Sprintf("%s (%s): %s", where, why, sourceOrUnknown(ep.Source)))
				continue
			}
			res.Warnings = append(res.Warnings, normalizer.Warning{
				Kind:     "auth",
				Code:     CodeAuthPublicNoReason,
				Severity: "warn",
				Op:       ep.RPC,
				Message:  fmt.Sprintf("%s is public on purpose and %s, but gives no reason", where, why),
				Hint:     `say why anyone may call it: auth: {public: true, reason: "..."}`,
				Path:     ep.Source,
			})
			continue
		}
		res.Warnings = append(res.Warnings, normalizer.Warning{
			Kind:     "auth",
			Code:     CodeAuthPublicMutation,
			Severity: "warn",
			Op:       ep.RPC,
			Message:  fmt.Sprintf("%s has no auth and %s: anyone can call it", where, why),
			Hint:     `require a caller with auth: {required: true} (or type: "jwt"), or mark it public on purpose: auth: {public: true, reason: "..."}`,
			Path:     ep.Source,
		})
	}
	sort.Strings(res.Errors)
	return out, res, nil
}

// endpointIsPublic: no auth block that needs a caller. An endpoint that only
// lists API key scopes is not public (RequireScopeMiddleware refuses it).
func endpointIsPublic(ep ir.Endpoint) bool {
	if len(ep.RequiredScopes) > 0 {
		return false
	}
	if ep.Auth == nil {
		return true
	}
	return strings.TrimSpace(ep.Auth.Type) == "" && ep.Auth.Permission == "" && len(ep.Auth.Roles) == 0
}

func endpointLabel(ep ir.Endpoint) string {
	return fmt.Sprintf("%s %s %s", ep.RPC, strings.ToUpper(ep.Method), ep.Path)
}

func sourceOrUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "source unknown"
	}
	return s
}

func methodIndex(schema *ir.Schema) map[string]*ir.Method {
	out := map[string]*ir.Method{}
	for si := range schema.Services {
		svc := &schema.Services[si]
		for mi := range svc.Methods {
			out[svc.Name+"."+svc.Methods[mi].Name] = &svc.Methods[mi]
		}
	}
	return out
}

// writeActions are flow actions that change state outside the request.
var writeActions = map[string]bool{
	"repo.Save": true, "repo.Delete": true, "repo.Upsert": true,
	"event.Publish": true, "event.Outbox": true, "event.EmitIf": true,
	"mail.Send": true, "notify.Dispatch": true, "notify.Email": true, "notify.Send": true,
	"storage.Upload": true, "storage.Delete": true,
	"queue.Enqueue": true, "state.Set": true, "state.Delete": true,
}

// rawGoWrite finds a repository write or an event publish in raw Go.
var rawGoWrite = regexp.MustCompile(`Repo\.(Save|SaveBatch|Delete|Insert|Update|Upsert)[A-Za-z]*\(|\.Publish[A-Z][A-Za-z0-9]*\(|\bpublisher\.Publish\(`)

// EndpointMutates reports whether calling an endpoint changes state, and why:
// a POST/PUT/PATCH/DELETE does; a GET does when its operation writes (a write
// flow action, or a repository write or event publish in its Go). WebSocket
// streams are not requests that mutate.
func EndpointMutates(ep ir.Endpoint, m *ir.Method) (bool, string) {
	method := strings.ToUpper(strings.TrimSpace(ep.Method))
	switch method {
	case "WS":
		return false, ""
	case "POST", "PUT", "PATCH", "DELETE":
		return true, "is a " + method
	}
	if m == nil {
		return false, ""
	}
	if action := firstWriteAction(m.Flow); action != "" {
		return true, "writes (" + action + ")"
	}
	if m.Impl != nil {
		if hit := rawGoWrite.FindString(m.Impl.Code); hit != "" {
			return true, "writes (" + strings.TrimSuffix(hit, "(") + ")"
		}
	}
	return false, ""
}

func firstWriteAction(steps []ir.FlowStep) string {
	for _, st := range steps {
		if writeActions[st.Action] {
			return st.Action
		}
		for _, v := range st.Args {
			if s, ok := v.(string); ok {
				if hit := rawGoWrite.FindString(s); hit != "" {
					return strings.TrimSuffix(hit, "(")
				}
			}
		}
		nested := [][]ir.FlowStep{st.Steps, st.Then, st.Else, st.Default, st.IfNew, st.IfExists}
		keys := make([]string, 0, len(st.Cases))
		for k := range st.Cases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			nested = append(nested, st.Cases[k])
		}
		for _, n := range nested {
			if a := firstWriteAction(n); a != "" {
				return a
			}
		}
	}
	return ""
}
