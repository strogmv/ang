package gamepoc

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func paintedInspectionFixture(t *testing.T) Compiled {
	t.Helper()
	c := paintedFixture()
	var life map[string]any
	if err := json.Unmarshal(c.Life, &life); err != nil {
		t.Fatal(err)
	}
	life["inspection"] = map[string]any{"objects": []map[string]any{{"id": "fixture.inspection", "level": "market"}}}
	var err error
	c.Life, err = json.Marshal(life)
	if err != nil {
		t.Fatal(err)
	}
	c.Art.Painted[0].InspectionHotspots = []PaintedInspectionHotspot{{ObjectID: "fixture.inspection", X: 210, Y: 120, Radius: 25}}
	return c
}

func TestPaintedInspectionHotspotsValidateGeometryAndBinding(t *testing.T) {
	if err := checkPaintedScenes(paintedInspectionFixture(t)); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Compiled){
		"missing object": func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].ObjectID = "missing" },
		"wrong level": func(c *Compiled) {
			c.Life = json.RawMessage(strings.ReplaceAll(string(c.Life), `"level":"market"`, `"level":"elsewhere"`))
		},
		"outside canvas":   func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].X = 512 },
		"negative y":       func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].Y = -1 },
		"zero radius":      func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].Radius = 0 },
		"excessive radius": func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].Radius = 513 },
		"nonfinite center": func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].X = math.NaN() },
		"nonfinite radius": func(c *Compiled) { c.Art.Painted[0].InspectionHotspots[0].Radius = math.Inf(1) },
		"count limit": func(c *Compiled) {
			for range 513 {
				c.Art.Painted[0].InspectionHotspots = append(c.Art.Painted[0].InspectionHotspots, c.Art.Painted[0].InspectionHotspots[0])
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := paintedInspectionFixture(t)
			mutate(&c)
			if checkPaintedScenes(c) == nil {
				t.Fatal("invalid hotspot accepted")
			}
		})
	}
	resource, err := clientResource(paintedInspectionFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Art struct {
			Painted []struct {
				Hotspots []map[string]any `json:"inspection_hotspots"`
			} `json:"painted"`
		} `json:"art"`
	}
	if err := json.Unmarshal(resource, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Art.Painted[0].Hotspots) != 1 {
		t.Fatal("geometry omitted from client projection")
	}
	row := result.Art.Painted[0].Hotspots[0]
	if len(row) != 4 || row["object_id"] != "fixture.inspection" || row["radius"] != float64(25) {
		t.Fatalf("hotspot must carry geometry/identity only: %+v", row)
	}
}

func TestPaintedInspectionHoverAuthorizesBeforeFloorFallback(t *testing.T) {
	draw := sceneTemplateSection(t, "painted.cs.tmpl", "  void DrawPaintedKnownDetails(", "  static bool PaintedInspectionFinite(")
	hit := strings.Index(draw, "PaintedInspectionHotspotAt(scene,mouse,fit,scale,view)")
	if hit < 0 || hit > strings.Index(draw, "if(ViewCell(view,x,y)=='0')return;") || hit < strings.Index(draw, "HudBlocksWorldInput(mouse)") {
		t.Fatal("hotspot must follow viewport/HUD guards but precede the mouse-floor visibility rejection")
	}
	for _, want := range []string{`L("ui.exit.enter_hint",marker.Name)`, "obj.X==x&&obj.Y==y", "DrawPaintedInspectionTooltip(obj,mouse)"} {
		if !strings.Contains(draw, want) {
			t.Errorf("lost legacy exit/floor fallback %q", want)
		}
	}
	helpers := sceneTemplateSection(t, "painted.cs.tmpl", "  static bool PaintedInspectionFinite(", "  GUIStyle paintedInspectionTitle;")
	for _, want := range []string{"state.Life.Inspections", "obj.Id==hotspot.object_id", "obj.LevelId==state.Life.LevelId", "HudBlocksWorldInput(mouse)", "!PaintedViewport().Contains(mouse)", "!fit.Contains(mouse)", `view.ActorId!="char.hero"`, "state.Life.Transit"} {
		if !strings.Contains(helpers, want) {
			t.Errorf("missing authorization guard %q", want)
		}
	}
	for _, bad := range []string{"resource.", "BaseText", "Fragments", "Context", "ViewCell(", "state.PlayerX", "localText"} {
		if strings.Contains(helpers, bad) {
			t.Errorf("hotspot geometry must not provide prose, cache perception, or redefine range: %q", bad)
		}
	}
}

// Execute the actual picker/coordinate helpers against real Unity value types.
// Only the authoritative snapshot and unrelated HUD predicate are stubbed.
func TestPaintedInspectionHotspotNative(t *testing.T) {
	editors, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app")
	if len(editors) == 0 {
		t.Skip("Unity assemblies unavailable")
	}
	editor := editors[len(editors)-1]
	mono := filepath.Join(editor, "Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	managed := filepath.Join(editor, "Contents/Resources/Scripting/Managed/UnityEngine")
	core := filepath.Join(managed, "UnityEngine.CoreModule.dll")
	facade := filepath.Join(filepath.Dir(mono), "../lib/mono/4.8-api/Facades/netstandard.dll")
	for _, p := range []string{mono, csc, core, facade} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("Unity dependency unavailable: %s", p)
		}
	}
	helpers := sceneTemplateSection(t, "painted.cs.tmpl", "  static bool PaintedInspectionFinite(", "  GUIStyle paintedInspectionTitle;")
	transforms := sceneTemplateSection(t, "painted.cs.tmpl", "  static Vector2 PaintedCanvasToScreen(", "  void QueuePaintedActor(")
	viewport := sceneTemplateSection(t, "painted.cs.tmpl", "  static Rect PaintedViewport(){", "  static Rect PaintedCameraFit(")
	harness := `using System;using System.Collections.Generic;using UnityEngine;
class PocPaintedInspectionHotspot {public string object_id;public float x,y,radius;}
class PocPaintedScene {public string level_id;public PocPaintedInspectionHotspot[] inspection_hotspots;}
class PocInspectionObject {public string Id,LevelId,Name,BaseText;}
class PocObserverView {public string ActorId="char.hero";}
class Life {public string LevelId="market";public bool Transit;public List<PocInspectionObject> Inspections=new List<PocInspectionObject>();}
class State {public Life Life=new Life();}
class Proof {
 State state=new State();bool uiBlocked;bool HudBlocksWorldInput(Vector2 p){return uiBlocked;}
` + helpers + transforms + viewport + `
 static void Need(bool value,string reason){if(!value)throw new Exception(reason);}
 public static void Main(){
  var p=new Proof();var known=new PocInspectionObject{Id="known",LevelId="market",Name="SERVER_LOCALIZED_NAME",BaseText="SERVER_FILTERED_ONLY"};p.state.Life.Inspections.Add(known);
  var s=new PocPaintedScene{level_id="market",inspection_hotspots=new[]{new PocPaintedInspectionHotspot{object_id="hidden",x=210,y=120,radius=30},new PocPaintedInspectionHotspot{object_id="known",x=210,y=120,radius=25}}};
  var fit=new Rect(100,80,1024,572);float scale=.5f;var v=new PocObserverView();Vector2 center=PaintedCanvasToScreen(new Vector2(210,120),fit,scale);
  Need(System.Object.ReferenceEquals(p.PaintedInspectionHotspotAt(s,center,fit,scale,v),known),"known ID returns exactly the current authorized snapshot object; unknown overlap must not mask it");
  Need(p.PaintedInspectionHotspotAt(s,PaintedCanvasToScreen(new Vector2(235,120),fit,scale),fit,scale,v)==known,"radius boundary is inclusive and in canvas pixels");
  Need(p.PaintedInspectionHotspotAt(s,PaintedCanvasToScreen(new Vector2(236,120),fit,scale),fit,scale,v)==null,"outside radius rejected");
  Need(p.PaintedInspectionHotspotAt(s,PaintedCanvasToScreen(new Vector2(230,140),fit,scale),fit,scale,v)==null,"circle corners are not a square hitbox");
  p.uiBlocked=true;Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"HUD blocks hotspot hover");p.uiBlocked=false;
  Need(p.PaintedInspectionHotspotAt(s,new Vector2(1285,center.y),new Rect(0,0,2000,1000),1,v)==null,"viewport guard precedes geometry");
  Need(p.PaintedInspectionHotspotAt(s,new Vector2(99,center.y),fit,scale,v)==null,"outside image fit cannot hover");
  p.state.Life.Transit=true;Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"in-transit inspection rejected");p.state.Life.Transit=false;
  v.ActorId="npc.observer";Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"debug observer cannot reveal a player tooltip");v.ActorId="char.hero";
  Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,null)==null,"missing observer rejected");
  Need(p.PaintedInspectionHotspotAt(s,center,fit,0,v)==null&&p.PaintedInspectionHotspotAt(s,center,fit,float.NaN,v)==null,"invalid projection scale rejected");
  p.state.Life.Inspections.Clear();Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"previously seen object must not be cached by hotspot metadata");
  var fresh=new PocInspectionObject{Id="known",LevelId="market",Name="FRESH_OTHER_LOCALE",BaseText="CURRENT_FILTERED_TEXT"};p.state.Life.Inspections.Add(fresh);
  Need(System.Object.ReferenceEquals(p.PaintedInspectionHotspotAt(s,center,fit,scale,v),fresh),"new snapshot/locale replaces old prose");
  fresh.LevelId="elsewhere";Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"same ID on a different level rejected");fresh.LevelId="market";
  s.level_id="old_scene";Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"stale scene rejected");s.level_id="market";
  var panFit=new Rect(-35,20,2560,1400);float panScale=1.25f;var moved=PaintedCanvasToScreen(new Vector2(210,120),panFit,panScale);
  Need(p.PaintedInspectionHotspotAt(s,moved,panFit,panScale,v)==fresh,"pan and scale use the same canvas transform as drawing");
  float score;Need(!PaintedInspectionCircle(new PocPaintedInspectionHotspot{object_id="known",x=210,y=120,radius=0},new Vector2(210,120),out score),"zero radius rejected");
  Need(!PaintedInspectionCircle(new PocPaintedInspectionHotspot{object_id="known",x=float.PositiveInfinity,y=120,radius=20},new Vector2(210,120),out score),"nonfinite center rejected");
  Need(!PaintedInspectionCircle(new PocPaintedInspectionHotspot{object_id="known",x=210,y=120,radius=float.NaN},new Vector2(210,120),out score),"nonfinite radius rejected");
  s.inspection_hotspots=null;Need(p.PaintedInspectionHotspotAt(s,center,fit,scale,v)==null,"old scenes without hotspots retain floor fallback");
  Console.WriteLine("painted inspection hotspot geometry / current authority / UI / viewport PASS");
 }
}
`
	dir := t.TempDir()
	source := filepath.Join(dir, "PaintedInspectionProof.cs")
	binary := filepath.Join(dir, "PaintedInspectionProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/r:"+facade, "/r:"+core, "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile painted inspection helpers: %v\n%s", err, out)
	}
	cmd := exec.Command(mono, binary)
	cmd.Env = append(os.Environ(), "MONO_PATH="+managed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("painted inspection helpers: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
