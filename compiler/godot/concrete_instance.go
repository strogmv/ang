package godot

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// ConcreteInstance is the per-wall maintenance history. It creates visual
// evidence only; it never adds repair jobs, robots, ownership or player actions.
type ConcreteInstance struct {
	Reason             string            `json:"reason,omitempty"`
	Tier               string            `json:"tier"`
	Age                float64           `json:"age"`
	Function           []string          `json:"function"`
	Exposure           string            `json:"exposure"`
	Seed               int               `json:"seed"`
	FloorM             float64           `json:"floor_m,omitempty"`
	Anchors            []string          `json:"anchors,omitempty"`
	Enamel             bool              `json:"enamel,omitempty"`
	ApprovedReference  string            `json:"approved_reference,omitempty"`
	OpenFacadeNetworks []string          `json:"open_facade_networks,omitempty"`
	Caries             *ConcreteCaries   `json:"caries,omitempty"`
	Patches            []ConcretePatch   `json:"patches,omitempty"`
	Stripes            []ConcreteStripe  `json:"stripe_segments,omitempty"`
	Openings           []ConcreteOpening `json:"openings,omitempty"`
	Ghosts             []ConcreteGhost   `json:"ghosts,omitempty"`
	Handwear           bool              `json:"handwear,omitempty"`
	Resin              bool              `json:"resin,omitempty"`
}

type ConcreteAnchor struct {
	Effect    string         `json:"effect,omitempty"`
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	SourceRef string         `json:"source_ref"`
	Position  placement.Vec3 `json:"position_plan_m"`
	Strength  float64        `json:"strength"`
	RadiusM   float64        `json:"radius_m"`
	Hidden    bool           `json:"hidden,omitempty"`
}
type ConcreteCaries struct {
	Strength float64          `json:"strength"`
	Hints    []ConcreteAnchor `json:"hints"`
}
type ConcretePatch struct {
	ID      string     `json:"id"`
	Anchor  string     `json:"anchor"`
	Size    [2]float64 `json:"size_m"`
	Atlas   [4]float64 `json:"atlas_rect"`
	Tagged  bool       `json:"tagged"`
	Number  string     `json:"number,omitempty"`
	Date    string     `json:"date,omitempty"`
	TextKey string     `json:"text_key,omitempty"`
}
type ConcreteStripe struct {
	ID        string         `json:"id"`
	SourceRef string         `json:"source_ref"`
	From      placement.Vec3 `json:"from_plan_m"`
	To        placement.Vec3 `json:"to_plan_m"`
	Status    string         `json:"status"`
}
type ConcreteOpening struct {
	ID            string         `json:"id"`
	SourceRef     string         `json:"source_ref"`
	Center        placement.Vec3 `json:"center_plan_m"`
	WidthM        float64        `json:"width_m"`
	HeightM       float64        `json:"height_m"`
	Tangent       placement.Vec3 `json:"tangent_plan"`
	LoadDirection string         `json:"load_direction"`
}
type ConcreteGhost struct {
	ID            string         `json:"id"`
	SourceRef     string         `json:"source_ref"`
	Position      placement.Vec3 `json:"position_plan_m"`
	Size          [2]float64     `json:"size_m"`
	AnchorColumns int            `json:"anchor_columns"`
	AnchorRows    int            `json:"anchor_rows"`
}
type ConcreteRoute struct {
	From   placement.Vec3 `json:"from_plan_m"`
	To     placement.Vec3 `json:"to_plan_m"`
	Weight float64        `json:"weight"`
	Source string         `json:"source"`
}

func validateConcreteInstance(id string, s ConcreteInstance, source map[string]ConcreteAnchor) error {
	fail := func(message string) error { return fmt.Errorf("concrete[%q]: %s", id, message) }
	if !validWallCaste(s.Tier) {
		return fail("tier copper/bronze/silver/gold is required")
	}
	if !finite(s.Age) || s.Age < 0 || s.Age > 1 || !finite(s.FloorM) {
		return fail("age must be 0..1 and floor_m finite")
	}
	if s.Exposure != "public" && s.Exposure != "service" {
		return fail("exposure must be public or service")
	}
	if (s.Tier == "copper" || s.Tier == "bronze") && s.Enamel {
		return fail("Copper/Bronze cannot have enamel")
	}
	if (s.Tier == "silver" || s.Tier == "gold") && len(s.OpenFacadeNetworks) > 0 {
		return fail("Silver/Gold facade networks must be enclosed; open_facade_networks must be empty")
	}
	kinds := map[string]bool{}
	for _, ref := range s.Anchors {
		anchor, ok := source[ref]
		if !ok {
			return fail("anchor " + ref + " is absent from the actual CUE source anchor table")
		}
		kinds[anchor.Kind] = true
	}
	for _, function := range s.Function {
		switch function {
		case "water":
			if !kinds["pipe"] && !kinds["steel"] {
				return fail("water staining requires an actual pipe or tie/steel anchor")
			}
		case "lamp":
			if !kinds["lamp"] && !kinds["fire"] {
				return fail("soot requires an actual lamp/fire anchor")
			}
		case "machinery":
			if !kinds["steel"] {
				return fail("machinery oil/rust requires an actual steel/machine anchor")
			}
		case "gate":
			if !kinds["gate"] && !kinds["threshold"] {
				return fail("gate marks require an actual gate/threshold anchor")
			}
		case "kitchen":
			if !kinds["fire"] && !kinds["pipe"] {
				return fail("kitchen deposits require actual fire/pipe anchors")
			}
		default:
			return fail("unknown maintenance function " + function)
		}
	}
	if s.Tier == "silver" || s.Tier == "gold" {
		hidden := false
		if s.Caries != nil {
			if len(s.Caries.Hints) > 16 {
				return fail("at most16 physical caries hints per scope; split scope, no silent cap")
			}
			if !finite(s.Caries.Strength) || s.Caries.Strength <= 0 || s.Caries.Strength > 1 {
				return fail("caries.strength must be >0..1")
			}
			for _, h := range s.Caries.Hints {
				if _, err := concreteSourceTable([]ConcreteAnchor{h}); err != nil {
					return fail(err.Error())
				}
				if h.Hidden && h.SourceRef != "" {
					hidden = true
				}
			}
		}
		if !hidden {
			return fail("Silver/Gold location or zone needs at least one hidden caries hint with an actual source_ref")
		}
	}
	if len(s.Patches) > 16 || len(s.Stripes) > 16 || len(s.Openings) > 8 || len(s.Ghosts) > 8 {
		return fail("split a wall scope: at most16patches/stripes,8openings/ghosts per material; no silent cap")
	}
	for _, p := range s.Patches {
		if p.ID == "" || p.Size[0] <= 0 || p.Size[1] <= 0 {
			return fail("patch needs ID and positive size_m")
		}
		if _, ok := source[p.Anchor]; !ok {
			return fail("patch " + p.ID + " requires an existing physical anchor")
		}
		if p.Tagged && (p.Number == "" || p.Date == "" || p.TextKey == "") {
			return fail("tagged patch " + p.ID + " needs number/date and localized text_key; never bake text into the atlas")
		}
		for _, v := range p.Atlas {
			if !finite(v) || v < 0 || v > 1 {
				return fail("patch atlas_rect must be normalized0..1")
			}
		}
	}
	for _, v := range s.Stripes {
		if s.Tier != "silver" || v.ID == "" || v.SourceRef == "" || (v.Status != "present" && v.Status != "painted_over" && v.Status != "missing") {
			return fail("stripe segments are explicit Silver present/painted_over/missing records with source_ref")
		}
	}
	if s.Resin && len(s.Openings) == 0 {
		return fail("Gold resin cracks require authored opening corners and load direction")
	}
	if s.Resin && s.Tier != "gold" {
		return fail("resin ornament is restricted to Gold and real opening corners")
	}
	for _, o := range s.Openings {
		if o.ID == "" || o.SourceRef == "" || o.WidthM <= 0 || o.HeightM <= 0 || o.LoadDirection != "diagonal_from_top_corners" {
			return fail("opening needs actual source_ref,width/height and diagonal_from_top_corners load logic")
		}
	}
	if s.Tier == "gold" && len(s.Ghosts) > 0 {
		return fail("Gold has no removed-machine ghosts")
	}
	for _, g := range s.Ghosts {
		if g.ID == "" || g.SourceRef == "" || g.Size[0] <= 0 || g.Size[1] <= 0 || g.AnchorRows < 1 || g.AnchorColumns < 1 {
			return fail("machine ghost needs authored rectangle, anchor hole rows/columns and source_ref")
		}
	}
	return nil
}

func concreteSourceTable(anchors []ConcreteAnchor) (map[string]ConcreteAnchor, error) {
	result := map[string]ConcreteAnchor{}
	for _, a := range anchors {
		if a.ID == "" || a.SourceRef == "" || !finite(a.RadiusM) || a.RadiusM <= 0 || !finite(a.Strength) || a.Strength < 0 || a.Strength > 1 {
			return nil, fmt.Errorf("concrete.source_anchors[%q]: actual source_ref, positive radius and strength0..1 required", a.ID)
		}
		if _, ok := result[a.ID]; ok {
			return nil, fmt.Errorf("duplicate concrete source anchor %q", a.ID)
		}
		switch a.Kind {
		case "pipe", "steel", "lamp", "fire", "gate", "threshold":
		default:
			return nil, fmt.Errorf("concrete source anchor %q: kind must identify a real pipe/steel/lamp/fire/gate/threshold", a.ID)
		}
		for _, v := range a.Position {
			if !finite(v) {
				return nil, fmt.Errorf("nonfinite concrete source anchor %q", a.ID)
			}
		}
		if a.Effect != "" && a.Effect != "damp" && a.Effect != "dust" && a.Effect != "patina" && a.Effect != "failed_led" {
			return nil, fmt.Errorf("concrete source anchor %q: unknown physical effect %q", a.ID, a.Effect)
		}
		result[a.ID] = a
	}
	return result, nil
}

func resolveConcreteInstances(f *Finishing) (*Finishing, error) {
	if f == nil || f.Architecture == nil || f.Architecture.Concrete == nil || f.Architecture.concreteResolved {
		return f, nil
	}
	out := *f
	a := *f.Architecture
	out.Architecture = &a
	out.Materials = map[string]Material{}
	for k, v := range f.Materials {
		out.Materials[k] = v
	}
	a.ConcreteRoles = map[string]string{}
	for k, v := range f.Architecture.ConcreteRoles {
		a.ConcreteRoles[k] = v
	}
	a.WallZones = map[string]WallZone{}
	for k, v := range f.Architecture.WallZones {
		a.WallZones[k] = v
	}
	out.SurfaceRoles = map[string]string{}
	for k, v := range f.SurfaceRoles {
		out.SurfaceRoles[k] = v
	}
	out.ObjectRoles = map[string]string{}
	for k, v := range f.ObjectRoles {
		out.ObjectRoles[k] = v
	}
	a.Draws = append([]ArchitectureDraw{}, a.Draws...)
	if a.Concrete.Tier != a.WallCaste {
		return nil, fmt.Errorf("architecture.concrete.tier must match wall_caste")
	}
	source, err := concreteSourceTable(a.SourceAnchors)
	if err != nil {
		return nil, err
	}
	cariesDraws := map[string]ArchitectureDraw{}
	apply := func(id string, s ConcreteInstance, base string) (string, error) {
		if err := validateConcreteInstance(id, s, source); err != nil {
			return "", err
		}
		if s.Caries != nil {
			for _, hint := range s.Caries.Hints {
				effect := hint.Effect
				if effect == "" {
					effect = "damp"
					if s.Tier == "gold" {
						effect = "dust"
					}
				}
				materialRole := "caries." + effect
				if _, ok := out.Materials[materialRole]; !ok {
					return "", fmt.Errorf("concrete hint %q: declare physical material %q", hint.ID, materialRole)
				}
				draw := ArchitectureDraw{ID: "caries." + hint.ID, Kind: "trim", MaterialRole: materialRole, Position: [3]float64(hint.Position), Size: [3]float64{hint.RadiusM * 2, hint.RadiusM, 0.001}, Reason: "Concealed " + effect + " sign anchored to CUE source " + hint.SourceRef + "; caries strength=" + number(s.Caries.Strength) + ", hidden=" + strconv.FormatBool(hint.Hidden)}
				if prior, ok := cariesDraws[draw.ID]; ok && prior != draw {
					return "", fmt.Errorf("concrete hint %q: conflicting physical source poses", hint.ID)
				}
				cariesDraws[draw.ID] = draw
			}
		}
		if s.ApprovedReference != "" {
			m, ok := out.Materials[base]
			if !ok || m.Resource == "" || m.SurfaceCaste != s.Tier {
				return "", fmt.Errorf("concrete approved reference %q: existing matching resource required", id)
			}
			a.ConcreteRoles[id] = base
			return base, nil
		}
		raw, ok := out.Materials["concrete."+s.Tier]
		if !ok || raw.Concrete == nil {
			return "", fmt.Errorf("concrete %q: familyShaderMaterial concrete.%s required", id, s.Tier)
		}
		sum := sha256.Sum256([]byte(id))
		role := fmt.Sprintf("concrete.instance.%x", sum[:6])
		material := raw
		profile := *raw.Concrete
		profile.Instance = &s
		profile.AnchorData = []ConcreteAnchor{}
		for _, ref := range s.Anchors {
			profile.AnchorData = append(profile.AnchorData, source[ref])
		}
		profile.PatchAnchors = []ConcreteAnchor{}
		for _, p := range s.Patches {
			profile.PatchAnchors = append(profile.PatchAnchors, source[p.Anchor])
		}
		material.Concrete = &profile
		material.Provenance += "; resolved CUE wall scope " + id
		out.Materials[role] = material
		a.ConcreteRoles[id] = role
		return role, nil
	}
	role, err := apply("default", *a.Concrete, a.ConcreteRole)
	if err != nil {
		return nil, err
	}
	a.ConcreteRole = role
	out.SurfaceRoles["wall"] = role
	zoneIDs := []string{}
	for id := range a.WallZones {
		zoneIDs = append(zoneIDs, id)
	}
	sort.Strings(zoneIDs)
	for _, id := range zoneIDs {
		zone := a.WallZones[id]
		if zone.Concrete == nil {
			continue
		}
		if zone.Concrete.Tier != zone.Caste {
			return nil, fmt.Errorf("concrete zone %q tier must match its declared caste", id)
		}
		r, err := apply(id, *zone.Concrete, "concrete."+zone.Caste)
		if err != nil {
			return nil, err
		}
		zone.MaterialRole = r
		a.WallZones[id] = zone
	}
	objectIDs := []string{}
	for id := range a.Walls {
		objectIDs = append(objectIDs, id)
	}
	sort.Strings(objectIDs)
	for _, id := range objectIDs {
		if a.Walls[id].Tier != a.WallCaste && strings.TrimSpace(a.Walls[id].Reason) == "" {
			return nil, fmt.Errorf("concrete wall %q: mixed-caste segment requires a written reason", id)
		}
		r, err := apply(id, a.Walls[id], "concrete."+a.Walls[id].Tier)
		if err != nil {
			return nil, err
		}
		out.ObjectRoles[id] = r
	}
	for i, d := range a.Draws {
		if d.Kind != "wall" {
			continue
		}
		r := a.ConcreteRoles[d.ID]
		if r == "" && d.Zone != "" {
			zone, ok := a.WallZones[d.Zone]
			if !ok {
				return nil, fmt.Errorf("architecture.draws[%q]: unknown zone %q", d.ID, d.Zone)
			}
			r = zone.MaterialRole
		}
		if r == "" {
			r = a.ConcreteRole
		}
		a.Draws[i].MaterialRole = r
		out.ObjectRoles[d.ID] = r
	}
	cariesIDs := []string{}
	for id := range cariesDraws {
		cariesIDs = append(cariesIDs, id)
	}
	sort.Strings(cariesIDs)
	for _, id := range cariesIDs {
		a.Draws = append(a.Draws, cariesDraws[id])
		out.ObjectRoles[id] = cariesDraws[id].MaterialRole
	}
	a.concreteResolved = true
	return &out, nil
}

// ResolveArchitectureForLevel derives touch wear from authored movement and
// service geometry at build time. Only coalesced actual route cells are used.
func ResolveArchitectureForLevel(f *Finishing, level locationsdk.CompiledLevel) (*Finishing, error) {
	resolved, err := ResolveArchitecture(f)
	if err != nil || resolved == nil {
		return resolved, err
	}
	if resolved.Architecture == nil || resolved.Architecture.Concrete == nil {
		return resolved, nil
	}
	sourceIDs := map[string]bool{}
	for _, id := range resolved.Architecture.SourceIDs {
		sourceIDs[id] = true
	}
	for _, i := range level.Instances {
		sourceIDs[i.ID] = true
	}
	for _, p := range level.Preview.Floor {
		sourceIDs[p.SourceID] = true
	}
	for _, light := range resolved.Lights {
		sourceIDs[light.ID] = true
	}
	for _, d := range resolved.Architecture.Draws {
		if !strings.HasPrefix(d.ID, "caries.") {
			sourceIDs[d.ID] = true
		}
	}
	if level.Navigation != nil {
		for _, o := range level.Navigation.Obstacles {
			sourceIDs[o.ID] = true
		}
		for _, s := range level.Navigation.Surfaces {
			sourceIDs[s.ID] = true
		}
	}
	if err := validateConcreteSources("architecture", resolved.Architecture.SourceAnchors, sourceIDs); err != nil {
		return nil, err
	}
	out := *resolved
	out.Materials = map[string]Material{}
	for k, v := range resolved.Materials {
		out.Materials[k] = v
	}
	for role, m := range out.Materials {
		if m.Concrete != nil && m.Concrete.Instance != nil && m.Concrete.ReviewNavigation == nil {
			if err := validateConcreteInstanceSources(role, *m.Concrete.Instance, m.Concrete.AnchorData, sourceIDs); err != nil {
				return nil, err
			}
		}
		if m.Concrete == nil || m.Concrete.Instance == nil || !m.Concrete.Instance.Handwear || m.Concrete.ReviewNavigation != nil {
			continue
		}
		copyMaterial := m
		profile := *m.Concrete
		profile.RouteData = buildConcreteRoutes(level, profile.Instance.FloorM)
		if len(profile.RouteData) > 128 {
			return nil, fmt.Errorf("concrete[%s]: %d NAV touch segments exceed128; split the wall scope, no silent truncation", role, len(profile.RouteData))
		}
		copyMaterial.Concrete = &profile
		out.Materials[role] = copyMaterial
	}
	return &out, nil
}

func validateConcreteSources(scope string, anchors []ConcreteAnchor, ids map[string]bool) error {
	for _, a := range anchors {
		if !ids[a.SourceRef] {
			return fmt.Errorf("concrete[%s] anchor %q: source_ref %q does not identify a compiled SDK instance/floor/obstacle, actual CUE light/draw or published source ID; fix that CUE anchor", scope, a.ID, a.SourceRef)
		}
	}
	return nil
}
func validateConcreteInstanceSources(scope string, s ConcreteInstance, anchors []ConcreteAnchor, ids map[string]bool) error {
	if err := validateConcreteSources(scope, anchors, ids); err != nil {
		return err
	}
	refs := []ConcreteAnchor{}
	if s.Caries != nil {
		refs = append(refs, s.Caries.Hints...)
	}
	for _, v := range s.Stripes {
		refs = append(refs, ConcreteAnchor{ID: v.ID, SourceRef: v.SourceRef})
	}
	for _, v := range s.Openings {
		refs = append(refs, ConcreteAnchor{ID: v.ID, SourceRef: v.SourceRef})
	}
	for _, v := range s.Ghosts {
		refs = append(refs, ConcreteAnchor{ID: v.ID, SourceRef: v.SourceRef})
	}
	return validateConcreteSources(scope, refs, ids)
}

func buildConcreteRoutes(level locationsdk.CompiledLevel, floor float64) []ConcreteRoute {
	weights := map[locationsdk.Cell]float64{}
	sources := map[locationsdk.Cell]string{}
	project := func(c locationsdk.Cell) (locationsdk.Cell, bool) {
		if level.Navigation == nil {
			return c, true
		}
		nav := level.Navigation
		for _, surface := range nav.Surfaces {
			if surface.Walkable && surface.NavRow >= 0 && c.Y >= surface.NavRow && c.Y < surface.NavRow+nav.PlanDepth && math.Abs(float64(surface.ElevationMM)/1000-floor) < 0.001 {
				return locationsdk.Cell{X: c.X, Y: c.Y - surface.NavRow}, true
			}
		}
		for _, connector := range nav.Connectors {
			for _, node := range connector.Nodes {
				if node.Nav == c && math.Abs(float64(node.HeightMM)/1000-floor) < .001 {
					return node.Plan, true
				}
			}
		}
		return locationsdk.Cell{}, false
	}
	add := func(nav locationsdk.Cell, w float64, source string) {
		if c, ok := project(nav); ok {
			weights[c] += w
			sources[c] = "NAV route/use/narrow density"
			_ = source
		}
	}
	for _, path := range level.ReservedPaths {
		for _, c := range path.Cells {
			add(c, 1, "route:"+path.ID)
		}
	}
	for _, clearance := range level.Clearances {
		if clearance.Kind == "use" {
			for _, c := range clearance.Cells {
				add(c, 2, "queue/use:"+clearance.InstanceID)
			}
		}
	}
	for y, row := range level.Terrain {
		for x, g := range row {
			if g != '.' {
				continue
			}
			count := 0
			for _, d := range []locationsdk.Cell{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				nx, ny := x+d.X, y+d.Y
				if ny >= 0 && ny < len(level.Terrain) && nx >= 0 && nx < len(level.Terrain[ny]) && level.Terrain[ny][nx] == '.' {
					count++
				}
			}
			if count <= 2 {
				add(locationsdk.Cell{X: x, Y: y}, 1.5, "narrow-passage")
			}
		}
	}
	cells := make([]locationsdk.Cell, 0, len(weights))
	for c := range weights {
		cells = append(cells, c)
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Y == cells[j].Y {
			return cells[i].X < cells[j].X
		}
		return cells[i].Y < cells[j].Y
	})
	result := []ConcreteRoute{}
	u := float64(level.UnitMM) / 1000
	for i := 0; i < len(cells); {
		a := cells[i]
		z := a
		w := weights[a]
		j := i + 1
		for j < len(cells) && cells[j].Y == z.Y && cells[j].X == z.X+1 && weights[cells[j]] == w {
			z = cells[j]
			j++
		}
		density := 1 - math.Exp(-w*.4)
		result = append(result, ConcreteRoute{From: placement.Vec3{(float64(a.X) + .5) * u, floor, (float64(a.Y) + .5) * u}, To: placement.Vec3{(float64(z.X) + .5) * u, floor, (float64(z.Y) + .5) * u}, Weight: density, Source: sources[a]})
		i = j
	}
	return result
}

func shaderVec3(v placement.Vec3) string {
	p := placement.FromPlan(v)
	return "Vector3(" + number(p[0]) + "," + number(p[1]) + "," + number(p[2]) + ")"
}
func shaderVec4(v [4]float64) string {
	return "Vector4(" + number(v[0]) + "," + number(v[1]) + "," + number(v[2]) + "," + number(v[3]) + ")"
}
func concreteInstanceParameters(c *ConcreteMaterial) string {
	var out strings.Builder
	s := c.Instance
	if s == nil {
		s = &ConcreteInstance{Tier: c.Tier, Age: .7, Exposure: "public"}
	}
	tier := map[string]int{"copper": 0, "bronze": 1, "silver": 2, "gold": 3}[s.Tier]
	fmt.Fprintf(&out, "shader_parameter/tier = %d\nshader_parameter/age = %s\nshader_parameter/floor_m = %s\nshader_parameter/exposure_public = %t\nshader_parameter/seed_phase = %s\n", tier, number(s.Age), number(s.FloorM), s.Exposure == "public", number(math.Mod(float64(s.Seed), 997)/997))
	capacities := map[string]int{"anchor_position": 64, "anchor_meta": 64, "patch_position": 16, "patch_size": 16, "patch_uv": 16, "stripe_a": 16, "stripe_b": 16, "resin_a": 16, "resin_b": 16, "route_a": 128, "route_b": 128, "ghost_position": 8, "ghost_size": 8, "caries_position": 16, "caries_meta": 16}
	vectors := func(name string, items []string) {
		for len(items) < capacities[name] {
			items = append(items, "Vector3(0,0,0)")
		}
		fmt.Fprintf(&out, "shader_parameter/%s = Array[Vector3]([%s])\n", name, strings.Join(items, ","))
	}
	fours := func(name string, items []string) {
		for len(items) < capacities[name] {
			items = append(items, "Vector4(0,0,0,0)")
		}
		fmt.Fprintf(&out, "shader_parameter/%s = Array[Vector4]([%s])\n", name, strings.Join(items, ","))
	}
	anchors, meta := []string{}, []string{}
	for _, a := range c.AnchorData {
		k := map[string]float64{"pipe": 0, "steel": 1, "lamp": 2, "fire": 3, "gate": 4, "threshold": 5}[a.Kind]
		anchors = append(anchors, shaderVec3(a.Position))
		meta = append(meta, shaderVec4([4]float64{k, a.Strength, a.RadiusM, 0}))
	}
	vectors("anchor_position", anchors)
	fours("anchor_meta", meta)
	fmt.Fprintf(&out, "shader_parameter/anchor_count = %d\n", len(anchors))
	patchA, patchB, patchUV := []string{}, []string{}, []string{}
	for i, p := range s.Patches {
		if i >= len(c.PatchAnchors) {
			continue
		}
		patchA = append(patchA, shaderVec3(c.PatchAnchors[i].Position))
		patchB = append(patchB, shaderVec4([4]float64{p.Size[0], p.Size[1], float64(i), 0}))
		patchUV = append(patchUV, shaderVec4(p.Atlas))
	}
	vectors("patch_position", patchA)
	fours("patch_size", patchB)
	fours("patch_uv", patchUV)
	fmt.Fprintf(&out, "shader_parameter/patch_count = %d\n", len(patchA))
	stripeA, stripeB := []string{}, []string{}
	for _, p := range s.Stripes {
		a, b := placement.FromPlan(p.From), placement.FromPlan(p.To)
		status := map[string]float64{"present": 0, "painted_over": 1, "missing": 2}[p.Status]
		stripeA = append(stripeA, shaderVec4([4]float64{a[0], a[1], a[2], status}))
		stripeB = append(stripeB, shaderVec4([4]float64{b[0], b[1], b[2], .05}))
	}
	fours("stripe_a", stripeA)
	fours("stripe_b", stripeB)
	fmt.Fprintf(&out, "shader_parameter/stripe_count = %d\n", len(stripeA))
	resinA, resinB := []string{}, []string{}
	if s.Resin {
		for _, p := range s.Openings {
			for _, side := range []float64{-1, 1} {
				a := p.Center
				for k := range a {
					a[k] += p.Tangent[k] * p.WidthM * .5 * side
				}
				a[1] += p.HeightM * .5
				b := a
				for k := range b {
					b[k] += p.Tangent[k] * side * .8
				}
				b[1] += .8
				resinA = append(resinA, shaderVec3(a))
				resinB = append(resinB, shaderVec3(b))
			}
		}
	}
	vectors("resin_a", resinA)
	vectors("resin_b", resinB)
	fmt.Fprintf(&out, "shader_parameter/resin_count = %d\n", len(resinA))
	routeA, routeB := []string{}, []string{}
	for _, r := range c.RouteData {
		a, b := placement.FromPlan(r.From), placement.FromPlan(r.To)
		routeA = append(routeA, shaderVec4([4]float64{a[0], a[1], a[2], r.Weight}))
		routeB = append(routeB, shaderVec4([4]float64{b[0], b[1], b[2], 0}))
	}
	fours("route_a", routeA)
	fours("route_b", routeB)
	fmt.Fprintf(&out, "shader_parameter/route_count = %d\n", len(routeA))
	ghostA, ghostB := []string{}, []string{}
	for _, g := range s.Ghosts {
		ghostA = append(ghostA, shaderVec3(g.Position))
		ghostB = append(ghostB, shaderVec4([4]float64{g.Size[0], g.Size[1], float64(g.AnchorColumns), float64(g.AnchorRows)}))
	}
	vectors("ghost_position", ghostA)
	fours("ghost_size", ghostB)
	fmt.Fprintf(&out, "shader_parameter/ghost_count = %d\n", len(ghostA))
	cariesA, cariesMeta := []string{}, []string{}
	if s.Caries != nil {
		for _, h := range s.Caries.Hints {
			effect := h.Effect
			if effect == "" {
				effect = "damp"
				if s.Tier == "gold" {
					effect = "dust"
				}
			}
			kind := map[string]float64{"damp": 0, "dust": 1, "patina": 2, "failed_led": 3}[effect]
			cariesA = append(cariesA, shaderVec3(h.Position))
			cariesMeta = append(cariesMeta, shaderVec4([4]float64{s.Caries.Strength * h.Strength, h.RadiusM, kind, 0}))
		}
	}
	vectors("caries_position", cariesA)
	fours("caries_meta", cariesMeta)
	fmt.Fprintf(&out, "shader_parameter/caries_count = %d\n", len(cariesA))
	if s.ApprovedReference != "" {
		fmt.Fprintf(&out, "metadata/approved_reference = %s\n", strconv.Quote(s.ApprovedReference))
	}
	return out.String()
}
