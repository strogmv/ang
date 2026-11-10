package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Compile and execute the exact generated state machine, independently of
// the production world. Geometry/nav integration is exercised by server tests.
func TestLiftingDeckRuntimeInterlocks(t *testing.T) {
	raw, err := os.ReadFile("templates/lifting_decks.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	first := source[strings.Index(source, "type liftingDeckCatalog"):strings.Index(source, "func(d liftingDeckDefinition)contains")]
	first = strings.ReplaceAll(first, "behavior.GateFull", "int64(1000000)")
	machine := source[strings.Index(source, "func(g *liftingDeckState)advance"):strings.Index(source, "func(r *Runtime)advanceLiftingDecks")]
	walk := source[strings.Index(source, "func(g liftingDeckState)walkable"):strings.Index(source, "func(r *Runtime)liftingDeckLifeModel")]
	testsRaw, err := os.ReadFile("templates/lifting_decks_test.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tests := string(testsRaw)
	tests = tests[strings.Index(tests, "func liftingDeckFixture"):strings.Index(tests, "func TestLiftingDeckPermitRefusal")]
	dir := t.TempDir()
	for name, data := range map[string]string{"go.mod": "module liftingfixture\n\ngo 1.23\n", "machine.go": "package poc\nimport \"math\"\n" + first + machine + walk, "machine_test.go": "package poc\nimport(\"encoding/json\";\"math\";\"reflect\";\"testing\")\n" + tests} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exact lifting-deck runtime: %v\n%s", err, output)
	}
	t.Log(string(output))
}
