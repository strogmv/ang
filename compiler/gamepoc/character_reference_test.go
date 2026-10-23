package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReferenceCharacterFrameFlipAndRuntimeBindings(t *testing.T) {
	frame := CharacterFrame{Row: 0, Column: 1, X: 10, Y: 20, W: 100, H: 300, FeetX: 40, FeetY: 295, BodyHeightPX: 280, FlipX: true}
	data, err := json.Marshal(frame)
	if err != nil || !strings.Contains(string(data), `"flip_x":true`) {
		t.Fatalf("frame flip lost in projection: %s %v", data, err)
	}
	source, err := sources.ReadFile("templates/unity.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"ResolveCharacterSheet(id, out key)",
		"GaitHash(id)%(uint)defaults.Length",
		"uv.width=-uv.width",
		"feetX=frameW-feetX",
		"characterFrames[sheet.id]=lookup",
		"DrawSpatialFloorLight(x,y,at,tw,th,visibility=='1')",
		"if(ordinaryFloor)floor=SpatialFloorTexture",
		"if (verifySdk) { SdkProofStep(now); return; }",
	} {
		if !strings.Contains(string(source), fragment) {
			t.Errorf("missing reference integration: %s", fragment)
		}
	}
}
