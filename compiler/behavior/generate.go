package behavior

import (
	"embed"
	"fmt"
	"go/format"
)

//go:embed templates/*
var sources embed.FS

// Fixtures are concrete JSON exported from CUE for generated tests only.
// Planning drives the HTN scenarios, Utility the personal-goal scenarios
// (each case already carries the psyche derived by CUE).
type Fixtures struct{ Planning, Utility []byte }

// Render returns files relative to a standalone Go package named behavior.
// Catalogs and optional model scenarios are concrete JSON exported from CUE.
// This compiler owns no transport, Unity, actor movement, or host event loop.
// JSON fixture files are embedded only by generated tests.
func Render(catalog, scenarios []byte) (map[string][]byte, error) {
	return RenderWith(catalog, Fixtures{Planning: scenarios})
}

// RenderWith is Render with every fixture kind; tests are emitted only when the
// planning fixture is present, the utility fixture defaults to an empty case list.
func RenderWith(catalog []byte, fixtures Fixtures) (map[string][]byte, error) {
	if err := ValidateCatalog(catalog); err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, name := range []string{"LICENSE.go-behaviortree", "NOTICE.go-behaviortree"} {
		data, err := sources.ReadFile("templates/" + name)
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	templates := map[string]string{"planner.go.tmpl": "planner.gen.go", "reactions.go.tmpl": "reactions.gen.go", "progress.go.tmpl": "progress.gen.go", "memory.go.tmpl": "memory.gen.go", "utility.go.tmpl": "utility.gen.go", "perception.go.tmpl": "perception.gen.go"}
	if len(fixtures.Planning) > 0 {
		templates["planner_test.go.tmpl"] = "planner_generated_test.go"
		templates["reactions_test.go.tmpl"] = "reactions_generated_test.go"
		templates["progress_test.go.tmpl"] = "progress_generated_test.go"
		templates["memory_test.go.tmpl"] = "memory_generated_test.go"
		templates["utility_test.go.tmpl"] = "utility_generated_test.go"
		templates["perception_test.go.tmpl"] = "perception_generated_test.go"
		files["catalog.json"] = catalog
		files["planning_scenarios.json"] = fixtures.Planning
		utility := fixtures.Utility
		if len(utility) == 0 {
			utility = []byte(`{"law":{},"cases":[]}`)
		}
		files["utility_scenarios.json"] = utility
	}
	for name, dest := range templates {
		source, err := sources.ReadFile("templates/" + name)
		if err != nil {
			return nil, err
		}
		data, err := format.Source(source)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		files[dest] = data
	}
	return files, nil
}
