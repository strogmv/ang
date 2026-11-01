package gamepoc

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/strogmv/ang/compiler/behavior"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/inventory"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"
)

// Recompiles actual CUE into a filesystem overlay. Generated project and running
// Editor files are never modified. Includes both kernels and the current host.
func TestMarketSupplyActualHostOverlay(t *testing.T) {
	book := os.Getenv("ANG_MARKET_SUPPLY_PROJECT")
	if book == "" {
		t.Skip("set ANG_MARKET_SUPPLY_PROJECT to book checkout")
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
				t.Fatal(target, err)
			}
		}
		name := filepath.Join(dir, strings.ReplaceAll(target, "/", "_"))
		if err = os.WriteFile(name, body, 0600); err != nil {
			t.Fatal(err)
		}
		replace[target] = name
	}
	name, data, err := emitter.GeneratePBGo(def, "angelsdemons/server")
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/transport/grpc/pb/game/v1", name), data)
	data, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/poc/world.json"), data)
	b, err := behavior.Render(c.AI, nil)
	if err != nil {
		t.Fatal(err)
	}
	for n, d := range b {
		write(filepath.Join(project, "internal/behavior", n), d)
	}
	inv, err := inventory.Render()
	if err != nil {
		t.Fatal(err)
	}
	for n, d := range inv {
		write(filepath.Join(project, "internal/inventory", n), d)
	}
	entries, err := sources.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		n := entry.Name()
		if n == "main.go.tmpl" {
			continue
		}
		if !strings.HasSuffix(n, ".go.tmpl") || strings.HasSuffix(n, "_test.go.tmpl") {
			continue
		}
		d, e := sources.ReadFile("templates/" + n)
		if e != nil {
			t.Fatal(e)
		}
		tmpl, e := template.New(n).Delims("<<ANG", "ANG>>").Parse(string(d))
		if e != nil {
			t.Fatal(e)
		}
		var buf bytes.Buffer
		if e = tmpl.Execute(&buf, c); e != nil {
			t.Fatal(e)
		}
		write(filepath.Join(project, "internal/poc", strings.TrimSuffix(n, ".go.tmpl")+".gen.go"), buf.Bytes())
	}
	for n, target := range map[string]string{"supply_test.go.tmpl": "supply_generated_test.go", "life_test.go.tmpl": "life_generated_test.go"} {
		d, e := sources.ReadFile("templates/" + n)
		if e != nil {
			t.Fatal(e)
		}
		write(filepath.Join(project, "internal/poc", target), d)
	}
	data, _ = json.Marshal(map[string]any{"Replace": replace})
	overlay := filepath.Join(dir, "overlay.json")
	if err = os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "./internal/poc", "-run", "TestMarketDeliveriesActualNightLife100|TestStorefrontProjectionHidesClosedStockAndOffscreenShops|TestLifeResidencePrivateProjectionPauseAndSave", "-count=1", "-v")
	cmd.Dir = project
	out, e := cmd.CombinedOutput()
	t.Log(string(out))
	if e != nil {
		t.Fatal(e)
	}
}
