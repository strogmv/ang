package blender

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

const authorityGeometry = `generated_geometry:{
 source_kind:"blend",source_sha256:"%x"
 width:4,depth:4,elevation:36
 floor:[{id:"floor",rect:{x:1,y:2,w:2,h:2}}]
 instances:[{id:"kit",module_id:"module",at:{x:2,y:3},rotation:0}]
 obstacles:[{id:"solid",surface:"floor",rect:{x:1,y:2,w:1,h:1},bottom_mm:0,top_mm:1000}]
 extras:[{instance_id:"lamp",model_ref:"lamp",source_kind:"authored_fixture",position_plan_m:[2,18,3],rotation_degrees:0}]
 locators:[{name:"lamp",instance_id:"lamp",kind:"light",position_plan_m:[2,18,3]}]
}`

const authorityLayout = `package geometry
import "list"
catalog:sdk_layout:{
 id:"test.layout"
 width:generated_geometry.width,depth:generated_geometry.depth,elevation:generated_geometry.elevation
 floor:generated_geometry.floor
 instances:list.Concat([generated_geometry.instances,[{id:"edge",module_id:"guard",at:{x:generated_geometry.floor[0].rect.x+1,y:generated_geometry.floor[0].rect.y},rotation:0}]])
 obstacles:generated_geometry.obstacles
}
`

const authorityPresentation = `package presentation
import geo "authority.local/geometry"
authoring:{pose_source:"blend",generated_cue:"../geometry/generated.cue",blend:"../author.blend"}
pipeline:{instances:edge:derived:true,locators:geo.generated_geometry.locators}
placement:extras:geo.generated_geometry.extras
_lamps:[for lamp in geo.generated_geometry.extras {id:lamp.instance_id,at:lamp.position_plan_m}]
finishing:lights:[for lamp in _lamps {
 id:lamp.id,at_plan_m:lamp.at
 aim_plan_m:[lamp.at[0]+0.1,geo.generated_geometry.elevation*500/1000+0.2,lamp.at[2]]
}]
`

func authorityFixture(t *testing.T, layout, presentation string, geometryOverride ...string) (PoseAuthority, string, cue.Value, cue.Value) {
	t.Helper()
	dir := t.TempDir()
	source := []byte("Blender source for authority tests")
	geometry := authorityGeometry
	if len(geometryOverride) != 0 {
		geometry = geometryOverride[0]
	}
	files := map[string]string{
		"cue.mod/module.cue":       "module:\"authority.local\"\nlanguage:version:\"v0.17.0\"\n",
		"author.blend":             string(source),
		"geometry/generated.cue":   "// Generated from the ID-tagged Blender source by ANG; do not hand-edit.\npackage geometry\n" + fmt.Sprintf(geometry, sha256.Sum256(source)),
		"geometry/layout.cue":      layout,
		"presentation/profile.cue": presentation,
		"sdk/layout.cue": `package sdk
import geo "authority.local/geometry"
#Point:{x:int & >=0,y:int & >=0}
#Instance:{id:string,module_id:string,at:#Point,rotation:0|90|180|270}
sdk:layouts:[geo.catalog.sdk_layout & {instances:[...#Instance]}]
`,
	}
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	loadValue := func(name string) cue.Value {
		t.Helper()
		instances := load.Instances([]string{"."}, &load.Config{Dir: filepath.Join(dir, name)})
		v := cuecontext.New().BuildInstance(instances[0])
		if err := v.Err(); err != nil {
			t.Fatal(err)
		}
		return v
	}
	profile := PoseAuthority{PoseSource: "blend", GeneratedCUE: "../geometry/generated.cue", Blend: "../author.blend"}
	return profile, filepath.Join(dir, "presentation"), loadValue("sdk").LookupPath(cue.ParsePath("sdk.layouts[0]")), loadValue("presentation")
}

func TestBlendPoseSourcesAllowGeneratedReferencesDerivedGeometryAndFixtureOffsets(t *testing.T) {
	profile, dir, layout, presentation := authorityFixture(t, authorityLayout, authorityPresentation)
	if err := ValidatePoseAuthority(profile, dir, ""); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePoseSources(profile, dir, layout, presentation); err != nil {
		t.Fatal(err)
	}
}

func TestBlendPoseSourcesRejectEqualLiteralAndAliasOwners(t *testing.T) {
	for _, test := range []struct {
		name         string
		layout       string
		presentation string
		path         string
	}{
		{"floor_literal", "catalog:sdk_layout:floor:[{rect:x:1}]", "", "floor"},
		{"floor_alias", "_hand:1\ncatalog:sdk_layout:floor:[{rect:x:_hand}]", "", "floor"},
		{"instance_alias", "_hand:2\ncatalog:sdk_layout:instances:[{at:x:_hand},...]", "", "instances"},
		{"rotation_zero", "catalog:sdk_layout:instances:[{rotation:0},...]", "", "rotation"},
		{"obstacle_literal", "catalog:sdk_layout:obstacles:[{top_mm:1000}]", "", "obstacles"},
		{"extra_literal", "", "placement:extras:[{position_plan_m:[2,18,3]}]", "placement.extras"},
		{"extra_alias", "", "_hand:[2,18,3]\nplacement:extras:[{position_plan_m:_hand}]", "placement.extras"},
		{"light_literal", "", "finishing:lights:[{at_plan_m:[2,18,3]}]", "finishing.lights"},
		{"light_alias", "", "_hand:[2,18,3]\nfinishing:lights:[{at_plan_m:_hand}]", "finishing.lights"},
		{"light_aim_alias", "", "_hand:[2.1,18.2,3]\nfinishing:lights:[{aim_plan_m:_hand}]", "finishing.lights"},
		{"locator_alias", "", "_hand:[2,18,3]\npipeline:locators:[{position_plan_m:_hand}]", "pipeline.locators"},
		{"dimension_alias", "_hand:4\ncatalog:sdk_layout:width:_hand", "", "width"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile, dir, layout, presentation := authorityFixture(t, authorityLayout+test.layout, authorityPresentation+test.presentation)
			err := ValidatePoseSources(profile, dir, layout, presentation)
			if err == nil || !strings.Contains(err.Error(), test.path) || !strings.Contains(err.Error(), ".cue:") {
				t.Fatalf("second pose owner not named: %v", err)
			}
		})
	}
}

func TestBlendPoseSourcesRejectRewiredGeometryAndHandEmptyLists(t *testing.T) {
	for _, test := range []struct{ name, layout string }{
		{"rewired_floor", strings.Replace(authorityLayout, "floor:generated_geometry.floor", "floor:[{id:\"floor\",rect:{x:0,y:0,w:1,h:1}}]", 1)},
		{"rewired_instance", strings.Replace(authorityLayout, "generated_geometry.instances,", "[{id:\"kit\",module_id:\"module\",at:{x:1,y:1},rotation:0}],", 1)},
		{"hand_empty_obstacles", strings.Replace(authorityLayout, "obstacles:generated_geometry.obstacles", "obstacles:[]", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile, dir, layout, presentation := authorityFixture(t, test.layout, authorityPresentation)
			if err := ValidatePoseSources(profile, dir, layout, presentation); err == nil {
				t.Fatal("hand geometry was accepted")
			}
			profile.PoseSource = "cue"
			if err := ValidatePoseSources(profile, dir, layout, presentation); err != nil {
				t.Fatalf("CUE authoring was restricted: %v", err)
			}
		})
	}
}

func TestBlendPoseSourcesRejectEqualEmptyListOwner(t *testing.T) {
	geometry := strings.Replace(authorityGeometry, `obstacles:[{id:"solid",surface:"floor",rect:{x:1,y:2,w:1,h:1},bottom_mm:0,top_mm:1000}]`, "obstacles:[]", 1)
	profile, dir, layout, presentation := authorityFixture(t, authorityLayout+"catalog:sdk_layout:obstacles:[]\n", authorityPresentation, geometry)
	if err := ValidatePoseSources(profile, dir, layout, presentation); err == nil || !strings.Contains(err.Error(), "empty pose list") {
		t.Fatalf("matching empty hand list was accepted: %v", err)
	}
}

func TestBlendDerivedInstancesRequireBothImportedCoordinateDependencies(t *testing.T) {
	for _, pose := range []string{
		`at:{x:99,y:99}`,
		`at:{x:2,y:2}`, // same values as the valid rule still have a second owner
		`at:{x:_hand,y:_hand}`,
		`at:{x:generated_geometry.floor[0].rect.x+1,y:2}`,
	} {
		t.Run(pose, func(t *testing.T) {
			layout := strings.Replace(authorityLayout, `at:{x:generated_geometry.floor[0].rect.x+1,y:generated_geometry.floor[0].rect.y}`, pose, 1) + "_hand:99\n"
			profile, dir, actual, presentation := authorityFixture(t, layout, authorityPresentation)
			err := ValidatePoseSources(profile, dir, actual, presentation)
			if err == nil || !strings.Contains(err.Error(), `instances["edge"].at.`) || !strings.Contains(err.Error(), ".cue:") {
				t.Fatalf("derived flag authorized a literal pose: %v", err)
			}
		})
	}
}

func TestBlendDerivedFlagCannotAuthorizeOtherGeometryKinds(t *testing.T) {
	for _, test := range []struct{ name, layout, presentation, path string }{
		{"floor", strings.Replace(authorityLayout, "floor:generated_geometry.floor", `floor:list.Concat([generated_geometry.floor,[{id:"new",rect:{x:0,y:0,w:1,h:1}}]])`, 1), authorityPresentation, "floor"},
		{"obstacle", strings.Replace(authorityLayout, "obstacles:generated_geometry.obstacles", `obstacles:list.Concat([generated_geometry.obstacles,[{id:"new",surface:"floor",rect:{x:0,y:0,w:1,h:1},bottom_mm:0,top_mm:1000}]])`, 1), authorityPresentation, "obstacles"},
		{"extra", authorityLayout, strings.Replace(authorityPresentation, "placement:extras:geo.generated_geometry.extras", `placement:extras:[geo.generated_geometry.extras[0],{instance_id:"new",position_plan_m:[2,18,3],rotation_degrees:0}]`, 1), "placement.extras"},
		{"locator", authorityLayout, strings.Replace(authorityPresentation, "locators:geo.generated_geometry.locators", `locators:[geo.generated_geometry.locators[0],{name:"new",instance_id:"lamp",position_plan_m:[2,18,3]}]`, 1), "pipeline.locators"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile, dir, layout, presentation := authorityFixture(t, test.layout, test.presentation+"\npipeline:instances:new:derived:true\n")
			err := ValidatePoseSources(profile, dir, layout, presentation)
			if err == nil || !strings.Contains(err.Error(), test.path) || !strings.Contains(err.Error(), ".cue:") {
				t.Fatalf("derived flag bypassed %s ownership: %v", test.name, err)
			}
		})
	}
}

func TestBlendDerivedCoordinateTracksGenericComprehensionBuiltinBindings(t *testing.T) {
	layout := `package geometry
import "list"
#Edges:{
 floor:[...{rect:{x:int,y:int,w:int,h:int},...}]
 _cells:[for area in floor
  for col in list.Range(area.rect.x,area.rect.x+area.rect.w,1)
  for row in list.Range(area.rect.y,area.rect.y+area.rect.h,1) {x:col,y:row}]
 instances:[for c in _cells if c.x==floor[0].rect.x && c.y==floor[0].rect.y {
  id:"edge",module_id:"guard",at:{x:c.x,y:c.y},rotation:0
 }]
}
catalog:boundaries:#Edges & {floor:generated_geometry.floor}
catalog:sdk_layout:{
 id:"test.layout",width:generated_geometry.width,depth:generated_geometry.depth,elevation:generated_geometry.elevation
 floor:generated_geometry.floor,obstacles:generated_geometry.obstacles
 instances:list.Concat([generated_geometry.instances,catalog.boundaries.instances])
}
`
	profile, dir, actual, presentation := authorityFixture(t, layout, authorityPresentation)
	if err := ValidatePoseSources(profile, dir, actual, presentation); err != nil {
		t.Fatal(err)
	}
	// A same-value hand constraint must not inherit the computed row's origin.
	layout += "catalog:sdk_layout:instances:[_,{id:\"edge\",at:x:1}]\n"
	profile, dir, actual, presentation = authorityFixture(t, layout, authorityPresentation)
	if err := ValidatePoseSources(profile, dir, actual, presentation); err == nil {
		t.Fatal("equal literal constraint on a computed boundary was accepted")
	}
}
