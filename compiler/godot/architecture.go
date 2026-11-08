package godot

import (
	"fmt"
	"strings"
)

// Architecture is authored policy; engine-neutral roles identify surfaces and
// CUE owns resource paths. Material names never imply a concrete wall.
type Architecture struct {
	concreteResolved bool
	Concrete         *ConcreteInstance           `json:"concrete,omitempty"`
	Walls            map[string]ConcreteInstance `json:"walls,omitempty"`
	SourceAnchors    []ConcreteAnchor            `json:"source_anchors,omitempty"`
	SourceIDs        []string                    `json:"source_ids,omitempty"`
	ConcreteRoles    map[string]string           `json:"concrete_roles,omitempty"`
	Draws            []ArchitectureDraw          `json:"draws,omitempty"`
	WallCaste        string                      `json:"wall_caste,omitempty"`
	WallZones        map[string]WallZone         `json:"wall_zones,omitempty"`
	Setting          string                      `json:"setting"`
	ConcreteRole     string                      `json:"concrete_role,omitempty"`
	WallOverrides    map[string]WallOverride     `json:"wall_overrides,omitempty"`
}

type ArchitectureDraw struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Zone         string     `json:"zone,omitempty"`
	MaterialRole string     `json:"material_role"`
	Position     [3]float64 `json:"position_plan_m"`
	Size         [3]float64 `json:"size_m"`
	Reason       string     `json:"reason"`
}

type WallZone struct {
	Concrete     *ConcreteInstance `json:"concrete,omitempty"`
	MaterialRole string            `json:"material_role,omitempty"`
	Caste        string            `json:"caste"`
	Reason       string            `json:"reason"`
}

func validWallCaste(c string) bool {
	return c == "copper" || c == "bronze" || c == "silver" || c == "gold"
}

type WallOverride struct {
	MaterialRole string `json:"material_role,omitempty"`
	Reason       string `json:"reason"`
}

// ResolveArchitecture returns a copy so implicit defaults never mutate CUE
// authoring. A legacy explicit concrete role keeps its resource/tint.
func ResolveArchitecture(f *Finishing) (*Finishing, error) {
	if f == nil || f.Architecture == nil {
		return f, nil
	}
	a := f.Architecture
	if a.Setting != "dome_interior" && a.Setting != "exterior" {
		return nil, fmt.Errorf("finishing.architecture.setting: choose dome_interior or exterior")
	}
	for id, o := range a.WallOverrides {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(o.Reason) == "" {
			return nil, fmt.Errorf("finishing.architecture.wall_overrides[%q]: per-object written reason required", id)
		}
		if o.MaterialRole != "" {
			if _, ok := f.Materials[o.MaterialRole]; !ok {
				return nil, fmt.Errorf("finishing.architecture.wall_overrides[%q]: unknown material_role %q", id, o.MaterialRole)
			}
		}
	}
	copyArchitecture := *a
	a = &copyArchitecture
	if a.Setting == "dome_interior" {
		if !validWallCaste(a.WallCaste) {
			return nil, fmt.Errorf("finishing.architecture.wall_caste: dome_interior requires copper, bronze, silver or gold")
		}
		if a.ConcreteRole == "" {
			a.ConcreteRole = "concrete." + a.WallCaste
		}
	}
	for id, z := range a.WallZones {
		if strings.TrimSpace(id) == "" || !validWallCaste(z.Caste) || strings.TrimSpace(z.Reason) == "" {
			return nil, fmt.Errorf("finishing.architecture.wall_zones[%q]: caste and written mixed-zone reason required", id)
		}
	}
	out := *f
	out.Architecture = a
	out.SurfaceRoles = map[string]string{}
	out.ObjectRoles = map[string]string{}
	for id, role := range f.ObjectRoles {
		out.ObjectRoles[id] = role
	}
	for _, draw := range a.Draws {
		if prior := out.ObjectRoles[draw.ID]; prior != "" && prior != draw.MaterialRole {
			return nil, fmt.Errorf("architecture.draws[%q]: object_roles %q conflicts with draw material_role %q", draw.ID, prior, draw.MaterialRole)
		}
		out.ObjectRoles[draw.ID] = draw.MaterialRole
	}
	for k, v := range f.SurfaceRoles {
		out.SurfaceRoles[k] = v
	}
	if a.Setting == "dome_interior" {
		concrete, ok := f.Materials[a.ConcreteRole]
		if a.ConcreteRole == "" || !ok || concrete.SurfaceKind != "concrete" || concrete.SurfaceCaste != a.WallCaste {
			return nil, fmt.Errorf("finishing.architecture.concrete_role: dome_interior needs a declared material with surface_kind concrete, matching surface_caste and existing resource/provenance")
		}
		role := out.SurfaceRoles["wall"]
		if role == "" {
			out.SurfaceRoles["wall"] = a.ConcreteRole
		} else if f.Materials[role].SurfaceKind != "concrete" || f.Materials[role].SurfaceCaste != a.WallCaste {
			return nil, fmt.Errorf("finishing.surface_roles.wall %q: dome_interior walls, enclosure and opening reveals must be concrete; use a per-object wall_overrides reason for an exception", role)
		}
	}
	return resolveConcreteInstances(&out)
}

// ValidateWallMaterial is shared by generated structural faces and ID-art
// validation. Overrides are explicit source data and must be reported by callers.
func ValidateWallMaterial(f *Finishing, id, role string) (*WallOverride, error) {
	resolved, err := ResolveArchitecture(f)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Architecture == nil || resolved.Architecture.Setting != "dome_interior" {
		return nil, nil
	}
	expected := resolved.Architecture.WallCaste
	if wall, ok := resolved.Architecture.Walls[id]; ok {
		expected = wall.Tier
	}
	if zone, ok := resolved.Architecture.WallZones[id]; ok {
		expected = zone.Caste
	}
	if resolved.Materials[role].SurfaceKind == "concrete" && resolved.Materials[role].SurfaceCaste == expected {
		return nil, nil
	}
	if o, ok := resolved.Architecture.WallOverrides[id]; ok {
		if o.MaterialRole != "" && o.MaterialRole != role {
			return nil, fmt.Errorf("wall %q: override material_role %q differs from actual %q", id, o.MaterialRole, role)
		}
		return &o, nil
	}
	return nil, fmt.Errorf("wall %q: material role %q is not concrete of the assigned wall caste in dome_interior; restore the concrete role or add a per-object written wall_overrides reason", id, role)
}
