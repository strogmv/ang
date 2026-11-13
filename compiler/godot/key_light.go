package godot

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/placement"
)

// KeyLight is a level's one strong side light. It is a real light of the level, named by the
// level's CUE lighting data (Source), drawn as one shadowed DirectionalLight3D. Angles use the
// game camera's own convention: AzimuthDegrees from plan +y towards plan +x is the side the
// light comes from, ElevationDegrees is its height above the floor. Colours are linear and, like
// Energy, are given per authoritative day / dusk / night weight; the client blends them by the
// server's weights. Presentation only: the server's gameplay light (lux) is not changed here.
//
// The fill is the scattered light that reaches what the key does not: drawn as a second,
// shadowless DirectionalLight3D from the opposite side, high up. It is a light rather than the
// environment's ambient because Godot's SDFGI ignores the environment's ambient colour.
type KeyLight struct {
	Source           string        `json:"source"`
	AzimuthDegrees   float64       `json:"azimuth_degrees"`
	ElevationDegrees float64       `json:"elevation_degrees"`
	Color            [3][3]float64 `json:"color"`
	Energy           [3]float64    `json:"energy"`
	// Angular size of the source (Godot light_angular_distance): the penumbra grows with the
	// distance from the occluder.
	ShadowAngularDegrees float64 `json:"shadow_angular_degrees"`
	ShadowBlur           float64 `json:"shadow_blur"`
	ShadowBias           float64 `json:"shadow_bias"`
	ShadowNormalBias     float64 `json:"shadow_normal_bias"`
	// Back faces draw the shadow map: closed bodies stop shadowing their own lit surface.
	ShadowReverseCullFace bool `json:"shadow_reverse_cull_face"`
	// Depth of the camera's view volume the one directional shadow map covers (near, far, metres
	// from the camera along its view). Godot fits an orthographic camera's directional shadow to
	// its whole near..far range, so this range sets the shadow's texel size.
	ShadowDepthM [2]float64 `json:"shadow_depth_m"`
	// People cast only this light's shadow (the client keeps them out of every other light's
	// shadow casters).
	PeopleShadow         bool          `json:"people_shadow"`
	FillElevationDegrees float64       `json:"fill_elevation_degrees"`
	FillColor            [3][3]float64 `json:"fill_color"`
	FillEnergy           [3]float64    `json:"fill_energy"`
	Provenance           string        `json:"provenance"`
}

func (k *KeyLight) validate() error {
	if k == nil {
		return nil
	}
	fail := func(s string) error { return fmt.Errorf("godot key light %q: %s", k.Source, s) }
	if strings.TrimSpace(k.Source) == "" || strings.TrimSpace(k.Provenance) == "" {
		return fail("needs the real source and its provenance")
	}
	values := []float64{k.AzimuthDegrees, k.ElevationDegrees, k.ShadowAngularDegrees, k.ShadowBlur, k.ShadowBias, k.ShadowNormalBias,
		k.ShadowDepthM[0], k.ShadowDepthM[1], k.FillElevationDegrees}
	values = append(values, k.Energy[:]...)
	values = append(values, k.FillEnergy[:]...)
	for _, c := range append(k.Color[:], k.FillColor[:]...) {
		values = append(values, c[:]...)
	}
	for _, v := range values {
		if !finite(v) {
			return fail("non-finite value")
		}
	}
	if k.ElevationDegrees <= 5 || k.ElevationDegrees >= 85 {
		return fail("elevation must lie in (5, 85) degrees")
	}
	if k.FillElevationDegrees <= 5 || k.FillElevationDegrees > 90 {
		return fail("fill elevation must lie in (5, 90] degrees")
	}
	if k.AzimuthDegrees < -180 || k.AzimuthDegrees > 180 {
		return fail("azimuth must lie in [-180, 180] degrees")
	}
	for _, e := range append(k.Energy[:], k.FillEnergy[:]...) {
		if e < 0 || e > 16 {
			return fail("energy must lie in [0, 16]")
		}
	}
	for _, c := range append(k.Color[:], k.FillColor[:]...) {
		for _, v := range c {
			if v < 0 || v > 1 {
				return fail("linear colour must lie in [0, 1]")
			}
		}
	}
	if k.ShadowAngularDegrees < 0 || k.ShadowAngularDegrees > 10 {
		return fail("shadow angular size must lie in [0, 10] degrees")
	}
	if k.ShadowBlur < 0 || k.ShadowBlur > 4 || k.ShadowBias < 0 || k.ShadowBias > 2 || k.ShadowNormalBias < 0 || k.ShadowNormalBias > 10 {
		return fail("shadow blur [0, 4], bias [0, 2], normal bias [0, 10]")
	}
	if k.ShadowDepthM[0] <= 0 || k.ShadowDepthM[1] <= k.ShadowDepthM[0] {
		return fail("shadow depth needs 0 < near < far")
	}
	return nil
}

// Basis returns the light's basis columns in the Godot world: +Z points towards the light, so the
// DirectionalLight3D shines along -Z. It is the camera's own basis at the light's angles.
func (k *KeyLight) Basis() [3]placement.Vec3 {
	return placement.CameraBasis(k.ElevationDegrees, k.AzimuthDegrees)
}

// FillBasis: the fill comes from the side opposite the key, at its own elevation.
func (k *KeyLight) FillBasis() [3]placement.Vec3 {
	az := math.Mod(k.AzimuthDegrees+360, 360) - 180
	return placement.CameraBasis(k.FillElevationDegrees, az)
}

func srgbColor(c [3]float64) string {
	return fmt.Sprintf("Color(%s, %s, %s, 1)", number(linearToSRGB(c[0])), number(linearToSRGB(c[1])), number(linearToSRGB(c[2])))
}

func modeMetadata(b *strings.Builder, energy [3]float64, color [3][3]float64) {
	fmt.Fprintf(b, "metadata/key_energy = Vector3(%s, %s, %s)\n", number(energy[0]), number(energy[1]), number(energy[2]))
	for i, mode := range []string{"day", "dusk", "night"} {
		fmt.Fprintf(b, "metadata/key_color_%s = %s\n", mode, srgbColor(color[i]))
	}
}

// render writes the key (name) and its fill (name + "Fill") as DirectionalLight3D nodes under
// parent. The day values are the nodes' own; all three modes travel as metadata for the client's
// blend by the server's weights.
func (k *KeyLight) render(name, parent string) (string, error) {
	if err := k.validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[node name=%s type=\"DirectionalLight3D\" parent=%s]\n", strconv.Quote(name), strconv.Quote(parent))
	fmt.Fprintf(&b, "transform = %s\n", transform(k.Basis(), placement.Vec3{}))
	fmt.Fprintf(&b, "light_color = %s\n", srgbColor(k.Color[0]))
	fmt.Fprintf(&b, "light_energy = %s\n", number(k.Energy[0]))
	fmt.Fprintf(&b, "light_angular_distance = %s\n", number(k.ShadowAngularDegrees))
	b.WriteString("light_bake_mode = 2\n")
	b.WriteString("shadow_enabled = true\n")
	fmt.Fprintf(&b, "shadow_bias = %s\n", number(k.ShadowBias))
	fmt.Fprintf(&b, "shadow_normal_bias = %s\n", number(k.ShadowNormalBias))
	fmt.Fprintf(&b, "shadow_reverse_cull_face = %t\n", k.ShadowReverseCullFace)
	fmt.Fprintf(&b, "shadow_blur = %s\n", number(k.ShadowBlur))
	// One split: an orthographic game camera sees one thin slab of the world.
	b.WriteString("directional_shadow_mode = 0\n")
	fmt.Fprintf(&b, "metadata/key_source = %s\n", strconv.Quote(k.Source))
	fmt.Fprintf(&b, "metadata/key_provenance = %s\n", strconv.Quote(k.Provenance))
	fmt.Fprintf(&b, "metadata/key_angles = Vector2(%s, %s)\n", number(k.AzimuthDegrees), number(k.ElevationDegrees))
	modeMetadata(&b, k.Energy, k.Color)
	fmt.Fprintf(&b, "metadata/key_shadow_depth_m = Vector2(%s, %s)\n", number(k.ShadowDepthM[0]), number(k.ShadowDepthM[1]))
	fmt.Fprintf(&b, "metadata/key_people_shadow = %t\n", k.PeopleShadow)
	fmt.Fprintf(&b, "\n[node name=%s type=\"DirectionalLight3D\" parent=%s]\n", strconv.Quote(name+"Fill"), strconv.Quote(parent))
	fmt.Fprintf(&b, "transform = %s\n", transform(k.FillBasis(), placement.Vec3{}))
	fmt.Fprintf(&b, "light_color = %s\n", srgbColor(k.FillColor[0]))
	fmt.Fprintf(&b, "light_energy = %s\n", number(k.FillEnergy[0]))
	b.WriteString("light_bake_mode = 2\n")
	b.WriteString("shadow_enabled = false\n")
	fmt.Fprintf(&b, "metadata/key_source = %s\n", strconv.Quote(k.Source+" (fill)"))
	fmt.Fprintf(&b, "metadata/key_fill = true\n")
	modeMetadata(&b, k.FillEnergy, k.FillColor)
	return b.String(), nil
}

// RenderKeyLight is the one writer of a key light's nodes; scene builders outside this package
// (the legacy market assembly) copy its text rather than re-deriving the bases.
func RenderKeyLight(k *KeyLight, name, parent string) (string, error) {
	if k == nil {
		return "", fmt.Errorf("godot key light: nothing to render")
	}
	return k.render(name, parent)
}
