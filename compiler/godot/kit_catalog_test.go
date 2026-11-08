package godot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestKitCatalogKeepsPassportsPurposeAndActualBinding(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "asset.tscn"), []byte("[gd_scene format=3]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	input := locationsdk.Input{Catalog: locationsdk.Catalog{UnitMM: 500, Geometries: []locationsdk.Geometry{{ID: "geometry", Width: 2, Depth: 1, Height: 3, Footprint: []locationsdk.Cell{{X: 0, Y: 0}}, Blocked: []locationsdk.Cell{{X: 0, Y: 0}}, Ports: []locationsdk.Port{{ID: "use", Approach: []locationsdk.Cell{{X: 1, Y: 0}}}}}}, Modules: []locationsdk.Module{{ID: "kit.shape", GeometryID: "geometry"}}}, Layouts: []locationsdk.Layout{{ID: "layout", Instances: []locationsdk.Instance{{ID: "first", ModuleID: "kit.shape", Reason: "Actual source purpose"}}}}}
	catalog, err := BuildKitCatalog(input, []KitBinding{{Name: "binding", LayoutID: "layout", Placement: placement.Authoring{ModuleModels: map[string]string{"kit.shape": "art"}}, Models: map[string]string{"art": "res://asset.tscn"}, Locators: map[string][]string{"first": {"textslot.actual"}}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Modules) != 1 {
		t.Fatal(catalog)
	}
	row := catalog.Modules[0]
	if !row.Bound || row.SizeMM != ([3]int{1000, 500, 1500}) || len(row.Ports) != 1 || len(row.Ports[0].Approach) != 1 || row.Uses[0].Purpose != "Actual source purpose" || row.Uses[0].Locators[0] != "textslot.actual" {
		t.Fatal("catalog lost passport data", row)
	}
	if err := WriteKitCatalog(t.TempDir(), catalog); err != nil {
		t.Fatal(err)
	}
	catalog, err = BuildKitCatalog(input, []KitBinding{{Name: "binding", LayoutID: "layout", Placement: placement.Authoring{ModuleModels: map[string]string{"kit.shape": "art"}}, Models: map[string]string{"art": "res://missing.tscn"}}}, root)
	if err != nil || catalog.Modules[0].Bound || len(catalog.Missing) != 1 {
		t.Fatal("catalog invented available art", err, catalog)
	}
}

func TestKitCatalogDocumentsUnusedPassportWithoutInventingRolesOrArt(t *testing.T) {
	input := locationsdk.Input{Catalog: locationsdk.Catalog{UnitMM: 500, Geometries: []locationsdk.Geometry{{ID: "shape", Width: 4, Depth: 2, Height: 1, Ports: []locationsdk.Port{{ID: "front", Kind: "use"}}, Sockets: []locationsdk.Socket{{ID: "mount"}}}}, Modules: []locationsdk.Module{{ID: "kit.unused", GeometryID: "shape", Variants: []locationsdk.Variant{{ID: "neutral", GeometryID: "shape", SurfaceID: "legacy.panel"}}}}}}
	docs := map[string]KitDocumentation{"kit.unused": {Purpose: "Authored sleeping-place reservation", SourceRefs: []string{"source.cue:shape"}}}
	c, err := BuildKitCatalog(input, nil, t.TempDir(), docs)
	if err != nil {
		t.Fatal(err)
	}
	row := c.Modules[0]
	if row.Purpose != docs[row.ID].Purpose || len(row.PurposeSources) != 1 || row.Bound || len(row.MaterialRoles) != 0 || len(row.SurfaceIDs) != 1 || row.SurfaceIDs[0] != "legacy.panel" {
		t.Fatal("catalog guessed purpose/material/art or lost the legacy surface distinction", row)
	}
	kinds := map[string]bool{}
	for _, l := range row.Locators {
		kinds[l.Kind] = true
	}
	if len(row.Locators) != 2 || !kinds["socket"] || !kinds["port"] || len(row.MetadataMissing) != 2 {
		t.Fatal("semantic datums were confused with authored Blender empties", row)
	}
	dir := t.TempDir()
	if err := WriteKitCatalog(dir, c); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(dir, "kit-catalog.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "### `kit.unused`") || !strings.Contains(string(text), docs[row.ID].Purpose) {
		t.Fatal("unused module omitted from readable catalog")
	}
	if _, err := BuildKitCatalog(input, nil, t.TempDir(), map[string]KitDocumentation{"typo": {Purpose: "WrongID", SourceRefs: []string{"source"}}}); err == nil {
		t.Fatal("unknown documentation key accepted")
	}
	if _, err := BuildKitCatalog(input, nil, t.TempDir()); err == nil {
		t.Fatal("undocumented unused passport did not teach the author to provide its purpose")
	}
}

func TestKitCatalogAggregatesActualRoleAndTypedLocatorDeclarations(t *testing.T) {
	input := locationsdk.Input{Catalog: locationsdk.Catalog{UnitMM: 500, Geometries: []locationsdk.Geometry{{ID: "shape", Width: 1, Depth: 1, Height: 1}}, Modules: []locationsdk.Module{{ID: "kit.shape", GeometryID: "shape"}}}, Layouts: []locationsdk.Layout{{ID: "layout", Instances: []locationsdk.Instance{{ID: "use", ModuleID: "kit.shape", Reason: "Actual use reason"}}, TextSlots: []locationsdk.TextSlot{{ID: "label", Instance: "use", Anchor: "textslot.label"}}}}}
	binding := KitBinding{Name: "declared", LayoutID: "layout", Finishing: &Finishing{ObjectRoles: map[string]string{"use": "steel"}}, Locators: map[string][]string{"use": {"textslot.label", "service.queue"}}, LocatorSpecs: map[string][]KitLocator{"use": {{Name: "service.queue", Kind: "service", PortID: "front", SourceRef: "pipeline.port_locators"}}}}
	c, err := BuildKitCatalog(input, []KitBinding{binding}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	row := c.Modules[0]
	if len(row.MaterialRoles) != 1 || row.MaterialRoles[0] != "steel" || len(row.Locators) != 2 || len(row.MetadataMissing) != 0 {
		t.Fatal("explicit metadata was not aggregated", row)
	}
	for _, l := range row.Locators {
		if l.Kind == "authored" {
			t.Fatal("typed declarations were duplicated as unknown bindings", l)
		}
	}
}
