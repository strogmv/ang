package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// Endpoint metadata keys of an explicitly public endpoint
// (auth: {public: true, reason: "..."}).
const (
	EndpointMetaAuthPublic       = "auth_public"
	EndpointMetaAuthPublicReason = "auth_public_reason"
)

// parseEndpointAuthIntent reads the auth keys that say whether an endpoint
// needs a caller at all:
//
//	auth: {required: true, scope: "orders:read"}   // signed-in caller (jwt)
//	auth: {public: true, reason: "webhook, signed"} // anyone, on purpose
//
// required: true without a type means type "jwt". public: true (or
// required: false) marks the endpoint public on purpose; it may carry a
// reason and nothing that needs a caller (type, roles, scope, check, inject,
// permission). Endpoints that use neither keep today's meaning: no auth block
// is public unless the target sets auth_default: "deny".
func parseEndpointAuthIntent(epVal cue.Value, ep *Endpoint) error {
	authVal := epVal.LookupPath(cue.ParsePath("auth"))
	if !authVal.Exists() {
		return nil
	}
	public := false
	if v := authVal.LookupPath(cue.ParsePath("public")); v.Exists() {
		b, err := v.Bool()
		if err != nil {
			return fmt.Errorf("auth.public must be true or false: %w", err)
		}
		public = b
	}
	requiredSet := false
	required := false
	if v := authVal.LookupPath(cue.ParsePath("required")); v.Exists() {
		b, err := v.Bool()
		if err != nil {
			return fmt.Errorf("auth.required must be true or false: %w", err)
		}
		requiredSet, required = true, b
	}
	reason := ""
	if v := authVal.LookupPath(cue.ParsePath("reason")); v.Exists() {
		s, err := v.String()
		if err != nil {
			return fmt.Errorf("auth.reason must be a string: %w", err)
		}
		reason = strings.TrimSpace(s)
	}
	if public && requiredSet && required {
		return fmt.Errorf("auth: public: true and required: true contradict each other")
	}
	if requiredSet && !required {
		public = true
	}
	if !public {
		if reason != "" {
			return fmt.Errorf("auth.reason explains a public endpoint; add public: true or drop the reason")
		}
		if required && ep.AuthType == "" {
			ep.AuthType = "jwt"
		}
		return nil
	}
	var needsCaller []string
	if ep.AuthType != "" && ep.AuthType != "none" {
		needsCaller = append(needsCaller, "type")
	}
	if len(ep.AuthRoles) > 0 {
		needsCaller = append(needsCaller, "roles")
	}
	if len(ep.RequiredScopes) > 0 {
		needsCaller = append(needsCaller, "scope")
	}
	if ep.AuthCheck != "" {
		needsCaller = append(needsCaller, "check")
	}
	if ep.Permission != "" {
		needsCaller = append(needsCaller, "permission")
	}
	if ep.AuthRight != "" {
		needsCaller = append(needsCaller, "right")
	}
	if v := authVal.LookupPath(cue.ParsePath("inject")); v.Exists() {
		needsCaller = append(needsCaller, "inject")
	}
	if len(needsCaller) > 0 {
		return fmt.Errorf("auth: a public endpoint has no caller, so it cannot also set %s", strings.Join(needsCaller, ", "))
	}
	ep.AuthType = ""
	if ep.Metadata == nil {
		ep.Metadata = map[string]any{}
	}
	ep.Metadata[EndpointMetaAuthPublic] = true
	if reason != "" {
		ep.Metadata[EndpointMetaAuthPublicReason] = reason
	}
	return nil
}

// EndpointIsExplicitlyPublic reports whether an endpoint's metadata marks it
// public on purpose (auth: {public: true}).
func EndpointIsExplicitlyPublic(meta map[string]any) bool {
	b, _ := meta[EndpointMetaAuthPublic].(bool)
	return b
}

// EndpointPublicReason is the reason an explicitly public endpoint gives.
func EndpointPublicReason(meta map[string]any) string {
	s, _ := meta[EndpointMetaAuthPublicReason].(string)
	return strings.TrimSpace(s)
}
