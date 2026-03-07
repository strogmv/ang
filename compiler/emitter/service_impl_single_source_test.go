package emitter

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func repoRootFromCaller(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// compiler/emitter/service_impl_single_source_test.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func TestServiceImplTemplateSingleSource(t *testing.T) {
	if serviceImplTemplatePath != "templates/service_impl.tmpl" {
		t.Fatalf("unexpected service impl template path: %q", serviceImplTemplatePath)
	}
	if serviceImplScaffoldTemplatePath != "templates/service_impl_scaffold.tmpl" {
		t.Fatalf("unexpected service impl scaffold template path: %q", serviceImplScaffoldTemplatePath)
	}
	if serviceImplMethodTemplatePath != "templates/service_impl_method.tmpl" {
		t.Fatalf("unexpected service impl method template path: %q", serviceImplMethodTemplatePath)
	}

	root := repoRootFromCaller(t)
	primary := filepath.Join(root, serviceImplTemplatePath)
	if _, err := os.Stat(primary); err != nil {
		t.Fatalf("primary service_impl template missing: %s (%v)", primary, err)
	}
	scaffold := filepath.Join(root, serviceImplScaffoldTemplatePath)
	if _, err := os.Stat(scaffold); err != nil {
		t.Fatalf("service_impl_scaffold template missing: %s (%v)", scaffold, err)
	}
	method := filepath.Join(root, serviceImplMethodTemplatePath)
	if _, err := os.Stat(method); err != nil {
		t.Fatalf("service_impl_method template missing: %s (%v)", method, err)
	}

	alternatives := []string{
		filepath.Join(root, "templates", "go", "service_impl.tmpl"),
		filepath.Join(root, "templates", "python", "service_impl.tmpl"),
	}
	for _, alt := range alternatives {
		if _, err := os.Stat(alt); err == nil {
			t.Fatalf("duplicate service_impl template found: %s", alt)
		}
	}
}
