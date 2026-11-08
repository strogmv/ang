package gamepoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestGodotWorldTextUsesPlacementFrame(t *testing.T) {
	for _, rot := range []int{0, 90, 180, 270} {
		level := locationsdk.RuntimeLevel{UnitMM: 500, Elevation: 6, Instances: []locationsdk.CompiledInstance{{ID: "host", At: locationsdk.Cell{X: 4, Y: 8}, Width: 10, Depth: 6, Rotation: rot}}}
		s := WorldTextSlotDrawn{Instance: "host", Face: "south", CenterMM: locationsdk.PointMM{X: 1500, Y: 700, Z: 2200}}
		f, err := godotTextFrame(level, s)
		if err != nil {
			t.Fatal(err)
		}
		pose := placement.Pose(level.Instances[0].At, 10, 6, 3, rot)
		expected := transformTextVector(pose, placement.Vec3{1.5, 2.2, 0.7})
		for i := range 3 {
			expected[i] += pose.Position[i]
			if math.Abs(expected[i]-f.Center[i]) > 1e-12 {
				t.Fatalf("rot%d %+v != %+v", rot, f.Center, expected)
			}
		}
		// Orthogonal reading frame; south text reads local west, not mirrored.
		if f.Up != (placement.Vec3{0, 1, 0}) {
			t.Fatal(f)
		}
		wantRight := transformTextVector(pose, placement.Vec3{-1, 0, 0})
		if f.Right != wantRight {
			t.Fatal("mirrored text", f)
		}
	}
	if _, err := godotTextFrame(locationsdk.RuntimeLevel{}, WorldTextSlotDrawn{Instance: "absent"}); err == nil {
		t.Fatal("missing host accepted")
	}
	if _, err := godotTextFrame(locationsdk.RuntimeLevel{}, WorldTextSlotDrawn{Vista: "absent"}); err == nil {
		t.Fatal("missing vista accepted")
	}
}
func TestGodotWorldTextAtlasIntegrityAndDelivery(t *testing.T) {
	project, c := worldTextFixture(t)
	c.LocationSDK.Levels[0].Vistas[0] = locationsdk.Vista{ID: "vista.a", SignKey: "ui.corridor", Direction: locationsdk.North, Width: 6, LengthMM: 9000}
	c.WorldTextDrawn, _ = compileWorldText(project, c)
	c.GodotTargets = []string{"a", "b"}
	root := filepath.Join(project, c.WorldText.MetricsDir)
	m := worldTextAssetManifest{Schema: "ang.world_text.assets.v1"}
	for _, f := range c.WorldText.Fonts {
		if err := os.WriteFile(filepath.Join(root, f.Style+".sdf.bytes"), []byte("atlas-"+f.Style), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for path, raw := range map[string]string{"fonts-manifest.json": `{"fonts":[{"license_file":"OFL.txt"},{"license_file":"OFL.txt"}]}`, "OFL.txt": "SIL OFL 1.1"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(root, e.Name()))
		sum := sha256.Sum256(b)
		m.Files = append(m.Files, worldTextAsset{e.Name(), hex.EncodeToString(sum[:])})
	}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(root, "assets-manifest.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeGodotWorldText(project, c); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runtime.json", "stencil.json", "stencil.sdf.bytes", "OFL.txt", "world_text.gd", "world_text.gdshader"} {
		a, e := os.ReadFile(filepath.Join(project, "a/generated/world_text", name))
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(filepath.Join(project, "b/generated/world_text", name))
		if e != nil || !bytes.Equal(a, b) {
			t.Fatalf("delivery differs %s", name)
		}
	}
	before, _ := os.ReadFile(filepath.Join(project, "a/generated/world_text/runtime.json"))
	if !bytes.Contains(before, []byte(`"backing"`)) {
		t.Fatal("vista backing omitted")
	}
	if err := writeGodotWorldText(project, c); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(project, "a/generated/world_text/runtime.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("nondeterministic")
	}
	if err := os.WriteFile(filepath.Join(root, "stencil.sdf.bytes"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeGodotWorldText(project, c); err == nil {
		t.Fatal("changed atlas accepted")
	}
}
func TestGodotWorldTextCompiledWidths(t *testing.T) {
	root, c := worldTextFixture(t)
	out, err := compileWorldText(root, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range out.Slots {
		for _, loc := range slot.Texts {
			if len(loc.WidthsMM) != len(loc.Lines) {
				t.Fatal("missing per-line widths")
			}
			for _, w := range loc.WidthsMM {
				if w <= 0 {
					t.Fatal("invalid width")
				}
			}
		}
	}
}
