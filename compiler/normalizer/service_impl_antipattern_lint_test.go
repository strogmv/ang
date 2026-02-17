package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractServices_ImplAntiPatterns(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
CreateInvite: {
	service: "company"
	output: {
		ok: bool
	}
	impls: go: {
		code: """
link := "https://app.example.com/confirm"
l.Info("sending invite", "link", link)
return resp, nil
"""
	}
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile CUE: %v", err)
	}

	n := New()
	var warnings []Warning
	n.WarningSink = func(w Warning) {
		warnings = append(warnings, w)
	}

	_, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}

	requireCode := func(code string) Warning {
		t.Helper()
		for _, w := range warnings {
			if w.Code == code {
				if strings.ToLower(strings.TrimSpace(w.Severity)) != "error" {
					t.Fatalf("expected error severity for %s, got %s", code, w.Severity)
				}
				if !strings.Contains(w.CUEPath, "impls.go.code") {
					t.Fatalf("expected CUEPath to include impls.go.code, got %q", w.CUEPath)
				}
				if strings.TrimSpace(w.Hint) == "" {
					t.Fatalf("expected hint for %s", code)
				}
				return w
			}
		}
		t.Fatalf("expected warning code %s, got %+v", code, warnings)
		return Warning{}
	}

	urlWarning := requireCode("IMPL_HARDCODED_URL_LITERAL")
	logWarning := requireCode("IMPL_LEGACY_LOGGER_ALIAS")
	if urlWarning.Line == 0 || logWarning.Line == 0 {
		t.Fatalf("expected concrete CUE line for anti-pattern warnings, got url=%d log=%d", urlWarning.Line, logWarning.Line)
	}
}
