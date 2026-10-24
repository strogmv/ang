package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runPaintedTextureNative(t *testing.T, name, source string) {
	t.Helper()
	monos, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	if len(monos) == 0 {
		t.Skip("Mono unavailable")
	}
	mono := monos[len(monos)-1]
	dir := t.TempDir()
	src, exe := filepath.Join(dir, name+".cs"), filepath.Join(dir, name+".exe")
	if err := os.WriteFile(src, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe"), "/nologo", "/out:"+exe, src).CombinedOutput(); err != nil {
		t.Fatalf("%s compile: %v\n%s", name, err, out)
	}
	command := exec.Command(mono, exe)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s execution: %v\n%s", name, err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}

// The source Texture2D deliberately throws on GetPixels32 when GPU-only, just
// like the retained Editor import that originally failed each OnGUI repaint.
// Execute the actual template cache/readback code, including GPU failure cleanup.
func TestPaintedTextureCPUReadNative(t *testing.T) {
	cache := sceneTemplateSection(t, "art_scene.cs.tmpl", "        sealed class ArtScenePixelData", "        bool HasArtScenes()")
	helper := sceneTemplateSection(t, "art_scene.cs.tmpl", "        ArtScenePixelData ArtScenePixels(", "        void OpenArtScene(")
	runPaintedTextureNative(t, "TextureCPURead", `using System;using System.Collections.Generic;
struct Color32 {public byte r,g,b,a;public Color32(byte x,byte y,byte z,byte w){r=x;g=y;b=z;a=w;}}
struct Rect {public float x,y,width,height;public Rect(float x,float y,float w,float h){this.x=x;this.y=y;width=w;height=h;}}
enum TextureFormat {RGBA32}enum RenderTextureFormat {ARGB32}enum RenderTextureReadWrite {sRGB}enum ColorSpace {Gamma,Linear}
class Texture2D {
 public string name="source";public int width,height,calls;public bool isReadable,readFail,incomplete;public Color32[] pixels;
 public Texture2D(int w,int h,TextureFormat f,bool mip,bool linear){width=w;height=h;isReadable=true;pixels=new Color32[w*h];}
 public Color32[] GetPixels32(){calls++;if(!isReadable)throw new InvalidOperationException("not readable");return incomplete?new Color32[0]:(Color32[])pixels.Clone();}
 public void ReadPixels(Rect r,int x,int y,bool recalculate){if(Graphics.source.readFail)throw new InvalidOperationException("readback failed");if(r.width!=width||r.height!=height||x!=0||y!=0||recalculate)throw new Exception("readback crop changed");pixels=(Color32[])RenderTexture.active.pixels.Clone();}
 public void Apply(bool mip,bool unreadable){if(mip||unreadable)throw new Exception("temporary copy must retain base pixels");}
}
class RenderTexture {
 public static RenderTexture active;public static int acquired,released;public int width,height;public Color32[] pixels;
 public static RenderTexture GetTemporary(int w,int h,int depth,RenderTextureFormat f,RenderTextureReadWrite rw){acquired++;if(depth!=0||rw!=RenderTextureReadWrite.sRGB)throw new Exception("unexpected copy format");return new RenderTexture{width=w,height=h};}
 public static void ReleaseTemporary(RenderTexture rt){released++;}
}
static class GL {public static bool sRGBWrite;}
static class Application {public static bool isPlaying=true;}
static class QualitySettings {public static ColorSpace activeColorSpace=ColorSpace.Linear;}
static class Graphics {
 public static int calls;public static bool fail;public static Texture2D source;
 public static void Blit(Texture2D s,RenderTexture d){calls++;source=s;RenderTexture.active=d;if(GL.sRGBWrite!=(QualitySettings.activeColorSpace==ColorSpace.Linear))throw new Exception("inherited GUI color conversion");GL.sRGBWrite=!GL.sRGBWrite;if(fail)throw new InvalidOperationException("blit failed");if(s.width!=d.width||s.height!=d.height)throw new Exception("resized source");d.pixels=(Color32[])s.pixels.Clone();}
}
static class Debug {public static int errors;public static void LogError(string value){errors++;}}
class Proof {
 static int destroyed,immediate;static void Destroy(Texture2D texture){destroyed++;}static void DestroyImmediate(Texture2D texture){immediate++;}
`+cache+helper+`
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 static Texture2D Source(bool readable){return new Texture2D(2,2,TextureFormat.RGBA32,false,false){isReadable=readable,pixels=new[]{new Color32(19,20,21,0),new Color32(40,41,42,70),new Color32(60,61,62,180),new Color32(80,81,82,255)}};}
 static void Pixels(Color32[] p){Need(p.Length==4&&p[0].r==19&&p[0].a==0&&p[1].a==70&&p[2].a==180&&p[3].r==80&&p[3].a==255,"copy lost orientation, source RGB or partial alpha");}
 public static void Main(){
  var proof=new Proof();var direct=Source(true);var a=proof.ArtScenePixels(direct);Pixels(a.pixels);Need(a.error==null&&direct.calls==1&&Graphics.calls==0,"readable source unexpectedly copied on GPU");Need(Object.ReferenceEquals(a,proof.ArtScenePixels(direct))&&direct.calls==1,"readable source fetched every frame");
  var before=new RenderTexture();RenderTexture.active=before;GL.sRGBWrite=false;var gpu=Source(false);var b=proof.ArtScenePixels(gpu);Pixels(b.pixels);Need(b.error==null&&gpu.calls==0&&!gpu.isReadable,"unreadable original was read or changed");Need(Graphics.calls==1&&RenderTexture.acquired==1&&RenderTexture.released==1&&destroyed==1,"cold copy leaked GPU/CPU resource");Need(Object.ReferenceEquals(RenderTexture.active,before)&&!GL.sRGBWrite,"copy leaked render target or sRGB state to HUD");for(int i=0;i<100;i++)Need(Object.ReferenceEquals(b,proof.ArtScenePixels(gpu)),"lost CPU cache");Need(Graphics.calls==1,"GPU copy repeated in frame loop");
  QualitySettings.activeColorSpace=ColorSpace.Gamma;GL.sRGBWrite=true;RenderTexture.active=null;Pixels(proof.ArtScenePixels(Source(false)).pixels);Need(RenderTexture.active==null&&GL.sRGBWrite,"gamma/backbuffer state not restored");
  var bad=Source(false);bad.readFail=true;var failed=proof.ArtScenePixels(bad);Need(failed.pixels==null&&failed.error.Contains("readback failed")&&Debug.errors==1,"GPU failure was blank/silent");int calls=Graphics.calls,reads=RenderTexture.acquired;for(int i=0;i<100;i++)Need(Object.ReferenceEquals(failed,proof.ArtScenePixels(bad)),"failed preparation did not latch");Need(Graphics.calls==calls&&RenderTexture.acquired==reads&&Debug.errors==1,"GPU failure retried/logged per repaint");Need(RenderTexture.active==null&&GL.sRGBWrite&&RenderTexture.acquired==RenderTexture.released&&destroyed==3,"readback exception leaked resource/state");
  Graphics.fail=true;var blitFailed=proof.ArtScenePixels(Source(false));Need(blitFailed.error.Contains("blit failed")&&RenderTexture.acquired==RenderTexture.released&&destroyed==3&&RenderTexture.active==null&&GL.sRGBWrite,"blit exception cleanup failed");
  var empty=Source(true);empty.incomplete=true;var incomplete=proof.ArtScenePixels(empty);Need(incomplete.pixels==null&&incomplete.error.Contains("Incomplete pixel data"),"empty/corrupt source silently became empty mask");
  Graphics.fail=false;Application.isPlaying=false;Pixels(proof.ArtScenePixels(Source(false)).pixels);Need(immediate==1&&destroyed==3&&RenderTexture.acquired==RenderTexture.released,"Editor cold read must destroy temporary texture immediately without scheduling Destroy");
  var editorBad=Source(false);editorBad.readFail=true;var editorError=proof.ArtScenePixels(editorBad);Need(editorError.pixels==null&&editorError.error.Contains("readback failed")&&immediate==2&&destroyed==3&&RenderTexture.acquired==RenderTexture.released&&RenderTexture.active==null&&GL.sRGBWrite,"Editor exception path must immediately destroy temporary texture and restore render state");
  Console.WriteLine("readable and retained GPU-only imports: once-only pixels, alpha/orientation, failure latch, Play/Edit cleanup/state PASS");
 }
}`)
	prepare := sceneTemplateSection(t, "painted.cs.tmpl", "  PaintedCache PreparePaintedScene(", "  static Rect[] BuildPaintedOpaqueRects(")
	if strings.Contains(prepare, ".GetPixels32(") || !strings.Contains(prepare, "ArtScenePixels(cache.background)") || !strings.Contains(prepare, "cache.preparationError=source.error;paintedCaches[scene.level_id]=cache;return cache;") {
		t.Fatal("painted cold preparation bypasses safe source cache/error latch")
	}
	draw := sceneTemplateSection(t, "painted.cs.tmpl", "  bool DrawPaintedGame(", "\n }\n}")
	guard := strings.Index(draw, "if(cache.preparationError!=null)")
	if guard < 0 || guard >= strings.Index(draw, "UpdatePaintedFog(") || !strings.Contains(draw[guard:strings.Index(draw, "UpdatePaintedFog(")], `L("ui.scene.unavailable"),hudWrap);return true;`) {
		t.Fatal("failed occlusion preparation must show its error and stop before actors/fog, not silently expose world")
	}
}

// Config changes must repair retained PNG imports without broad project reload,
// changing the current scene, or reimporting assets while a game is running.
func TestPaintedBackgroundImportRepairNative(t *testing.T) {
	data, err := sources.ReadFile("templates/editor.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, "[InitializeOnLoad]")
	if start < 0 {
		t.Fatal("retained background imports need an Editor initialization repair")
	}
	runPaintedTextureNative(t, "BackgroundImportRepair", `using System;using System.IO;using System.Collections.Generic;using UnityEditor;using UnityEngine;
namespace UnityEngine {
 public enum FilterMode {Point,Bilinear,Trilinear}public enum TextureWrapMode {Clamp,Repeat}
 public static class Application {public static bool isBatchMode;}
 public static class JsonUtility {public static PocResource resource;public static T FromJson<T>(string value){return (T)(object)resource;}}
}
public class PocResource {public PocArt art;public PocSurfaceBundle location_sdk;}public class PocArt {public PocScene[] painted;}public class PocScene {public string background;}public class PocSurfaceBundle {public PocSurface[] surfaces;}public class PocSurface {public string top_image,side_image;}
namespace UnityEditor {
 [AttributeUsage(AttributeTargets.Class)]public class InitializeOnLoadAttribute:Attribute{}
 public enum PlayModeStateChange {EnteredEditMode,EnteredPlayMode}public enum TextureImporterNPOTScale {ToNearest,None}public enum TextureImporterCompression {Compressed,Uncompressed}public enum TextureImporterType {Default,Sprite}
 public class AssetImporter {public string assetPath;public static readonly Dictionary<string,AssetImporter> assets=new Dictionary<string,AssetImporter>();public static AssetImporter GetAtPath(string path){AssetImporter a;return assets.TryGetValue(path,out a)?a:null;}}
 public class TextureImporterPlatformSettings {public bool overridden;}
 public class TextureImporter:AssetImporter {public int saves,maxTextureSize;public bool isReadable,mipmapEnabled,alphaIsTransparency,sRGBTexture;public TextureImporterType textureType;public TextureImporterNPOTScale npotScale;public FilterMode filterMode;public TextureImporterCompression textureCompression;public TextureWrapMode wrapMode;public void SaveAndReimport(){saves++;}public TextureImporterPlatformSettings GetPlatformTextureSettings(string name){return new TextureImporterPlatformSettings();}public void ClearPlatformTextureSettings(string name){}}
 public class AssetPostprocessor {protected string assetPath;protected AssetImporter assetImporter;public virtual uint GetVersion(){return 0;}}
 public static class EditorApplication {public static bool isPlayingOrWillChangePlaymode;public static event Action delayCall;public static event Action<PlayModeStateChange> playModeStateChanged;public static void Flush(){var call=delayCall;delayCall=null;if(call!=null)call();}public static void Edit(){isPlayingOrWillChangePlaymode=false;if(playModeStateChanged!=null)playModeStateChanged(PlayModeStateChange.EnteredEditMode);}}
}
`+s[start:]+`
class Proof {
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 static TextureImporter Add(string name){var p="Assets/Resources/MarketArt/"+name+".png";var t=new TextureImporter{assetPath=p};AssetImporter.assets[p]=t;return t;}
 static void Config(params string[] names){var a=new List<PocScene>();foreach(var n in names)a.Add(new PocScene{background="MarketArt/"+n});JsonUtility.resource=new PocResource{art=new PocArt{painted=a.ToArray()}};}
 static void Import(params string[] paths){typeof(PocCharacterTextureImporter).GetMethod("OnPostprocessAllAssets",System.Reflection.BindingFlags.Static|System.Reflection.BindingFlags.NonPublic).Invoke(null,new object[]{paths,new string[0],new string[0],new string[0]});}
 public static void Main(){
  Directory.CreateDirectory("Assets/Resources");File.WriteAllText("Assets/Resources/PocConfig.json","{}");var market=Add("market");var prop=Add("prop");Config("market","market");new PocCharacterTextureImporter();EditorApplication.Flush();Need(market.isReadable&&market.saves==1&&prop.saves==0,"stale background not repaired exactly once / unrelated texture changed");Need(market.maxTextureSize==4096&&market.mipmapEnabled&&market.textureCompression==TextureImporterCompression.Uncompressed,"repair lost established art import policy");
  Import("Assets/Resources/PocConfig.json");EditorApplication.Flush();Need(market.saves==1,"already-correct retained import reimported on every config event");
  var home=Add("home");Config("market","home");EditorApplication.isPlayingOrWillChangePlaymode=true;Import("Assets/Resources/PocConfig.json");EditorApplication.Flush();Need(home.saves==0&&!home.isReadable,"live Editor game was disrupted by reimport");EditorApplication.Edit();EditorApplication.Flush();Need(home.saves==1&&home.isReadable&&market.saves==1,"deferred config repair not applied in Edit mode");
  var other=Add("other");Config("other");Import("Assets/Other.json");EditorApplication.Flush();Need(other.saves==0,"unrelated import triggered broad repair");Import("Assets/Resources/PocConfig.json");EditorApplication.Flush();Need(other.saves==1&&other.isReadable&&prop.saves==0,"config changed but existing PNG remained stale");Need(!PocCharacterTextureImporter.ConfigureArtScene(other),"stable importer changed repeatedly");PocCharacterTextureImporter.ConfigureArtScene(prop);Need(!prop.isReadable,"GPU-only non-background textures unnecessarily retain CPU data");
  Console.WriteLine("targeted retained background repair: startup/config, dedupe, no-op, defer live game, unrelated imports untouched PASS");
 }
}`)
}
