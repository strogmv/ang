package locationsdk

import (
	"fmt"
	"regexp"
)

const SurfaceBudget = 256

// Keys deliberately exclude extensions, traversal, schemes, absolute paths and
// platform separators. The integration resolves them under Assets/Resources.
var surfaceResourceKey = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)

func prepareSurfaces(c Catalog) (map[string]Surface, *ValidationError) {
	if len(c.Surfaces) > SurfaceBudget {
		return nil, issue("surface_budget", "catalog.surfaces", "surface catalog exceeds 256 definitions")
	}
	out := make(map[string]Surface, len(c.Surfaces))
	for i, surface := range c.Surfaces {
		path := fmt.Sprintf("catalog.surfaces[%d]", i)
		if !stableID.MatchString(surface.ID) {
			return nil, issue("invalid_id", path+".id", "surface needs a stable identifier")
		}
		if _, duplicate := out[surface.ID]; duplicate {
			return nil, issue("duplicate_id", path+".id", "surface ID is repeated", surface.ID)
		}
		for _, image := range []struct{ field, key string }{{"top_image", surface.TopImage}, {"side_image", surface.SideImage}} {
			if len(image.key) > 256 || !surfaceResourceKey.MatchString(image.key) {
				return nil, issue("surface_resource", path+"."+image.field, "surface image must be a relative extensionless Resources key of 1..256 ASCII characters with alphanumeric, underscore or hyphen path segments")
			}
		}
		for _, repeat := range []struct {
			field string
			value SurfaceRepeatMM
		}{{"top_repeat_mm", surface.TopRepeatMM}, {"side_repeat_mm", surface.SideRepeatMM}} {
			if repeat.value.X < 1 || repeat.value.Y < 1 || repeat.value.X > 1000000 || repeat.value.Y > 1000000 {
				return nil, issue("surface_repeat", path+"."+repeat.field, "texture repeat must be 1..1000000 millimetres on both axes")
			}
		}
		out[surface.ID] = surface
	}
	if c.DefaultFloorSurface != "" {
		if _, ok := out[c.DefaultFloorSurface]; !ok {
			return nil, issue("unknown_surface", "catalog.default_floor_surface", "floor references an unknown surface", c.DefaultFloorSurface)
		}
	}
	return out, nil
}
