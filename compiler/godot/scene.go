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

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

//go:embed location.tscn.tmpl
var sceneTemplate string

type Options struct {
	// Models belongs to the client: stable model_ref -> res:// PackedScene.
	Finishing           *Finishing
	FloorOnly           bool
	WorldText           bool
	InstanceAttachments map[string][]Attachment
	Attachments         map[string][]Attachment
	Models              map[string]string
	Camera              *placement.Camera
}

type resource struct{ ID, Path string }
type node struct{ Name, InstanceID, ModelRef, ResourceID, Transform string }
type box struct{ Name, SourceID, Position, Size, Material string }
type floorWedge struct{ Name, SourceID, Transform, Material string }
type scene struct {
	ToneMapper                                 int
	FinishHeader, FinishBindings, FinishLights string
	FinishProfile                              string
	KeyEnergy                                  float64
	KeyLight                                   string
	FloorOnly                                  bool
	WorldText                                  bool
	LayoutID, Frame, CameraTransform           string
	Resources                                  []resource
	Attachments                                []attachmentNode
	Nodes                                      []node
	Boxes                                      []box
	FloorWedges                                []floorWedge
	CameraSize, Elevation, Azimuth             float64
}

// Render validates all model bindings before rendering any output. No asset or
// path is inferred from a Unity registry or from an instance's spelling.
func Render(level locationsdk.CompiledLevel, manifest placement.Manifest, options Options) ([]byte, error) {
	if err := validateSceneManifest(level, manifest); err != nil {
		return nil, err
	}
	var architectureErr error
	options.Finishing, architectureErr = ResolveArchitectureForLevel(options.Finishing, level)
	if architectureErr != nil {
		return nil, architectureErr
	}
	s := scene{KeyEnergy: .8, FloorOnly: options.FloorOnly, WorldText: options.WorldText, LayoutID: level.ID, Frame: placement.WorldFrame, Elevation: 35, Azimuth: 37}
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
		attachments := append([]Attachment{}, options.Attachments[i.ModelRef]...)
		for _, specific := range options.InstanceAttachments[id] {
			replaced := false
			for n, existing := range attachments {
				if existing.Name == specific.Name {
					attachments[n] = specific
					replaced = true
					break
				}
			}
			if !replaced {
				attachments = append(attachments, specific)
			}
		}
		for _, a := range attachments {
			s.Attachments = append(s.Attachments, attachmentNode{a.Name, fmt.Sprintf("Placements/Placement_%03d", j), vector(a.Position), a.SourcePath})
		}

	}
	// Semantic floor/draw identities do not need an independent model, but
	// their source-local locator frame remains the canonical zero plan frame.
	sourceIDs := map[string]bool{}
	for _, floor := range level.Preview.Floor {
		sourceIDs[floor.SourceID] = true
	}
	if level.Navigation != nil {
		sourceIDs[level.Navigation.BaseSurface] = true
	}
	if options.Finishing != nil && options.Finishing.Architecture != nil {
		for _, draw := range options.Finishing.Architecture.Draws {
			sourceIDs[draw.ID] = true
		}
	}
	extraIDs := make([]string, 0, len(options.InstanceAttachments))
	for id := range options.InstanceAttachments {
		if _, ok := manifest.Instances[id]; !ok {
			extraIDs = append(extraIDs, id)
		}
	}
	sort.Strings(extraIDs)
	for j, id := range extraIDs {
		if !sourceIDs[id] {
			return nil, fmt.Errorf("godot: instance attachment owner %q is absent from compiled CUE", id)
		}
		name := fmt.Sprintf("AttachmentRoot_%03d", j)
		pose := placement.PlanTransform(placement.Vec3{}, 0)
		s.Nodes = append(s.Nodes, node{Name: name, InstanceID: id, Transform: transform(pose.BasisColumns(), pose.Position)})
		for _, a := range options.InstanceAttachments[id] {
			s.Attachments = append(s.Attachments, attachmentNode{a.Name, "Placements/" + name, vector(a.Position), a.SourcePath})
		}
	}
	surfaces := proceduralSurfaces(level, options.Finishing)
	header, bindings, lights, roles, replaced, err := options.Finishing.render(manifest, surfaces)
	if err != nil {
		return nil, err
	}
	s.FinishHeader = header
	s.FinishBindings = bindings
	s.FinishLights = lights
	if options.Finishing != nil {
		if options.Finishing.Environment != nil {
			s.ToneMapper = options.Finishing.Environment.ToneMapper
		}
		s.FinishProfile, err = options.Finishing.Environment.render()
		if err != nil {
			return nil, err
		}
		if e := options.Finishing.Environment; e != nil && !e.DefaultFill {
			s.KeyEnergy = 0
		}
		if options.Finishing.KeyLight != nil {
			s.KeyLight, err = options.Finishing.KeyLight.render("Key", "LightingRig")
			if err != nil {
				return nil, err
			}
		}
	}
	boxes, err := proceduralBoxes(level, manifest, options.Finishing, roles, replaced, options.FloorOnly)
	if err != nil {
		return nil, err
	}
	s.Boxes = applyArchitectureNearCuts(boxes, options.Finishing)
	floorDraws, err := ResolveFloorDraws(level, manifest, options.Finishing)
	if err != nil {
		return nil, err
	}
	for _, draw := range floorDraws {
		if draw.Triangle != nil {
			ref := roles["object:"+draw.SourceID]
			if ref == "" {
				ref = roles["floor"]
			}
			if ref == "" {
				ref = "SubResource(\"floor\")"
			}
			s.FloorWedges = append(s.FloorWedges, floorWedge{fmt.Sprintf("FloorCorner_%03d", len(s.FloorWedges)), draw.SourceID, draw.GodotTransform, ref})
		}
	}
	t, err := parseGeometryTemplate("location", sceneTemplate)
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
