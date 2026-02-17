package mcp

import "testing"

func TestAssessArchitectureDiff_Breaking(t *testing.T) {
	diff := map[string]any{
		"entities": map[string]any{
			"added":          []string{},
			"removed":        []string{"Order"},
			"fields_added":   []any{},
			"fields_removed": []any{map[string]any{"entity": "User", "field": "email"}},
		},
		"services": map[string]any{
			"added":           []string{},
			"removed":         []string{},
			"methods_added":   []any{},
			"methods_removed": []any{map[string]any{"service": "Auth", "method": "Login"}},
		},
		"endpoints": map[string]any{
			"added":   []any{},
			"removed": []any{map[string]any{"method": "POST", "path": "/api/login", "rpc": "Login"}},
		},
	}
	got := assessArchitectureDiff(diff)
	if got["breaking_changes"].(int) <= 0 {
		t.Fatalf("expected breaking changes, got %#v", got)
	}
	flags, _ := got["risk_flags"].([]string)
	if len(flags) == 0 || flags[0] == "low_risk" {
		t.Fatalf("expected non-low risk flags, got %#v", flags)
	}
}

func TestAssessArchitectureDiff_NonBreaking(t *testing.T) {
	diff := map[string]any{
		"entities": map[string]any{
			"added":          []string{"Invoice"},
			"removed":        []string{},
			"fields_added":   []any{map[string]any{"entity": "Invoice", "field": "status"}},
			"fields_removed": []any{},
		},
		"services": map[string]any{
			"added":           []string{"Billing"},
			"removed":         []string{},
			"methods_added":   []any{},
			"methods_removed": []any{},
		},
		"endpoints": map[string]any{
			"added":   []any{map[string]any{"method": "GET", "path": "/api/invoices", "rpc": "ListInvoices"}},
			"removed": []any{},
		},
	}
	got := assessArchitectureDiff(diff)
	if got["breaking_changes"].(int) != 0 {
		t.Fatalf("expected no breaking changes, got %#v", got)
	}
	if got["non_breaking_changes"].(int) == 0 {
		t.Fatalf("expected non-breaking changes, got %#v", got)
	}
}
