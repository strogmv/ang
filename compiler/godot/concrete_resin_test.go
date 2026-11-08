package godot

import (
	"math"
	"strings"
	"testing"
)

func TestConcreteResinControlsPreserveUnspecifiedProfile(t *testing.T) {
	profile := ConcreteMaterial{
		Albedo: "res://concrete_diff.jpg", Normal: "res://concrete_normal.jpg", Roughness: "res://concrete_rough.jpg",
		TextureScale: .25, RoughnessBase: .18, NormalStrength: .25,
	}
	extBefore, before, err := profile.render("test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(before, "shader_parameter/resin_color") || strings.Contains(before, "shader_parameter/resin_metallic") {
		t.Fatal("an omitted resin control overrides the old shader defaults")
	}
	if !strings.Contains(concreteShader, "uniform vec3 resin_color = vec3(0.63,0.43,0.13);") || !strings.Contains(concreteShader, "uniform float resin_metallic = 0.7;") {
		t.Fatal("old resin defaults changed")
	}
	color, metallic := [3]float64{.95, .62, .12}, .25
	profile.ResinColor, profile.ResinMetallic = &color, &metallic
	extAfter, after, err := profile.render("test")
	if err != nil {
		t.Fatal(err)
	}
	colorLine := "shader_parameter/resin_color = Vector3(0.950000000000,0.620000000000,0.120000000000)\n"
	metallicLine := "shader_parameter/resin_metallic = 0.250000000000\n"
	if !strings.Contains(after, colorLine+metallicLine) {
		t.Fatalf("resin controls not emitted in key order:\n%s", after)
	}
	if extBefore != extAfter || before != strings.ReplaceAll(strings.ReplaceAll(after, colorLine, ""), metallicLine, "") {
		t.Fatal("resin controls changed unrelated emitted materials or parameters")
	}
	zeroColor, zeroMetallic := [3]float64{}, 0.0
	profile.ResinColor, profile.ResinMetallic = &zeroColor, &zeroMetallic
	_, zero, err := profile.render("test")
	if err != nil || !strings.Contains(zero, "shader_parameter/resin_color = Vector3(0.000000000000,0.000000000000,0.000000000000)\nshader_parameter/resin_metallic = 0.000000000000\n") {
		t.Fatalf("explicit zero must remain distinct from omitted: %v\n%s", err, zero)
	}
}

func TestConcreteResinControlsRejectOutOfRangeAndNonfinite(t *testing.T) {
	for _, value := range []float64{-.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		profile := ConcreteMaterial{
			Albedo: "res://concrete_diff.jpg", Normal: "res://concrete_normal.jpg", Roughness: "res://concrete_rough.jpg", TextureScale: .25,
		}
		color := [3]float64{.95, value, .12}
		profile.ResinColor = &color
		if _, _, err := profile.render("test.wall"); err == nil || !strings.Contains(err.Error(), "test.wall: resin_color") {
			t.Fatalf("invalid color %v needs actionable error, got %v", value, err)
		}
		profile.ResinColor, profile.ResinMetallic = nil, &value
		if _, _, err := profile.render("test.wall"); err == nil || !strings.Contains(err.Error(), "test.wall: resin_metallic") {
			t.Fatalf("invalid metallic %v needs actionable error, got %v", value, err)
		}
	}
}
