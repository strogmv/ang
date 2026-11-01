package gamepoc

import (
	"strings"
	"testing"
)

// Native URP captures check pixels/depth; this protects the source contract
// against a shader silently falling back to the retired built-in pipeline.
func TestURPShadersHaveExplicitPipelineAndMaterialBindings(t *testing.T) {
	for _, name := range []string{"location_sdk_surface", "location_sdk_actor", "wayfinding", "storefront", "painted_passage", "location_sdk_diagnostic"} {
		t.Run(name, func(t *testing.T) {
			source := hudTemplate(t, name+".shader.tmpl")
			for _, required := range []string{`"RenderPipeline"="UniversalPipeline"`, "HLSLPROGRAM", "Packages/com.unity.render-pipelines.universal/ShaderLibrary/Core.hlsl", "CBUFFER_START(UnityPerMaterial)", "CBUFFER_END", "TransformObjectToHClip(v.vertex.xyz)"} {
				if !strings.Contains(source, required) {
					t.Errorf("URP material is missing %q", required)
				}
			}
			for _, forbidden := range []string{"UnityCG.cginc", "CGPROGRAM", "CGINCLUDE", "sampler2D ", "tex2D(", "UnityObjectToClipPos(", "fixed4", "TRANSFORM_TEX("} {
				if strings.Contains(source, forbidden) {
					t.Errorf("legacy binding or duplicate authored UV transform: %q", forbidden)
				}
			}
		})
	}
}

func TestURPActorSilhouetteIsAnExplicitSeparatePass(t *testing.T) {
	actor := hudTemplate(t, "location_sdk_actor.shader.tmpl")
	for _, required := range []string{`"LightMode"="UniversalForward"`, `"LightMode"="ANGObservedOccluded"`, "#pragma fragment normalFrag", "#pragma fragment occludedFrag", "ZTest Greater", "ZTest LEqual", "Blend SrcAlpha OneMinusSrcAlpha"} {
		if strings.Count(actor, required) == 0 {
			t.Errorf("explicit depth-aware actor pass is missing %q", required)
		}
	}
	if strings.Count(actor, "CBUFFER_START(UnityPerMaterial)") != 1 || strings.Contains(actor, `"LightMode"="SRPDefaultUnlit"`) {
		t.Fatal("actor passes must share one material layout; silhouette must be requested by its renderer feature")
	}
	diagnostic := hudTemplate(t, "location_sdk_diagnostic.shader.tmpl")
	for _, required := range []string{`Shader "ANG/LocationSDKDiagnostic"`, "return _Color", `"LightMode"="UniversalForward"`, "ZWrite On"} {
		if !strings.Contains(diagnostic, required) {
			t.Errorf("SDK fallback must remain a real URP opaque material: %q", required)
		}
	}
}

func TestURPCompositorsKeepIncomingUVAndAlphaSemantics(t *testing.T) {
	store := hudTemplate(t, "storefront.shader.tmpl")
	passage := hudTemplate(t, "painted_passage.shader.tmpl")
	for name, source := range map[string]string{"storefront": store, "passage": passage} {
		if !strings.Contains(source, "o.uv=v.uv") || !strings.Contains(source, "TEXTURE2D_PARAM(image,imageSampler)") || !strings.Contains(source, "SAMPLE_TEXTURE2D(image,imageSampler,") {
			t.Errorf("%s must preserve mesh UV and a bound texture/sampler pair", name)
		}
		for _, forbidden := range []string{"GetFullScreenTriangle", "ComputeScreenPos", "UNITY_UV_STARTS_AT_TOP", "SRGBToLinear", "LinearToSRGB"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s adds a second projection, UV flip or color conversion: %s", name, forbidden)
			}
		}
	}
	if !strings.Contains(store, "Blend One Zero") || !strings.Contains(store, "max(.0001,alpha)") || !strings.Contains(passage, "Blend Off") || !strings.Contains(passage, "alpha*clip.a") {
		t.Fatal("offscreen compositors must retain straight-alpha output rather than blending it twice")
	}
	wayfinding := hudTemplate(t, "wayfinding.shader.tmpl")
	for _, required := range []string{"Blend [_SrcBlend] [_DstBlend]", "ZTest [_ZTest]", "return color*_Color", "clip(i.route.x-_RouteProgress.x+.0001)"} {
		if !strings.Contains(wayfinding, required) {
			t.Errorf("dual-use SDK/offscreen floor paint lost %q", required)
		}
	}
}
