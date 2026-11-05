package gamepoc

import (
	"context"
	"encoding/json"
	"go/format"
	"math"
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
func TestMedicalCapsulesActualHost(t *testing.T) {
	book := os.Getenv("ANG_MEDICAL_PROJECT")
	if book == "" {
		t.Skip("set ANG_MEDICAL_PROJECT to book checkout")
	}
	project := filepath.Join(book, "server")
	c, def, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	if c.MedicalCapsules == nil {
		t.Fatal("missing medical CUE")
	}
	for name, mutate := range map[string]func(*MedicalCapsuleCatalog){"intake replacement": func(m *MedicalCapsuleCatalog) { m.Units[0].InstanceID = "wall.med.1" }, "duplicate anchor": func(m *MedicalCapsuleCatalog) { m.Units[1] = m.Units[0] }, "nonfinite": func(m *MedicalCapsuleCatalog) { m.Units[0].InsidePose.X = math.Inf(1) }, "unbacked account": func(m *MedicalCapsuleCatalog) { m.ReceiverAccount = "absent" }, "scan in wall": func(m *MedicalCapsuleCatalog) { m.Units[0].ScanCell = Point{X: 3, Y: 65} }, "unsafe old scan": func(m *MedicalCapsuleCatalog) { m.Units[0].ScanPose.X = 4.25 }} {
		m := *c.MedicalCapsules
		m.Units = append([]MedicalUnit(nil), m.Units...)
		mutate(&m)
		if m.validate(c) == nil {
			t.Fatalf("accepted invalid %s", name)
		}
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
	currentWorld, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/poc/world.json"), currentWorld)
	emitted, err := behavior.Render(c.AI, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range emitted {
		write(filepath.Join(project, "internal/behavior", name), body)
	}
	behaviorTest, err := os.ReadFile(filepath.Join("..", "behavior", "templates", "life_residence_test.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/behavior/life_residence_generated_test.go"), behaviorTest)
	names := map[string]string{"runtime_test.go.tmpl": "runtime_generated_test.go", "sanitary_services.go.tmpl": "sanitary_services.gen.go", "sanitary_services_view.go.tmpl": "sanitary_services_view.gen.go", "sanitary_services_test.go.tmpl": "sanitary_services_generated_test.go", "life_economy.go.tmpl": "life_economy.gen.go", "runtime.go.tmpl": "runtime.gen.go", "life.go.tmpl": "life.gen.go", "capture.go.tmpl": "capture.gen.go", "combat.go.tmpl": "combat.gen.go", "combat_care.go.tmpl": "combat_care.gen.go", "combat_wounds.go.tmpl": "combat_wounds.gen.go", "medical_capsules.go.tmpl": "medical_capsules.gen.go", "medical_capsules_view.go.tmpl": "medical_capsules_view.gen.go", "medical_capsules_test.go.tmpl": "medical_capsules_generated_test.go", "train_motion.go.tmpl": "train_motion.gen.go"}
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
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "./internal/poc", "./internal/behavior", "-count=1", "-run", "^(TestMedical|TestSanitary|TestTrainMotion|TestClock|TestSaveRestoresInFlightRoute|TestLifeServiceAttachment)", "-v")
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("actual generated host overlay: %v", err)
	}
	vet := exec.CommandContext(ctx, "go", "vet", "-overlay", overlay, "./internal/poc")
	vet.Dir = project
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("actual generated host vet: %v\n%s", err, out)
	}
}
