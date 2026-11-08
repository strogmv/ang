package blender

import (
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// checkFinishingPoses checks level-space origins, leaving dimensions, vectors,
// UV atlas coordinates and material policy in CUE. An unbound palette entry is
// not a level pose owner. Explicit bindings make its world-space inputs active.
func (g poseSourceGuard) checkFinishingPoses(presentation cue.Value, exportedRoles ...map[string][]string) error {
	finishing := presentation.LookupPath(cue.ParsePath("finishing"))
	architecture := finishing.LookupPath(cue.ParsePath("architecture"))
	if err := g.positionRows(architecture.LookupPath(cue.ParsePath("draws")), "finishing.architecture.draws", "id", "position_plan_m"); err != nil {
		return err
	}
	if err := g.positionRows(architecture.LookupPath(cue.ParsePath("source_anchors")), "finishing.architecture.source_anchors", "id", "position_plan_m"); err != nil {
		return err
	}
	if err := g.concreteInstancePoses(architecture.LookupPath(cue.ParsePath("concrete")), "finishing.architecture.concrete"); err != nil {
		return err
	}
	for _, group := range []string{"wall_zones", "walls"} {
		if entries, err := architecture.LookupPath(cue.MakePath(cue.Str(group))).Fields(); err == nil {
			for entries.Next() {
				value := entries.Value()
				path := fmt.Sprintf("finishing.architecture.%s[%q]", group, entries.Selector().Unquoted())
				if group == "wall_zones" {
					value = value.LookupPath(cue.ParsePath("concrete"))
					path += ".concrete"
				}
				if err = g.concreteInstancePoses(value, path); err != nil {
					return err
				}
			}
		}
	}
	active := map[string]bool{}
	objectsByRole := map[string][]string{}
	for _, usage := range exportedRoles {
		for role, objects := range usage {
			active[role] = true
			objectsByRole[role] = append(objectsByRole[role], objects...)
		}
	}
	addRole := func(v cue.Value) {
		if role, err := v.String(); err == nil && role != "" {
			active[role] = true
		}
	}
	for _, path := range []string{"surface_roles", "object_roles", "material_bindings", "architecture.concrete_roles"} {
		if entries, err := finishing.LookupPath(cue.ParsePath(path)).Fields(); err == nil {
			for entries.Next() {
				addRole(entries.Value())
			}
		}
	}
	addRole(architecture.LookupPath(cue.ParsePath("concrete_role")))
	for _, path := range []string{"draws", "wall_zones", "wall_overrides"} {
		value := architecture.LookupPath(cue.MakePath(cue.Str(path)))
		if items, err := value.List(); err == nil {
			for items.Next() {
				addRole(items.Value().LookupPath(cue.ParsePath("material_role")))
			}
		}
		if items, err := value.Fields(); err == nil {
			for items.Next() {
				addRole(items.Value().LookupPath(cue.ParsePath("material_role")))
			}
		}
	}
	if specs, err := presentation.LookupPath(cue.ParsePath("pipeline.instances")).Fields(); err == nil {
		for specs.Next() {
			addRole(specs.Value().LookupPath(cue.ParsePath("material_role")))
		}
	}
	// The architecture resolver obtains each scoped variant from this family
	// role even when concrete_role is implicit. Its origins are checked above.
	for _, path := range []string{"concrete", "wall_zones", "walls"} {
		value := architecture.LookupPath(cue.MakePath(cue.Str(path)))
		if path == "concrete" {
			if tier, err := value.LookupPath(cue.ParsePath("tier")).String(); err == nil {
				active["concrete."+tier] = true
			}
		} else if items, err := value.Fields(); err == nil {
			for items.Next() {
				spec := items.Value()
				if path == "wall_zones" {
					spec = spec.LookupPath(cue.ParsePath("concrete"))
				}
				if tier, e := spec.LookupPath(cue.ParsePath("tier")).String(); e == nil {
					active["concrete."+tier] = true
				}
			}
		}
	}
	roles := []string{}
	for role := range active {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		profile := finishing.LookupPath(cue.MakePath(cue.Str("materials"), cue.Str(role), cue.Str("concrete")))
		instance := profile.LookupPath(cue.ParsePath("instance"))
		// Without an instance these tables are not emitted as mask uniforms.
		if !profile.Exists() || !instance.Exists() {
			continue
		}
		path := fmt.Sprintf("finishing.materials[%q].concrete", role)
		usedBy := func(err error) error {
			if objects := objectsByRole[role]; len(objects) > 0 {
				return fmt.Errorf("%w; material role %q is used by Blender/art object(s): %s", err, role, strings.Join(objects, ", "))
			}
			return err
		}
		if err := g.concreteInstancePoses(instance, path+".instance"); err != nil {
			return usedBy(err)
		}
		for _, field := range []string{"anchor_data", "patch_anchors"} {
			if err := g.positionRows(profile.LookupPath(cue.MakePath(cue.Str(field))), path+"."+field, "id", "position_plan_m"); err != nil {
				return usedBy(err)
			}
		}
		// The renderer canonically replaces route_data with compiled NAV when
		// handwear is enabled, or with fixture NAV in an explicit review profile.
		handwear, _ := instance.LookupPath(cue.ParsePath("handwear")).Bool()
		if !handwear && !profile.LookupPath(cue.ParsePath("review_navigation")).Exists() {
			if err := g.positionRows(profile.LookupPath(cue.ParsePath("route_data")), path+".route_data", "source", "from_plan_m", "to_plan_m"); err != nil {
				return usedBy(err)
			}
		}
	}
	return nil
}

func (g poseSourceGuard) concreteInstancePoses(spec cue.Value, path string) error {
	if !spec.Exists() {
		return nil
	}
	if floor := spec.LookupPath(cue.ParsePath("floor_m")); floor.Exists() {
		if err := g.checkValue(floor, path+".floor_m", true, 0); err != nil {
			return err
		}
	}
	for _, row := range []struct {
		field string
		poses []string
	}{
		{"stripe_segments", []string{"from_plan_m", "to_plan_m"}},
		{"openings", []string{"center_plan_m"}},
		{"ghosts", []string{"position_plan_m"}},
		{"caries.hints", []string{"position_plan_m"}},
	} {
		if err := g.positionRows(spec.LookupPath(cue.ParsePath(row.field)), path+"."+row.field, "id", row.poses...); err != nil {
			return err
		}
	}
	return nil
}

func (g poseSourceGuard) positionRows(value cue.Value, path, idField string, fields ...string) error {
	rows, err := value.List()
	if err != nil {
		return nil
	}
	for rows.Next() {
		row := rows.Value()
		id, _ := row.LookupPath(cue.MakePath(cue.Str(idField))).String()
		for _, field := range fields {
			pose := row.LookupPath(cue.MakePath(cue.Str(field)))
			name := fmt.Sprintf("%s[%q].%s", path, id, field)
			if !pose.Exists() {
				return g.fail(row, name, "requires a three-coordinate origin derived from imported geometry")
			}
			coordinates, e := pose.List()
			length := 0
			if e == nil {
				for coordinates.Next() {
					length++
				}
			}
			if e != nil || length != 3 {
				return g.fail(pose, name, "requires a three-coordinate origin derived from imported geometry")
			}
			if err := g.checkValue(pose, name, true, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
