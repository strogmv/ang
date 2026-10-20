package emitter

import "github.com/strogmv/ang/angir/normalizer"

// Default security headers of an API: its answers are JSON and files, never a
// page, so nothing may load, frame or script anything from them.
const (
	DefaultContentSecurityPolicy   = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'"
	DefaultCrossOriginOpenerPolicy = "same-origin"
	DefaultPermissionsPolicy       = "camera=(), microphone=(), geolocation=()"
)

// SecurityHeaders are the values SecurityHeadersMiddleware sets on every
// response besides nosniff, X-Frame-Options and Referrer-Policy; an empty
// value means the header is not sent.
type SecurityHeaders struct {
	ContentSecurityPolicy   string
	CrossOriginOpenerPolicy string
	PermissionsPolicy       string
}

// ResolveSecurityHeaders applies target.security_headers to the defaults: a
// key the target leaves out keeps its default, "" switches the header off.
func ResolveSecurityHeaders(def *normalizer.SecurityHeadersDef) SecurityHeaders {
	out := SecurityHeaders{
		ContentSecurityPolicy:   DefaultContentSecurityPolicy,
		CrossOriginOpenerPolicy: DefaultCrossOriginOpenerPolicy,
		PermissionsPolicy:       DefaultPermissionsPolicy,
	}
	if def == nil {
		return out
	}
	if def.CSP != nil {
		out.ContentSecurityPolicy = *def.CSP
	}
	if def.COOP != nil {
		out.CrossOriginOpenerPolicy = *def.COOP
	}
	if def.PermissionsPolicy != nil {
		out.PermissionsPolicy = *def.PermissionsPolicy
	}
	return out
}
