package gamepoc

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func sdkSurfaceImageBundle(top, side string) locationsdk.RuntimeBundle {
	return locationsdk.RuntimeBundle{Surfaces: []locationsdk.Surface{{ID: "surface.test", TopImage: top, SideImage: side}}}
}
func sdkSurfacePNG(t *testing.T, root, key string, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	sdkSurfaceFile(t, root, key, buf.Bytes())
	return buf.Bytes()
}
func sdkSurfaceFile(t *testing.T, root, key string, data []byte) string {
	t.Helper()
	path := filepath.Join(root, "Assets", "Resources", filepath.FromSlash(key)+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func sdkOpaqueImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x*41 + 19), G: uint8(y*71 + 12), B: 38, A: 255})
		}
	}
	return img
}

func TestLocationSDKSurfaceImagesOpaqueAndOptional(t *testing.T) {
	if err := checkLocationSDKSurfaceImages(filepath.Join(t.TempDir(), "not-created"), locationsdk.RuntimeBundle{}); err != nil {
		t.Fatal("untextured diagnostic kit now requires an asset folder:", err)
	}
	root := t.TempDir()
	sdkSurfacePNG(t, root, "LocationSDK/top", sdkOpaqueImage())
	// An unused transparent palette entry is not a transparent surface pixel.
	palette := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.NRGBA{R: 41, A: 255}, color.NRGBA{A: 0}})
	sdkSurfacePNG(t, root, "LocationSDK/side", palette)
	bundle := sdkSurfaceImageBundle("LocationSDK/top", "LocationSDK/side")
	before := append([]locationsdk.Surface(nil), bundle.Surfaces...)
	if err := checkLocationSDKSurfaceImages(root, bundle); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bundle.Surfaces, before) {
		t.Fatal("asset validation changed authored materials")
	}
	for _, rect := range []image.Rectangle{image.Rect(0, 0, 4096, 1), image.Rect(0, 0, 1, 4096)} {
		sdkSurfacePNG(t, root, "LocationSDK/edge", image.NewGray(rect))
		if err := checkLocationSDKSurfaceImages(root, sdkSurfaceImageBundle("LocationSDK/edge", "LocationSDK/edge")); err != nil {
			t.Fatal("valid 4096px dimension rejected:", err)
		}
	}
}

func TestLocationSDKSurfaceImagesRejectBadAssets(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		modify     func(*testing.T, string)
	}{
		{"missing", "side_image", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "Assets/Resources/LocationSDK/side.png")); err != nil {
				t.Fatal(err)
			}
		}},
		{"not PNG", "valid PNG", func(t *testing.T, root string) {
			var data bytes.Buffer
			if err := jpeg.Encode(&data, sdkOpaqueImage(), nil); err != nil {
				t.Fatal(err)
			}
			sdkSurfaceFile(t, root, "LocationSDK/side", data.Bytes())
		}},
		{"truncated pixels", "damaged PNG pixels", func(t *testing.T, root string) {
			data := sdkSurfacePNG(t, root, "LocationSDK/side", sdkOpaqueImage())
			at := bytes.Index(data, []byte("IDAT"))
			if at < 0 {
				t.Fatal("fixture has no IDAT")
			}
			data[at+4] ^= 0xff
			sdkSurfaceFile(t, root, "LocationSDK/side", data)
		}},
		{"transparent", "fully opaque", func(t *testing.T, root string) {
			img := sdkOpaqueImage()
			img.SetNRGBA(2, 1, color.NRGBA{R: 30, A: 0})
			sdkSurfacePNG(t, root, "LocationSDK/side", img)
		}},
		{"partly transparent", "fully opaque", func(t *testing.T, root string) {
			img := sdkOpaqueImage()
			img.SetNRGBA(0, 1, color.NRGBA{R: 30, A: 254})
			sdkSurfacePNG(t, root, "LocationSDK/side", img)
		}},
		{"16-bit partly transparent", "fully opaque", func(t *testing.T, root string) {
			img := image.NewNRGBA64(image.Rect(0, 0, 1, 1))
			img.SetNRGBA64(0, 0, color.NRGBA64{R: 0xffff, A: 0xfffe})
			sdkSurfacePNG(t, root, "LocationSDK/side", img)
		}},
		{"wide", "dimensions", func(t *testing.T, root string) {
			sdkSurfacePNG(t, root, "LocationSDK/side", image.NewGray(image.Rect(0, 0, 4097, 1)))
		}},
		{"tall", "dimensions", func(t *testing.T, root string) {
			sdkSurfacePNG(t, root, "LocationSDK/side", image.NewGray(image.Rect(0, 0, 1, 4097)))
		}},
		{"oversized file", "PNG exceeds", func(t *testing.T, root string) {
			file, err := os.OpenFile(filepath.Join(root, "Assets/Resources/LocationSDK/side.png"), os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			err = file.Truncate(locationSDKSurfaceMaxFileBytes + 1)
			closeErr := file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}},
		{"directory", "regular file", func(t *testing.T, root string) {
			path := filepath.Join(root, "Assets/Resources/LocationSDK/side.png")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sdkSurfacePNG(t, root, "LocationSDK/top", sdkOpaqueImage())
			sdkSurfacePNG(t, root, "LocationSDK/side", sdkOpaqueImage())
			tc.modify(t, root)
			if err := checkLocationSDKSurfaceImages(root, sdkSurfaceImageBundle("LocationSDK/top", "LocationSDK/side")); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestLocationSDKSurfaceImagesContainResources(t *testing.T) {
	root := t.TempDir()
	sdkSurfacePNG(t, root, "LocationSDK/top", sdkOpaqueImage())
	for _, key := range []string{"", "../outside", "/tmp/outside", "LocationSDK/../top", "LocationSDK/./top", "LocationSDK/top.png", `LocationSDK\top`, "LocationSDK//top", "https://example.org/top", "LocationSDK/пол", "LocationSDK/top\n", strings.Repeat("a", 257)} {
		if err := checkLocationSDKSurfaceImages(root, sdkSurfaceImageBundle("LocationSDK/top", key)); err == nil || !strings.Contains(err.Error(), "unsafe") {
			t.Fatalf("unsafe Resources key %q accepted or wrong failure: %v", key, err)
		}
	}
	outside := sdkSurfaceFile(t, t.TempDir(), "outside", []byte("not PNG"))
	escape := filepath.Join(root, "Assets/Resources/LocationSDK/escape.png")
	if err := os.Symlink(outside, escape); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := checkLocationSDKSurfaceImages(root, sdkSurfaceImageBundle("LocationSDK/top", "LocationSDK/escape")); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("external symlink bypassed Resources root: %v", err)
	}
	inside := filepath.Join(root, "Assets/Resources/LocationSDK/inside.png")
	if err := os.Symlink("top.png", inside); err != nil {
		t.Fatal(err)
	}
	if err := checkLocationSDKSurfaceImages(root, sdkSurfaceImageBundle("LocationSDK/top", "LocationSDK/inside")); err != nil {
		t.Fatal("safe relative symlink inside Resources rejected:", err)
	}
}

func TestLocationSDKSurfaceImagesUniqueResourceBudgets(t *testing.T) {
	root := t.TempDir()
	// Gray is opaque and compact on disk, so this reaches the decoded texel
	// limit independently of the encoded-file limit or transparency validation.
	data := sdkSurfacePNG(t, root, "LocationSDK/pixels0", image.NewGray(image.Rect(0, 0, 4096, 4096)))
	bundle := locationsdk.RuntimeBundle{}
	for i, key := range []string{"LocationSDK/pixels0", "LocationSDK/pixels1", "LocationSDK/pixels2", "LocationSDK/pixels3", "LocationSDK/pixels4"} {
		if i > 0 {
			sdkSurfaceFile(t, root, key, data)
		}
		bundle.Surfaces = append(bundle.Surfaces, locationsdk.Surface{ID: key, TopImage: key, SideImage: key})
	}
	if err := checkLocationSDKSurfaceImages(root, bundle); err == nil || !strings.Contains(err.Error(), "total pixels") {
		t.Fatalf("expected decoded-pixel budget: %v", err)
	}
	// A legal ancillary chunk consumes encoded bytes without inventing a giant
	// decoded image. Five separate near-16MiB files exceed the batch budget.
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, sdkOpaqueImage()); err != nil {
		t.Fatal(err)
	}
	plain := encoded.Bytes()
	payload := make([]byte, locationSDKSurfaceMaxFileBytes-1024)
	var padded bytes.Buffer
	padded.Write(plain[:len(plain)-12])
	if err := binary.Write(&padded, binary.BigEndian, uint32(len(payload))); err != nil {
		t.Fatal(err)
	}
	typeAndData := append([]byte("vpAg"), payload...)
	padded.Write(typeAndData)
	if err := binary.Write(&padded, binary.BigEndian, crc32.ChecksumIEEE(typeAndData)); err != nil {
		t.Fatal(err)
	}
	padded.Write(plain[len(plain)-12:])
	bundle.Surfaces = nil
	for _, key := range []string{"LocationSDK/bytes0", "LocationSDK/bytes1", "LocationSDK/bytes2", "LocationSDK/bytes3", "LocationSDK/bytes4"} {
		sdkSurfaceFile(t, root, key, padded.Bytes())
		bundle.Surfaces = append(bundle.Surfaces, locationsdk.Surface{ID: key, TopImage: key, SideImage: key})
	}
	if err := checkLocationSDKSurfaceImages(root, bundle); err == nil || !strings.Contains(err.Error(), "unique PNG files") {
		t.Fatalf("expected encoded-byte budget: %v", err)
	}
	// Many material references to one tile must be charged once, on both budgets.
	for i := range bundle.Surfaces {
		bundle.Surfaces[i].TopImage = "LocationSDK/bytes0"
		bundle.Surfaces[i].SideImage = "LocationSDK/bytes0"
	}
	if err := checkLocationSDKSurfaceImages(root, bundle); err != nil {
		t.Fatal("reused tile was repeatedly charged:", err)
	}
}

func TestLocationSDKSurfaceImporterUsesDeclaredGPUOnlyTilesNative(t *testing.T) {
	data, err := sources.ReadFile("templates/editor.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, "[InitializeOnLoad]")
	if start < 0 {
		t.Fatal("missing targeted importer initialization")
	}
	runPaintedTextureNative(t, "SDKSurfaceImport", `using System;using System.IO;using System.Collections.Generic;using UnityEditor;using UnityEngine;
namespace UnityEngine {
 public enum FilterMode {Point,Bilinear,Trilinear}public enum TextureWrapMode {Clamp,Repeat}
 public static class Application {public static bool isBatchMode;}
 public static class JsonUtility {public static PocResource resource;public static T FromJson<T>(string value){return (T)(object)resource;}}
}
public class PocResource {public PocArt art;public PocSurfaceBundle location_sdk;}public class PocArt {public PocScene[] painted;}public class PocScene {public string background;}public class PocSurfaceBundle {public PocSurface[] surfaces;}public class PocSurface {public string top_image,side_image;}
namespace UnityEditor {
 [AttributeUsage(AttributeTargets.Class)]public class InitializeOnLoadAttribute:Attribute{}
 public enum PlayModeStateChange {EnteredEditMode,EnteredPlayMode}public enum TextureImporterNPOTScale {ToNearest,None}public enum TextureImporterCompression {Compressed,Uncompressed}public enum TextureImporterType {Default,Sprite}
 public class AssetImporter {public string assetPath;public static int lookups;public static readonly Dictionary<string,AssetImporter> assets=new Dictionary<string,AssetImporter>();public static AssetImporter GetAtPath(string path){lookups++;AssetImporter a;return assets.TryGetValue(path,out a)?a:null;}}
 public class TextureImporterPlatformSettings {public bool overridden=true;}
 public class TextureImporter:AssetImporter {public int saves,maxTextureSize;public bool isReadable,mipmapEnabled,alphaIsTransparency,sRGBTexture;public TextureImporterType textureType;public TextureImporterNPOTScale npotScale;public FilterMode filterMode;public TextureImporterCompression textureCompression;public TextureWrapMode wrapMode;public readonly TextureImporterPlatformSettings desktop=new TextureImporterPlatformSettings();public void SaveAndReimport(){saves++;}public TextureImporterPlatformSettings GetPlatformTextureSettings(string name){if(name!="Standalone")throw new Exception("unrelated platform changed");return desktop;}public void ClearPlatformTextureSettings(string name){if(name!="Standalone")throw new Exception("unrelated platform changed");desktop.overridden=false;}}
 public class AssetPostprocessor {protected string assetPath;protected AssetImporter assetImporter;public virtual uint GetVersion(){return 0;}}
 public static class EditorApplication {public static bool isPlayingOrWillChangePlaymode;public static event Action delayCall;public static event Action<PlayModeStateChange> playModeStateChanged;public static void Flush(){var call=delayCall;delayCall=null;if(call!=null)call();}public static void Edit(){isPlayingOrWillChangePlaymode=false;if(playModeStateChanged!=null)playModeStateChanged(PlayModeStateChange.EnteredEditMode);}}
}
`+s[start:]+`
class Proof {
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 static TextureImporter Add(string key){var t=new TextureImporter{assetPath="Assets/Resources/"+key+".png",isReadable=true,textureType=TextureImporterType.Sprite,alphaIsTransparency=true};AssetImporter.assets[t.assetPath]=t;return t;}
 static void Config(params string[] keys){var a=new List<PocSurface>();foreach(string key in keys)a.Add(new PocSurface{top_image=key,side_image=key});JsonUtility.resource=new PocResource{location_sdk=new PocSurfaceBundle{surfaces=a.ToArray()}};}
 static void Import(){typeof(PocCharacterTextureImporter).GetMethod("OnPostprocessAllAssets",System.Reflection.BindingFlags.Static|System.Reflection.BindingFlags.NonPublic).Invoke(null,new object[]{new[]{"Assets/Resources/PocConfig.json"},new string[0],new string[0],new string[0]});}
 static void Preprocess(TextureImporter texture){var p=new PocCharacterTextureImporter();typeof(AssetPostprocessor).GetField("assetPath",System.Reflection.BindingFlags.Instance|System.Reflection.BindingFlags.NonPublic).SetValue(p,texture.assetPath);typeof(AssetPostprocessor).GetField("assetImporter",System.Reflection.BindingFlags.Instance|System.Reflection.BindingFlags.NonPublic).SetValue(p,texture);typeof(PocCharacterTextureImporter).GetMethod("OnPreprocessTexture",System.Reflection.BindingFlags.Instance|System.Reflection.BindingFlags.NonPublic).Invoke(p,new object[0]);}
 static void Policy(TextureImporter t){Need(!t.isReadable&&t.textureType==TextureImporterType.Default&&t.sRGBTexture&&t.maxTextureSize==4096&&t.npotScale==TextureImporterNPOTScale.None&&t.mipmapEnabled&&t.filterMode==FilterMode.Trilinear&&t.wrapMode==TextureWrapMode.Repeat&&t.textureCompression==TextureImporterCompression.Uncompressed&&!t.alphaIsTransparency&&!t.desktop.overridden,"declared surface import policy lost GPU-only color/repeat/NPOT/mipmaps or retained desktop compression override");}
 public static void Main(){
  Directory.CreateDirectory("Assets/Resources");File.WriteAllText("Assets/Resources/PocConfig.json","{}");var tile=Add("LocationSDK/tile");var other=Add("LocationSDK/other");Config("LocationSDK/tile","LocationSDK/tile");var importer=new PocCharacterTextureImporter();Need(importer.GetVersion()==5,"global texture importer version changed");EditorApplication.Flush();Policy(tile);Need(tile.saves==1&&other.saves==0&&other.isReadable,"startup repaired wrong tile or duplicate reference twice");
  PocCharacterTextureImporter.PrepareSDKSurfaceImports(JsonUtility.resource);Need(tile.saves==1,"Prepare reimported already-correct tile");
  Config("LocationSDK/other");EditorApplication.isPlayingOrWillChangePlaymode=true;Import();EditorApplication.Flush();Need(other.saves==0&&other.isReadable,"config reimport disrupted a live game");EditorApplication.Edit();EditorApplication.Flush();Policy(other);Need(other.saves==1,"SDK-only config with no painted art did not repair in Edit mode");
  var fresh=Add("LocationSDK/fresh");Config("LocationSDK/fresh");Preprocess(fresh);Policy(fresh);Need(fresh.saves==0,"preprocess recursively reimported itself");Need(!PocCharacterTextureImporter.Configure(fresh,true),"character/sprite Prepare overrode a declared surface policy");Policy(fresh);var unknown=Add("LocationSDK/unknown");Preprocess(unknown);Need(unknown.isReadable&&unknown.textureType==TextureImporterType.Sprite,"unreferenced asset was reconfigured");
  Config("../outside");int before=AssetImporter.lookups;bool rejected=false;try{PocCharacterTextureImporter.PrepareSDKSurfaceImports(JsonUtility.resource);}catch(InvalidOperationException){rejected=true;}Need(rejected&&AssetImporter.lookups==before,"unsafe config path reached AssetDatabase lookup");
  Console.WriteLine("declared SDK color tiles: GPU-only/repeat/trilinear, startup/config/Prepare/preprocess, live defer, no broad reimport PASS");
 }
}`)
	if strings.Count(s, "PrepareSDKSurfaceImports(resource);") < 2 {
		t.Fatal("SDK surface importer must run in Prepare and retained-config repair")
	}
	for _, bad := range []string{"GetPixels", "ReadPixels", "Resources.Load"} {
		if strings.Contains(s[start:], bad) {
			t.Errorf("Editor importer must not read runtime texture pixels: %s", bad)
		}
	}
}
