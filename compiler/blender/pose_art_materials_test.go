package blender

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/godot"
)

func materialUsageReport(t *testing.T, role, object, material string) []byte {
	t.Helper()
	component := map[string]any{"id": "authored.prop", "objects": []any{map[string]any{"name": object, "materials": []string{material}}}}
	if role != "" {
		component["applied_material_roles"] = map[string]string{object: role}
	}
	raw, err := json.Marshal(map[string]any{"schema": "ang.blender-export.v1", "geometry_contract_sha256": "current", "validation": map[string]any{}, "components": []any{component}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBlendArtMaterialUsageRejectsArtistOnlyPaletteWorldPoses(t *testing.T) {
	text := authorityPresentation + `
finishing:materials:palette:concrete:{instance:ghosts:[{id:"ghost",position_plan_m:[2,18,3]}]}
`
	profile, dir, layout, presentation := authorityFixture(t, authorityLayout, text)
	if err := ValidatePoseSources(profile, dir, layout, presentation); err != nil {
		t.Fatalf("unbound local palette was falsely rejected initially: %v", err)
	}
	contract := Contract{GeometryContractSHA: "current", Materials: map[string]godot.Material{"palette": {}}}
	for _, report := range [][]byte{
		materialUsageReport(t, "palette", "Blender_Casing_17", ""),
		materialUsageReport(t, "", "Blender_Casing_17", "ANG_ROLE__palette"),
		materialUsageReport(t, "", "Blender_Casing_17", "palette"),
	} {
		err := ValidateArtMaterialPoseSources(profile, dir, presentation, contract, report)
		if err == nil || !strings.Contains(err.Error(), "Blender_Casing_17") || !strings.Contains(err.Error(), "palette") || !strings.Contains(err.Error(), ".cue:") {
			t.Fatalf("artist-only role did not name its object and CUE pose: %v", err)
		}
	}
}

func TestBlendArtMaterialUsageResolvesOriginalMaterialBindings(t *testing.T) {
	text := authorityPresentation + `
finishing:{material_bindings:SourcePaint:"palette",materials:palette:concrete:{instance:floor_m:18}}
`
	profile, dir, _, presentation := authorityFixture(t, authorityLayout, text)
	contract := Contract{GeometryContractSHA: "current", Materials: map[string]godot.Material{"palette": {}}}
	err := ValidateArtMaterialPoseSources(profile, dir, presentation, contract, materialUsageReport(t, "", "Original_Source_Mesh", "SourcePaint"))
	if err == nil || !strings.Contains(err.Error(), "Original_Source_Mesh") || !strings.Contains(err.Error(), "floor_m") {
		t.Fatalf("source material binding escaped final pose ownership: %v", err)
	}
}

func TestBlendArtMaterialArtistOnlyRoleCanFollowImportedOrigins(t *testing.T) {
	text := authorityPresentation + `
finishing:materials:palette:concrete:{
 instance:floor_m:geo.generated_geometry.elevation*0.5
 anchor_data:[{id:"anchor",position_plan_m:geo.generated_geometry.extras[0].position_plan_m}]
}
`
	profile, dir, _, presentation := authorityFixture(t, authorityLayout, text)
	contract := Contract{GeometryContractSHA: "current", Materials: map[string]godot.Material{"palette": {}}}
	if err := ValidateArtMaterialPoseSources(profile, dir, presentation, contract, materialUsageReport(t, "palette", "Actual_Art_Mesh", "")); err != nil {
		t.Fatal(err)
	}
}

func TestBlendArtMaterialGeneratedScopesUseResolverMapInsteadOfNamePrefixes(t *testing.T) {
	profile, dir, _, presentation := authorityFixture(t, authorityLayout, authorityPresentation+importedFinishingOrigins)
	contract := Contract{GeometryContractSHA: "current", Materials: map[string]godot.Material{"scope_without_magic_prefix": {}, "concrete.instance.fake": {}}, Architecture: &godot.Architecture{ConcreteRoles: map[string]string{"default": "scope_without_magic_prefix"}}}
	if err := ValidateArtMaterialPoseSources(profile, dir, presentation, contract, materialUsageReport(t, "scope_without_magic_prefix", "Derived_Wall_Mesh", "")); err != nil {
		t.Fatal(err)
	}
	err := ValidateArtMaterialPoseSources(profile, dir, presentation, contract, materialUsageReport(t, "concrete.instance.fake", "Unscoped_Wall_Mesh", ""))
	if err == nil || !strings.Contains(err.Error(), "Unscoped_Wall_Mesh") || !strings.Contains(err.Error(), "architecture scope") {
		t.Fatalf("role-name prefix became an authority exception: %v", err)
	}
}

func TestBlendArtMaterialUsageRequiresCurrentCompleteReport(t *testing.T) {
	profile, dir, _, presentation := authorityFixture(t, authorityLayout, authorityPresentation)
	contract := Contract{GeometryContractSHA: "different"}
	if err := ValidateArtMaterialPoseSources(profile, dir, presentation, contract, materialUsageReport(t, "anything", "Any_Mesh", "")); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale usage supplied pose authority: %v", err)
	}
}
