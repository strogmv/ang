package gamepoc

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCartParkingAuthoredGeometryAndStrictValidation(t *testing.T) {
	project := os.Getenv("ANG_CART_PARKING_PROJECT")
	if project == "" {
		t.Skip("set staged server path")
	}
	w, _, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("ANG_CART_COMPILED_OUT"); out != "" {
		raw, err := json.MarshalIndent(w, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(out, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := w.CartParking
	if c == nil {
		t.Fatal("missing persistent cart catalog")
	}
	if err = c.validate(w); err != nil {
		t.Fatal(err)
	}
	if len(c.Bays) != 2 || c.Bays[0].Handle != (Point{81, 66}) || c.Bays[1].Handle != (Point{13, 54}) {
		t.Fatal("accepted physical handle cells changed")
	}
	// The collision envelope must contain the measured exported mesh, not only
	// its nominal design passport. A rear grip is part of the swept footprint.
	meshRaw, err := os.ReadFile(filepath.Join(project, "..", "art-source", "peddler-cart", "v2", "export-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mesh struct {
		Bounds [][3]float64          `json:"actual_envelope_unity_m"`
		Grips  map[string][3]float64 `json:"actual_grips_unity_m"`
	}
	if err := json.Unmarshal(meshRaw, &mesh); err != nil {
		t.Fatal(err)
	}
	if len(mesh.Bounds) != 2 {
		t.Fatal("missing measured cart export bounds")
	}
	if mesh.Bounds[0][0] < -c.HalfWidthM || mesh.Bounds[1][0] > c.HalfWidthM || mesh.Bounds[0][2] < -c.BackM || mesh.Bounds[1][2] > c.FrontM {
		t.Fatalf("server sweep envelope excludes exported mesh: bounds=%v half_width=%g front=%g back=%g", mesh.Bounds, c.HalfWidthM, c.FrontM, c.BackM)
	}
	// Measured exporter coordinates are already converted to Unity metres.
	// Reach correction changes vertical presentation only; coupling stays .75m.
	if c.GripYM != 1.02 {
		t.Fatalf("approved rig-reachable grip height: got %g", c.GripYM)
	}
	for name, wantX := range map[string]float64{"grip_l": -.22, "grip_r": .22} {
		grip, ok := mesh.Grips[name]
		if !ok || math.Abs(grip[0]-wantX) > 1e-6 || math.Abs(grip[1]-c.GripYM) > 1e-6 || math.Abs(grip[2]+.48) > 1e-6 {
			t.Fatalf("exported %s does not match CUE/physical coupling: %v, height=%g", name, grip, c.GripYM)
		}
	}
	raw, _ := json.Marshal(c)
	for name, change := range map[string]func(*CartParkingCatalog){"unsafe prefab": func(x *CartParkingCatalog) { x.PrefabResource = "../cart" }, "nonfinite": func(x *CartParkingCatalog) { x.Bays[0].Pose.X = math.NaN() }, "misaligned grip": func(x *CartParkingCatalog) { x.Bays[0].Pose.Z += .25 }, "different inventory": func(x *CartParkingCatalog) { x.StoreID = "fake/store" }, "walk-through dock": func(x *CartParkingCatalog) { x.Bays[0].InstanceID = "wall.sanitary" }, "unbounded wait": func(x *CartParkingCatalog) { x.ApproachTimeoutSeconds = 0 }} {
		t.Run(name, func(t *testing.T) {
			var v CartParkingCatalog
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			change(&v)
			if v.validate(w) == nil {
				t.Fatal("invalid cart authority accepted")
			}
		})
	}
}
