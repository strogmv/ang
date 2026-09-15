package emitter

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/imports"
)

// The fast path (own import fixing, then goimports without fixing) must give
// the bytes full goimports gives.
func TestFormatGoStrictFastPathMatchesGoimports(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	unit := "internal/service/x.gen.go"
	if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		previous string
		src      string
	}{
		"unused import removed":      {src: "package service\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc F() string { return strings.TrimSpace(\" x \") }\n"},
		"blank and dot imports kept": {src: "package service\n\nimport (\n\t_ \"embed\"\n\t. \"math\"\n\t\"os\"\n)\n\nvar X = Pi\n"},
		"missing import from previous output": {
			previous: "package service\n\nimport (\n\t\"context\"\n\t\"strconv\"\n)\n",
			src:      "package service\n\nimport \"context\"\n\nfunc F(ctx context.Context) string { return strconv.Itoa(1) }\n",
		},
		"nothing to fix": {src: "package service\n\nimport \"strings\"\n\nfunc F() string { return strings.ToUpper(\"x\") }\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_ = os.Remove(unit)
			if c.previous != "" {
				if err := os.WriteFile(unit, []byte(c.previous), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			fixed, ok := withPreviousImports([]byte(c.src), unit)
			if !ok {
				t.Fatal("fast path must apply")
			}
			fast, err := imports.Process(unit, fixed, formatOnlyOptions)
			if err != nil {
				t.Fatal(err)
			}
			full, err := imports.Process(unit, []byte(c.src), nil)
			if err != nil {
				t.Fatal(err)
			}
			if string(fast) != string(full) {
				t.Fatalf("fast:\n%s\nfull goimports:\n%s", fast, full)
			}
		})
	}
}
