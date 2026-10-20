package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// The API's responses carried nosniff, X-Frame-Options and Referrer-Policy
// only (dealingi-back security audit, item 23): no CSP, no COOP, no
// Permissions-Policy, and nothing a project could set. They are on by
// default now and target.security_headers changes or switches each off.
func TestSecurityHeadersMiddlewareSetsAPIPolicyFromTarget(t *testing.T) {
	t.Parallel()
	render := func(def *normalizer.SecurityHeadersDef) string {
		tmp := t.TempDir()
		em := New(tmp, "", "templates")
		em.SecurityHeaders = ResolveSecurityHeaders(def)
		if err := em.EmitHTTPCommon(nil); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
		if err != nil {
			t.Fatal(err)
		}
		body := string(data)
		start := strings.Index(body, "func SecurityHeadersMiddleware(")
		return body[start : start+strings.Index(body[start:], "\n}\n")]
	}
	def := render(nil)
	for _, want := range []string{
		`header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")`,
		`header.Set("Cross-Origin-Opener-Policy", "same-origin")`,
		`header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")`,
		`header.Set("X-Content-Type-Options", "nosniff")`,
	} {
		if !strings.Contains(def, want) {
			t.Errorf("default middleware lacks %s:\n%s", want, def)
		}
	}
	csp, off := "default-src 'none'; img-src 'self'", ""
	custom := render(&normalizer.SecurityHeadersDef{CSP: &csp, COOP: &off})
	if !strings.Contains(custom, `"default-src 'none'; img-src 'self'"`) || strings.Contains(custom, "Cross-Origin-Opener-Policy") || !strings.Contains(custom, "Permissions-Policy") {
		t.Fatalf("target.security_headers: csp replaced, coop off, permissions policy kept:\n%s", custom)
	}
}
