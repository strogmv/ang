package gamepoc

import (
	"strings"
	"testing"
)

func TestUnityRenderingRequiresPinnedURPAndUserOwnedAsset(t *testing.T) {
	got := (UnityRendering{}).normalized()
	if got.Pipeline != "urp" || got.PackageVersion != "17.6.0" || got.AssetPath != "Assets/Rendering/ANGUniversalRenderPipeline.asset" {
		t.Fatal("unstable pipeline default")
	}
	for _, r := range []UnityRendering{got, {Pipeline: "urp", PackageVersion: "17.6.0", AssetPath: "Assets/ArtistSettings/MyPipeline.asset"}} {
		if err := r.validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []UnityRendering{{Pipeline: "builtin"}, {PackageVersion: "latest"}, {PackageVersion: "17.0.4"}, {AssetPath: "../Other.asset"}, {AssetPath: "Assets/../Other.asset"}, {AssetPath: "Assets/Generated/Auto.asset"}, {AssetPath: "Assets/Resources/Auto.asset"}} {
		if r.validate() == nil {
			t.Fatalf("unsafe rendering ownership/version accepted: %+v", r)
		}
	}
}
func TestURPUsesRenderGraphAndPreservesGUIComposition(t *testing.T) {
	feature := hudTemplate(t, "urp_feature.cs.tmpl")
	for _, need := range []string{"RecordRenderGraph", "ANGObservedOccluded", "BeforeRenderingTransparents", "RenderQueueRange.transparent", "AccessFlags.ReadWrite", "AccessFlags.Read", "DrawRendererList"} {
		if !strings.Contains(feature, need) {
			t.Fatalf("missing real URP actor pass: %s", need)
		}
	}
	core := hudTemplate(t, "urp.cs.tmpl")
	for _, need := range []string{"GraphicsSettings.currentRenderPipeline as UniversalRenderPipelineAsset", "SupportsRenderRequest", "SubmitRenderRequest", "SingleCameraRequest", "feature.isActive"} {
		if !strings.Contains(core, need) {
			t.Fatal("pipeline may silently fall back: " + need)
		}
	}
	for _, file := range []string{"location_sdk.cs.tmpl", "location_sdk_proof.cs.tmpl"} {
		s := hudTemplate(t, file)
		if strings.Contains(s, "locationSDKCamera.Render()") || !strings.Contains(s, "PocRendering.RenderCamera(locationSDKCamera,locationSDKTarget)") {
			t.Fatal("SDK still bypasses URP render requests: " + file)
		}
	}
	setup := hudTemplate(t, "urp_editor.cs.tmpl")
	for _, need := range []string{"RequirePackage()", "GraphicsSettings.defaultRenderPipeline!=pipeline", "File.Exists(rendererPath)", "SaveAssetIfDirty(renderer)", "customRenderPipeline", "feature is PocObservedActorFeature"} {
		if !strings.Contains(setup, need) {
			t.Fatal("missing reproducibility/preservation guard: " + need)
		}
	}
	if strings.Contains(setup, "SaveAssets()") || strings.Contains(setup, "enableRenderCompatibilityMode=") || strings.Contains(setup, "PlayerSettings.colorSpace") {
		t.Fatal("URP setup silently changes manual assets, rendering compatibility or colour space")
	}
}

func TestURPSDKPreparesBeforeIMGUIAndRendersAfterMainFrame(t *testing.T) {
	sdk := hudTemplate(t, "location_sdk.cs.tmpl")
	draw := sdk[strings.Index(sdk, "bool DrawLocationSDKGame()"):strings.Index(sdk, "void ReleaseLocationSDK()")]
	if strings.Contains(draw, "RenderCamera(") || strings.Contains(draw, "PrepareLocationSDK(") || strings.Contains(draw, "LocationSDKUpdateActors(") {
		t.Fatal("IMGUI must only compose the ready texture and handle input; nested render requests corrupt URP camera frame data")
	}
	frame := sdk[strings.Index(sdk, "void RenderLocationSDKFrame()"):strings.Index(sdk, "bool DrawLocationSDKGame()")]
	if !strings.Contains(frame, "ViewCell(view,floor.at.x,floor.at.y)=='0'") || !strings.Contains(frame, "LocationSDKUpdateActors()") || !strings.Contains(frame, "PocRendering.RenderCamera(locationSDKCamera,locationSDKTarget)") || !strings.Contains(frame, "locationSDKFrameReady=true") {
		t.Fatal("pre-render frame must apply current visibility/actors before publishing the texture")
	}
	request := frame[strings.Index(frame, "System.Collections.IEnumerator LocationSDKRenderAfterFrame()"):]
	if strings.Index(request, "yield return end") > strings.Index(request, "PocRendering.RenderCamera(") || !strings.Contains(request, "new WaitForEndOfFrame()") {
		t.Fatal("SDK requests must follow the main frame and avoid re-entering URP during camera composition")
	}
	unity := hudTemplate(t, "unity.cs.tmpl")
	late := unity[strings.Index(unity, "void LateUpdate()"):strings.Index(unity, "void FinishProof(")]
	if !strings.Contains(late, "UpdateMotion();\n            HeroBodyPredict();\n            RenderLocationSDKFrame();") {
		t.Fatal("SDK capture must run in LateUpdate after authoritative replies and motion sampling")
	}
}
