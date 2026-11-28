package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingFixtureCannotBecomeAnUnlistedSkip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "missing_test.go")
	source := "package fixture\nimport (\"os\";\"testing\")\nfunc TestFixture(t *testing.T) { if _,err:=os.ReadFile(\"missing.json\");err!=nil { t.Skipf(\"fixture missing: %v\",err) } }\n"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	rows, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(validate(rows, nil)) != 1 {
		t.Fatalf("missing fixture skip escaped lint: %v", rows)
	}
	bad := []allowance{{rows[0].Key, "missing-input", "missing fixture is not acceptable"}}
	if len(validate(rows, bad)) != 1 {
		t.Fatal("missing-input allowance accepted")
	}
	good := []allowance{{rows[0].Key, "tool", "separate test of allowlist mechanics"}}
	if len(validate(rows, good)) != 0 {
		t.Fatal("explicitly allowed skip rejected")
	}
	if len(validate(nil, good)) != 1 {
		t.Fatal("stale allowance accepted")
	}
}
