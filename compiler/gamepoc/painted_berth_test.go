package gamepoc

import (
	"math"
	"strings"
	"testing"
)

func paintedBerthFixture() Compiled {
	c := paintedAmbientFixture()
	p := &c.Art.Painted[0].AmbientPassages[0]
	p.StopProgress = .5
	p.ApproachSeconds = 8
	p.DwellSeconds = 20
	p.DurationSeconds = 36
	p.ShearY = .02
	p.BerthDoors = []PaintedBerthDoor{{Train: PaintedPoint{40, 40}, Platform: PaintedPoint{210, 419.2}}, {Train: PaintedPoint{120, 40}, Platform: PaintedPoint{290, 420.8}}}
	return c
}

func TestPaintedBerthRejectsOverlappingServicesAndMisalignedDoors(t *testing.T) {
	if err := checkPaintedScenes(paintedBerthFixture()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*PaintedAmbientPassage){
		"one second between day/night departures": func(p *PaintedAmbientPassage) { p.DayEndSecond++ },
		"one second between night/day departures": func(p *PaintedAmbientPassage) { p.DayStartSecond++ },
		"offset beyond short day":                 func(p *PaintedAmbientPassage) { p.DayEndSecond = p.DayStartSecond + 20; p.OffsetSeconds = 25 },
		"zero approach":                           func(p *PaintedAmbientPassage) { p.ApproachSeconds = 0 },
		"no departure interval":                   func(p *PaintedAmbientPassage) { p.DurationSeconds = 28 },
		"overflow approach":                       func(p *PaintedAmbientPassage) { p.ApproachSeconds = math.MaxInt64 },
		"overflow dwell":                          func(p *PaintedAmbientPassage) { p.DwellSeconds = math.MaxInt64 },
		"bad source datum":                        func(p *PaintedAmbientPassage) { p.BerthDoors[1].Train.X = p.Width + 1 },
		"bad platform datum":                      func(p *PaintedAmbientPassage) { p.BerthDoors[0].Platform.Y = -1 },
		"misaligned second sill":                  func(p *PaintedAmbientPassage) { p.BerthDoors[1].Platform.Y += .51 },
		"lost camera shear":                       func(p *PaintedAmbientPassage) { p.ShearY = 0 },
		"duplicate door datum":                    func(p *PaintedAmbientPassage) { p.BerthDoors[1] = p.BerthDoors[0] },
		"nonfinite datum":                         func(p *PaintedAmbientPassage) { p.BerthDoors[0].Platform.X = math.NaN() },
		"doors without berth":                     func(p *PaintedAmbientPassage) { p.StopProgress = 0; p.ApproachSeconds = 0; p.DwellSeconds = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			c := paintedBerthFixture()
			change(&c.Art.Painted[0].AmbientPassages[0])
			if err := checkPaintedScenes(c); err == nil {
				t.Fatal("invalid berth accepted")
			}
		})
	}
}

func TestPaintedBerthNativeCalendarAndAuthorityLead(t *testing.T) {
	mathCode := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static double PaintedAmbientClock(", "  sealed class PaintedAmbientClockCursor")
	runPaintedTextureNative(t, "PaintedBerthCalendar", `using System;
class PocAmbientPassage {public double period_seconds=300,night_period_seconds=3600,day_start_second=21600,day_end_second=79200,offset_seconds,duration_seconds=36,stop_progress=.5,approach_seconds=8,dwell_seconds=20;}
class Proof {`+mathCode+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}static void Near(double a,double b,string why){Need(Math.Abs(a-b)<.00001,why+" got="+a+" expected="+b);}
 static void Phase(PocAmbientPassage p,double t,double expected){double got;Need(PaintedPassagePhase(p,t,out got),"expected actual berth interval at "+t);Near(got,expected,"berth phase at "+t);}
 static void Cycle(PocAmbientPassage p,long departure){
  double phase;Need(!PaintedPassagePhase(p,departure-28.01,out phase),"train appears before approach");Phase(p,departure-28,0);Phase(p,departure-24,.375);Phase(p,departure-20,.5);Phase(p,departure-19,.5);Phase(p,departure-1,.5);Phase(p,departure,.5);Phase(p,departure+4,.625);Need(!PaintedPassagePhase(p,departure+8,out phase),"departure persisted after full interval");
  double previous=-1;for(int i=0;i<360;i++){double now=departure-28+i*.1;Need(PaintedPassagePhase(p,now,out phase)&&phase>=previous-1e-8,"train rewinds across approach/dwell/departure");previous=phase;}
  double actual;Need(PaintedBerthDeparture(p,departure-25,out actual),"no departure identity during approach");Near(actual,departure,"approach belongs to exact scheduled departure");
 }
 public static void Main(){
  var p=new PocAmbientPassage();Cycle(p,25200);Cycle(p,21600);Cycle(p,79200);Cycle(p,86400);Cycle(p,90000);Cycle(p,108000);
  p.offset_seconds=20;Cycle(p,21620);Cycle(p,79220);Cycle(p,86420);
  p.day_start_second=25000;p.day_end_second=79000;p.night_period_seconds=5000;p.offset_seconds=100;Cycle(p,84100);Cycle(p,89100);double phase;Need(!PaintedPassagePhase(p,86400,out phase),"night cadence restarted at civil midnight");
  p=new PocAmbientPassage();Need(!PaintedPassageRenderPhase(p,25171,25174,out phase),"render lead invented an early approaching train");Need(PaintedPassageRenderPhase(p,25200,25500,out phase)&&phase>.999,"render lead jumped into the next service rather than finishing current train");Need(PaintedPassageRenderPhase(p,25190,25180,out phase),"stale sample removed authoritative berth");Near(phase,.5,"stale sample rewound within dwell");
  double paused=PaintedAmbientClock(25190,1000000000,2000000000,30,10,true);Near(paused,25190,"pause ignored authoritative clock");Phase(p,paused,.5);
  p.night_period_seconds=0;p.day_start_second=p.day_end_second=0;p.offset_seconds=17;Cycle(p,317);Cycle(p,617);
  Need(!PaintedPassagePhase(p,double.NaN,out phase)&&!PaintedPassagePhase(p,double.PositiveInfinity,out phase),"invalid time enters berth schedule");
  Console.WriteLine("approach/dwell/departure, exact service identity, dusk/dawn/midnight/offset, pause and same-service lead clamp PASS");
 }} `)
	shader := hudTemplate(t, "painted_passage.shader.tmpl")
	if !strings.Contains(shader, "sourceCanvas.y-=_TrainShear.x*(canvas.x-_TrainRect.x-_TrainShear.y)") {
		t.Fatal("GPU inverse shear must use the shared feet datum")
	}
	copy := sceneTemplateSection(t, "painted_ambient_proof.cs.tmpl", "  static Material AmbientProofMaterial(", "  const string AmbientProofScenario")
	if !strings.Contains(copy, `"_TrainShear"`) {
		t.Fatal("train-only GPU proof lost runtime shear")
	}
}

func TestStationBerthNativeTargetAndMeasuredSills(t *testing.T) {
	phase := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static double PaintedAmbientClock(", "  sealed class PaintedAmbientClockCursor")
	helpers := sceneTemplateSection(t, "station_berth_proof.cs.tmpl", "  static long StationBerthNextDeparture(", "  bool verifyStationBerth")
	runPaintedTextureNative(t, "StationBerthTargets", `using System;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}}
struct Vector4 {public float x,y,z,w;public Vector4(float a,float b,float c,float d){x=a;y=b;z=c;w=d;}}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
class PocAmbientPassage {public double period_seconds=300,night_period_seconds=3600,day_start_second=21600,day_end_second=79200,offset_seconds,duration_seconds=36,stop_progress=.5,approach_seconds=8,dwell_seconds=20;}
class Proof {`+phase+helpers+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}static void Near(double a,double b,string why){Need(Math.Abs(a-b)<.0001,why);}
 public static void Main(){
  var p=new PocAmbientPassage();foreach(long now in new long[]{0,21550,21590,25200,79190,86399,86400,107990,172799}){long next=StationBerthNextDeparture(p,now);Need(next>=now+40&&next<=now+86440,"upcoming service has no capture margin");double identity;Need(PaintedBerthDeparture(p,next,out identity)&&identity==next,"target is not a scheduled departure");double phase;Need(PaintedPassagePhase(p,next-16,out phase)&&phase==.5,"target dwell missing");Need(PaintedPassagePhase(p,next+3,out phase)&&phase>.5&&phase<1,"target departure missing");}
  var rect=new Vector4(170,380,160,60);var shear=new Vector4(.02f,80,0,0);var first=StationBerthDoorAt(rect,shear,new PocPaintedPoint(40,40));var second=StationBerthDoorAt(rect,shear,new PocPaintedPoint(120,40));Near(first.x,210,"first sill x");Near(first.y,419.2,"first sill y");Near(second.x,290,"second sill x");Near(second.y,420.8,"second sill y");
  foreach(var local in new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(40,40),new PocPaintedPoint(120,40),new PocPaintedPoint(160,60)}){var canvas=StationBerthDoorAt(rect,shear,local);double inverseY=canvas.y-shear.x*(canvas.x-rect.x-shear.y);Near(canvas.x-rect.x,local.x,"shader source x");Near(inverseY-rect.y,local.y,"GPU inverse shear round trip");}
  Console.WriteLine("actual timetable targets with command margin; both source sill transforms and inverse GPU shear PASS");
 }} `)
	s := hudTemplate(t, "station_berth_proof.cs.tmpl")
	for _, want := range []string{`"--poc-verify-station-berth"`, `LifeCommand("enter",exit.EdgeId)`, `new CopperRouteLegAudit`, `AmbientProofWalkToEdge()`, `AmbientProofReadback(cache,out layer)`, `AmbientProofTrainOnly(cache,out body)`, `cache.material.GetVector("_TrainRect")`, `cache.material.GetVector("_TrainShear")`, `foreach(var door in passage.berth_doors)`, `ScreenCapture.CaptureScreenshotAsTexture()`, `a.layer_hash!=b.layer_hash`, `d.phase<=c.phase`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing native berth evidence %q", want)
		}
	}
	for _, bad := range []string{"state.GameSeconds=", "state.PlayerX=", "File.Copy(", "SceneAgents.Add"} {
		if strings.Contains(strings.ReplaceAll(s, "==", "EQ"), bad) {
			t.Errorf("berth proof fabricates result: %q", bad)
		}
	}
}
