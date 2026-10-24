package gamepoc

import (
	"context"
	"encoding/json"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strogmv/ang/compiler/behavior"
	"github.com/strogmv/ang/compiler/emitter"
)

// Overlay the source templates onto the generated host in a scratch build. No
// generated project file is written; the real book population and inventory are
// used for the elapsed-night replay/performance regression.
func TestInstantTravelActualHostAndLife100ReplayNative(t *testing.T) {
	book := os.Getenv("ANG_C7_TRANSIT_PROJECT")
	if book == "" {
		t.Skip("set ANG_C7_TRANSIT_PROJECT to book checkout")
	}
	project := filepath.Join(book, "server")
	c, def, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	replace := map[string]string{}
	write := func(target string, body []byte) {
		t.Helper()
		if strings.HasSuffix(target, ".go") {
			body, err = format.Source(body)
			if err != nil {
				t.Fatalf("format %s: %v", target, err)
			}
		}
		name := filepath.Join(dir, strings.ReplaceAll(filepath.ToSlash(target), "/", "_"))
		if err := os.WriteFile(name, body, 0600); err != nil {
			t.Fatal(err)
		}
		replace[target] = name
	}
	pbName, pb, err := emitter.GeneratePBGo(def, "angelsdemons/server")
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/transport/grpc/pb/game/v1", pbName), pb)
	emitted, err := behavior.Render(c.AI, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range emitted {
		write(filepath.Join(project, "internal/behavior", name), body)
	}
	names := map[string]string{"runtime.go.tmpl": "runtime.gen.go", "life.go.tmpl": "life.gen.go", "transit_fare.go.tmpl": "transit_fare.gen.go", "instant_travel.go.tmpl": "instant_travel.gen.go", "instant_travel_test.go.tmpl": "instant_travel_test.go", "life_test.go.tmpl": "life_generated_test.go", "transit_fare_test.go.tmpl": "transit_fare_test.go"}
	names["population_test.go.tmpl"] = "population_generated_test.go"
	names["region_test.go.tmpl"] = "region_generated_test.go"
	for name, target := range names {
		data, err := os.ReadFile(filepath.Join("templates", name))
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(project, "internal/poc", target), data)
	}
	data, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "./internal/poc", "-count=1", "-v")
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("actual generated host overlay: %v", err)
	}
}
