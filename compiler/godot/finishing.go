package godot

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/placement"
)

// Finishing changes presentation only. Stable surface IDs retain all compiled
// collision, navigation and perception semantics, even when art replaces a draw.
type Finishing struct {
	FloorRecesses    []FloorRecess       `json:"floor_recesses,omitempty"`
	FloorOpenings    []FloorOpening      `json:"floor_openings,omitempty"`
	Architecture     *Architecture       `json:"architecture,omitempty"`
	ObjectRoles      map[string]string   `json:"object_roles,omitempty"`
	Environment      *EnvironmentProfile `json:"environment"`
	Materials        map[string]Material `json:"materials"`
	SurfaceRoles     map[string]string   `json:"surface_roles"`
	MaterialBindings map[string]string   `json:"material_bindings"`
	Replacements     []Replacement       `json:"replacements"`
	Lights           []Light             `json:"lights"`
}

// Linear scene colours, matching glTF and Blender; the emitter converts to
// Godot's sRGB-facing material/light properties exactly once.
type EnvironmentProfile struct {
	ToneMapper    int        `json:"tone_mapper"`
	AmbientColor  [3]float64 `json:"ambient_color"`
	AmbientEnergy [3]float64 `json:"ambient_energy"`
	Exposure      float64    `json:"exposure"`
	DefaultFill   bool       `json:"default_fill"`
	Provenance    string     `json:"provenance"`
}

func linearToSRGB(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}
func (e *EnvironmentProfile) render() (string, error) {
	if e == nil {
		return "", nil
	}
	if e.Provenance == "" || e.Exposure <= 0 || !finite(e.Exposure) || e.ToneMapper < 0 || e.ToneMapper > 4 {
		return "", fmt.Errorf("godot finishing: invalid environment profile")
	}
	for _, v := range append(e.AmbientColor[:], e.AmbientEnergy[:]...) {
		if v < 0 || !finite(v) {
			return "", fmt.Errorf("godot finishing: invalid ambient")
		}
	}
	return fmt.Sprintf("metadata/presentation_ambient_color = Color(%s, %s, %s, 1)\nmetadata/presentation_ambient_energy = Vector3(%s, %s, %s)\nmetadata/presentation_exposure = %s\nmetadata/presentation_default_fill = %t\n", number(linearToSRGB(e.AmbientColor[0])), number(linearToSRGB(e.AmbientColor[1])), number(linearToSRGB(e.AmbientColor[2])), number(e.AmbientEnergy[0]), number(e.AmbientEnergy[1]), number(e.AmbientEnergy[2]), number(e.Exposure), e.DefaultFill), nil
}

type Material struct {
	SurfaceCaste   string            `json:"surface_caste,omitempty"`
	Concrete       *ConcreteMaterial `json:"concrete,omitempty"`
	SurfaceKind    string            `json:"surface_kind,omitempty"`
	DoubleSided    bool              `json:"double_sided"`
	Resource       string            `json:"resource"`
	Color          [4]float64        `json:"color"`
	Roughness      float64           `json:"roughness"`
	Metallic       float64           `json:"metallic"`
	Emission       [3]float64        `json:"emission"`
	EmissionEnergy float64           `json:"emission_energy"`
	Provenance     string            `json:"provenance"`
}
type Replacement struct {
	SurfaceID  string `json:"surface_id"`
	InstanceID string `json:"instance_id"`
	Reason     string `json:"reason"`
}
type Light struct {
	Shadows          bool           `json:"shadows"`
	PresentationOnly bool           `json:"presentation_only"`
	AreaSize         [2]float64     `json:"area_size_m"`
	ID               string         `json:"id"`
	At               placement.Vec3 `json:"at_plan_m"`
	Aim              placement.Vec3 `json:"aim_plan_m"`
	Color            [3]float64     `json:"color"`
	Energy           float64        `json:"energy"`
	Range            float64        `json:"range_m"`
	Angle            float64        `json:"angle_degrees"` // zero = omni, otherwise full spot cone
	Mode             [3]float64     `json:"mode"`          // authoritative day / dusk / night weights
	Source           string         `json:"source"`
}

func canonicalResource(s, ext string) bool {
	p := strings.TrimPrefix(s, "res://")
	return strings.HasPrefix(s, "res://") && p != "" && !strings.ContainsAny(p, "\n\r\x00\\") && path.Clean(p) == p && !strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../") && strings.EqualFold(path.Ext(p), ext)
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (f *Finishing) render(manifest placement.Manifest, surfaces map[string]bool) (header, bindings, lights string, roles map[string]string, replaced map[string]bool, err error) {
	roles = map[string]string{}
	replaced = map[string]bool{}
	if f == nil {
		return
	}
	var policyErr error
	f, policyErr = ResolveArchitecture(f)
	if policyErr != nil {
		err = policyErr
		return
	}
	fail := func(s string) { err = fmt.Errorf("godot finishing: %s", s) }
	keys := make([]string, 0, len(f.Materials))
	for k := range f.Materials {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	refs := map[string]string{}
	var ext, sub strings.Builder
	concreteShaderWritten := false
	for i, k := range keys {
		m := f.Materials[k]
		id := fmt.Sprintf("finish_%03d", i)
		if k == "" || m.Provenance == "" {
			fail("material needs identity and provenance")
			return
		}
		if m.Concrete != nil {
			if m.Resource != "" || m.SurfaceKind != "concrete" || !validWallCaste(m.SurfaceCaste) {
				fail("concrete variant needs surface_kind concrete, surface_caste and no resource override: " + k)
				return
			}
			if !concreteShaderWritten {
				fmt.Fprintf(&sub, "[sub_resource type=\"Shader\" id=\"concrete_shader\"]\ncode = %s\n", strconv.Quote(concreteShader))
				concreteShaderWritten = true
			}
			var x, y string
			profile := *m.Concrete
			if profile.Tier == "" {
				profile.Tier = m.SurfaceCaste
			}
			x, y, err = profile.render(id)
			if err != nil {
				return
			}
			ext.WriteString(x)
			sub.WriteString(y)
			refs[k] = "SubResource(" + strconv.Quote(id) + ")"
		} else if m.Resource != "" {
			if !canonicalResource(m.Resource, ".tres") {
				fail("material needs canonical res:// .tres: " + k)
				return
			}
			fmt.Fprintf(&ext, "[ext_resource type=\"Material\" path=%s id=%s]\n", strconv.Quote(m.Resource), strconv.Quote(id))
			refs[k] = "ExtResource(" + strconv.Quote(id) + ")"
		} else {
			values := append(m.Color[:], m.Roughness, m.Metallic, m.EmissionEnergy)
			values = append(values, m.Emission[:]...)
			for _, v := range values {
				if !finite(v) || v < 0 {
					fail("invalid PBR value: " + k)
					return
				}
			}
			for _, v := range append(m.Color[:], m.Roughness, m.Metallic) {
				if v > 1 {
					fail("PBR parameter exceeds one: " + k)
					return
				}
			}
			refs[k] = "SubResource(" + strconv.Quote(id) + ")"
			fmt.Fprintf(&sub, "[sub_resource type=\"StandardMaterial3D\" id=%s]\nresource_name = %s\nalbedo_color = Color(%s, %s, %s, %s)\nroughness = %s\nmetallic = %s\n", strconv.Quote(id), strconv.Quote(k), number(linearToSRGB(m.Color[0])), number(linearToSRGB(m.Color[1])), number(linearToSRGB(m.Color[2])), number(m.Color[3]), number(m.Roughness), number(m.Metallic))
			if m.DoubleSided {
				fmt.Fprintln(&sub, "cull_mode = 2")
			}
			if m.Color[3] < 1 {
				fmt.Fprintln(&sub, "transparency = 1\ncull_mode = 2")
			}
			if m.EmissionEnergy > 0 {
				fmt.Fprintf(&sub, "emission_enabled = true\nemission = Color(%s, %s, %s, 1)\nemission_energy_multiplier = %s\n", number(linearToSRGB(m.Emission[0])), number(linearToSRGB(m.Emission[1])), number(linearToSRGB(m.Emission[2])), number(m.EmissionEnergy))
			}
		}
	}
	for id, key := range f.ObjectRoles {
		if !surfaces[id] {
			if _, ok := manifest.Instances[id]; !ok {
				fail("object_roles references unknown SDK identity: " + id)
				return
			}
		}
		if refs[key] == "" {
			fail("object_roles references unknown material: " + key)
			return
		}
		roles["object:"+id] = refs[key]
	}
	for role, k := range f.SurfaceRoles {
		if role != "floor" && role != "wall" && role != "steel" {
			fail("unknown surface role: " + role)
			return
		}
		if refs[k] == "" {
			fail("unknown role material: " + k)
			return
		}
		roles[role] = refs[k]
	}
	materialBindings := map[string]string{}
	for name, role := range f.MaterialBindings {
		materialBindings[name] = role
	}
	for role := range f.Materials {
		sentinel := "ANG_ROLE__" + role
		if previous, ok := materialBindings[sentinel]; ok && previous != role {
			fail("conflicting role sentinel binding: " + sentinel)
			return
		}
		materialBindings[sentinel] = role
	}
	names := make([]string, 0, len(materialBindings))
	for name := range materialBindings {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		ref := refs[materialBindings[name]]
		if name == "" || ref == "" {
			fail("invalid named material binding: " + name)
			return
		}
		fmt.Fprintf(&b, "%s: %s,\n", strconv.Quote(name), ref)
	}
	bindings = b.String()
	for _, r := range f.Replacements {
		if !surfaces[r.SurfaceID] || replaced[r.SurfaceID] || manifest.Instances[r.InstanceID].ModelRef == "" || r.Reason == "" {
			fail("replacement needs a unique compiled surface, bound art and reason: " + r.SurfaceID)
			return
		}
		replaced[r.SurfaceID] = true
	}
	var l strings.Builder
	seen := map[string]bool{}
	for i, a := range f.Lights {
		if a.ID == "" || seen[a.ID] || a.Source == "" || a.Range <= 0 || a.Energy < 0 || a.Angle < 0 || a.Angle >= 180 {
			fail("invalid authored light: " + a.ID)
			return
		}
		seen[a.ID] = true
		for _, v := range append(append(append(a.At[:], a.Aim[:]...), a.Color[:]...), append(a.Mode[:], a.Energy, a.Range, a.Angle)...) {
			if !finite(v) {
				fail("non-finite light: " + a.ID)
				return
			}
		}
		for _, v := range append(a.Mode[:], a.Color[:]...) {
			if v < 0 || v > 1 {
				fail("invalid light color/mode: " + a.ID)
				return
			}
		}
		if (a.AreaSize[0] > 0) != (a.AreaSize[1] > 0) || !finite(a.AreaSize[0]) || !finite(a.AreaSize[1]) || a.AreaSize[0] < 0 || a.AreaSize[1] < 0 {
			fail("invalid rectangle size: " + a.ID)
			return
		}
		kind := "OmniLight3D"
		if a.Angle > 0 {
			kind = "SpotLight3D"
		}
		if a.AreaSize[0] > 0 {
			kind = "AreaLight3D"
		}
		fmt.Fprintf(&l, "[node name=\"Authored_%03d\" type=%s parent=\"LightingRig\"]\nposition = %s\nlight_color = Color(%s, %s, %s, 1)\nlight_energy = %s\nshadow_enabled = %t\nmetadata/source_id = %s\nmetadata/source = %s\nmetadata/mode = Vector3(%s, %s, %s)\n", i, strconv.Quote(kind), vector(placement.FromPlan(a.At)), number(linearToSRGB(a.Color[0])), number(linearToSRGB(a.Color[1])), number(linearToSRGB(a.Color[2])), number(a.Energy), a.Shadows, strconv.Quote(a.ID), strconv.Quote(a.Source), number(a.Mode[0]), number(a.Mode[1]), number(a.Mode[2]))
		fmt.Fprintf(&l, "metadata/presentation_only = %t\n", a.PresentationOnly)
		if a.Angle > 0 || a.AreaSize[0] > 0 {
			// Basis columns for a -Z-forward Godot lamp. No Euler-angle convention.
			p, q := placement.FromPlan(a.At), placement.FromPlan(a.Aim)
			z := placement.Vec3{p[0] - q[0], p[1] - q[1], p[2] - q[2]}
			n := math.Sqrt(z[0]*z[0] + z[1]*z[1] + z[2]*z[2])
			if n < 1e-8 {
				fail("light aim equals position")
				return
			}
			for j := range z {
				z[j] /= n
			}
			up := placement.Vec3{0, 1, 0}
			if math.Abs(z[1]) > .99 {
				up = placement.Vec3{1, 0, 0}
			}
			cross := func(a, b placement.Vec3) placement.Vec3 {
				return placement.Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
			}
			x := cross(up, z)
			n = math.Sqrt(x[0]*x[0] + x[1]*x[1] + x[2]*x[2])
			for j := range x {
				x[j] /= n
			}
			y := cross(z, x)
			fmt.Fprintf(&l, "transform = %s\n", transform([3]placement.Vec3{x, y, z}, p))
			if a.AreaSize[0] > 0 {
				fmt.Fprintf(&l, "area_size = Vector2(%s, %s)\narea_range = %s\narea_attenuation = 1.3\narea_normalize_energy = true\n", number(a.AreaSize[0]), number(a.AreaSize[1]), number(a.Range))
			} else {
				fmt.Fprintf(&l, "spot_range = %s\nspot_angle = %s\nspot_attenuation = 1.3\n", number(a.Range), number(a.Angle/2))
			}
		} else {
			fmt.Fprintf(&l, "omni_range = %s\nomni_attenuation = 1.3\n", number(a.Range))
		}
	}
	if bindings != "" {
		ext.WriteString("[ext_resource type=\"Script\" path=\"res://location_finish.gd\" id=\"location_finish\"]\n")
	}
	header = ext.String() + sub.String()
	lights = l.String()
	return
}
