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

func paintedAmbientFixture() Compiled {
	c := paintedFixture()
	z := 18.0
	s := &c.Art.Painted[0]
	s.FloorElevationM = &z
	s.AmbientPassages = []PaintedAmbientPassage{{
		ID: "lower.rail", Image: "MarketArt/train", Width: 160, Height: 60, FeetX: 80, FeetY: 55,
		Path:             []PaintedPoint{{-100, 470}, {600, 400}},
		VisibleStrip:     []PaintedPoint{{10, 400}, {500, 400}, {500, 500}, {10, 500}},
		ObservationCells: []Point{{1, 1}, {2, 2}}, PeriodSeconds: 300, NightPeriodSeconds: 3600, DayStartSecond: 21600, DayEndSecond: 79200, DurationSeconds: 15, ElevationM: 12,
		Track: &PaintedAmbientTrack{Image: "MarketArt/track", X: 0, Y: 400, Width: 512, Height: 112},
	}}
	s.Daylight = &PaintedDaylight{Source: "passive_lightguides", SunriseSecond: 21600, SunsetSecond: 72000, TwilightSeconds: 3600, DayBrightness: 1, NightBrightness: .6}
	return c
}

func TestPaintedAmbientRejectsInvalidLayerGeometryAndSchedule(t *testing.T) {
	if err := checkPaintedScenes(paintedAmbientFixture()); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		mutate func(*PaintedScene)
		reason string
	}{
		"missing floor elevation": {func(s *PaintedScene) { s.FloorElevationM = nil }, "below the floor"},
		"above floor train":       {func(s *PaintedScene) { s.AmbientPassages[0].ElevationM = 18.001 }, "at or below the floor"},
		"nonfinite floor":         {func(s *PaintedScene) { v := math.NaN(); s.FloorElevationM = &v }, "below the floor"},
		"duplicate identity":      {func(s *PaintedScene) { s.AmbientPassages = append(s.AmbientPassages, s.AmbientPassages[0]) }, "duplicate identity"},
		"zero period":             {func(s *PaintedScene) { s.AmbientPassages[0].PeriodSeconds = 0 }, "authoritative schedule"},
		"zero night period":       {func(s *PaintedScene) { s.AmbientPassages[0].NightPeriodSeconds = 0 }, "day/night authoritative schedule"},
		"night too short":         {func(s *PaintedScene) { s.AmbientPassages[0].NightPeriodSeconds = 10 }, "day/night authoritative schedule"},
		"reversed day window":     {func(s *PaintedScene) { s.AmbientPassages[0].DayEndSecond = 21600 }, "day/night authoritative schedule"},
		"longer than period":      {func(s *PaintedScene) { s.AmbientPassages[0].DurationSeconds = 301 }, "authoritative schedule"},
		"offset at period":        {func(s *PaintedScene) { s.AmbientPassages[0].OffsetSeconds = 300 }, "authoritative schedule"},
		"unsafe image":            {func(s *PaintedScene) { s.AmbientPassages[0].Image = "../../secret" }, "sprite/image feet"},
		"bad crop":                {func(s *PaintedScene) { s.AmbientPassages[0].SourceRect = &ArtSceneSourceRect{X: -1, W: 10, H: 10} }, "sprite/image feet"},
		"feet outside sprite":     {func(s *PaintedScene) { s.AmbientPassages[0].FeetY = 61 }, "sprite/image feet"},
		"singleton path":          {func(s *PaintedScene) { s.AmbientPassages[0].Path = []PaintedPoint{{0, 0}} }, "two to 64"},
		"no movement":             {func(s *PaintedScene) { s.AmbientPassages[0].Path = []PaintedPoint{{0, 0}, {0, 0}} }, "empty movement"},
		"nonfinite path":          {func(s *PaintedScene) { s.AmbientPassages[0].Path[0].X = math.Inf(1) }, "path point"},
		"strip outside canvas":    {func(s *PaintedScene) { s.AmbientPassages[0].VisibleStrip[0].X = -1 }, "strip boundary"},
		"crossing strip": {func(s *PaintedScene) {
			s.AmbientPassages[0].VisibleStrip = []PaintedPoint{{0, 400}, {500, 500}, {500, 400}, {0, 500}}
		}, "self-crossing"},
		"strip over floor": {func(s *PaintedScene) {
			s.AmbientPassages[0].VisibleStrip = []PaintedPoint{{100, 50}, {300, 50}, {300, 180}, {100, 180}}
		}, "intersects playable floor"},
		"no observation cells":    {func(s *PaintedScene) { s.AmbientPassages[0].ObservationCells = nil }, "observation cells required"},
		"blocked observation":     {func(s *PaintedScene) { s.AmbientPassages[0].ObservationCells[0] = Point{0, 0} }, "observation cell"},
		"outside observation":     {func(s *PaintedScene) { s.AmbientPassages[0].ObservationCells[0] = Point{4, 1} }, "observation cell"},
		"duplicate observation":   {func(s *PaintedScene) { s.AmbientPassages[0].ObservationCells[1] = Point{1, 1} }, "observation cell"},
		"bad track":               {func(s *PaintedScene) { s.AmbientPassages[0].Track.Width = 0 }, "track overlay"},
		"electronic daylight":     {func(s *PaintedScene) { s.Daylight.Source = "network" }, "passive daylight"},
		"crossing midnight dawn":  {func(s *PaintedScene) { s.Daylight.SunriseSecond = 1 }, "passive daylight"},
		"night blackout":          {func(s *PaintedScene) { s.Daylight.NightBrightness = 0 }, "passive daylight"},
		"night brighter than day": {func(s *PaintedScene) { s.Daylight.NightBrightness = 1.1 }, "passive daylight"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := paintedAmbientFixture()
			tc.mutate(&c.Art.Painted[0])
			if err := checkPaintedScenes(c); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("wanted %q rejection, got %v", tc.reason, err)
			}
		})
	}
}

// A station berth may have the platform's elevation while its rail remains
// outside every navigable cell. Height never replaces the horizontal clip rule.
func TestPaintedAmbientCoplanarStationStillRequiresDisjointFloor(t *testing.T) {
	c := paintedAmbientFixture()
	s := &c.Art.Painted[0]
	s.AmbientPassages[0].ElevationM = *s.FloorElevationM
	if err := checkPaintedScenes(c); err != nil {
		t.Fatalf("coplanar but separated station rail: %v", err)
	}
	s.AmbientPassages[0].VisibleStrip = []PaintedPoint{{100, 50}, {300, 50}, {300, 180}, {100, 180}}
	if err := checkPaintedScenes(c); err == nil || !strings.Contains(err.Error(), "intersects playable floor") {
		t.Fatalf("coplanar train may not enter public floor: %v", err)
	}
	c = paintedAmbientFixture()
	c.Art.Painted[0].AmbientPassages[0].ElevationM = *c.Art.Painted[0].FloorElevationM + .001
	if err := checkPaintedScenes(c); err == nil || !strings.Contains(err.Error(), "at or below the floor") {
		t.Fatalf("above-floor decorative rail accepted: %v", err)
	}
}

func TestPaintedAmbientRasterRetainsTrainDetailWithinColdCacheBudgetNative(t *testing.T) {
	size := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static void PaintedAmbientRasterSize(", "  static Rect PaintedPassageRect(")
	runPaintedTextureNative(t, "AmbientRasterBudget", `using System;
struct Rect{public float width,height;public Rect(float w,float h){width=w;height=h;}}
class Proof{
`+size+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 public static void Main(){int w,h;
  PaintedAmbientRasterSize(new Rect(2560,872),out w,out h);
  Need(w==2048&&h==698,"station strip was reduced to a blurry small target");
  Need(1350d*w/2560>=1080,"two-car train loses native detail before GUI composition");
  Need((long)w*h*8<12L*1024*1024,"actual station mask+RGBA target exceed cold-cache budget");
  PaintedAmbientRasterSize(new Rect(1672,520),out w,out h);Need(w==1672&&h==520,"native housing canvas unnecessarily resampled or upscaled");
  PaintedAmbientRasterSize(new Rect(4096,4096),out w,out h);Need(w==2048&&h==2048&&(long)w*h*8==32L*1024*1024,"maximum per-passage cache is unbounded");
  PaintedAmbientRasterSize(new Rect(4096,2048),out w,out h);Need(w==2048&&h==1024,"aspect ratio lost");
  PaintedAmbientRasterSize(new Rect(17,13),out w,out h);Need(w==17&&h==13,"small authored target upscaled");
  PaintedAmbientRasterSize(new Rect(.1f,.2f),out w,out h);Need(w==1&&h==1,"tiny legal strip has zero-sized target");
  Console.WriteLine("2048 bounded shared color/clip raster, station train1080px, native canvases unscaled PASS");
 }
}`)
	prepare := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  void PreparePaintedAmbient(", "  readonly PaintedAmbientClockCursor ")
	for _, required := range []string{"PaintedAmbientRasterSize(bounds,out width,out height)", "new Texture2D(width,height", "new RenderTexture(width,height", "filterMode=FilterMode.Point", "PaintedPolygonContains(passage.visible_strip,px,py)"} {
		if !strings.Contains(prepare, required) {
			t.Errorf("color target and exact clip must share the cold raster: %s", required)
		}
	}
}

func TestPaintedAmbientFloorOverlapIncludesCoincidentAndCrossingEdges(t *testing.T) {
	diamond := []PaintedPoint{{0, -1}, {2, 0}, {0, 1}, {-2, 0}}
	cases := []struct {
		name    string
		poly    []PaintedPoint
		overlap bool
	}{
		{"same diamond", append([]PaintedPoint(nil), diamond...), true},
		{"thin crossing without contained vertices", []PaintedPoint{{-.1, -2}, {.1, -2}, {.1, 2}, {-.1, 2}}, true},
		{"enclosed", []PaintedPoint{{-.1, -.1}, {.1, -.1}, {.1, .1}, {-.1, .1}}, true},
		{"containing", []PaintedPoint{{-3, -3}, {3, -3}, {3, 3}, {-3, 3}}, true},
		{"touch vertex only", []PaintedPoint{{2, 0}, {3, -1}, {3, 1}}, false},
		{"shared boundary only", []PaintedPoint{{0, -1}, {2, 0}, {3, -2}}, false},
		{"separate", []PaintedPoint{{5, 5}, {6, 5}, {6, 6}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ambientPolygonsOverlap(tc.poly, diamond); got != tc.overlap {
				t.Fatalf("positive-area overlap=%v, want %v", got, tc.overlap)
			}
		})
	}
}

func TestPaintedAmbientProjectionIsPresentationOnly(t *testing.T) {
	c := paintedAmbientFixture()
	before := string(c.Life)
	resource, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Art struct {
			Painted []PaintedScene `json:"painted"`
		} `json:"art"`
		Life json.RawMessage `json:"life"`
	}
	if err := json.Unmarshal(resource, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Art.Painted) != 1 || len(out.Art.Painted[0].AmbientPassages) != 1 || out.Art.Painted[0].AmbientPassages[0].PeriodSeconds != 300 || out.Art.Painted[0].AmbientPassages[0].Track.X != 0 || out.Art.Painted[0].AmbientPassages[0].Track.Width != 512 || out.Art.Painted[0].Daylight.NightBrightness != .6 {
		t.Fatal("public passage/schedule/profile did not round-trip through the actual resource")
	}
	if string(c.Life) != before || len(out.Life) != 0 {
		t.Fatal("presentation must not alter or publish the private Life catalog")
	}
	drawBytes, err := os.ReadFile("templates/painted.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	draw := string(drawBytes)[strings.Index(string(drawBytes), "  bool DrawPaintedGame("):]
	background, ambient, fog := strings.Index(draw, "GUI.DrawTexture(localFit,cache.background"), strings.Index(draw, "DrawPaintedAmbient(scene"), strings.Index(draw, "GUI.DrawTexture(localFit,cache.fog")
	if background < 0 || ambient <= background || fog <= ambient {
		t.Fatal("lower passage must draw after background and before opaque fog")
	}
	if strings.Index(draw, "DrawPaintedDaylight(scene") > strings.Index(draw, "GUI.EndGroup()") {
		t.Fatal("daylight overlay must remain inside world viewport, before HUD")
	}
	prepare := sceneTemplateSection(t, "painted.cs.tmpl", "  PaintedCache PreparePaintedScene(", "  static Rect[] BuildPaintedOpaqueRects(")
	if !strings.Contains(prepare, "&&!PaintedAmbientStripContains(scene,") || strings.Contains(draw, "PaintedAmbientStripContains") {
		t.Fatal("strip hole belongs only to cold foreground cache, before unchanged opaque fog/actors")
	}
	module, err := os.ReadFile("templates/painted_ambient.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	frame := string(module)[strings.Index(string(module), "  void DrawPaintedAmbient("):]
	for _, bad := range []string{"new Color32[", "new Texture2D", "new RenderTexture", "GetPixels", "PaintedPolygonContains", "clock.Elapsed", "Time.deltaTime"} {
		if strings.Contains(frame, bad) {
			t.Errorf("per-frame motion must not allocate/process full clip pixels or accumulate time: %s", bad)
		}
	}
	if !strings.Contains(frame, "=='2'") || !strings.Contains(frame, "observationChanged") || !strings.Contains(frame, "state.Life.Transit") {
		t.Fatal("live visibility / transit guards lost")
	}
	shader, err := os.ReadFile("templates/painted_passage.shader.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shader), "train.a*=tex2D(_ObservationTex") || !strings.Contains(string(shader), "alpha*clip.a") {
		t.Fatal("live train needs per-segment current observation and authored strip clipping")
	}
}

// Run the exact generated math on Unity value types. No second implementation
// of schedule, extrapolation, interpolation, or daylight lives in this test.
func TestPaintedAmbientNativeClockPathDaylightAndProjection(t *testing.T) {
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
	mathCode := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static double PaintedAmbientClock(", "  // End pure functions;")
	transform := sceneTemplateSection(t, "painted.cs.tmpl", "  static Vector2 PaintedCanvasToScreen(", "  void QueuePaintedActor(")
	point := sceneTemplateSection(t, "painted.cs.tmpl", "  static Vector2 PaintedPoint(", "  static bool PaintedPolygonContains(")
	target := sceneTemplateSection(t, "painted.cs.tmpl", "  static Rect PaintedTarget(", "  static Rect PaintedViewport(")
	polygon := sceneTemplateSection(t, "painted.cs.tmpl", "  static bool PaintedPolygonContains(", "  PocPaintedScene CurrentPaintedScene(")
	strip := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static bool PaintedAmbientStripContains(", "  sealed class PaintedAmbientCache {")
	harness := `using System;using UnityEngine;
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float px,float py){x=px;y=py;}}
class PocPaintedScene {public PocPaintedPoint origin,axis_x,axis_y;public PocAmbientPassage[] ambient_passages;}
class PocAmbientPassage {public double period_seconds=300,duration_seconds=15,offset_seconds,night_period_seconds,day_start_second,day_end_second;public float width=160,height=60,feet_x=80,feet_y=55;public PocPaintedPoint[] path,observation_cells,visible_strip;}
class PocPaintedDaylight {public string source="passive_lightguides";public double sunrise_second=21600,sunset_second=72000,twilight_seconds=3600;public float day_brightness=1,night_brightness=.6f;}
class Proof {
` + mathCode + transform + point + target + polygon + strip + `
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 static void Near(double a,double b,string why){Need(Math.Abs(a-b)<.0001,why+" got="+a+" want="+b);}
 public static void Main(){
  var p=new PocAmbientPassage{path=new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(30,0),new PocPaintedPoint(30,40)}};double phase;
  Need(PaintedPassagePhase(p,0,out phase),"passage begins at its epoch");Near(phase,0,"start phase");
  Need(PaintedPassagePhase(p,7.5,out phase),"mid passage");Near(phase,.5,"mid phase");
  Need(!PaintedPassagePhase(p,15,out phase)&&!PaintedPassagePhase(p,299.9,out phase),"duration excludes waiting interval");
  Need(PaintedPassagePhase(p,300,out phase),"next five-minute passage");Near(phase,0,"loop phase");
  p.offset_seconds=42;Need(!PaintedPassagePhase(p,41,out phase)&&PaintedPassagePhase(p,42,out phase),"authored phase offset");
  Need(PaintedPassagePhase(p,342,out phase),"offset repeats at 300 seconds");Near(phase,0,"offset repeat");
  Need(!PaintedPassagePhase(p,double.NaN,out phase)&&!PaintedPassagePhase(p,double.PositiveInfinity,out phase),"invalid clock cannot animate");p.offset_seconds=0;
  p.night_period_seconds=3600;p.day_start_second=21600;p.day_end_second=79200;
  Need(!PaintedPassagePhase(p,21599.999,out phase)&&PaintedPassagePhase(p,21600,out phase),"one departure at sunrise boundary");Near(phase,0,"day starts at window origin");
  Need(PaintedPassagePhase(p,21900,out phase)&&!PaintedPassagePhase(p,21915,out phase),"five minute day schedule");
  Need(!PaintedPassagePhase(p,79199.999,out phase)&&PaintedPassagePhase(p,79200,out phase),"one departure at night boundary");Near(phase,0,"night window starts once");
  Need(!PaintedPassagePhase(p,79500,out phase)&&PaintedPassagePhase(p,82800,out phase),"night period is one hour, not five minutes");
  Need(PaintedPassagePhase(p,86400,out phase)&&PaintedPassagePhase(p,0,out phase),"night window stays continuous across midnight");Near(phase,0,"midnight hourly phase");
  Need(!PaintedPassagePhase(p,86400+300,out phase)&&PaintedPassagePhase(p,86400+3600,out phase),"postmidnight night cadence remains hourly");
  p.offset_seconds=20;Need(!PaintedPassagePhase(p,21600,out phase)&&!PaintedPassagePhase(p,21619,out phase)&&PaintedPassagePhase(p,21620,out phase),"offset is delay from each window start");Need(!PaintedPassagePhase(p,79200,out phase)&&PaintedPassagePhase(p,79220,out phase),"night uses same offset from night window origin");
  p.offset_seconds=299;Need(!PaintedPassagePhase(p,79199,out phase),"departure crossing the next window is suppressed, not restarted");
  p.night_period_seconds=0;p.day_start_second=0;p.day_end_second=0;p.offset_seconds=0;
  var at=PaintedPassagePosition(p,.5);Near(at.x,30,"constant arc length x");Near(at.y,5,"constant arc length y");
  Near(PaintedPassagePosition(p,-1).x,0,"clamped beginning");Near(PaintedPassagePosition(p,2).y,40,"clamped endpoint");
  var zero=new PocAmbientPassage{path=new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(0,0),new PocPaintedPoint(30,40)}};Near(PaintedPassagePosition(zero,.5).x,15,"zero leg safely skipped");
  double paused=PaintedAmbientClock(307,10000000000L,10900000000d,1,10,true);Near(paused,307,"pause uses authoritative calendar even with old lead");
  double normal=PaintedAmbientClock(307,10000000000L,10050000000d,1,10,false);Near(normal,307.5,"normal seconds-per-tick interpolation");
  double faster=PaintedAmbientClock(307,10000000000L,10100000000d,30,10,false);Near(faster,337,"accelerated calendar uses same sampled simulation time");
  Near(PaintedAmbientClock(307,10000000000L,9000000000d,1,10,false),307,"load never extrapolates backwards");
  // Epoch/load resets sampledNS in UpdateMotion; identical saved authority must
  // produce exactly the same phase and position, without any saved client timer.
  double savedClock=PaintedAmbientClock(307,10000000000L,10000000000d,1,10,false);Need(PaintedPassagePhase(p,savedClock,out phase),"loaded active passage");var saved=PaintedPassagePosition(p,phase);
  double loadedClock=PaintedAmbientClock(307,990000000000L,990000000000d,1,10,false);Need(PaintedPassagePhase(p,loadedClock,out phase),"fresh epoch loaded passage");var loaded=PaintedPassagePosition(p,phase);Near(saved.x,loaded.x,"save load same train x");Near(saved.y,loaded.y,"save load same train y");
  var d=new PocPaintedDaylight();Near(PaintedDaylightBrightness(d,0),.6,"weak night backup");Near(PaintedDaylightBrightness(d,21600),.8,"smooth sunrise midpoint");Near(PaintedDaylightBrightness(d,43200),1,"day collector");Near(PaintedDaylightBrightness(d,72000),.8,"smooth sunset midpoint");Near(PaintedDaylightBrightness(d,84600),.6,"late night");Near(PaintedDaylightBrightness(d,43200+86400),1,"daily clock repeats after save/load");
  Near(PaintedDaylightBrightness(null,0),1,"legacy scenes retain original light");
  var absent=new PocPaintedDaylight{source=null,sunrise_second=0,sunset_second=0,twilight_seconds=0,day_brightness=0,night_brightness=0};
  Near(PaintedDaylightBrightness(absent,47723),1,"JsonUtility materialized absent optional daylight must not black out stair or copper scene");
  absent.source="";Near(PaintedDaylightBrightness(absent,0),1,"empty optional profile preserves original night artwork");
  absent.source="other";Near(PaintedDaylightBrightness(absent,43200),1,"unrecognized optional profile cannot darken the world");
  var bounds=PaintedAmbientBounds(new[]{new PocPaintedPoint(10,400),new PocPaintedPoint(500,400),new PocPaintedPoint(500,500),new PocPaintedPoint(10,500)});Near(bounds.x,10,"strip bounds x");Near(bounds.height,100,"bounded render texture height");
  var rect=PaintedPassageRect(p,new Vector2(200,450));Near(rect.x,120,"feet anchored sprite x");Near(rect.y,395,"feet anchored sprite y");
  var fit=new Rect(-35,20,2560,1441);float scale=1.25f;var screen=PaintedCanvasToScreen(new Vector2(120,395),fit,scale);var target=PaintedTarget(rect,fit,scale);Near(target.x,screen.x,"same camera affine as world");Near(target.y,screen.y,"same camera affine y");Near(target.width,200,"viewport zoom scales train consistently");
  var scene=new PocPaintedScene{origin=new PocPaintedPoint(200,50),axis_x=new PocPaintedPoint(50,25),axis_y=new PocPaintedPoint(-50,25)};p.observation_cells=new[]{new PocPaintedPoint(1,1),new PocPaintedPoint(2,1)};
  Need(PaintedNearestObservation(scene,p,new Vector2(150,200))==0,"strip sample maps to first platform segment");Need(PaintedNearestObservation(scene,p,new Vector2(270,200))==1,"far strip sample maps to another platform segment, without granting whole-strip vision");
  // Intra-tick snapshot PhaseNS and rolling measured Hz are not calendar
  // origins. The nominal boundary cursor never rewinds on fresh snapshots,
  // slowdown, or pause, and a presentation lead cannot flash an inactive train.
  var cursor=new PaintedAmbientClockCursor();double ca=cursor.Sample("e","market",14,100,10115000000d,1,10,false),cb=cursor.Sample("e","market",14,100,10115000000d,1,10,false);Near(ca,15.15,"tick-boundary sample");Near(cb,ca,"same tick status snapshot must not reset the clock to its PhaseNS");
  Need(PaintedPassageRenderPhase(p,14,ca,out phase)&&phase>.99,"authoritative active train stays visible at the end even when lead samples past its duration");Need(!PaintedPassageRenderPhase(p,299,300.5,out phase),"lead must not flash the next train before authoritative departure");
  double fast=cursor.Sample("rate","market",300,100,10050000000d,30,10,false),slow=cursor.Sample("rate","market",300,100,10050000000d,1,10,false),freeze=cursor.Sample("rate","market",300,100,10050000000d,1,10,true);Need(slow>=fast&&freeze==slow,"rate reduction and pause preserve the already rendered pose");Near(cursor.Sample("loaded","market",307,100,10000000000d,1,10,true),307,"load epoch resets to saved calendar");
  cursor=new PaintedAmbientClockCursor();double previous=-1;bool wasActive=false;int starts=0,ends=0;
  for(int frame=0;frame<120;frame++){double sim=100000000000d+frame*25000000d;long tick=1000+(long)Math.Floor(frame*.025/.1),game=296+tick-1000;double clock=cursor.Sample("continuous","market",game,tick,sim+15000000d,1,10,false);Need(clock>=previous,"snapshot jitter cannot reverse sampled phase");bool active=PaintedPassageRenderPhase(p,game,clock,out phase);if(active&&!wasActive)starts++;if(!active&&wasActive)ends++;wasActive=active;previous=clock;}Need(starts==1&&ends==1,"one real passage must have one activation and one deactivation, without blinking");
  p.visible_strip=new[]{new PocPaintedPoint(10,400),new PocPaintedPoint(500,400),new PocPaintedPoint(500,500),new PocPaintedPoint(10,500)};scene.ambient_passages=new[]{p};
  Need(PaintedAmbientStripContains(scene,150,450),"old foreground replay must leave a hole for actual lower rail");Need(!PaintedAmbientStripContains(scene,150,399)&&!PaintedAmbientStripContains(scene,200,100),"hole cannot extend into ordinary architecture or walking floor");scene.ambient_passages=null;Need(!PaintedAmbientStripContains(scene,150,450),"legacy foreground remains solid");
  Console.WriteLine("ambient calendar/pause/load/speed/path/daylight/viewport/observation PASS");
 }
}`
	dir := t.TempDir()
	source := filepath.Join(dir, "PaintedAmbientProof.cs")
	binary := filepath.Join(dir, "PaintedAmbientProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/r:"+facade, "/r:"+core, "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile ambient math: %v\n%s", err, out)
	}
	cmd := exec.Command(mono, binary)
	cmd.Env = append(os.Environ(), "MONO_PATH="+managed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ambient math: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
