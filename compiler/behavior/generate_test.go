package behavior

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestRenderIndependentBehaviorPackage(t *testing.T) {
	catalog := []byte(`{"actions":{},"tasks":{}}`)
	files, err := Render(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 20 {
		t.Fatalf("unexpected output count: %d", len(files))
	}
	for _, name := range []string{"courtesy.gen.go", "population.gen.go", "practice.gen.go", "development_life.gen.go", "inspection.gen.go", "maintenance.gen.go", "life.gen.go", "life_travel.gen.go", "approach.gen.go", "memory.gen.go", "utility.gen.go", "gestures.gen.go"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
	source := string(files["planner.gen.go"])
	ast, err := parser.ParseFile(token.NewFileSet(), "planner.gen.go", source, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	if ast.Name.Name != "behavior" {
		t.Fatal("wrong package")
	}
	for _, dependency := range []string{"angelsdemons/", "Unity", "grpc", "Poc", "Runtime", "task.deliver_medblock"} {
		if strings.Contains(source, dependency) {
			t.Fatalf("planner depends on game/transport: %s", dependency)
		}
	}
	withTests, err := Render(catalog, []byte(`{"cases":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(withTests) != 39 || string(withTests["catalog.json"]) != string(catalog) || string(withTests["utility_scenarios.json"]) != `{"law":{},"cases":[]}` {
		t.Fatalf("standalone fixtures missing: %d", len(withTests))
	}
	for _, name := range []string{"utility.gen.go", "memory.gen.go", "life_travel.gen.go", "utility_generated_test.go", "gestures.gen.go", "gestures_generated_test.go"} {
		if strings.Contains(string(withTests[name]), "angelsdemons/") || strings.Contains(string(withTests[name]), "Poc") {
			t.Fatalf("%s depends on the game host", name)
		}
	}
	if _, err := Render([]byte(`{"tasks":{"bad":{"id":"bad","methods":[{"id":"loop","steps":["bad"]}]}}}`), nil); err == nil {
		t.Fatal("accepted recursive catalog")
	}
}
