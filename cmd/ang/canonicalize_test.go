package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteCueActionAliases(t *testing.T) {
	t.Parallel()
	src := `package api

flow: [{
	action: "notify.Dispatch"
}, {
	action: "idem.DeriveKey"
}, {
	action: "idem.Check"
}, {
	action: "idem.SaveResult"
}]
`
	out, replaced := rewriteCueActionAliases(src, aliasRuleMap())
	if replaced != 4 {
		t.Fatalf("expected 4 replacements, got %d", replaced)
	}
	if out == src {
		t.Fatalf("expected changed output")
	}
	if strings.Contains(out, `"notify.Dispatch"`) ||
		strings.Contains(out, `"idem.DeriveKey"`) ||
		strings.Contains(out, `"idem.Check"`) ||
		strings.Contains(out, `"idem.SaveResult"`) {
		t.Fatalf("expected aliases to be rewritten, got:\n%s", out)
	}
}

func TestCanonicalizeCueAliases_CheckOnly(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	file := filepath.Join(tmp, "a.cue")
	src := "package x\n\na: { action: \"notify.Dispatch\" }\n"
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	res, err := canonicalizeCueAliases(tmp, true)
	if err != nil {
		t.Fatalf("canonicalize check: %v", err)
	}
	if res.FilesChanged != 1 || res.Replacements != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(raw) != src {
		t.Fatalf("check mode must not modify files")
	}
}
