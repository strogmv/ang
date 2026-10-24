package gamepoc

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func paintedArchitectureFixture() Compiled {
	c := paintedFixture()
	c.Art.Painted[0].Occluders = []PaintedOccluder{{
		ID: "bench", Polygon: []PaintedPoint{{100, 100}, {150, 90}, {180, 160}}, DepthY: 160,
		VisibilityAnchor: &Point{X: 1, Y: 2},
	}}
	return c
}

func TestPaintedArchitectureAnchorUsesRealWalkableCell(t *testing.T) {
	c := paintedArchitectureFixture()
	if err := checkPaintedScenes(c); err != nil {
		t.Fatal(err)
	}
	for _, p := range []Point{{-1, 2}, {1, -1}, {4, 2}, {2, 4}, {0, 0}} {
		invalid := paintedArchitectureFixture()
		invalid.Art.Painted[0].Occluders[0].VisibilityAnchor = &p
		if err := checkPaintedScenes(invalid); err == nil || !strings.Contains(err.Error(), "visibility_anchor") {
			t.Fatalf("invalid architectural approach accepted or wrong error at %+v: %v", p, err)
		}
	}
	before := string(c.Life)
	data, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	var projected struct {
		Art struct {
			Painted []struct {
				Known     bool `json:"known_architecture"`
				Occluders []struct {
					Anchor    *Point `json:"visibility_anchor"`
					HasAnchor bool   `json:"has_visibility_anchor"`
				} `json:"occluders"`
			} `json:"painted"`
		} `json:"art"`
	}
	if err = json.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	p := projected.Art.Painted[0]
	if p.Known || !p.Occluders[0].HasAnchor || p.Occluders[0].Anchor == nil || *p.Occluders[0].Anchor != (Point{1, 2}) || string(c.Life) != before {
		t.Fatal("architecture anchor changed authoritative memory or was lost in projection")
	}
	c.Art.Painted[0].Occluders[0].VisibilityAnchor = nil
	if err = checkPaintedScenes(c); err != nil {
		t.Fatalf("old unanchored foreground is no longer supported: %v", err)
	}
}

func TestPaintedArchitectureAnchorPresenceDoesNotGuessZeroCoordinates(t *testing.T) {
	for _, anchor := range []*Point{nil, {X: 0, Y: 0}, {X: 1, Y: 2}} {
		data, err := json.Marshal(PaintedOccluder{ID: "surface", VisibilityAnchor: anchor})
		if err != nil {
			t.Fatal(err)
		}
		var projected map[string]json.RawMessage
		if err := json.Unmarshal(data, &projected); err != nil {
			t.Fatal(err)
		}
		if string(projected["has_visibility_anchor"]) != map[bool]string{true: "true", false: "false"}[anchor != nil] {
			t.Fatalf("presence was not computed from the optional pointer: %s", data)
		}
		if anchor == nil && projected["visibility_anchor"] != nil {
			t.Fatal("absent anchor acquired zero coordinates")
		}
	}
	// A real zero anchor must pass authoring validation when that cell is floor.
	c := paintedArchitectureFixture()
	c.Art.Painted[0].Navigation[0] = ".###"
	c.Art.Painted[0].ProofBlocked = Point{3, 0}
	c.Art.Painted[0].Occluders[0].VisibilityAnchor = &Point{0, 0}
	var life map[string]any
	if err := json.Unmarshal(c.Life, &life); err != nil {
		t.Fatal(err)
	}
	market := life["locations"].(map[string]any)["market"].(map[string]any)
	market["terrain"].([]any)[0] = ".###"
	var err error
	c.Life, err = json.Marshal(life)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPaintedScenes(c); err != nil {
		t.Fatalf("real floor anchor at origin rejected: %v", err)
	}
	prepare := sceneTemplateSection(t, "painted.cs.tmpl", "  PaintedCache PreparePaintedScene(", "  static Rect[] BuildPaintedOpaqueRects(")
	if !strings.Contains(prepare, "visibilityAnchor=PaintedOccluderAnchor(occluder)") {
		t.Fatal("Unity cache bypasses explicit optional-anchor presence")
	}
}

func TestPaintedArchitectureUpdatesBeforeSpatialDirtyGuard(t *testing.T) {
	body := sceneTemplateSection(t, "painted.cs.tmpl", "  void UpdatePaintedFog(", "  static char PaintedArchitectureAnchorState(")
	anchor := strings.Index(body, "if(cut.visibilityAnchor!=null)")
	spatial := strings.Index(body, "if(!PaintedFogRegionChanged(")
	if anchor < 0 || spatial < anchor || strings.Contains(body, "==0&&cache.fogReady)return") {
		t.Fatal("an anchor outside its raised silhouette must refresh even when its texture region has no dirty floor samples")
	}
	for _, bad := range []string{"knownArchitecture=true", "cache.visibility[i]=", "state.LifeExploration"} {
		helpers := sceneTemplateSection(t, "painted.cs.tmpl", "  static char PaintedArchitectureAnchorState(", "  static void SoftenPaintedFog(")
		if strings.Contains(helpers, bad) {
			t.Fatalf("architectural visibility promotes authoritative knowledge: %s", bad)
		}
	}
}

func TestPaintedArchitectureAnchorPrivacyAndUpdatesNative(t *testing.T) {
	mono, err := exec.LookPath("mono")
	if err != nil {
		matches, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
		if len(matches) == 0 {
			t.Skip("Mono unavailable")
		}
		mono = matches[len(matches)-1]
	}
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	if _, err = os.Stat(csc); err != nil {
		t.Skip("Mono compiler unavailable")
	}
	body := sceneTemplateSection(t, "painted.cs.tmpl", "  static char PaintedArchitectureAnchorState(", "  static void SoftenPaintedFog(")
	body += sceneTemplateSection(t, "painted.cs.tmpl", "  static Color32 CompositePaintedFog(", "  static void ShadePaintedPixels(")
	body += sceneTemplateSection(t, "painted.cs.tmpl", "  static Color32 PaintedFogColor(", "  static Rect PaintedFit(")
	harness := `using System;
struct Color32 { public byte r,g,b,a; public Color32(byte red,byte green,byte blue,byte alpha){r=red;g=green;b=blue;a=alpha;} }
class PocPaintedPoint { public float x,y; public PocPaintedPoint(float xx,float yy){x=xx;y=yy;} }
class PocPaintedOccluder {public bool has_visibility_anchor;public PocPaintedPoint visibility_anchor;}
class PaintedCache { public int gridWidth=2,gridHeight=2; public char[] visibility=new[]{'0','0','0','0'}; public bool knownArchitecture; public bool[] publicShell=new bool[4]; }
class PaintedCutout { public PocPaintedPoint visibilityAnchor=new PocPaintedPoint(1,1); public bool anchorFogReady; public char anchorVisibility; public Color32[] pixels=new[]{new Color32(200,150,100,255),new Color32(50,60,70,0),new Color32(80,100,120,123)}; public Color32[] shadedPixels=new Color32[3]; }
class Proof {
` + body + `
 static void Require(bool value,string why){if(!value)throw new Exception(why);}
 static bool Equal(Color32 a,Color32 b){return a.r==b.r&&a.g==b.g&&a.b==b.b&&a.a==b.a;}
 static void Privacy(){
  var c=new PaintedCache();var cut=new PaintedCutout();var original=(char[])c.visibility.Clone();c.visibility[3]='2';original[3]='2';
  Require(ShadePaintedArchitecture(cut,c),"visible raised object not refreshed");
  for(int i=0;i<cut.pixels.Length;i++)Require(Equal(cut.pixels[i],cut.shadedPixels[i]),"visible source or transparent silhouette changed");
  Require(PaintedFogColor(c,0).a==255,"raised surface revealed adjacent unknown floor");
  for(int i=0;i<c.visibility.Length;i++)Require(c.visibility[i]==original[i],"surface changed server observation");
  c.visibility[3]='1';Require(ShadePaintedArchitecture(cut,c),"remembered object not refreshed");
  Require(Equal(cut.shadedPixels[0],CompositePaintedFog(cut.pixels[0],new Color32(18,23,25,145))),"memory is not dimmed");
  for(int i=0;i<cut.pixels.Length;i++)Require(cut.shadedPixels[i].a==cut.pixels[i].a,"polygon alpha changed");
  c.visibility[3]='0';Require(ShadePaintedArchitecture(cut,c),"unknown object retained visible image");Require(cut.shadedPixels[0].r==8&&cut.shadedPixels[0].g==13&&cut.shadedPixels[0].b==16,"unknown architecture leaked");
 }
 static void Updates(){
  var c=new PaintedCache();var cut=new PaintedCutout();Require(ShadePaintedArchitecture(cut,c),"cold unknown object not shaded");Require(!ShadePaintedArchitecture(cut,c),"unchanged anchor causes texture upload");
  // The silhouette maps to unknown floor cell zero; its physical approach is
  // cell three, outside that texture's floor samples and dirty bounds.
  foreach(char seen in new[]{'2','1','0'}){c.visibility[3]=seen;Require(ShadePaintedArchitecture(cut,c),"remote physical anchor change skipped");Require(!ShadePaintedArchitecture(cut,c),"stable state uploads again");Require(PaintedFogColor(c,0).a==255,"anchor update changed unobserved floor");}
 }
 static void Invalid(){
  var c=new PaintedCache{knownArchitecture=true,publicShell=new[]{true,true,true,true},visibility=new[]{'2','2','2','2'}};
  var absent=new PocPaintedOccluder{visibility_anchor=new PocPaintedPoint(0,0)};
  Require(PaintedOccluderAnchor(absent)==null,"JsonUtility's omitted zero DTO became a real anchor");
  absent.has_visibility_anchor=true;Require(PaintedArchitectureAnchorState(PaintedOccluderAnchor(absent),c)=='2',"explicit origin anchor was guessed absent");
  Require(PaintedOccluderAnchor(null)==null,"missing occluder acquired an anchor");
  foreach(var p in new[]{new PocPaintedPoint(-1,0),new PocPaintedPoint(2,0),new PocPaintedPoint(0,2),new PocPaintedPoint(.5f,0),new PocPaintedPoint(float.NaN,0),new PocPaintedPoint(0,float.PositiveInfinity)})Require(PaintedArchitectureAnchorState(p,c)=='0',"invalid anchor revealed architecture");
  Require(PaintedArchitectureAnchorState(null,c)=='0'&&PaintedArchitectureAnchorState(new PocPaintedPoint(0,0),null)=='0',"missing anchor/cache revealed architecture");
  c.visibility[3]='0';var cut=new PaintedCutout();ShadePaintedArchitecture(cut,c);Require(cut.shadedPixels[0].r==8,"preknown shell overrides unknown anchor");
 }
 public static void Main(string[] args){if(args[0]=="privacy")Privacy();else if(args[0]=="updates")Updates();else Invalid();Console.WriteLine(args[0]+" architecture proof PASS");}
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "ArchitectureProof.cs"), filepath.Join(dir, "ArchitectureProof.exe")
	if err = os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile architectural visibility: %v\n%s", err, out)
	}
	for _, name := range []string{"privacy", "updates", "invalid"} {
		t.Run(name, func(t *testing.T) {
			out, err := exec.Command(mono, binary, name).CombinedOutput()
			if err != nil {
				t.Fatalf("architecture %s: %v\n%s", name, err, out)
			}
			t.Log(strings.TrimSpace(string(out)))
		})
	}
}
