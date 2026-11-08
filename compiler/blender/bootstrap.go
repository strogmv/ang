package blender

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// KitContract bootstraps Blender-first authoring before a location has any SDK
// floor. The published CUE catalog supplies IDs, dimensions and source art.
func KitContract(kit godot.KitCatalog, layoutID, clientRoot string) (Contract, error) {
	if kit.Schema != "ang.kit-catalog.v1" || kit.UnitMM != locationsdk.UnitMM {
		return Contract{}, fmt.Errorf("blender kit catalog: expected ang.kit-catalog.v1 with %d mm SDK units", locationsdk.UnitMM)
	}
	level := locationsdk.CompiledLevel{ID: layoutID, UnitMM: kit.UnitMM, Width: 1, Depth: 1}
	layout := locationsdk.Layout{ID: layoutID, UnitMM: kit.UnitMM, Width: 1, Depth: 1}
	input := locationsdk.Catalog{UnitMM: kit.UnitMM}
	models := map[string]string{}
	bindings := map[string]string{}
	spec := &Pipeline{ToleranceM: .02, Instances: map[string]ArtSpec{}}
	for _, module := range kit.Modules {
		input.Modules = append(input.Modules, locationsdk.Module{ID: module.ID, GeometryID: module.GeometryID})
		input.Geometries = append(input.Geometries, locationsdk.Geometry{ID: module.GeometryID, Width: module.SizeMM[0] / kit.UnitMM, Depth: module.SizeMM[1] / kit.UnitMM, Height: module.SizeMM[2] / kit.UnitMM})
		uses := append(append([]godot.KitUse{}, module.Art...), module.Uses...)
		for _, use := range uses {
			if use.ModelRef == "" {
				continue
			}
			if bindings[module.ID] == "" {
				bindings[module.ID] = use.ModelRef
			}
			models[use.ModelRef] = use.Resource
			if use.SourceAsset != "" {
				spec.Instances[module.ID] = ArtSpec{Asset: use.SourceAsset, AssetBasis: use.AssetBasis}
				spec.Instances[use.ModelRef] = spec.Instances[module.ID]
				bindings[module.ID] = use.ModelRef
				break
			}
		}
	}
	for _, use := range kit.Extras {
		if use.ModelRef != "" {
			models[use.ModelRef] = use.Resource
			if use.SourceAsset != "" {
				spec.Instances[use.ModelRef] = ArtSpec{Asset: use.SourceAsset, AssetBasis: use.AssetBasis}
			}
		}
	}
	manifest := placement.Manifest{LayoutID: layoutID, WorldFrame: placement.WorldFrame, Instances: map[string]placement.Instance{}}
	c, err := Build(level, layout, manifest, spec, nil, models, nil, "", clientRoot, CatalogContext{Catalog: input, ModuleModels: bindings, PoseSource: "blend"})
	if err != nil {
		return c, err
	}
	// Namespaced catalog material roles remain available for explicit artist tags;
	// the new level's own CUE supplies setting/caste and concrete defaults later.
	roles := make([]string, 0, len(kit.Materials))
	for role := range kit.Materials {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		material := kit.Materials[role]
		c.Materials[role] = material
		c.MaterialRoles = append(c.MaterialRoles, role)
		if i := strings.IndexByte(role, '/'); i >= 0 {
			short := role[i+1:]
			if _, ok := c.Materials[short]; !ok {
				c.Materials[short] = material
				c.MaterialRoles = append(c.MaterialRoles, short)
			}
		}
	}
	sort.Strings(c.MaterialRoles)
	if err := refreshSemanticHash(&c); err != nil {
		return c, err
	}
	return c, nil
}
