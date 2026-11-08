package blender

import (
	"strings"
	"testing"
)

const importedFinishingOrigins = `
finishing:architecture:{
 draws:[{id:"wall",position_plan_m:geo.generated_geometry.extras[0].position_plan_m,size_m:[1,2,0.2],material_role:"concrete.bronze"}]
 source_anchors:[for locator in geo.generated_geometry.locators {id:locator.name,source_ref:locator.instance_id,position_plan_m:locator.position_plan_m}]
 concrete:{floor_m:geo.generated_geometry.elevation*0.5
  ghosts:[{id:"ghost",source_ref:"kit",position_plan_m:geo.generated_geometry.extras[0].position_plan_m,size_m:[1,1]}]
  stripe_segments:[{id:"stripe",source_ref:"kit",from_plan_m:geo.generated_geometry.extras[0].position_plan_m,to_plan_m:geo.generated_geometry.extras[0].position_plan_m}]
  openings:[{id:"opening",source_ref:"kit",center_plan_m:geo.generated_geometry.extras[0].position_plan_m,tangent_plan:[1,0,0],width_m:1,height_m:2}]
  caries:hints:[{id:"hint",source_ref:"kit",position_plan_m:geo.generated_geometry.extras[0].position_plan_m}]
 }
}
`

func TestBlendFinishingOriginsFollowImportedGeometry(t *testing.T) {
	profile, dir, layout, presentation := authorityFixture(t, authorityLayout, authorityPresentation+importedFinishingOrigins)
	if err := ValidatePoseSources(profile, dir, layout, presentation); err != nil {
		t.Fatal(err)
	}
}

func TestBlendFinishingConversionsAllowConstrainedSemanticUnits(t *testing.T) {
	text := authorityPresentation + `
_unit:500 & 500
finishing:architecture:draws:[{
 id:"derived.wall",size_m:[1,2,0.2]
 position_plan_m:[geo.generated_geometry.floor[0].rect.x*_unit/1000,geo.generated_geometry.elevation*_unit/1000+1,geo.generated_geometry.floor[0].rect.y*_unit/1000]
}]
`
	profile, dir, layout, presentation := authorityFixture(t, authorityLayout, text)
	if err := ValidatePoseSources(profile, dir, layout, presentation); err != nil {
		t.Fatal(err)
	}
}

func TestBlendFinishingRejectsEqualLiteralAndAliasOrigins(t *testing.T) {
	for _, test := range []struct{ name, hand, path string }{
		{"draw", `draws:[{position_plan_m:_hand}]`, "architecture.draws"},
		{"anchor", `source_anchors:[{position_plan_m:_hand}]`, "architecture.source_anchors"},
		{"floor", `concrete:floor_m:18.0`, "concrete.floor_m"},
		{"ghost", `concrete:ghosts:[{position_plan_m:_hand}]`, "concrete.ghosts"},
		{"stripe_from", `concrete:stripe_segments:[{from_plan_m:_hand}]`, "stripe_segments"},
		{"stripe_to", `concrete:stripe_segments:[{to_plan_m:_hand}]`, "stripe_segments"},
		{"opening", `concrete:openings:[{center_plan_m:_hand}]`, "concrete.openings"},
		{"caries", `concrete:caries:hints:[{position_plan_m:_hand}]`, "caries.hints"},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := authorityPresentation + importedFinishingOrigins + "\n_hand:[2,18,3]\nfinishing:architecture:{" + test.hand + "}\n"
			profile, dir, layout, presentation := authorityFixture(t, authorityLayout, text)
			err := ValidatePoseSources(profile, dir, layout, presentation)
			if err == nil || !strings.Contains(err.Error(), test.path) || !strings.Contains(err.Error(), ".cue:") {
				t.Fatalf("second finishing pose owner not named: %v", err)
			}
		})
	}
}

func TestBlendFinishingRejectsNewPrimitiveAndScopedMaterialOrigins(t *testing.T) {
	for _, test := range []struct{ name, text, path string }{
		{"literal_draw", `finishing:architecture:draws:[{id:"new",position_plan_m:[99,18,99],size_m:[1,2,0.2]}]`, "architecture.draws"},
		{"missing_draw_origin", `finishing:architecture:draws:[{id:"new",size_m:[1,2,0.2]}]`, "position_plan_m"},
		{"zone", `finishing:architecture:wall_zones:z:concrete:ghosts:[{id:"g",position_plan_m:[2,18,3]}]`, "wall_zones"},
		{"wall", `finishing:architecture:walls:w:floor_m:18`, "architecture.walls"},
		{"material_anchor", `finishing:{surface_roles:wall:"selected",materials:selected:concrete:{instance:{},anchor_data:[{id:"a",position_plan_m:[2,18,3]}]}}`, "anchor_data"},
		{"material_patch", `finishing:{object_roles:kit:"selected",materials:selected:concrete:{instance:{},patch_anchors:[{id:"a",position_plan_m:[2,18,3]}]}}`, "patch_anchors"},
		{"material_route", `finishing:{material_bindings:source:"selected",materials:selected:concrete:{instance:{},route_data:[{source:"route",from_plan_m:[2,18,3],to_plan_m:[3,18,3]}]}}`, "route_data"},
		{"material_ghost", `finishing:{surface_roles:wall:"selected",materials:selected:concrete:instance:ghosts:[{id:"g",position_plan_m:[2,18,3]}]}`, "ghosts"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile, dir, layout, presentation := authorityFixture(t, authorityLayout, authorityPresentation+test.text)
			err := ValidatePoseSources(profile, dir, layout, presentation)
			if err == nil || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("hand finishing origin accepted: %v", err)
			}
		})
	}
}

func TestBlendFinishingAllowsUnboundLocalPaletteAndCanonicallyDerivedRoutes(t *testing.T) {
	text := authorityPresentation + `
finishing:materials:unused:concrete:{
 instance:{floor_m:0,ghosts:[{id:"fixture",position_plan_m:[3,1,0]}]}
 anchor_data:[{id:"fixture",position_plan_m:[0,1,0]}]
 fixture_draws:[{id:"local",position_plan_m:[0,1,0],size_m:[1,2,0.1]}]
}
finishing:{surface_roles:wall:"selected",materials:selected:concrete:{
 instance:handwear:true
 route_data:[{source:"shadowed",from_plan_m:[0,1,0],to_plan_m:[3,1,0]}]
}}
`
	profile, dir, layout, presentation := authorityFixture(t, authorityLayout, text)
	if err := ValidatePoseSources(profile, dir, layout, presentation); err != nil {
		t.Fatal(err)
	}
	// The same palette becomes level data once explicitly selected for art.
	profile, dir, layout, presentation = authorityFixture(t, authorityLayout, text+"\nfinishing:material_bindings:actual_source:\"unused\"\n")
	if err := ValidatePoseSources(profile, dir, layout, presentation); err == nil {
		t.Fatal("explicitly bound local palette retained hand world origins")
	}
}
