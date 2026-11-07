// Package godot renders ANG placement manifests as Godot 4 text scenes.
package godot

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

//go:embed location.tscn.tmpl
var sceneTemplate string

type Options struct {
	// Models belongs to the client: stable model_ref -> res:// PackedScene.
	Models map[string]string
	Camera *placement.Camera
}

type resource struct{ ID, Path string }
type node struct{ Name, InstanceID, ModelRef, ResourceID, Transform string }
type box struct{ Name, SourceID, Position, Size, Material string }
type scene struct {
	LayoutID, Frame, CameraTransform string
	Resources                        []resource
	Nodes                            []node
	Boxes                            []box
	CameraSize, Elevation, Azimuth   float64
}

// Render validates all model bindings before rendering any output. No asset or
// path is inferred from a Unity registry or from an instance's spelling.
func Render(level locationsdk.CompiledLevel, manifest placement.Manifest, options Options) ([]byte, error) {
	if manifest.Schema != placement.Schema || manifest.WorldFrame != placement.WorldFrame || manifest.LayoutID != level.ID || manifest.StructuralHash != level.StructuralHash {
		return nil, fmt.Errorf("godot: manifest does not match compiled level")
	}
	s := scene{LayoutID: level.ID, Frame: placement.WorldFrame, Elevation: 35, Azimuth: 37}
	if level.Camera != nil {
		s.Elevation = level.Camera.ElevationDegrees
		s.Azimuth = level.Camera.AzimuthDegrees
	}
	unit := float64(level.UnitMM) / 1000
	framing := placement.Camera{FocusPlanM: placement.Vec3{float64(level.Width) * unit / 2, float64(level.Elevation) * unit, float64(level.Depth) * unit / 2}, SizeM: math.Max(float64(level.Width), float64(level.Depth)) * unit, DistanceM: 120}
	if options.Camera != nil {
		framing = *options.Camera
	}
	for _, v := range append(framing.FocusPlanM[:], framing.SizeM, framing.DistanceM) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("godot: non-finite camera")
		}
	}
	if framing.SizeM <= 0 || framing.DistanceM <= 0 {
		return nil, fmt.Errorf("godot: camera size and distance must be positive")
	}
	s.CameraSize = framing.SizeM
	basis := placement.CameraBasis(s.Elevation, s.Azimuth)
	focus := placement.FromPlan(framing.FocusPlanM)
	for j := range focus {
		focus[j] += basis[2][j] * framing.DistanceM
	}
	s.CameraTransform = transform(basis, focus)
	ids := make([]string, 0, len(manifest.Instances))
	refs := map[string]bool{}
	for id, i := range manifest.Instances {
		ids = append(ids, id)
		if i.ModelRef != "" {
			refs[i.ModelRef] = true
		}
	}
	sort.Strings(ids)
	models := make([]string, 0, len(refs))
	for ref := range refs {
		models = append(models, ref)
	}
	sort.Strings(models)
	resourceIDs := map[string]string{}
	for j, ref := range models {
		p, ok := options.Models[ref]
		if !ok || !strings.HasPrefix(p, "res://") || strings.ContainsAny(p, "\n\r\x00\\") || path.Clean(strings.TrimPrefix(p, "res://")) != strings.TrimPrefix(p, "res://") || strings.HasPrefix(strings.TrimPrefix(p, "res://"), "../") || strings.HasPrefix(strings.TrimPrefix(p, "res://"), "/") {
			return nil, fmt.Errorf("godot: model_ref %q needs a canonical res:// resource path", ref)
		}
		ext := strings.ToLower(path.Ext(p))
		if ext != ".glb" && ext != ".gltf" && ext != ".tscn" && ext != ".scn" {
			return nil, fmt.Errorf("godot: model_ref %q must map to a PackedScene", ref)
		}
		id := fmt.Sprintf("model_%03d", j)
		resourceIDs[ref] = id
		s.Resources = append(s.Resources, resource{id, p})
	}
	for j, id := range ids {
		i := manifest.Instances[id]
		s.Nodes = append(s.Nodes, node{fmt.Sprintf("Placement_%03d", j), id, i.ModelRef, resourceIDs[i.ModelRef], transform(i.WorldTransform.BasisColumns(), i.WorldTransform.Position)})
	}
	addBox := func(id string, r locationsdk.Rect, bottom, top int, material string) {
		if top <= bottom {
			return
		}
		p := placement.FromPlan(placement.Vec3{(float64(r.X) + float64(r.W)/2) * unit, float64(bottom+top) / 2000, (float64(r.Y) + float64(r.H)/2) * unit})
		s.Boxes = append(s.Boxes, box{fmt.Sprintf("Structure_%03d", len(s.Boxes)), id, vector(p), vector(placement.Vec3{float64(r.W) * unit, float64(top-bottom) / 1000, float64(r.H) * unit}), material})
	}
	if n := level.Navigation; n != nil {
		for _, surface := range n.Surfaces {
			if manifest.Instances[surface.ID].ModelRef != "" {
				continue
			}
			for _, r := range surface.Slab {
				addBox(surface.ID, r, surface.ElevationMM-surface.SlabMM, surface.ElevationMM, "floor")
			}
		}
		for _, o := range n.Obstacles {
			if manifest.Instances[o.ID].ModelRef == "" {
				addBox(o.ID, o.Rect, o.BottomMM, o.TopMM, "wall")
			}
		}
	} else {
		for _, r := range level.Preview.Floor {
			addBox("base", locationsdk.Rect{X: r.At.X, Y: r.At.Y, W: 1, H: 1}, level.Elevation*level.UnitMM-250, level.Elevation*level.UnitMM, "floor")
		}
	}
	t, err := template.New("location").Funcs(template.FuncMap{"quote": strconv.Quote}).Parse(sceneTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = t.Execute(&out, s); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func vector(v placement.Vec3) string {
	return fmt.Sprintf("Vector3(%s, %s, %s)", number(v[0]), number(v[1]), number(v[2]))
}
func number(v float64) string {
	if math.Abs(v) < 1e-12 {
		v = 0
	}
	return strconv.FormatFloat(v, 'f', 12, 64)
}
func transform(b [3]placement.Vec3, p placement.Vec3) string {
	values := make([]string, 0, 12)
	// TSCN's Transform3D scalar constructor serializes the basis ROWS, while
	// placement and Godot's Basis.x/y/z expose its columns.
	for row := range 3 {
		for column := range 3 {
			values = append(values, number(b[column][row]))
		}
	}
	for _, n := range p {
		values = append(values, number(n))
	}
	return "Transform3D(" + strings.Join(values, ", ") + ")"
}
