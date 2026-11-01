package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDepthSpriteShaderSharesMetricReconstructionAcrossPasses(t *testing.T) {
	data, err := os.ReadFile("templates/location_sdk_depth_sprite.shader.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	shader := string(data)
	for _, required := range []string{`Shader "ANG/LocationSDKDepthSprite"`, `_ImageForward.xyz*(depth-_TargetDepth)`, `_BottomLeft.xyz+_ImageRight.xyz*(uv.x*_ImageWidth)+_ImageUp.xyz*(uv.y*_ImageHeight)`, `TransformObjectToHClip(local)`, `positionCS.z/positionCS.w`, `SHADER_API_GLCORE`, `SHADER_API_GLES3`, `deviceDepth=deviceDepth*.5+.5`, `float depth : SV_Depth`, `float fragDepth(Varyings i) : SV_Depth`, `return ReconstructedDepth(i.uv)`, `float3 local=ReconstructedLocal(i.uv)`, `o.depth=DeviceDepth(local)`, `float ReconstructedDepth(float2 uv) {return DeviceDepth(ReconstructedLocal(uv));}`, `"LightMode"="DepthOnly"`, `ColorMask 0`, `ZWrite On`, `clip(depth-.000001)`} {
		if !strings.Contains(shader, required) {
			t.Errorf("metric depth shader missing %q", required)
		}
	}
	if strings.Contains(shader, "Blend SrcAlpha") || strings.Contains(shader, "ZTest Always") || strings.Contains(shader, "1-deviceDepth") {
		t.Fatal("opaque depth sprite must preserve platform GPU projection and depth-test silhouette")
	}
	generate, err := os.ReadFile("generate.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"location_sdk_depth_sprite.shader.tmpl", "location_sdk_depth_proof.cs.tmpl"} {
		if !strings.Contains(string(generate), name) {
			t.Fatalf("%s not emitted", name)
		}
	}
}

func TestDepthSpriteGPUAcceptanceComparisonNative(t *testing.T) {
	mono, err := exec.LookPath("mono")
	if err != nil {
		matches, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
		if len(matches) == 0 {
			t.Skip("Mono unavailable")
		}
		mono = matches[len(matches)-1]
	}
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	if _, err := os.Stat(csc); err != nil {
		t.Skip("Mono compiler unavailable")
	}
	dto := sceneTemplateSection(t, "location_sdk_depth_proof.cs.tmpl", " [Serializable] public sealed class PocDepthSpriteProbe ", " // This entry point")
	helpers := sceneTemplateSection(t, "location_sdk_depth_proof.cs.tmpl", "  static bool Same(", "  public static PocDepthSpriteReport Verify(")
	harness := `using System;using System.Collections.Generic;struct Color32 {public byte r,g,b;public Color32(byte rr,byte gg,byte bb){r=rr;g=gg;b=bb;}}` + dto + `class Proof {` + helpers + `
 static void Require(bool ok,string why){if(!ok)throw new Exception(why);}
 static void Main(){
  var plane=new Color32[1024];var original=new Color32[1024];var baked=new Color32[1024];var flat=new Color32[1024];
  for(int i=0;i<1024;i++){plane[i]=new Color32(255,0,255);original[i]=baked[i]=i<500?new Color32(0,0,128):plane[i];flat[i]=i<200?new Color32(0,0,128):plane[i];}
  var matched=Compare(plane,original,baked,flat,.25f);Require(matched.reference_occluded_pixels==500&&matched.baked_occluded_pixels==500&&matched.mismatch_pixels==0&&matched.flat_difference_pixels==300,"occlusion masks compare image colors rather than visibility");
  var records=new List<PocDepthSpriteProbe>();for(int i=0;i<5;i++)records.Add(matched);Require(Accept(records),"matching nonflat depth rejected");
  var noEffect=Compare(plane,original,baked,baked,.25f);records[0]=noEffect;records[1]=noEffect;records[2]=noEffect;records[3]=noEffect;records[4]=noEffect;Require(!Accept(records),"flat-depth counterfactual without GPU effect accepted");
  baked[600]=new Color32(0,0,128);var wrong=Compare(plane,original,baked,flat,.25f);Require(wrong.mismatch_pixels==1&&wrong.mismatch_ratio>0,"baked occlusion outside geometry ignored");
  for(int i=500;i<800;i++)baked[i]=new Color32(0,0,128);wrong=Compare(plane,original,baked,flat,.25f);records[0]=wrong;Require(!Accept(records),"wrong depth with mismatched geometry accepted");
  records.Clear();Require(!Accept(records),"empty GPU evidence accepted");Console.WriteLine("depth sprite GPU evidence comparison PASS");
 }}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "Masks.cs"), filepath.Join(dir, "Masks.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile native mask comparison: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("GPU evidence acceptance: %v\n%s", err, out)
	}
	// Type-check the complete fixture against Unity's actual managed API without
	// launching an Editor, creating a project, or claiming a GPU render occurred.
	modules, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/Managed/UnityEngine/UnityEngine.CoreModule.dll")
	if len(modules) == 0 {
		return
	}
	moduleDir := filepath.Dir(modules[len(modules)-1])
	data, err := os.ReadFile("templates/location_sdk_depth_proof.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	stub := `namespace AngelsDemons.Poc {public static class PocRendering {public static void RequireActive(){} public static void RenderCamera(UnityEngine.Camera c,UnityEngine.RenderTexture t){}}}`
	if err := os.WriteFile(source, append(data, []byte(stub)...), 0600); err != nil {
		t.Fatal(err)
	}
	refs := []string{filepath.Join(filepath.Dir(csc), "Facades/netstandard.dll"), filepath.Join(moduleDir, "UnityEngine.ScriptingModule.dll"), filepath.Join(moduleDir, "UnityEngine.CoreModule.dll"), filepath.Join(moduleDir, "UnityEngine.ImageConversionModule.dll"), filepath.Join(moduleDir, "UnityEngine.JSONSerializeModule.dll")}
	args := []string{csc, "/nologo", "/target:library", "/out:" + filepath.Join(dir, "Fixture.dll"), source}
	for _, ref := range refs {
		args = append(args, "/reference:"+ref)
	}
	if out, err := exec.Command(mono, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile complete GPU fixture against Unity API: %v\n%s", err, out)
	}
}
