package godot

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

//go:embed concrete.gdshader
var concreteShader string

// ConcreteMaterial derives a caste finish from existing concrete maps. Scalar
// masks are deterministic world-space shader parameters; no new texture is made.
type ConcreteMaterial struct {
	ReviewNavigation *locationsdk.CompiledLevel `json:"review_navigation,omitempty"`
	FixtureDraws     []ArchitectureDraw         `json:"fixture_draws,omitempty"`
	FixtureTextSlots []locationsdk.TextSlot     `json:"fixture_text_slots,omitempty"`
	FixtureSourceIDs []string                   `json:"fixture_source_ids,omitempty"`
	Tier             string                     `json:"tier,omitempty"`
	Instance         *ConcreteInstance          `json:"instance,omitempty"`
	AnchorData       []ConcreteAnchor           `json:"anchor_data,omitempty"`
	PatchAnchors     []ConcreteAnchor           `json:"patch_anchors,omitempty"`
	RouteData        []ConcreteRoute            `json:"route_data,omitempty"`
	RepairsAtlas     string                     `json:"repairs_atlas,omitempty"`
	Albedo           string                     `json:"albedo"`
	Normal           string                     `json:"normal"`
	Roughness        string                     `json:"roughness"`
	Tint             [3]float64                 `json:"tint"`
	StainColor       [3]float64                 `json:"stain_color"`
	RustColor        [3]float64                 `json:"rust_color"`
	StripeColor      [3]float64                 `json:"stripe_color"`
	ResinColor       *[3]float64                `json:"resin_color,omitempty"`
	ResinMetallic    *float64                   `json:"resin_metallic,omitempty"`
	TextureScale     float64                    `json:"texture_scale"`
	RoughnessBase    float64                    `json:"roughness_base"`
	PoreContrast     float64                    `json:"pore_contrast"`
	NormalStrength   float64                    `json:"normal_strength"`
	Water            float64                    `json:"water"`
	Rust             float64                    `json:"rust"`
	Patches          float64                    `json:"patches"`
	Soot             float64                    `json:"soot"`
	ProtocolStripe   float64                    `json:"protocol_stripe"`
	Damage           float64                    `json:"damage"`
}

func (c *ConcreteMaterial) render(id string) (ext, sub string, err error) {
	var e, s strings.Builder
	if c.ReviewNavigation != nil {
		if c.Instance == nil {
			return "", "", fmt.Errorf("concrete %s: review_navigation needs an explicit fixture instance", id)
		}
		copyProfile := *c
		copyProfile.ReviewNavigation = nil
		copyProfile.RouteData = buildConcreteRoutes(*c.ReviewNavigation, c.Instance.FloorM)
		return copyProfile.render(id)
	}
	if c.Instance != nil {
		source, sourceErr := concreteSourceTable(c.AnchorData)
		if sourceErr != nil {
			return "", "", sourceErr
		}
		if instanceErr := validateConcreteInstance(id, *c.Instance, source); instanceErr != nil {
			return "", "", instanceErr
		}
		if len(c.FixtureSourceIDs) > 0 || len(c.FixtureDraws) > 0 {
			ids := map[string]bool{}
			for _, id := range c.FixtureSourceIDs {
				ids[id] = true
			}
			for _, draw := range c.FixtureDraws {
				ids[draw.ID] = true
			}
			if sourceErr := validateConcreteInstanceSources(id, *c.Instance, c.AnchorData, ids); sourceErr != nil {
				return "", "", sourceErr
			}
		}
	}
	if len(c.AnchorData) > 64 || len(c.RouteData) > 128 {
		return "", "", fmt.Errorf("concrete %s: splitwallanchor/route scope, no silentcap", id)
	}
	for i, p := range []string{c.Albedo, c.Normal, c.Roughness} {
		suffix := strings.ToLower(strings.TrimPrefix(p, "res://"))
		valid := canonicalResource(p, ".jpg") || canonicalResource(p, ".png") || canonicalResource(p, ".jpeg")
		if !valid || suffix == "" {
			return "", "", fmt.Errorf("concrete %s: canonical existing albedo/normal/roughness texture resources required", id)
		}
		fmt.Fprintf(&e, "[ext_resource type=\"Texture2D\" path=%s id=%s]\n", strconv.Quote(p), strconv.Quote(fmt.Sprintf("%s_tex%d", id, i)))
	}
	vals := []float64{c.RoughnessBase, c.PoreContrast, c.Water, c.Rust, c.Patches, c.Soot, c.ProtocolStripe, c.Damage}
	for _, v := range vals {
		if !finite(v) || v < 0 || v > 1 {
			return "", "", fmt.Errorf("concrete %s: mask/PBR parameters must be finite 0..1", id)
		}
	}
	if !finite(c.NormalStrength) || c.NormalStrength < 0 || c.NormalStrength > 2 {
		return "", "", fmt.Errorf("concrete %s: normal_strength must be0..2", id)
	}
	if !finite(c.TextureScale) || c.TextureScale <= 0 {
		return "", "", fmt.Errorf("concrete %s: positive texture_scale required", id)
	}
	if c.RepairsAtlas != "" {
		if !canonicalResource(c.RepairsAtlas, ".png") {
			return "", "", fmt.Errorf("concrete %s: repairs_atlas requires existingPNGresource", id)
		}
		fmt.Fprintf(&e, "[ext_resource type=\"Texture2D\" path=%s id=%s]\n", strconv.Quote(c.RepairsAtlas), strconv.Quote(id+"_repair"))
	}
	fmt.Fprintf(&s, "[sub_resource type=\"ShaderMaterial\" id=%s]\nshader = SubResource(\"concrete_shader\")\n", strconv.Quote(id))
	if c.RepairsAtlas != "" {
		fmt.Fprintf(&s, "shader_parameter/repairs_atlas = ExtResource(%s)\n", strconv.Quote(id+"_repair"))
	}
	for i, key := range []string{"concrete_albedo", "concrete_normal", "concrete_roughness"} {
		fmt.Fprintf(&s, "shader_parameter/%s = ExtResource(%s)\n", key, strconv.Quote(fmt.Sprintf("%s_tex%d", id, i)))
	}
	for _, row := range []struct {
		k string
		v [3]float64
	}{{"tint", c.Tint}, {"stain_color", c.StainColor}, {"rust_color", c.RustColor}, {"stripe_color", c.StripeColor}} {
		for _, v := range row.v {
			if !finite(v) || v < 0 || v > 1 {
				return "", "", fmt.Errorf("concrete %s: colors must be linear finite 0..1", id)
			}
		}
		fmt.Fprintf(&s, "shader_parameter/%s = Vector3(%s,%s,%s)\n", row.k, number(row.v[0]), number(row.v[1]), number(row.v[2]))
	}
	keys := []string{"roughness_base", "pore_contrast", "water", "rust", "patches", "soot", "protocol_stripe", "damage"}
	for i, key := range keys {
		fmt.Fprintf(&s, "shader_parameter/%s = %s\n", key, number(vals[i]))
	}
	fmt.Fprintf(&s, "shader_parameter/normal_strength = %s\n", number(c.NormalStrength))
	fmt.Fprintf(&s, "shader_parameter/texture_scale = %s\n", number(c.TextureScale))
	// Optional controls are emitted in key order. Omitted values preserve the
	// original shader defaults, including for pre-existing concrete profiles.
	if c.ResinColor != nil {
		for _, value := range *c.ResinColor {
			if !finite(value) || value < 0 || value > 1 {
				return "", "", fmt.Errorf("concrete %s: resin_color must be linear finite 0..1", id)
			}
		}
		fmt.Fprintf(&s, "shader_parameter/resin_color = Vector3(%s,%s,%s)\n", number(c.ResinColor[0]), number(c.ResinColor[1]), number(c.ResinColor[2]))
	}
	if c.ResinMetallic != nil {
		if !finite(*c.ResinMetallic) || *c.ResinMetallic < 0 || *c.ResinMetallic > 1 {
			return "", "", fmt.Errorf("concrete %s: resin_metallic must be finite 0..1", id)
		}
		fmt.Fprintf(&s, "shader_parameter/resin_metallic = %s\n", number(*c.ResinMetallic))
	}
	s.WriteString(concreteInstanceParameters(c))
	return e.String(), s.String(), nil
}
