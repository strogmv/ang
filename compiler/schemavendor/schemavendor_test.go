package schemavendor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newProject(t *testing.T, module string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cue.mod", "module.cue"), "module: \""+module+"\"\nlanguage: version: \"v0.9.0\"\n")
	return root
}

// evalPackage loads one package of the project with the full schema (no
// stubs, unlike ANG's build loader): what cue vet and editors see.
func evalPackage(t *testing.T, root, dir string) cue.Value {
	t.Helper()
	bis := load.Instances([]string{"."}, &load.Config{Dir: filepath.Join(root, dir), ModuleRoot: root})
	if len(bis) != 1 || bis[0].Err != nil {
		t.Fatalf("load %s: %v", dir, bis[0].Err)
	}
	return cuecontext.New().BuildInstance(bis[0])
}

// The schema is vendored into a project whose module is not ANG's. It used to
// import github.com/strogmv/ang/cue/project, a package only ANG's repository
// had, so a copy evaluated only after rewriting that import to the project's.
func TestVendoredSchemaEvaluatesInAProject(t *testing.T) {
	root := newProject(t, "example.com/shop")
	writeFile(t, filepath.Join(root, "cue", "api", "orders.cue"), `package api

import "github.com/strogmv/ang/cue/schema"

GetOrder: schema.#Operation & {
	service: "orders"
	input: {id: string}
	output: {ok: bool}
	transactional: true
	flow: [
		{action: "repo.Exists", source: "Order", input: "req.ID", output: "found"},
		{action: "list.Find", from: "items", condition: "item.ID == req.ID", output: "hit"},
		{action: "mapping.Assign", to: "resp.Ok", value: "found"},
	]
}

Helpers: schema.#Helpers & {
	orderTotal: {code: "func orderTotal() int { return 1 }"}
}
`)
	status, err := Sync(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Applies || len(status.Changes) == 0 {
		t.Fatalf("expected a fresh vendor directory, got %+v", status)
	}
	v := evalPackage(t, root, filepath.Join("cue", "api"))
	if err := v.Validate(); err != nil {
		t.Fatalf("the vendored schema must evaluate in a project: %v", err)
	}

	// The schema stays closed: a key ANG does not know is an error.
	writeFile(t, filepath.Join(root, "cue", "api", "bad.cue"), `package api

import "github.com/strogmv/ang/cue/schema"

Bad: schema.#Operation & {service: "orders", notAKey: true}
`)
	v = evalPackage(t, root, filepath.Join("cue", "api"))
	if err := v.Validate(); err == nil || !strings.Contains(err.Error(), "notAKey") {
		t.Fatalf("an unknown operation key must fail against the vendored schema, got %v", err)
	}
}

func TestPlanAndSyncKeepTheVendorDirectoryEqualToTheBinary(t *testing.T) {
	root := newProject(t, "example.com/shop")
	writeFile(t, filepath.Join(root, "cue", "api", "a.cue"), "package api\n\nimport \"github.com/strogmv/ang/cue/schema\"\n\nX: schema.#Operation\n")

	expected, err := Expected()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := expected["types.cue"]; !ok {
		t.Fatalf("the embedded schema has no types.cue: %v", expected)
	}
	plan, err := Plan(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applies || len(plan.Changes) != len(expected) {
		t.Fatalf("a project without the vendor directory needs every file created, got %+v", plan)
	}
	for _, change := range plan.Changes {
		if change.Action != "create" || !strings.HasPrefix(change.Path, "cue.mod/pkg/github.com/strogmv/ang/cue/schema/") {
			t.Fatalf("unexpected change %+v", change)
		}
	}
	if _, err := Sync(root, Options{}); err != nil {
		t.Fatal(err)
	}
	if plan, _ := Plan(root, Options{}); !plan.InSync() {
		t.Fatalf("after Sync nothing differs, got %+v", plan.Changes)
	}
	data, err := os.ReadFile(filepath.Join(Dir(root), "types.cue"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), Header) {
		t.Fatalf("vendored files start with the generated-code header, got %q", string(data[:80]))
	}

	// A hand edit and a stray file are both differences, and Sync undoes them.
	if err := os.WriteFile(filepath.Join(Dir(root), "types.cue"), append(data, []byte("\n#Mine: string\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(Dir(root), "extra.cue"), "package schema\n")
	plan, err = Plan(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, change := range plan.Changes {
		got[change.Path] = change.Action
	}
	want := map[string]string{
		"cue.mod/pkg/github.com/strogmv/ang/cue/schema/types.cue": "update",
		"cue.mod/pkg/github.com/strogmv/ang/cue/schema/extra.cue": "delete",
	}
	if len(got) != len(want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	for path, action := range want {
		if got[path] != action {
			t.Fatalf("changes = %v, want %v", got, want)
		}
	}
	if _, err := Sync(root, Options{}); err != nil {
		t.Fatal(err)
	}
	if plan, _ := Plan(root, Options{}); !plan.InSync() {
		t.Fatalf("Sync must undo edits, got %+v", plan.Changes)
	}
	if _, err := os.Stat(filepath.Join(Dir(root), "extra.cue")); !os.IsNotExist(err) {
		t.Fatalf("Sync must remove files ANG does not ship: %v", err)
	}
}

func TestWhenAProjectGetsAVendoredSchema(t *testing.T) {
	t.Run("ANG's own module imports the repository's schema", func(t *testing.T) {
		root := newProject(t, ModulePath)
		writeFile(t, filepath.Join(root, "examples", "x.cue"), "package x\n\nimport \"github.com/strogmv/ang/cue/schema\"\n")
		status, err := Plan(root, Options{Always: true})
		if err != nil || status.Applies {
			t.Fatalf("status = %+v, err = %v", status, err)
		}
	})
	t.Run("a project in a subfolder of ANG's module", func(t *testing.T) {
		root := newProject(t, ModulePath)
		blog := filepath.Join(root, "examples", "blog")
		writeFile(t, filepath.Join(blog, "cue", "api", "a.cue"), "package api\n\nimport \"github.com/strogmv/ang/cue/schema\"\n")
		status, err := Plan(blog, Options{Always: true})
		if err != nil || status.Applies || !strings.Contains(status.Reason, "ANG's module") {
			t.Fatalf("status = %+v, err = %v", status, err)
		}
	})
	t.Run("a project in a subfolder of its CUE module vendors at the module root", func(t *testing.T) {
		root := newProject(t, "example.com/mono")
		svc := filepath.Join(root, "services", "billing")
		writeFile(t, filepath.Join(svc, "cue", "api", "a.cue"), "package api\n\nimport \"github.com/strogmv/ang/cue/schema\"\n")
		status, err := Sync(svc, Options{})
		if err != nil || !status.Applies || status.Dir != "../../cue.mod/pkg/github.com/strogmv/ang/cue/schema" {
			t.Fatalf("status = %+v, err = %v", status, err)
		}
		if _, err := os.Stat(filepath.Join(root, "cue.mod", "pkg", "github.com", "strogmv", "ang", "cue", "schema", "types.cue")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("no cue.mod", func(t *testing.T) {
		status, err := Plan(t.TempDir(), Options{Always: true})
		if err != nil || status.Applies {
			t.Fatalf("status = %+v, err = %v", status, err)
		}
	})
	t.Run("a build leaves a project that does not import the schema alone", func(t *testing.T) {
		root := newProject(t, "example.com/plain")
		writeFile(t, filepath.Join(root, "cue", "domain", "user.cue"), "package domain\n\nUser: {id: string}\n")
		status, err := Plan(root, Options{})
		if err != nil || status.Applies {
			t.Fatalf("status = %+v, err = %v", status, err)
		}
		if status, _ := Plan(root, Options{Always: true}); !status.Applies {
			t.Fatalf("ang schema sync vendors on request: %+v", status)
		}
	})
	t.Run("an existing vendor directory is kept in sync", func(t *testing.T) {
		root := newProject(t, "example.com/plain")
		if err := os.MkdirAll(Dir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		status, err := Plan(root, Options{})
		if err != nil || !status.Applies || status.InSync() {
			t.Fatalf("status = %+v, err = %v", status, err)
		}
	})
	t.Run("custom cue root", func(t *testing.T) {
		root := newProject(t, "example.com/custom")
		writeFile(t, filepath.Join(root, "model", "api", "a.cue"), "package api\n\nimport \"github.com/strogmv/ang/cue/schema\"\n")
		if status, _ := Plan(root, Options{}); status.Applies {
			t.Fatalf("the default cue root has no import: %+v", status)
		}
		if status, _ := Plan(root, Options{CueRoot: "model"}); !status.Applies {
			t.Fatalf("the import under the configured cue root counts: %+v", status)
		}
	})
}

func TestLegacyCopyFindsAProjectCopyOfTheSchema(t *testing.T) {
	root := newProject(t, "example.com/old")
	if _, ok := LegacyCopy(root, "cue"); ok {
		t.Fatal("no cue/schema, no copy")
	}
	writeFile(t, filepath.Join(root, "cue", "schema", "notification_types.cue"), "package schema\n\n#NotificationType: \"a\" | \"b\"\n")
	if _, ok := LegacyCopy(root, "cue"); ok {
		t.Fatal("a project package named schema with its own definitions is not a copy of ANG's")
	}
	writeFile(t, filepath.Join(root, "cue", "schema", "types.cue"), "package schema\n\n#Operation: {}\n#FlowStep: {}\n")
	if dir, ok := LegacyCopy(root, "cue"); !ok || dir != "cue/schema" {
		t.Fatalf("LegacyCopy = %q, %v", dir, ok)
	}
}
