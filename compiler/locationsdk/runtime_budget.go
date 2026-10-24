package locationsdk

import "fmt"

// These limits belong to the first isolated Unity workbench, not to the generic
// authoring compiler. Keep them aligned with the renderer's defensive checks.
const (
	RuntimeCellBudget       = 65536
	RuntimeInstanceBudget   = 512
	RuntimeDecorationBudget = 512
	RuntimeGuidePointBudget = 16384
	RuntimePartBudget       = 1024
)

// runtimePreflight bounds allocations before Build creates terrain, occupancy,
// preview cells and fully expanded instance geometry. Invalid semantic records
// are still diagnosed by Build; this pass never repairs or rewrites input.
func runtimePreflight(input Input) *ValidationError {
	for i, layout := range input.Layouts {
		if layout.Width > 0 && layout.Depth > 0 && layout.Depth > RuntimeCellBudget/layout.Width {
			return issue("runtime_area_budget", fmt.Sprintf("layouts[%d]", i), "isolated Unity SDK scenes support at most 65536 navigation cells; the authoring compiler supports larger plans")
		}
		if len(layout.Instances) > RuntimeInstanceBudget {
			return issue("runtime_instance_budget", fmt.Sprintf("layouts[%d].instances", i), "isolated Unity SDK scenes support at most 512 module instances")
		}
	}
	// Avoid constructing reference tables for an already-invalid huge catalog.
	// Build reports the established catalog diagnostic without allocating a level.
	if len(input.Catalog.Geometries) > 4096 || len(input.Catalog.Modules) > 4096 || len(input.Catalog.Decorations) > 4096 {
		return nil
	}
	geometries := make(map[string]Geometry, len(input.Catalog.Geometries))
	for _, geometry := range input.Catalog.Geometries {
		geometries[geometry.ID] = geometry
	}
	modules := make(map[string]string, len(input.Catalog.Modules))
	for _, module := range input.Catalog.Modules {
		modules[module.ID] = module.GeometryID
	}
	decorations := make(map[string]Decoration, len(input.Catalog.Decorations))
	for _, decoration := range input.Catalog.Decorations {
		decorations[decoration.ID] = decoration
	}
	for i, layout := range input.Layouts {
		if layout.Width < 1 || layout.Depth < 1 {
			continue
		}
		path := fmt.Sprintf("layouts[%d]", i)
		// Area multiplication is safe after the division guard above.
		floor := make([]bool, layout.Width*layout.Depth)
		cells, parts, attachments, points := 0, 0, 0, 0
		for _, area := range layout.Floor {
			r := area.Rect
			if r.X < 0 || r.Y < 0 || r.W < 1 || r.H < 1 || r.X >= layout.Width || r.Y >= layout.Depth || r.W > layout.Width-r.X || r.H > layout.Depth-r.Y {
				continue
			}
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := r.X; x < r.X+r.W; x++ {
					n := y*layout.Width + x
					if !floor[n] {
						floor[n] = true
						cells++
					}
				}
			}
		}
		for j, instance := range layout.Instances {
			ip := fmt.Sprintf("%s.instances[%d]", path, j)
			if geometry, ok := geometries[modules[instance.ModuleID]]; ok {
				if len(geometry.Footprint) > RuntimeCellBudget-cells {
					return issue("runtime_geometry_budget", ip, "supported floor plus physical footprints and unique decoration boxes exceeds 65536 rendered cells")
				}
				cells += len(geometry.Footprint)
				if len(geometry.Footprint) > 0 {
					parts++
				}
			}
			if len(instance.Decorations) > RuntimeDecorationBudget-attachments {
				return issue("runtime_decoration_budget", ip+".decorations", "isolated Unity SDK scenes support at most 512 unique decorative bindings")
			}
			attachments += len(instance.Decorations)
			parts += len(instance.Decorations)
			for k, binding := range instance.Decorations {
				definition, ok := decorations[binding.DecorationID]
				if !ok || definition.Width < 1 || definition.Depth < 1 {
					continue
				}
				if definition.Width > (RuntimeCellBudget-cells)/definition.Depth {
					return issue("runtime_geometry_budget", fmt.Sprintf("%s.decorations[%d]", ip, k), "decorative box exceeds the remaining 65536 rendered-cell budget")
				}
				cells += definition.Width * definition.Depth
			}
		}
		for j, guide := range layout.ReservedPaths {
			if len(guide.Points) > RuntimeGuidePointBudget-points {
				return issue("runtime_guide_budget", fmt.Sprintf("%s.reserved_paths[%d].points", path, j), "isolated Unity SDK scenes support at most 16384 ordered guide points")
			}
			points += len(guide.Points)
			if len(guide.Points) >= 2 {
				parts++
			}
		}
		for _, drop := range layout.Drops {
			if drop.Barrier {
				parts++
			}
		}
		if parts > RuntimePartBudget {
			return issue("runtime_part_budget", path, "isolated Unity SDK scenes support at most 1024 rendered parts: physical modules, unique decorations, guide meshes and requested barriers")
		}
	}
	return nil
}
