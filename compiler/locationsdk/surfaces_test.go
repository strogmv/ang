package locationsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func surfaceTestCatalog() Catalog {
	c := testCatalog()
	c.Surfaces = []Surface{
		{ID: "surface.warm", TopImage: "LocationSDK/Surfaces/panel_top", SideImage: "LocationSDK/Surfaces/panel_side", TopRepeatMM: SurfaceRepeatMM{X: 750, Y: 1250}, SideRepeatMM: SurfaceRepeatMM{X: 2000, Y: 1500}},
		{ID: "surface.cool", TopImage: "LocationSDK/Surfaces/floor_top", SideImage: "LocationSDK/Surfaces/floor_side", TopRepeatMM: SurfaceRepeatMM{X: 1000, Y: 1000}, SideRepeatMM: SurfaceRepeatMM{X: 1000, Y: 500}},
	}
	c.DefaultFloorSurface = "surface.cool"
	for i := range c.Modules {
		for j := range c.Modules[i].Variants {
			c.Modules[i].Variants[j].SurfaceID = c.Surfaces[j%len(c.Surfaces)].ID
		}
	}
	return c
}

func TestSurfacesPreserveAuthoritativeGeometryAcrossLayoutsAndRotations(t *testing.T) {
	plain, textured := testCatalog(), surfaceTestCatalog()
	for _, source := range testLayouts() {
		for _, rotation := range []int{0, 90, 180, 270} {
			// The central calibration bay has sufficient front/rear space at every
			// rotation; compare all emitted structural records, not only a hash.
			layout := testLayouts()[0]
			layout.ID = source.ID
			layout.Instances = []Instance{{ID: "material.calibration", ModuleID: "module.shape.box", At: Cell{X: 6, Y: 4}, Rotation: rotation, Reason: "same physical module with an interchangeable surface"}}
			layout.ReservedPaths = nil
			a, b := requireBuilt(t, plain, layout), requireBuilt(t, textured, layout)
			if b.Instances[0].SurfaceID == "" || a.Instances[0].SurfaceID != "" {
				t.Fatal("selected visual variant lost its material reference")
			}
			b.Instances[0].SurfaceID = ""
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("surface changed physical layout or state geometry: %s / %d", source.ID, rotation)
			}
		}
	}
	for _, layout := range testLayouts() {
		a, b := requireBuilt(t, plain, layout), requireBuilt(t, textured, layout)
		for i := range b.Instances {
			b.Instances[i].SurfaceID = ""
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("surface changed shared-kit layout %s", layout.ID)
		}
	}
}

func TestRuntimeSurfaceContractCanonicalMutableSafeAndAppearanceSensitive(t *testing.T) {
	in := Input{Catalog: surfaceTestCatalog(), Layouts: testLayouts()}
	before := copyValue(t, in)
	textured := requireRuntime(t, in, runtimeTestOptions())
	if !reflect.DeepEqual(before, in) || textured.DefaultFloorSurface != "surface.cool" || len(textured.Surfaces) != 2 || textured.Surfaces[0].ID != "surface.cool" {
		t.Fatal("material catalog mutated its source or lost canonical references")
	}
	plain := requireRuntime(t, runtimeTestInput(), runtimeTestOptions())
	if plain.ContentHash == textured.ContentHash {
		t.Fatal("render content hash ignored surface definitions")
	}
	for i, level := range textured.Levels {
		if level.StructuralHash != plain.Levels[i].StructuralHash || !reflect.DeepEqual(level.Terrain, plain.Levels[i].Terrain) {
			t.Fatal("texture changed collision/navigation identity")
		}
	}
	shuffled := copyValue(t, in)
	slices.Reverse(shuffled.Catalog.Surfaces)
	if other := requireRuntime(t, shuffled, runtimeTestOptions()); !reflect.DeepEqual(textured, other) {
		t.Fatal("surface definition order changed canonical runtime output")
	}
	changed := copyValue(t, in)
	changed.Catalog.Surfaces[0].TopRepeatMM.X++
	other := requireRuntime(t, changed, runtimeTestOptions())
	if other.ContentHash == textured.ContentHash || other.Levels[0].StructuralHash != textured.Levels[0].StructuralHash {
		t.Fatal("metric repeat is visual content, not structural identity")
	}
	encoded, err := json.Marshal(textured)
	if err != nil {
		t.Fatal(err)
	}
	var restored RuntimeBundle
	if err := json.Unmarshal(encoded, &restored); err != nil || !reflect.DeepEqual(restored, textured) {
		t.Fatal("flat Unity material contract failed JSON round trip", err)
	}
	textured.Surfaces[0].TopImage = "changed"
	textured.Surfaces[1].TopRepeatMM.X = 99
	if !reflect.DeepEqual(before, in) {
		t.Fatal("caller-owned runtime materials alias authoring input")
	}
	in.Catalog.Surfaces[0].SideImage = "changed"
	if restored.Surfaces[1].SideImage == "changed" {
		t.Fatal("authoring edits changed returned material definitions")
	}
}

func TestSurfaceValidationHasBoundedResourcesMetricRepeatAndReferences(t *testing.T) {
	tests := []struct {
		name, code, path string
		edit             func(*Catalog)
	}{
		{"duplicate", "duplicate_id", "catalog.surfaces[1].id", func(c *Catalog) { c.Surfaces[1].ID = c.Surfaces[0].ID }},
		{"identity", "invalid_id", "catalog.surfaces[0].id", func(c *Catalog) { c.Surfaces[0].ID = "" }},
		{"missing top", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "" }},
		{"missing side", "surface_resource", "catalog.surfaces[0].side_image", func(c *Catalog) { c.Surfaces[0].SideImage = "" }},
		{"traversal", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "Materials/../private" }},
		{"dot", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "Materials/./panel" }},
		{"absolute", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "/Materials/panel" }},
		{"url", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "https://example.org/panel" }},
		{"windows", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = `C:\\Materials\\panel` }},
		{"empty segment", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "Materials//panel" }},
		{"trailing slash", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "Materials/panel/" }},
		{"file extension", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage += ".png" }},
		{"unicode", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "Materials/панель" }},
		{"control", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = "Materials/panel\n" }},
		{"path length", "surface_resource", "catalog.surfaces[0].top_image", func(c *Catalog) { c.Surfaces[0].TopImage = strings.Repeat("a", 257) }},
		{"zero top repeat", "surface_repeat", "catalog.surfaces[0].top_repeat_mm", func(c *Catalog) { c.Surfaces[0].TopRepeatMM.X = 0 }},
		{"negative side repeat", "surface_repeat", "catalog.surfaces[0].side_repeat_mm", func(c *Catalog) { c.Surfaces[0].SideRepeatMM.Y = -1 }},
		{"overflow repeat", "surface_repeat", "catalog.surfaces[0].top_repeat_mm", func(c *Catalog) { c.Surfaces[0].TopRepeatMM.Y = math.MaxInt }},
		{"unknown floor", "unknown_surface", "catalog.default_floor_surface", func(c *Catalog) { c.DefaultFloorSurface = "missing" }},
		{"unknown unused variant", "unknown_surface", "catalog.modules[2].variants[1].surface_id", func(c *Catalog) { c.Modules[2].Variants[1].SurfaceID = "missing" }},
		{"silhouette still uncalibrated", "unsupported_unverified_asset", "catalog.modules[0].variants[0].image_ref", func(c *Catalog) { c.Modules[0].Variants[0].ImageRef = "LocationSDK/Surfaces/panel_top" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := Input{Catalog: surfaceTestCatalog(), Layouts: testLayouts()}
			tt.edit(&in.Catalog)
			out, err := CompileRuntime(in, runtimeTestOptions())
			var invalid *ValidationError
			if !errors.As(err, &invalid) || len(invalid.Diagnostics) != 1 || invalid.Diagnostics[0].Code != tt.code || invalid.Diagnostics[0].Path != tt.path {
				t.Fatalf("wrong diagnostic: %v", err)
			}
			if !reflect.DeepEqual(out, RuntimeBundle{}) {
				t.Fatal("invalid material emitted a partially usable runtime bundle")
			}
		})
	}
}

func TestSurfaceDefinitionBudgetAndExactResourceRepeatBounds(t *testing.T) {
	c := surfaceTestCatalog()
	c.Surfaces[0].TopImage = strings.Repeat("a", 256)
	c.Surfaces[0].TopRepeatMM = SurfaceRepeatMM{X: 1, Y: 1000000}
	for len(c.Surfaces) < SurfaceBudget {
		s := c.Surfaces[0]
		s.ID = fmt.Sprintf("surface.extra%d", len(c.Surfaces))
		c.Surfaces = append(c.Surfaces, s)
	}
	requireBuilt(t, c, testLayouts()[0])
	c.Surfaces = append(c.Surfaces, Surface{ID: "surface.extra"})
	_, err := Build(c, testLayouts()[0])
	var invalid *ValidationError
	if !errors.As(err, &invalid) || invalid.Diagnostics[0].Code != "surface_budget" || invalid.Diagnostics[0].Path != "catalog.surfaces" {
		t.Fatalf("surface definition budget was not enforced: %v", err)
	}
	plain := requireRuntime(t, runtimeTestInput(), runtimeTestOptions())
	b, err := json.Marshal(plain)
	if err != nil || strings.Contains(string(b), `"surfaces"`) || strings.Contains(string(b), `"default_floor_surface"`) || strings.Contains(string(b), `"surface_id"`) {
		t.Fatal("optional material contract changed geometry-only payload", err)
	}
}
