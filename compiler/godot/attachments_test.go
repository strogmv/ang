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

func TestSemanticInstanceAttachmentsHaveCanonicalNativeRoots(t *testing.T) {
	nodes := []Attachment{{Name: "exit.public", Position: placement.Vec3{1.25, 3, 2.75}, SourcePath: "source/exit.public"}}
	catalog := AttachmentCatalog{Schema: "ang.model-attachments.v1", SourceFrame: "model-local", Instances: map[string][]Attachment{"floor.area": nodes}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	level := locationsdk.CompiledLevel{ID: "room", UnitMM: 500, Width: 8, Depth: 8, Preview: locationsdk.PreviewData{Floor: []locationsdk.PreviewFloor{{SourceID: "floor.area"}}}}
	manifest, err := placement.Build(level, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Render(level, manifest, Options{InstanceAttachments: catalog.Instances})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`name="AttachmentRoot_000"`, `metadata/instance_id = "floor.area"`, `name="exit.public" type="Node3D" parent="Placements/AttachmentRoot_000"`, `position = Vector3(1.250000000000, 3.000000000000, 2.750000000000)`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing semantic local frame %s", want)
		}
	}
	catalog.Instances["floor.area"] = append(nodes, nodes[0])
	if catalog.Validate() == nil {
		t.Fatal("duplicate semantic locator accepted")
	}
	if _, err = Render(level, manifest, Options{InstanceAttachments: map[string][]Attachment{"unknown": nodes}}); err == nil {
		t.Fatal("unknown semantic attachment owner accepted")
	}
}
