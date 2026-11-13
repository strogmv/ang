package godot

import (
	"math"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/placement"
)

func sideKey() *KeyLight {
	return &KeyLight{Source: "lgc.deflected", AzimuthDegrees: -53, ElevationDegrees: 40,
		Color:  [3][3]float64{{1, .7, .45}, {1, .6, .35}, {.5, .6, 1}},
		Energy: [3]float64{2.4, 1, .3}, ShadowAngularDegrees: 2.3, ShadowBlur: 1, ShadowBias: .1, ShadowNormalBias: 2,
		ShadowDepthM: [2]float64{60, 200}, PeopleShadow: true,
		FillElevationDegrees: 60, FillColor: [3][3]float64{{.4, .5, .8}, {.4, .5, .8}, {.2, .3, .6}}, FillEnergy: [3]float64{.5, .3, .1},
		Provenance: "market_hub lighting.cue guides"}
}

// The key's -Z (the way it shines) comes down at its elevation from its azimuth, in the
// camera's own convention: a key 90 degrees off the camera azimuth lies across the view.
func TestKeyLightShinesFromItsSideAtItsElevation(t *testing.T) {
	k := sideKey()
	b := k.Basis()
	toLight := b[2]
	if el := math.Asin(toLight[1]) * 180 / math.Pi; math.Abs(el-40) > 1e-9 {
		t.Fatal("elevation", el)
	}
	camera := placement.CameraBasis(35, 37)[2]
	h := math.Hypot(toLight[0], toLight[2]) * math.Hypot(camera[0], camera[2])
	if dot := (toLight[0]*camera[0] + toLight[2]*camera[2]) / h; math.Abs(dot) > 1e-9 {
		t.Fatal("a key at camera azimuth - 90 must lie across the camera's view on the floor", dot)
	}
	// The fill comes from the opposite side, high up, and casts nothing.
	fill := k.FillBasis()[2]
	if fill[0]*toLight[0]+fill[2]*toLight[2] >= 0 || math.Abs(math.Asin(fill[1])*180/math.Pi-60) > 1e-9 {
		t.Fatal("fill must come from the side opposite the key at its own elevation", fill)
	}
	// Screen right of the camera is the side this key comes from.
	if right := placement.CameraBasis(35, 37)[0]; toLight[0]*right[0]+toLight[2]*right[2] <= 0 {
		t.Fatal("key -53 must come from the frame's right")
	}
	out, err := k.render("Key", "LightingRig")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`type="DirectionalLight3D" parent="LightingRig"`, "shadow_enabled = true", "light_angular_distance = 2.300000000000",
		"directional_shadow_mode = 0", `metadata/key_source = "lgc.deflected"`, "metadata/key_energy = Vector3(2.400000000000, 1.000000000000, 0.300000000000)",
		"metadata/key_people_shadow = true", "metadata/key_shadow_depth_m = Vector2(60.000000000000, 200.000000000000)", "metadata/key_color_night = Color(",
		`[node name="KeyFill" type="DirectionalLight3D" parent="LightingRig"]`, "metadata/key_fill = true"} {
		if !strings.Contains(out, part) {
			t.Fatal("missing", part, out)
		}
	}
}

func TestKeyLightRejectsUnusableData(t *testing.T) {
	for name, edit := range map[string]func(*KeyLight){
		"no source":     func(k *KeyLight) { k.Source = "" },
		"overhead":      func(k *KeyLight) { k.ElevationDegrees = 88 },
		"grazing":       func(k *KeyLight) { k.ElevationDegrees = 3 },
		"hdr colour":    func(k *KeyLight) { k.Color[1][0] = 1.2 },
		"negative":      func(k *KeyLight) { k.Energy[2] = -1 },
		"huge penumbra": func(k *KeyLight) { k.ShadowAngularDegrees = 12 },
		"depth":         func(k *KeyLight) { k.ShadowDepthM = [2]float64{200, 60} },
		"fill below":    func(k *KeyLight) { k.FillElevationDegrees = 0 },
		"fill colour":   func(k *KeyLight) { k.FillColor[0][2] = 2 },
		"nan":           func(k *KeyLight) { k.AzimuthDegrees = math.NaN() },
	} {
		k := sideKey()
		edit(k)
		if _, err := RenderKeyLight(k, "Key", "."); err == nil {
			t.Fatal("accepted", name)
		}
	}
}

// A finishing with a key renders it in place of the placeholder key; without one the scene keeps
// the old placeholder.
func TestKeyLightReplacesPlaceholderInScene(t *testing.T) {
	f := &Finishing{KeyLight: sideKey()}
	s := scene{}
	var err error
	s.KeyLight, err = f.KeyLight.render("Key", "LightingRig")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := parseGeometryTemplate("location", sceneTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var with, without strings.Builder
	if err = tmpl.Execute(&with, s); err != nil {
		t.Fatal(err)
	}
	if err = tmpl.Execute(&without, scene{KeyEnergy: .8}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(with.String(), `[node name="Key"`) != 1 || strings.Count(with.String(), `[node name="KeyFill"`) != 1 || strings.Contains(with.String(), "rotation_degrees = Vector3(-55, -25, 0)") {
		t.Fatal("key not rendered in place of the placeholder")
	}
	if !strings.Contains(without.String(), "rotation_degrees = Vector3(-55, -25, 0)") {
		t.Fatal("placeholder lost without a key")
	}
}
