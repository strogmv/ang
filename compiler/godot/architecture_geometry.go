package godot

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

//go:embed architecture_geometry.tscn.tmpl
var architectureGeometryTemplate string

//go:embed procedural_boxes.tscn.tmpl
var proceduralBoxesTemplate string

func parseGeometryTemplate(name, body string) (*template.Template, error) {
	return template.New(name).Funcs(template.FuncMap{"quote": strconv.Quote}).Parse(strings.TrimSpace(proceduralBoxesTemplate) + body)
}

// Camera skins never change the full geometry used by ResolveProceduralSolids.
func applyArchitectureNearCuts(boxes []box, finishing *Finishing) []box {
	if finishing != nil && finishing.Architecture != nil {
		for _, d := range finishing.Architecture.Draws {
			if d.NearCut == nil {
				continue
			}
			for i := range boxes {
				if boxes[i].SourceID != d.ID {
					continue
				}
				position, size := d.Position, d.Size
				position[1] -= (size[1] - d.NearCut.HeightM) / 2
				size[1] = d.NearCut.HeightM
				boxes[i].Position = vector(placement.FromPlan(placement.Vec3(position)))
				boxes[i].Size = vector(placement.Vec3(size))
			}
		}
	}
	return boxes
}

// RenderArchitectureGeometry gives another scene builder the exact common
// architecture draws, materials and camera skins. It includes no SDK floors,
// obstacles, models, lamps or camera, so an existing consumer can opt in once.
func RenderArchitectureGeometry(level locationsdk.CompiledLevel, manifest placement.Manifest, finishing *Finishing) ([]byte, error) {
	if err := validateSceneManifest(level, manifest); err != nil {
		return nil, err
	}
	resolved, err := ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return nil, err
	}
	header, bindings, _, roles, replaced, err := resolved.render(manifest, proceduralSurfaces(level, resolved))
	if err != nil {
		return nil, err
	}
	boxes, err := proceduralBoxes(level, manifest, resolved, roles, replaced, false)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	plainBindings := map[string]string{}
	if resolved != nil && resolved.Architecture != nil {
		for _, draw := range resolved.Architecture.Draws {
			ids[draw.ID] = true
			material := resolved.Materials[draw.MaterialRole]
			// The common adapter preserves CUE-owned non-emissive plain materials
			// against a legacy consumer's fallback, within this subscene only.
			if material.Resource == "" && material.Concrete == nil {
				if existing, ok := resolved.MaterialBindings[draw.MaterialRole]; ok {
					if existing != draw.MaterialRole {
						return nil, fmt.Errorf("architecture material %q conflicts with its named binding", draw.MaterialRole)
					}
				} else {
					plainBindings[draw.MaterialRole] = roles["object:"+draw.ID]
				}
			}
		}
	}
	selected := []box{}
	for _, draw := range boxes {
		if ids[draw.SourceID] {
			selected = append(selected, draw)
		}
	}
	names := []string{}
	for name := range plainBindings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		bindings += fmt.Sprintf("%s: %s,\n", strconv.Quote(name), plainBindings[name])
	}
	data := scene{LayoutID: level.ID, Frame: placement.WorldFrame, FinishHeader: header, FinishBindings: bindings, Boxes: applyArchitectureNearCuts(selected, resolved)}
	t, err := parseGeometryTemplate("architecture-geometry", architectureGeometryTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
