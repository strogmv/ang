package emitter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/strogmv/ang/angir/ir"
)

func TestFrontendSDKSkipValidatesNames(t *testing.T) {
	em := &Emitter{FrontendSDKSkip: []string{"mocks", "format"}}
	if err := em.ValidateFrontendSDKSkip(); err != nil {
		t.Fatal(err)
	}
	if em.sdkModuleEnabled("mocks") || !em.sdkModuleEnabled("routes") {
		t.Fatal("sdkModuleEnabled does not follow the skip list")
	}
	em.FrontendSDKSkip = []string{"mock"}
	if err := em.ValidateFrontendSDKSkip(); err == nil || !strings.Contains(err.Error(), "mocks") {
		t.Fatalf("a typo must fail and list the modules, got %v", err)
	}
	if sdkModuleOfTemplate("msw-server") != "mocks" || sdkModuleOfTemplate("hooks") != "" {
		t.Fatal("template to module mapping is wrong")
	}
}

func TestIndexTemplateOmitsSkippedModules(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "templates", "frontend", "index.ts.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	render := func(skip []string) string {
		em := &Emitter{FrontendSDKSkip: skip}
		tmpl := template.Must(template.New("index").Funcs(template.FuncMap{"SDKModuleEnabled": em.sdkModuleEnabled}).Parse(string(src)))
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, nil); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if all := render(nil); !strings.Contains(all, "'./routes'") || !strings.Contains(all, "'./prefetch'") {
		t.Fatalf("default index must export routes and prefetch:\n%s", all)
	}
	if skipped := render([]string{"routes", "prefetch"}); strings.Contains(skipped, "'./routes'") || strings.Contains(skipped, "'./prefetch'") || !strings.Contains(skipped, "'./hooks'") {
		t.Fatalf("skipped modules must not be exported:\n%s", skipped)
	}
}

func TestConventionsDocListsSDKModuleStatus(t *testing.T) {
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.FrontendSDKSkip = []string{"mocks"}
	if err := em.EmitConventionsDoc(&ir.Schema{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "ang", "conventions.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Frontend SDK modules", "| `mocks` | skipped |", "| `routes` | generated |", "frontend_sdk_skip", "## Generated files", "ang-generated.txt"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("conventions.md lacks %q", want)
		}
	}
}
