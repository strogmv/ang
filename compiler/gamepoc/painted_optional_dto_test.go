package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// JsonUtility can create a zero-filled nested object for an absent JSON member.
// Exercise that real observed input shape, rather than only C# null references.
func TestPaintedOptionalDTONative(t *testing.T) {
	monos, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	if len(monos) == 0 {
		t.Skip("Mono unavailable")
	}
	mono := monos[len(monos)-1]
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	uv := sceneTemplateSection(t, "art_scene.cs.tmpl", "        static bool ArtSceneHasCrop(", "        void DrawArtScene(")
	track := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static bool PaintedTrackPresent(", "  static void BlitPaintedAmbient(")
	harness := `using System;
struct Rect {public float x,y,width,height;public Rect(float a,float b,float c,float d){x=a;y=b;width=c;height=d;}}
class Texture2D {public int width=1672,height=941;}
class PocArtSceneSourceRect {public int x,y,w,h;}
class PocAmbientTrack {public string image;public float width,height;}
class Proof {
` + uv + track + `
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 static void Full(Rect r){Need(r.x==0&&r.y==0&&r.width==1&&r.height==1,"absent materialized crop must sample full image, not a single empty UV");}
 public static void Main(){
  var image=new Texture2D();Full(ArtSceneUV(null,image));Full(ArtSceneUV(new PocArtSceneSourceRect(),image));
  Need(!ArtSceneHasCrop(null)&&!ArtSceneHasCrop(new PocArtSceneSourceRect()),"null and zero DTO both mean absent crop");
  var crop=new PocArtSceneSourceRect{x=100,y=40,w=500,h=300};var rect=ArtSceneUV(crop,image);
  Need(ArtSceneHasCrop(crop)&&Math.Abs(rect.x-100f/1672)<.00001&&Math.Abs(rect.y-(1-340f/941))<.00001&&Math.Abs(rect.width-500f/1672)<.00001&&Math.Abs(rect.height-300f/941)<.00001,"real source crop preserves top-left to bottom-left UV conversion");
  Need(!PaintedTrackPresent(null)&&!PaintedTrackPresent(new PocAmbientTrack()),"absent static track must never load a null Resources path");
  Need(!PaintedTrackPresent(new PocAmbientTrack{image="MarketArt/rail"})&&!PaintedTrackPresent(new PocAmbientTrack{width=100,height=30}),"empty image/dimensions do not create a track");
  Need(PaintedTrackPresent(new PocAmbientTrack{image="MarketArt/rail",width=100,height=30}),"real optional track remains enabled");
  Console.WriteLine("empty nested DTO: optional track absent / whole-image UV / real crop preserved PASS");
 }
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "OptionalDto.cs"), filepath.Join(dir, "OptionalDto.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("optional DTO compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("optional DTO execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
	s := hudTemplate(t, "painted_ambient.cs.tmpl")
	for _, bad := range []string{"if(passage.track!=null)", "c.passage.track==null"} {
		if strings.Contains(s, bad) {
			t.Errorf("presence check still trusts materialized optional DTO: %s", bad)
		}
	}
}

func TestPaintedAmbientBlitRestoresRenderTargetNative(t *testing.T) {
	monos, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	if len(monos) == 0 {
		t.Skip("Mono unavailable")
	}
	mono := monos[len(monos)-1]
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	helper := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static void BlitPaintedAmbient(", "  string paintedAmbientLevel;")
	harness := `using System;
class Texture2D {public static Texture2D whiteTexture=new Texture2D();}
class Material {}
class RenderTexture {public static RenderTexture active;}
static class GL {public static bool sRGBWrite;}
static class Graphics {public static bool fail;public static int calls;public static void Blit(Texture2D image,RenderTexture target,Material material){calls++;RenderTexture.active=target;GL.sRGBWrite=!GL.sRGBWrite;if(fail)throw new InvalidOperationException("GPU failure");}}
class Proof {
` + helper + `
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 public static void Main(){
  var before=new RenderTexture();var layer=new RenderTexture();RenderTexture.active=before;GL.sRGBWrite=true;BlitPaintedAmbient(layer,new Material());Need(Object.ReferenceEquals(RenderTexture.active,before)&&GL.sRGBWrite,"blit leaked its target/color state to the following HUD");
  RenderTexture.active=null;GL.sRGBWrite=false;BlitPaintedAmbient(layer,new Material());Need(RenderTexture.active==null&&!GL.sRGBWrite,"backbuffer must remain active after blit");
  Graphics.fail=true;RenderTexture.active=before;GL.sRGBWrite=true;bool threw=false;try{BlitPaintedAmbient(layer,new Material());}catch(InvalidOperationException){threw=true;}Need(threw&&Object.ReferenceEquals(RenderTexture.active,before)&&GL.sRGBWrite,"exception path must restore target/color state too");Need(Graphics.calls==3,"guard suppressed real GPU work");
  Console.WriteLine("GPU blit restores prior RT/backbuffer and sRGB state on success and exception PASS");
 }
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "BlitGuard.cs"), filepath.Join(dir, "BlitGuard.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("blit guard compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("blit guard execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
	module := hudTemplate(t, "painted_ambient.cs.tmpl")
	if strings.Count(module, "Graphics.Blit(") != 1 || !strings.Contains(module, "BlitPaintedAmbient(layer,material)") || !strings.Contains(module, "BlitPaintedAmbient(c.layer,c.material)") {
		t.Fatal("cold and moving blits must both use the target guard")
	}
}

func TestAmbientGPUProbeTransfersActualRuntimeUniformsNative(t *testing.T) {
	monos, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	if len(monos) == 0 {
		t.Skip("Mono unavailable")
	}
	mono := monos[len(monos)-1]
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	helper := sceneTemplateSection(t, "painted_ambient_proof.cs.tmpl", "  static Material AmbientProofMaterial(", "  const string AmbientProofScenario=")
	harness := `using System;using System.Collections.Generic;
struct Vector4 {public float x,y,z,w;public Vector4(float a,float b,float c,float d){x=a;y=b;z=c;w=d;}}
class Shader {} class Texture {}
class Material {public Shader shader;readonly Dictionary<string,Vector4> vectors=new Dictionary<string,Vector4>();readonly Dictionary<string,float> floats=new Dictionary<string,float>();readonly Dictionary<string,Texture> textures=new Dictionary<string,Texture>();public Material(Shader value){shader=value;}public void SetVector(string key,Vector4 value){vectors[key]=value;}public Vector4 GetVector(string key){Vector4 value;return vectors.TryGetValue(key,out value)?value:new Vector4();}public void SetFloat(string key,float value){floats[key]=value;}public float GetFloat(string key){float value;return floats.TryGetValue(key,out value)?value:0;}public void SetTexture(string key,Texture value){textures[key]=value;}public Texture GetTexture(string key){Texture value;return textures.TryGetValue(key,out value)?value:null;}}
class Proof {
` + helper + `
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 public static void Main(){
  var source=new Material(new Shader());var vectors=new[]{"_LayerRect","_TrainRect","_TrainUV","_TrackRect","_TrackUV"};var floats=new[]{"_TrainEnabled","_TrackEnabled","_ObservationWidth"};var textures=new[]{"_MaskTex","_ObservationTex","_TrainTex","_TrackTex"};
  for(int i=0;i<vectors.Length;i++)source.SetVector(vectors[i],new Vector4(100+i,200+i,300+i,400+i));for(int i=0;i<floats.Length;i++)source.SetFloat(floats[i],20+i);foreach(var key in textures)source.SetTexture(key,new Texture());
  var probe=AmbientProofMaterial(source);Need(Object.ReferenceEquals(probe.shader,source.shader),"body-only probe changed its shader");
  foreach(var key in vectors){var a=probe.GetVector(key);var b=source.GetVector(key);Need(a.x==b.x&&a.y==b.y&&a.z==b.z&&a.w==b.w,"runtime vector not transferred: "+key);}foreach(var key in floats)Need(probe.GetFloat(key)==source.GetFloat(key),"runtime float not transferred: "+key);foreach(var key in textures)Need(Object.ReferenceEquals(probe.GetTexture(key),source.GetTexture(key)),"current GPU texture not transferred: "+key);
  probe.SetFloat("_TrackEnabled",0);Need(source.GetFloat("_TrackEnabled")==21,"body-only probe altered the real rendered material");
  Console.WriteLine("actual runtime vectors/floats/textures copied explicitly; original renderer untouched PASS");
 }
}`
	dir := t.TempDir()
	src, exe := filepath.Join(dir, "ProbeUniforms.cs"), filepath.Join(dir, "ProbeUniforms.exe")
	if err := os.WriteFile(src, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+exe, src).CombinedOutput(); err != nil {
		t.Fatalf("probe uniform compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, exe).CombinedOutput(); err != nil {
		t.Fatalf("probe uniform execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
