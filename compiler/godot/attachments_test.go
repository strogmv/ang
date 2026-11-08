package godot

import (
	"math"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestSourceArtAttachmentsPreserveLocalFrame(t *testing.T) {
	nodes := []Attachment{{Name: "textslot.sign", Position: placement.Vec3{1, 2, 3}, SourcePath: "asset/model/textslot.sign"}}
	c := AttachmentCatalog{Schema: "ang.model-attachments.v1", SourceFrame: "model-local", Models: map[string][]Attachment{"model": nodes}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	l := locationsdk.CompiledLevel{ID: "room", UnitMM: 500, Width: 20, Depth: 20, Instances: []locationsdk.CompiledInstance{{ID: "a", ModuleID: "model", Width: 2, Depth: 2, Rotation: 90}}}
	m, err := placement.Build(l, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Render(l, m, Options{Models: map[string]string{"model": "res://model.glb"}, WorldText: true, Attachments: c.Models})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`name="WorldText"`, `level_id = "room"`, `name="textslot.sign" type="Node3D" parent="Placements/Placement_000"`, `position = Vector3(1.000000000000, 2.000000000000, 3.000000000000)`} {
		if !strings.Contains(string(raw), part) {
			t.Fatal("lost local anchor/frame", part)
		}
	}
	c.Models["model"] = append(nodes, nodes[0])
	if c.Validate() == nil {
		t.Fatal("duplicate anchor accepted")
	}
	nodes[0].Position[0] = math.NaN()
	c.Models["model"] = nodes
	if c.Validate() == nil {
		t.Fatal("nonfinite anchor accepted")
	}
}
