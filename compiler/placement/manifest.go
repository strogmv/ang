package placement

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

const Schema = "ang.placement.v1"

// Authoring is optional CUE presentation intent, independent of engine resources.
// ModuleModels uses layout module/passport IDs, never asset GUIDs. An instance
// can select a stocked/model variant or declare an intentionally empty batch.
type Authoring struct {
	LayoutID     string                  `json:"layout_id"`
	Camera       *Camera                 `json:"camera,omitempty"`
	ModuleModels map[string]string       `json:"module_models,omitempty"`
	Instances    map[string]InstanceSpec `json:"instances,omitempty"`
	Extras       []Extra                 `json:"extras,omitempty"`
}

// Camera framing is authored in plan metres. Angles come from the compiled
// layout camera; framing does not introduce another artistic yaw.
type Camera struct {
	FocusPlanM Vec3    `json:"focus_plan_m"`
	SizeM      float64 `json:"size_m"`
	DistanceM  float64 `json:"distance_m"`
}

type InstanceSpec struct {
	ModelRef *string       `json:"model_ref,omitempty"`
	Bindings Bindings      `json:"bindings,omitempty"`
	Override *PoseOverride `json:"pose_override,omitempty"`
}

type PoseOverride struct {
	PositionPlanM   Vec3    `json:"position_plan_m"`
	RotationDegrees float64 `json:"rotation_degrees"`
	Reason          string  `json:"reason"`
}

// Extra is an authored static pose, e.g. a train evaluated at its CUE epoch.
// Runtime movement is deliberately outside this manifest.
type Extra struct {
	InstanceID      string   `json:"instance_id"`
	ModelRef        string   `json:"model_ref"`
	SourceKind      string   `json:"source_kind"`
	PositionPlanM   Vec3     `json:"position_plan_m"`
	RotationDegrees float64  `json:"rotation_degrees"`
	Reason          string   `json:"reason"`
	Bindings        Bindings `json:"bindings,omitempty"`
}

type Bindings struct {
	Lights      []LightBinding      `json:"light,omitempty"`
	Storefronts []StorefrontBinding `json:"storefront,omitempty"`
	Gates       []string            `json:"gate,omitempty"`
}

type LightBinding struct {
	Path          string   `json:"path"`
	ActiveTimeIDs []string `json:"active_time_ids,omitempty"`
	MinQuality    string   `json:"min_quality,omitempty"`
	Daylight      bool     `json:"daylight,omitempty"`
	LensPaths     []string `json:"lens_paths,omitempty"`
}

type StorefrontBinding struct {
	StorefrontID string `json:"storefront_id"`
	GoodsPath    string `json:"goods_path"`
	ClosedPath   string `json:"closed_path"`
	OpenPath     string `json:"open_path"`
}

type Instance struct {
	WorldTransform  Transform  `json:"world_transform"`
	ModelRef        string     `json:"model_ref"`
	SourceKind      string     `json:"source_kind"`
	ModuleID        string     `json:"module_id,omitempty"`
	Bindings        Bindings   `json:"bindings"`
	NavRefs         []string   `json:"nav_refs"`
	ClearanceRefs   []string   `json:"clearance_refs"`
	SourceTransform *Transform `json:"source_transform,omitempty"`
	OverrideReason  string     `json:"override_reason,omitempty"`
}

type Manifest struct {
	Schema         string `json:"schema"`
	LayoutID       string `json:"layout_id"`
	WorldFrame     string `json:"world_frame"`
	UnitMM         int    `json:"unit_mm"`
	StructuralHash string `json:"structural_hash"`
	// References in instances are JSON pointers relative to this compiled level.
	CompiledRef string              `json:"compiled_ref"`
	Instances   map[string]Instance `json:"instances"`
}

// Build expands the compiler's stable identities; it never consults client assets.
func Build(level locationsdk.CompiledLevel, spec Authoring) (Manifest, error) {
	m := Manifest{Schema: Schema, LayoutID: level.ID, WorldFrame: WorldFrame, UnitMM: level.UnitMM,
		StructuralHash: level.StructuralHash, CompiledRef: "compiled.json#/levels/0", Instances: map[string]Instance{}}
	if level.UnitMM != locationsdk.UnitMM {
		return m, fmt.Errorf("placement: unit_mm must be %d", locationsdk.UnitMM)
	}
	if spec.LayoutID != "" && spec.LayoutID != level.ID {
		return m, fmt.Errorf("placement: layout %q does not match %q", spec.LayoutID, level.ID)
	}
	add := func(id, kind, module string, pose Transform, refs ...string) {
		if _, exists := m.Instances[id]; exists {
			return
		}
		model := module
		if v, ok := spec.ModuleModels[module]; ok {
			model = v
		}
		if s, ok := spec.Instances[id]; ok && s.ModelRef != nil {
			model = *s.ModelRef
		}
		m.Instances[id] = Instance{WorldTransform: pose, ModelRef: model, SourceKind: kind, ModuleID: module,
			NavRefs: append([]string{}, refs...), ClearanceRefs: []string{}}
	}
	for j, i := range level.Instances {
		if i.Rotation != 0 && i.Rotation != 90 && i.Rotation != 180 && i.Rotation != 270 {
			return m, fmt.Errorf("placement %s: invalid quarter rotation", i.ID)
		}
		root := "#/instances/" + strconv.Itoa(j)
		add(i.ID, "instance", i.ModuleID, Pose(i.At, i.Width, i.Depth, float64(level.Elevation*level.UnitMM)/1000, i.Rotation), root+"/blocked", root+"/footprint")
	}
	n := level.Navigation
	if n != nil {
		for j, s := range n.Surfaces {
			if s.ID == n.BaseSurface {
				continue
			}
			if len(s.Slab) == 0 {
				return m, fmt.Errorf("placement %s: empty slab", s.ID)
			}
			add(s.ID, "surface", "", Pose(locationsdk.Cell{X: s.Slab[0].X, Y: s.Slab[0].Y}, 1, 1, float64(s.ElevationMM-s.SlabMM)/1000, 0), pointer("surfaces", j))
		}
		for j, o := range n.Obstacles {
			add(o.ID, "obstacle", "", Pose(locationsdk.Cell{X: o.Rect.X, Y: o.Rect.Y}, o.Rect.W, o.Rect.H, float64(o.BottomMM)/1000, 0), pointer("obstacles", j))
		}
		for j, s := range n.Supports {
			if s.Kind == "wall_bearing" {
				continue
			}
			at, w, d, err := extent(s.Cells)
			if err != nil {
				return m, fmt.Errorf("placement %s: %w", s.ID, err)
			}
			add(s.ID, "support", "", Pose(at, w, d, float64(s.BottomMM)/1000, 0), pointer("supports", j))
		}
		for j, r := range n.Railings {
			at, w, d, _ := extent([]locationsdk.Cell{r.From, r.To})
			add(r.ID, "railing", "", Pose(at, w, d, float64(r.BaseMM)/1000, 0), pointer("railings", j))
		}
		for j, c := range n.Connectors {
			at, w, d, err := extent(c.Footprint)
			if err != nil {
				return m, fmt.Errorf("placement %s: %w", c.ID, err)
			}
			add(c.ID, "connector", "", Pose(at, w, d, float64(c.BottomMM)/1000, 0), pointer("connectors", j))
		}
	}
	for _, e := range spec.Extras {
		if _, exists := m.Instances[e.InstanceID]; exists || e.InstanceID == "" || e.ModelRef == "" || e.Reason == "" || e.SourceKind == "" {
			return m, fmt.Errorf("placement: invalid or duplicate extra %q", e.InstanceID)
		}
		add(e.InstanceID, e.SourceKind, "", PlanTransform(e.PositionPlanM, e.RotationDegrees))
		i := m.Instances[e.InstanceID]
		i.ModelRef = e.ModelRef
		i.Bindings = e.Bindings
		m.Instances[e.InstanceID] = i
	}
	for id, s := range spec.Instances {
		i, exists := m.Instances[id]
		if !exists {
			return m, fmt.Errorf("placement: unknown instance %q", id)
		}
		i.Bindings = s.Bindings
		if s.Override != nil {
			if strings.TrimSpace(s.Override.Reason) == "" {
				return m, fmt.Errorf("placement %s: pose override needs a reason", id)
			}
			source := i.WorldTransform
			i.SourceTransform = &source
			i.OverrideReason = s.Override.Reason
			i.WorldTransform = PlanTransform(s.Override.PositionPlanM, s.Override.RotationDegrees)
		}
		m.Instances[id] = i
	}
	for j, c := range level.Clearances {
		i, ok := m.Instances[c.InstanceID]
		if ok {
			i.ClearanceRefs = append(i.ClearanceRefs, "#/clearances/"+strconv.Itoa(j))
			m.Instances[c.InstanceID] = i
		}
	}
	for j, p := range level.Ports {
		i, ok := m.Instances[p.InstanceID]
		if ok {
			i.NavRefs = append(i.NavRefs, "#/ports/"+strconv.Itoa(j))
			m.Instances[p.InstanceID] = i
		}
	}
	for id, i := range m.Instances {
		for _, v := range append(append([]float64{}, i.WorldTransform.Position[:]...), i.WorldTransform.RotationDegrees[:]...) {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return m, fmt.Errorf("placement %s: non-finite transform", id)
			}
		}
		sort.Strings(i.NavRefs)
		sort.Strings(i.ClearanceRefs)
		m.Instances[id] = i
	}
	return m, nil
}

func pointer(kind string, index int) string {
	return "#/navigation/" + kind + "/" + strconv.Itoa(index)
}

func extent(cells []locationsdk.Cell) (locationsdk.Cell, int, int, error) {
	if len(cells) == 0 {
		return locationsdk.Cell{}, 0, 0, fmt.Errorf("empty geometry")
	}
	lo, hi := cells[0], cells[0]
	for _, c := range cells {
		lo.X = min(lo.X, c.X)
		lo.Y = min(lo.Y, c.Y)
		hi.X = max(hi.X, c.X)
		hi.Y = max(hi.Y, c.Y)
	}
	return lo, hi.X - lo.X + 1, hi.Y - lo.Y + 1, nil
}
