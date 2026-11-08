// Package blender owns the ID-based source-art contract shared by all locations.
// It emits Blender programs; callers run them in their private serial workbench.
package blender

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

//go:embed convention.json
var conventionData []byte

//go:embed runtime.py
var runtime string

//go:embed import.py
var importer string

//go:embed assets.py
var assets string

//go:embed start.py
var start string

//go:embed export.py
var exporter string

type Matrix [4][4]float64

type Convention struct {
	Schema          string             `json:"schema"`
	WorldFrame      string             `json:"world_frame"`
	Units           string             `json:"units"`
	BlenderAxes     []string           `json:"blender_axes"`
	BlenderToWorld  Matrix             `json:"blender_to_world"`
	ModelReflection Matrix             `json:"model_reflection"`
	Properties      map[string]string  `json:"properties"`
	IdentityRule    string             `json:"identity_rule"`
	LocatorKinds    []string           `json:"locator_kinds"`
	DefaultBudget   Budget             `json:"default_budget"`
	Camera          map[string]float64 `json:"camera"`
}

type Budget struct {
	Profile        string `json:"profile,omitempty"`
	Triangles      int    `json:"triangles"`
	Lights         int    `json:"lights"`
	ShadowedLights int    `json:"shadowed_lights"`
	Provenance     string `json:"provenance,omitempty"`
}

type ArtSpec struct {
	Derived      bool   `json:"derived,omitempty"`
	Reason       string `json:"reason,omitempty"`
	AssetBasis   string `json:"asset_basis,omitempty"`
	Greybox      bool   `json:"greybox,omitempty"`
	Asset        string `json:"asset,omitempty"`
	MaterialRole string `json:"material_role,omitempty"`
}

type Locator struct {
	Name          string         `json:"name"`
	InstanceID    string         `json:"instance_id,omitempty"`
	Kind          string         `json:"kind"`
	PositionPlanM placement.Vec3 `json:"position_plan_m"`
}

// Pipeline is CUE presentation intent. No level names or art paths live in ANG.
type KnownConflict struct {
	ObjectID      string           `json:"object_id"`
	Cell          locationsdk.Cell `json:"cell"`
	Layer         string           `json:"layer"`
	MaxIntrusionM float64          `json:"max_intrusion_m"`
	Reason        string           `json:"reason"`
}

type PortLocator struct {
	Name       string `json:"name"`
	InstanceID string `json:"instance_id"`
	PortID     string `json:"port_id"`
	Kind       string `json:"kind"`
}

type Pipeline struct {
	PortLocators   []PortLocator      `json:"port_locators,omitempty"`
	Legacy         bool               `json:"legacy,omitempty"`
	KnownConflicts []KnownConflict    `json:"known_conflicts,omitempty"`
	SourceBlend    string             `json:"source_blend,omitempty"`
	ToleranceM     float64            `json:"tolerance_m"`
	Instances      map[string]ArtSpec `json:"instances"`
	Locators       []Locator          `json:"locators,omitempty"`
	Budget         *Budget            `json:"budget,omitempty"`
}

type Identity struct {
	RenderedBy         string `json:"rendered_by,omitempty"`
	Floor              bool   `json:"floor,omitempty"`
	Layer              string `json:"layer,omitempty"`
	Procedural         bool   `json:"procedural,omitempty"`
	DeclaredAssetBasis string `json:"declared_asset_basis,omitempty"`
	SourceAsset        string `json:"source_asset,omitempty"`
	SourceAssetBasis   string `json:"source_asset_basis,omitempty"`
	Derived            bool   `json:"derived"`
	ModuleSizeCells    [3]int `json:"module_size_cells"`
	Reason             string `json:"reason,omitempty"`
	AssetBasis         string `json:"asset_basis"`
	ModelRef           string `json:"model_ref"`
	Kind               string `json:"kind"`
	ModuleID           string `json:"module_id,omitempty"`
	World              Matrix `json:"world"`
	Blockout           []Box  `json:"blockout"`
	Asset              string `json:"asset,omitempty"`
	Greybox            bool   `json:"greybox"`
	MaterialRole       string `json:"material_role,omitempty"`
	Required           bool   `json:"required"`
}

type Box struct {
	Center placement.Vec3 `json:"center_blender_m"`
	Size   placement.Vec3 `json:"size_blender_m"`
}

type WalkCell struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	ElevationM float64 `json:"elevation_m"`
	Surface    string  `json:"surface"`
}

type Contract struct {
	SceneGeometry       godot.ProceduralGeometry  `json:"scene_geometry"`
	PoseSource          string                    `json:"pose_source"`
	Schema              string                    `json:"schema"`
	GeometryContractSHA string                    `json:"geometry_contract_sha256"`
	Convention          Convention                `json:"convention"`
	LayoutID            string                    `json:"layout_id"`
	StructuralHash      string                    `json:"structural_hash"`
	UnitM               float64                   `json:"unit_m"`
	ToleranceM          float64                   `json:"tolerance_m"`
	Identities          map[string]Identity       `json:"identities"`
	Locators            []Locator                 `json:"locators"`
	Walkable            []WalkCell                `json:"walkable"`
	MaterialRoles       []string                  `json:"material_roles"`
	LightMetricsStatus  string                    `json:"light_metrics_status"`
	Lights              []godot.Light             `json:"lights"`
	Budget              Budget                    `json:"budget"`
	CameraWorld         Matrix                    `json:"camera_world"`
	Legacy              bool                      `json:"legacy"`
	KnownConflicts      []KnownConflict           `json:"known_conflicts"`
	BaseSurfaceID       string                    `json:"base_surface_id"`
	Architecture        *godot.Architecture       `json:"architecture,omitempty"`
	SurfaceRoles        map[string]string         `json:"surface_roles"`
	Materials           map[string]godot.Material `json:"materials"`
	PlanWidth           int                       `json:"plan_width"`
	PlanDepth           int                       `json:"plan_depth"`
	FrameM              float64                   `json:"frame_m"`
}

func transformMatrix(t placement.Transform) Matrix {
	b := t.BasisColumns()
	return Matrix{{b[0][0], b[1][0], b[2][0], t.Position[0]}, {b[0][1], b[1][1], b[2][1], t.Position[1]}, {b[0][2], b[1][2], b[2][2], t.Position[2]}, {0, 0, 0, 1}}
}

func resolveAsset(p, packageDir, clientRoot string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "res://") {
		rel := strings.TrimPrefix(p, "res://")
		if clientRoot == "" {
			return "", nil
		}
		if strings.ContainsAny(rel, "\n\r\x00\\") || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") {
			return "", fmt.Errorf("blender: invalid asset resource %q", p)
		}
		p = filepath.Join(clientRoot, rel)
	} else if !filepath.IsAbs(p) {
		p = filepath.Join(packageDir, p)
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".tscn" {
		raw, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		lines := []string{}
		for _, line := range strings.Split(string(raw), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) != 5 || lines[0] != "[gd_scene format=3]" || lines[2] != "[node name=\"Art\" type=\"Node3D\"]" || lines[3] != "[node name=\"Source\" parent=\".\" instance=ExtResource(\"asset\")]" || lines[4] != "scale = Vector3(1, 1, -1)" {
			return "", nil
		}
		match := regexp.MustCompile(`^\[ext_resource type="PackedScene" path=("[^"\r\n]+") id="asset"\]$`).FindStringSubmatch(lines[1])
		if len(match) != 2 {
			return "", nil
		}
		var child string
		if err = json.Unmarshal([]byte(match[1]), &child); err != nil {
			return "", err
		}
		if !strings.HasPrefix(child, "res://") || (filepath.Ext(child) != ".glb" && filepath.Ext(child) != ".gltf") {
			return "", fmt.Errorf("blender: generated wrapper child must use canonical res:// resource")
		}
		return resolveAsset(child, packageDir, clientRoot)
	}
	if ext != ".glb" && ext != ".gltf" {
		return "", nil
	}
	return filepath.Abs(p)
}

type CatalogContext struct {
	PoseSource   string
	Catalog      locationsdk.Catalog
	ModuleModels map[string]string
}

// Build constructs an authoring file from live compiled CUE, without reading art.
func Build(level locationsdk.CompiledLevel, layout locationsdk.Layout, manifest placement.Manifest, pipeline *Pipeline, finishing *godot.Finishing, models map[string]string, camera *placement.Camera, packageDir, clientRoot string, contexts ...CatalogContext) (Contract, error) {
	var err error
	finishing, err = godot.ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return Contract{}, err
	}
	var conv Convention
	if err := json.Unmarshal(conventionData, &conv); err != nil {
		return Contract{}, err
	}
	c := Contract{Schema: "ang.blender-authoring.v1", PoseSource: "cue", Convention: conv, LayoutID: level.ID, StructuralHash: level.StructuralHash, UnitM: float64(level.UnitMM) / 1000, ToleranceM: .02, Identities: map[string]Identity{}, Locators: []Locator{}, Walkable: []WalkCell{}, MaterialRoles: []string{}, Lights: []godot.Light{}, LightMetricsStatus: "unmeasured", SurfaceRoles: map[string]string{}, Budget: conv.DefaultBudget, FrameM: 12, BaseSurfaceID: layout.BaseSurfaceID, Materials: map[string]godot.Material{}, PlanWidth: level.Width, PlanDepth: level.Depth, KnownConflicts: []KnownConflict{}}
	if manifest.LayoutID != level.ID || manifest.WorldFrame != conv.WorldFrame {
		return c, fmt.Errorf("blender: placement/layout world-frame mismatch")
	}
	if pipeline != nil {
		if !finite(pipeline.ToleranceM) || pipeline.ToleranceM <= 0 || pipeline.ToleranceM >= c.UnitM/2 {
			return c, fmt.Errorf("pipeline.tolerance_m: state an intrusion tolerance greater than zero and below half a cell (%g m)", c.UnitM/2)
		}
		c.ToleranceM = pipeline.ToleranceM
		c.Legacy = pipeline.Legacy
		c.KnownConflicts = append(c.KnownConflicts, pipeline.KnownConflicts...)
		if !c.Legacy && len(c.KnownConflicts) > 0 {
			return c, fmt.Errorf("pipeline.known_conflicts: a new/strict layout cannot grandfather conflicts; legacy authoring must be explicitly declared")
		}
		seenConflicts := map[string]bool{}
		for _, conflict := range c.KnownConflicts {
			key := fmt.Sprintf("%s/%d/%d/%s", conflict.ObjectID, conflict.Cell.X, conflict.Cell.Y, conflict.Layer)
			if conflict.ObjectID == "" || conflict.Layer == "" || strings.TrimSpace(conflict.Reason) == "" || seenConflicts[key] || !finite(conflict.MaxIntrusionM) || conflict.MaxIntrusionM <= c.ToleranceM || conflict.MaxIntrusionM > c.UnitM/2+1e-8 {
				return c, fmt.Errorf("pipeline.known_conflicts[%q]: unique object/cell/layer, reason and measured intrusion > tolerance and <= half a cell required", key)
			}
			seenConflicts[key] = true
		}
		if pipeline.Budget != nil {
			c.Budget = *pipeline.Budget
			if c.Budget.Triangles <= 0 || c.Budget.Lights < 0 || c.Budget.ShadowedLights < 0 || c.Budget.Provenance == "" {
				return c, fmt.Errorf("pipeline.budget: positive triangles, nonnegative light counts and provenance required")
			}
		}
		c.Locators = append(c.Locators, pipeline.Locators...)
	}
	for id, i := range manifest.Instances {
		c.Identities[id] = Identity{ModelRef: i.ModelRef, Kind: i.SourceKind, ModuleID: i.ModuleID, World: transformMatrix(i.WorldTransform), Required: true, Blockout: []Box{}}
	}
	// The base floor has no model placement, but remains an authored ID/blockout.
	base := layout.BaseSurfaceID
	if base == "" {
		base = "sdk.floor"
	}
	if level.Navigation != nil {
		base = level.Navigation.BaseSurface
		for _, area := range layout.Floor {
			if _, ok := c.Identities[area.ID]; !ok {
				c.Identities[area.ID] = Identity{Kind: "floor_alias", Floor: true, Layer: base, World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), Blockout: []Box{}}
			}
		}
		if previous, ok := c.Identities[base]; !ok || previous.Kind == "floor_alias" {
			c.Identities[base] = Identity{Kind: "floor", Floor: true, Layer: base, World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), Required: true, Blockout: []Box{}}
		}
	} else {
		for _, f := range layout.Floor {
			c.Identities[f.ID] = Identity{Kind: "floor", Floor: true, Layer: base, World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), Required: true, Blockout: []Box{}}
		}
	}
	addBox := func(id string, r locationsdk.Rect, bottom, top float64) {
		q, ok := c.Identities[id]
		if !ok {
			return
		}
		q.Blockout = append(q.Blockout, Box{Center: placement.Vec3{(float64(r.X) + float64(r.W)/2) * c.UnitM, (float64(r.Y) + float64(r.H)/2) * c.UnitM, (bottom + top) / 2}, Size: placement.Vec3{float64(r.W) * c.UnitM, float64(r.H) * c.UnitM, math.Max(.02, top-bottom)}})
		c.Identities[id] = q
	}
	for _, i := range level.Instances {
		for _, cell := range i.Blocked {
			addBox(i.ID, locationsdk.Rect{X: cell.X, Y: cell.Y, W: 1, H: 1}, float64(level.Elevation)*c.UnitM, float64(level.Elevation+i.Height)*c.UnitM)
		}
	}
	if n := level.Navigation; n != nil {
		for _, s := range n.Surfaces {
			if q, ok := c.Identities[s.ID]; ok {
				q.Floor = true
				q.Layer = s.ID
				c.Identities[s.ID] = q
			}
			for _, r := range s.Slab {
				addBox(s.ID, r, float64(s.ElevationMM-s.SlabMM)/1000, float64(s.ElevationMM)/1000)
			}
			if !s.Walkable {
				continue
			}
			for y := 0; y < n.PlanDepth; y++ {
				for x := 0; x < n.PlanWidth; x++ {
					row := s.NavRow + y
					if row >= 0 && row < len(level.Terrain) && x < len(level.Terrain[row]) && locationsdk.IsWalkableTerrainGlyph(rune(level.Terrain[row][x])) {
						c.Walkable = append(c.Walkable, WalkCell{X: x, Y: y, ElevationM: float64(s.ElevationMM) / 1000, Surface: s.ID})
					}
				}
			}
		}
		for _, o := range n.Obstacles {
			addBox(o.ID, o.Rect, float64(o.BottomMM)/1000, float64(o.TopMM)/1000)
		}
	} else {
		for _, f := range layout.Floor {
			addBox(f.ID, f.Rect, float64(level.Elevation)*c.UnitM-.1, float64(level.Elevation)*c.UnitM)
		}
		for y, row := range level.Terrain {
			for x, k := range row {
				if locationsdk.IsWalkableTerrainGlyph(k) {
					c.Walkable = append(c.Walkable, WalkCell{X: x, Y: y, ElevationM: float64(level.Elevation) * c.UnitM, Surface: base})
				}
			}
		}
	}
	// Kit IDs are accepted as reusable art roots. Instance roots take precedence.
	for _, i := range manifest.Instances {
		for _, kit := range []string{i.ModuleID, i.ModelRef} {
			if kit == "" {
				continue
			}
			if _, ok := c.Identities[kit]; ok {
				continue
			}
			q := Identity{ModelRef: kit, Kind: "kit", World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), Required: false, Blockout: []Box{}}
			if kit == i.ModuleID {
				q.ModuleID = i.ModuleID
			}
			c.Identities[kit] = q
		}
	}
	for _, context := range contexts {
		if context.PoseSource != "" {
			c.PoseSource = context.PoseSource
		}
		geometries := map[string]locationsdk.Geometry{}
		for _, g := range context.Catalog.Geometries {
			geometries[g.ID] = g
		}
		for _, module := range context.Catalog.Modules {
			ref := module.ID
			if r, ok := context.ModuleModels[module.ID]; ok {
				ref = r
			}
			if _, ok := c.Identities[module.ID]; !ok {
				c.Identities[module.ID] = Identity{Kind: "kit", ModelRef: ref, ModuleID: module.ID, World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), Blockout: []Box{}}
			}
			g := geometries[module.GeometryID]
			q := c.Identities[module.ID]
			if q.Kind == "kit" {
				q.ModelRef = ref
				q.ModuleID = module.ID
			}
			q.ModuleSizeCells = [3]int{g.Width, g.Depth, g.Height}
			c.Identities[module.ID] = q
			for id, q := range c.Identities {
				if q.ModuleID == module.ID {
					q.ModuleSizeCells = [3]int{g.Width, g.Depth, g.Height}
					c.Identities[id] = q
				}
			}
		}
	}
	for ref := range models {
		if _, ok := c.Identities[ref]; !ok {
			c.Identities[ref] = Identity{Kind: "kit", ModelRef: ref, World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), Blockout: []Box{}}
		}
	}
	for id, q := range c.Identities {
		if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\n\r\x00") {
			return c, fmt.Errorf("blender: identity %q must be a stable ID, not a path", id)
		}
		var spec ArtSpec
		if pipeline != nil {
			spec = pipeline.Instances[id]
		}
		q.DeclaredAssetBasis = spec.AssetBasis
		source := spec.Asset
		if source == "" {
			source = models[q.ModelRef]
		}
		q.SourceAssetBasis = spec.AssetBasis
		if q.SourceAssetBasis == "" {
			q.SourceAssetBasis = "source_plan"
			if strings.EqualFold(filepath.Ext(source), ".tscn") {
				q.SourceAssetBasis = "godot"
			}
		}
		if q.SourceAssetBasis != "source_plan" && q.SourceAssetBasis != "godot" {
			return c, fmt.Errorf("pipeline.instances[%q].asset_basis: source_plan or godot required", id)
		}
		q.Reason = spec.Reason
		q.Derived = spec.Derived
		q.Greybox = spec.Greybox
		q.MaterialRole = spec.MaterialRole
		var err error
		q.SourceAsset, err = resolveAsset(source, packageDir, clientRoot)
		if err != nil {
			return c, err
		}
		active := models[q.ModelRef]
		if active == "" {
			active = source
		}
		q.AssetBasis = q.SourceAssetBasis
		if strings.EqualFold(filepath.Ext(active), ".tscn") {
			q.AssetBasis = "godot"
		}
		q.Asset, err = resolveAsset(active, packageDir, clientRoot)
		if err != nil {
			return c, err
		}
		if q.Asset == "" && spec.Asset != "" {
			q.Asset = q.SourceAsset
			q.AssetBasis = q.SourceAssetBasis
		}

		c.Identities[id] = q
	}
	if pipeline != nil {
		for id := range pipeline.Instances {
			if _, ok := c.Identities[id]; !ok {
				return c, fmt.Errorf("pipeline.instances[%q]: absent from SDK placement/module/model IDs; fix this CUE key", id)
			}
		}
	}
	if finishing != nil {
		for role, material := range finishing.Materials {
			c.Materials[role] = material
		}
		for role, material := range finishing.SurfaceRoles {
			c.SurfaceRoles[role] = material
		}
		c.Architecture = finishing.Architecture
		c.Lights = append(c.Lights, finishing.Lights...)
		if finishing.Lights != nil {
			c.LightMetricsStatus = "cue_declared"
		}
		for _, light := range finishing.Lights {
			present := false
			for _, l := range c.Locators {
				present = present || l.Name == light.ID
			}
			if !present {
				c.Locators = append(c.Locators, Locator{Name: light.ID, Kind: "light", PositionPlanM: light.At})
			}
		}
		for role := range finishing.SurfaceRoles {
			c.MaterialRoles = append(c.MaterialRoles, role)
		}
		for k := range finishing.Materials {
			c.MaterialRoles = append(c.MaterialRoles, k)
		}
		sort.Strings(c.MaterialRoles)
	}
	if finishing != nil && finishing.SurfaceRoles["floor"] != "" {
		role := finishing.SurfaceRoles["floor"]
		if _, exists := finishing.Materials[role]; !exists {
			return c, fmt.Errorf("finishing.surface_roles.floor: material %q is absent", role)
		}
		for id, q := range c.Identities {
			if q.Kind == "floor" {
				q.Procedural = true
				q.Greybox = false
				q.MaterialRole = role
				q.Reason = "SDK floor geometry rendered with its declared CUE material role"
				c.Identities[id] = q
			}
		}
	}
	if c.Architecture != nil {
		for _, draw := range c.Architecture.Draws {
			if _, exists := c.Identities[draw.ID]; exists {
				return c, fmt.Errorf("architecture.draws[%q]: duplicate placement identity", draw.ID)
			}
			c.Identities[draw.ID] = Identity{Kind: "procedural_" + draw.Kind, Required: true, Derived: true, Procedural: true, AssetBasis: "source_plan", World: transformMatrix(placement.PlanTransform(placement.Vec3{}, 0)), MaterialRole: draw.MaterialRole, Reason: draw.Reason, Blockout: []Box{{Center: placement.Vec3{draw.Position[0], draw.Position[2], draw.Position[1]}, Size: placement.Vec3{draw.Size[0], draw.Size[2], draw.Size[1]}}}}
		}
	}
	if finishing != nil {
		surfaces := map[string]bool{}
		if level.Navigation != nil {
			for _, surface := range level.Navigation.Surfaces {
				surfaces[surface.ID] = true
			}
			for _, obstacle := range level.Navigation.Obstacles {
				surfaces[obstacle.ID] = true
			}
		}
		seen := map[string]bool{}
		for _, replacement := range finishing.Replacements {
			owner, exists := manifest.Instances[replacement.InstanceID]
			if !surfaces[replacement.SurfaceID] || seen[replacement.SurfaceID] || !exists || owner.ModelRef == "" || strings.TrimSpace(replacement.Reason) == "" {
				return c, fmt.Errorf("finishing.replacements[%q]: unique compiled surface/obstacle, bound model owner and reason required", replacement.SurfaceID)
			}
			seen[replacement.SurfaceID] = true
			q, exists := c.Identities[replacement.SurfaceID]
			if !exists {
				return c, fmt.Errorf("finishing.replacements[%q]: absent from compiled authoring IDs", replacement.SurfaceID)
			}
			q.RenderedBy = replacement.InstanceID
			c.Identities[replacement.SurfaceID] = q
		}
	}
	var geometryErr error
	c.SceneGeometry, geometryErr = godot.PlanProceduralGeometry(level, manifest, finishing, false)
	if geometryErr != nil {
		return c, geometryErr
	}
	if pipeline != nil {
		for _, binding := range pipeline.PortLocators {
			instance, ok := manifest.Instances[binding.InstanceID]
			if !ok {
				return c, fmt.Errorf("pipeline.port_locators[%q]: unknown SDK instance %q", binding.Name, binding.InstanceID)
			}
			var selected *locationsdk.CompiledPort
			for index := range level.Ports {
				port := &level.Ports[index]
				if port.InstanceID == binding.InstanceID && port.ID == binding.PortID {
					if selected != nil {
						return c, fmt.Errorf("pipeline.port_locators[%q]: ambiguous port %q on %q", binding.Name, binding.PortID, binding.InstanceID)
					}
					selected = port
				}
			}
			if selected == nil || len(selected.Approach) == 0 {
				return c, fmt.Errorf("pipeline.port_locators[%q]: port %q on %q needs an authored public approach cell", binding.Name, binding.PortID, binding.InstanceID)
			}
			at := selected.Approach[0]
			if binding.Kind == "service" {
				at, _ = locationsdk.PublicServiceApproach(*selected)
			}
			c.Locators = append(c.Locators, Locator{Name: binding.Name, InstanceID: binding.InstanceID, Kind: binding.Kind, PositionPlanM: placement.Vec3{(float64(at.X) + .5) * c.UnitM, instance.WorldTransform.Position[1], (float64(at.Y) + .5) * c.UnitM}})
		}
	}
	// Every CUE text anchor is required, even if the pipeline omitted it.
	exists := map[string]bool{}
	for _, l := range c.Locators {
		exists[l.InstanceID+"/"+l.Name] = true
	}
	for _, s := range layout.TextSlots {
		name := s.Anchor
		if name == "" {
			name = s.ID
		}
		if exists[s.Instance+"/"+name] {
			continue
		}
		p := placement.Vec3{float64(s.CenterMM.X) / 1000, float64(s.CenterMM.Z) / 1000, float64(s.CenterMM.Y) / 1000}
		if i, ok := manifest.Instances[s.Instance]; ok {
			b := i.WorldTransform.BasisColumns()
			v := placement.Vec3{}
			for j := range v {
				v[j] = i.WorldTransform.Position[j] + b[0][j]*p[0] + b[1][j]*p[1] + b[2][j]*p[2]
			}
			p = placement.Vec3{v[0], v[1], -v[2]}
		}
		c.Locators = append(c.Locators, Locator{Name: name, InstanceID: s.Instance, Kind: "text", PositionPlanM: p})
	}
	seen := map[string]bool{}
	for _, l := range c.Locators {
		key := l.InstanceID + "/" + l.Name
		if l.Name == "" || strings.ContainsAny(l.Name, "/\n\r\x00") || seen[key] {
			return c, fmt.Errorf("pipeline.locators: duplicate/invalid locator %q of %q", l.Name, l.InstanceID)
		}
		seen[key] = true
		if l.InstanceID != "" {
			if _, ok := c.Identities[l.InstanceID]; !ok {
				return c, fmt.Errorf("pipeline.locators[%q]: unknown owner %q", l.Name, l.InstanceID)
			}
		}
		known := false
		for _, k := range conv.LocatorKinds {
			known = known || k == l.Kind
		}
		if !known {
			return c, fmt.Errorf("pipeline.locators[%q]: kind must be text, light, service, exit or rail", l.Name)
		}
		for _, v := range l.PositionPlanM {
			if !finite(v) {
				return c, fmt.Errorf("pipeline.locators[%q]: non-finite position", l.Name)
			}
		}
	}
	focus := placement.Vec3{float64(level.Width) * c.UnitM / 2, float64(level.Elevation) * c.UnitM, float64(level.Depth) * c.UnitM / 2}
	distance := 120.
	if camera != nil {
		focus = camera.FocusPlanM
		distance = camera.DistanceM
	}
	p := placement.FromPlan(focus)
	b := placement.CameraBasis(35, 37)
	for j := range p {
		p[j] += b[2][j] * distance
	}
	c.CameraWorld = Matrix{{b[0][0], b[1][0], b[2][0], p[0]}, {b[0][1], b[1][1], b[2][1], p[1]}, {b[0][2], b[1][2], b[2][2], p[2]}, {0, 0, 0, 1}}
	if err := refreshSemanticHash(&c); err != nil {
		return c, err
	}

	return c, nil
}
func refreshSemanticHash(contract *Contract) error {
	c := *contract
	c.GeometryContractSHA = ""
	semantic := c
	semantic.Identities = map[string]Identity{}
	for id, q := range c.Identities {
		q.Asset = ""
		q.SourceAsset = ""
		q.AssetBasis = ""
		q.SourceAssetBasis = ""
		semantic.Identities[id] = q
	}
	raw, err := json.Marshal(semantic)
	if err != nil {
		return err
	}
	h := sha256.Sum256(raw)
	contract.GeometryContractSHA = hex.EncodeToString(h[:])
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Files is deterministic: a contract and its runtime never embed output paths.
func Files(c Contract) (map[string][]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string][]byte{"authoring.json": append(data, '\n'), "convention.json": conventionData, "runtime.py": []byte(runtime), "start.py": []byte(start), "export.py": []byte(exporter), "assets.py": []byte(assets), "import.py": []byte(importer)}, nil
}

type ArtReport struct {
	PartialIDs          []string `json:"partial_ids"`
	Schema              string   `json:"schema"`
	ContractSHA         string   `json:"contract_sha256"`
	GeometryContractSHA string   `json:"geometry_contract_sha256"`
	Validation          struct {
		Errors     []string `json:"errors"`
		MissingArt []string `json:"missing_art"`
		Warnings   []string `json:"warnings"`
	} `json:"validation"`
}

// ValidateReport prevents generation from silently skipping authoring validation.
func ValidateReport(c Contract, data []byte) ([]string, error) {
	var r ArtReport
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("art report: %w", err)
	}
	if len(r.PartialIDs) > 0 {
		return nil, fmt.Errorf("art report: --only-id is a partial kit export; validate the complete level before generation")
	}
	if r.Schema != "ang.blender-export.v1" || r.GeometryContractSHA != c.GeometryContractSHA {
		return nil, fmt.Errorf("art report: stale contract; rerun ID export from this CUE layout")
	}
	if len(r.Validation.Errors)+len(r.Validation.MissingArt) > 0 {
		return nil, fmt.Errorf("art report: %s", strings.Join(append(r.Validation.Errors, r.Validation.MissingArt...), "; "))
	}
	return r.Validation.Warnings, nil
}

// ToolFiles can be emitted before a Blender-first layout has any compiled floor.
func ToolFiles() map[string][]byte {
	return map[string][]byte{"convention.json": conventionData, "runtime.py": []byte(runtime), "start.py": []byte(start), "export.py": []byte(exporter), "assets.py": []byte(assets), "import.py": []byte(importer)}
}
