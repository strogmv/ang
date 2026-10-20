package compiler

import "testing"

// An {{if}} (or {{range}}, {{with}}) without {{else}} used to crash the
// build: its nil ElseList reached the walker as a non-nil interface.
func TestCollectTemplateVarPaths_IfWithoutElse(t *testing.T) {
	used, err := collectTemplateVarPaths(`{{if .Location}}Wo: {{.Location}}{{end}} {{range .Rows}}{{.}}{{end}} {{with .Company}}{{.}}{{end}} {{.IP}}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Location", "Rows", "Company", "IP"} {
		if !used[want] {
			t.Fatalf("variable %s not collected: %v", want, used)
		}
	}
}
