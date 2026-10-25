package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmbientProofIsOptInAndReadsActualGPU(t *testing.T) {
	s := hudTemplate(t, "painted_ambient_proof.cs.tmpl")
	for _, want := range []string{
		`"--poc-verify-ambient"`, `Path.IsPathRooted`, `NewGame(777,AmbientProofScenario)`, `const string AmbientProofScenario="life"`, `ambientProofNamedNPCs.Count==0`, `npc.Id.StartsWith("population.")`,
		`LifeCommand("goto",ambientProofLevels[0])`, `state.Life.Activity=="Прибыл"`,
		`AmbientProofObservedEdgeStep(terrain,cells`, `ViewCell(view,actor.X,actor.Y)!='2'`, `AmbientProofMayReplan(ackMS>=0`, `ClickMove(x,y)`, `ambient walk entered unknown floor`,
		`LifeCommand("rate","",rate)`, `Options(true,1)`, `state.Paused`,
		`layer.IsCreated()`, `readback.ReadPixels(`, `readback.GetPixels32()`,
		`PaintedPolygonContains(c.passage.visible_strip,px,py)`, `empty/black train layer`,
		`AmbientProofPixelHash(pixels)!=hash`, `state.Tick!=tick`, `state.GameSeconds!=seconds`,
		`ReferenceEquals(ambientProofSession,session)`, `ambientProofCaptures.Count!=6`,
		`AmbientProofDifferentPoses()`, `layer_hash!=ambientProofCaptures[1].layer_hash`,
		`ScreenCapture.CaptureScreenshotAsTexture()`, `SplashScreen.isFinished`, `AmbientProofTrainOnly(c,out body)`, `train_alpha_pixels=bodyCount`, `AmbientProofContinuousBody()`, `continuous_pass_verified=ambientProofContinuityVerified`, `actual train body blinked out and back`,
		`"market-day-b.png"`, `"housing-night.png"`, `record.layer_file`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing real-session GPU acceptance guard: %s", want)
		}
	}
	for _, forbidden := range []string{`state.GameSeconds=`, `state.PlayerX=`, `state.PlayerY=`, `state.Life.LevelId=`, `new PocGameState`, `scene.navigation`, `state.Life.SecondsPerTick=`} {
		if strings.Contains(strings.ReplaceAll(s, "==", "EQ"), forbidden) {
			t.Errorf("GPU proof fabricates authority or accesses hidden floor: %s", forbidden)
		}
	}
	if strings.Count(s, "NewGame(") != 1 {
		t.Fatal("GPU proof must retain a single actual game session")
	}
	unity := hudTemplate(t, "unity.cs.tmpl")
	assertNativeLifeProofFlags(t, unity)
	for _, want := range []string{`InitAmbientProof();`, `if (verifyAmbient) AmbientProofDiagnostics(now);`, `if (verifyAmbient) { AmbientProofStep(now); return; }`, `AmbientProofReportJSON()`, `if (verifyAmbient) return ambientProofDirectory;`} {
		if !strings.Contains(unity, want) {
			t.Errorf("GPU proof has no native CLI/report dispatch: %s", want)
		}
	}
	normal := hudTemplate(t, "painted_ambient.cs.tmpl")
	for _, forbidden := range []string{"ReadPixels", "GetPixels32", "CaptureScreenshot", "AmbientProof"} {
		if strings.Contains(normal, forbidden) {
			t.Errorf("normal rendering inherited proof-only work: %s", forbidden)
		}
	}
}

func TestAmbientProofNativeWindowTargetsAndRateController(t *testing.T) {
	monos, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	if len(monos) == 0 {
		t.Skip("Mono unavailable")
	}
	mono := monos[len(monos)-1]
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	controller := sceneTemplateSection(t, "painted_ambient_proof.cs.tmpl", "  static long AmbientProofTarget(", "  static Material AmbientProofMaterial(")
	position := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static Vector2 PaintedPassagePosition(", "  static double PaintedDaylightSmooth(")
	phase := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static double PaintedAmbientClock(", "  static Vector2 PaintedPassagePosition(")
	records := sceneTemplateSection(t, "painted_ambient_proof.cs.tmpl", "  [Serializable] sealed class AmbientFrameSample", "  readonly List<AmbientFrameSample>")
	validator := sceneTemplateSection(t, "painted_ambient_proof.cs.tmpl", "  string AmbientProofValidateContinuity(){", "  void InitAmbientProof(){")
	harness := `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public static Vector2 zero {get{return new Vector2();}}}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
struct Color32 {public byte r,g,b,a;public Color32(byte red,byte green,byte blue,byte alpha){r=red;g=green;b=blue;a=alpha;}}
class PocAmbientPassage {public double stop_progress,approach_seconds,dwell_seconds;public PocPaintedPoint[] path=new[]{new PocPaintedPoint(-500,1257),new PocPaintedPoint(2172,363)};public double period_seconds=300,night_period_seconds=3600,day_start_second=21600,day_end_second=79200,offset_seconds,duration_seconds=15;}
class Proof {
` + controller + phase + position + records + validator + `
 readonly List<AmbientFrameSample> ambientFrameSamples=new List<AmbientFrameSample>();readonly List<AmbientBodySample> ambientBodySamples=new List<AmbientBodySample>();long ambientProofTraceDeparture=1000,ambientProofTargetSeconds=1017;

 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 static void Target(PocAmbientPassage p,long now,bool night,int pose,long expected){long got=AmbientProofTarget(p,now,night,pose);Need(got==expected,"wrong target at "+now+" night="+night+" got="+got+" want="+expected);double progress;Need(PaintedPassagePhase(p,got,out progress)&&Math.Abs(progress-pose/p.duration_seconds)<.00001,"controller target is not a real configured passing pose");}
 public static void Main(){
  var p=new PocAmbientPassage();
  Need(AmbientProofPoseSecond(p,429,false,false)==5,"left observed station segment needs an early actual pose");Need(AmbientProofPoseSecond(p,1432,false,false)==11,"right housing segment needs a later pose");p.path=new[]{new PocPaintedPoint(-700,2330),new PocPaintedPoint(3260,845)};Need(AmbientProofPoseSecond(p,2460,true,false)==10&&AmbientProofPoseSecond(p,2460,true,true)==12,"market must show two bodies in its actually observed right segment");
  Target(p,25200,false,8,25208);Target(p,25207,false,11,25211);Target(p,25210,false,11,25511);
  Target(p,79196,true,8,79208);Target(p,86401,true,8,86408);Target(p,20000,true,8,79208);
  Target(p,79196,false,8,108008);Target(p,21600,true,8,79208);
  p.offset_seconds=20;Target(p,21600,false,8,21628);Target(p,79200,true,8,79228);p.offset_seconds=0;
  for(long now=0;now<172800;now+=137){for(int mode=0;mode<2;mode++){bool night=mode==1;long target=AmbientProofTarget(p,now,night,8);double progress;Need(target>=now+2&&target<now+86400,"target must leave command margin and exist within a day");Need(PaintedPassagePhase(p,target,out progress),"controller aimed into an inactive slot");double day=target%86400;Need(night?(day<21600||day>=79200):(day>=21600&&day<79200),"wrong requested day/night window");}}
  for(long gap=1;gap<4200;gap++){long left=gap;int count=0;while(left>0){int rate=AmbientProofRate(left);Need(rate==1||rate==30,"only existing normal rate commands are allowed");Need(rate<=left,"coarse rate skips requested pose");left-=rate;if(++count>1000)throw new Exception("rate controller failed to converge");}}
  bool rejected=false;try{AmbientProofTarget(p,0,false,15);}catch(ArgumentException){rejected=true;}Need(rejected,"pose outside passage rejected");
  var berth=new PocAmbientPassage{stop_progress=.5,approach_seconds=8,dwell_seconds=20,duration_seconds=36};Need(AmbientProofPoseSecond(berth,429,false,false)==-10,"station capture should use a dwell pose before departure");
  foreach(long current in new long[]{0,21570,21595,25171,25200,79196,86399,86400})foreach(bool night in new[]{false,true}){long aimed=AmbientProofTarget(berth,current,night,-10);double actual;Need(aimed>=current+2&&PaintedPassagePhase(berth,aimed,out actual)&&actual==.5,"ambient controller missed the station berth");double daytime=aimed%86400;Need(night?daytime<21600||daytime>=79200:daytime>=21600&&daytime<79200,"berth target escaped requested daylight window");}
  var terrain=new[]{"#######","#.....#","#.....#","#######"};var cells=new[]{"2222222","2222222","2222222","2222222"};var goals=new[]{new PocPaintedPoint(5,1),new PocPaintedPoint(2,2)};int x,y;var blocked=new HashSet<int>();
  Need(AmbientProofObservedEdgeStep(terrain,cells,1,1,goals,blocked,out x,out y)&&x==2&&y==1,"choose nearest reachable observation cell, not fixed array index");
  blocked.Add(9);Need(AmbientProofObservedEdgeStep(terrain,cells,1,1,goals,blocked,out x,out y)&&x==1&&y==2,"visible body on first step requires observed detour");
  blocked.Remove(9);blocked.Add(16);Need(AmbientProofObservedEdgeStep(terrain,cells,1,1,goals,blocked,out x,out y)&&x==2&&y==1,"occupied closest goal is skipped for a reachable free observation cell");
  Need(!AmbientProofObservedEdgeStep(new[]{"..."},new[]{"202"},0,0,new[]{new PocPaintedPoint(2,0)},null,out x,out y),"visible distant edge cannot route through unknown middle cell");
  blocked.Clear();blocked.Add(12);blocked.Add(16);Need(!AmbientProofObservedEdgeStep(terrain,cells,1,1,goals,blocked,out x,out y),"all occupied observation cells do not grant passage through bodies");
  Need(AmbientProofMayReplan(true,0,"Ждёт проход",1),"confirmed physical wait can reselect a proof approach");Need(!AmbientProofMayReplan(false,0,"Ждёт проход",10)&&!AmbientProofMayReplan(true,1,"Ждёт проход",10)&&!AmbientProofMayReplan(true,0,"Идёт",10)&&!AmbientProofMayReplan(true,0,"Ждёт проход",.5),"replan must preserve unacknowledged or accepted movement and normal cadence");
  var continuous=new Proof();for(int frameID=0;frameID<18;frameID++){bool active=frameID<15;continuous.ambientFrameSamples.Add(new AmbientFrameSample{epoch="e",observation_flags="visible",game_seconds=1000+frameID,calendar_sample=1000+frameID+.2,phase=active?Math.Min(.999,(frameID+.2)/15):0,author_active=active,observed=true,draw_active=active});if(frameID<17)continuous.ambientBodySamples.Add(new AmbientBodySample{observation_flags="visible",colored_alpha=frameID>=3&&frameID<=12?100:0});}
  Need(continuous.AmbientProofValidateContinuity()==null,"valid full native train span must pass");
  double oldClock=continuous.ambientFrameSamples[6].calendar_sample;continuous.ambientFrameSamples[6].calendar_sample=1004;Need(continuous.AmbientProofValidateContinuity().Contains("backwards"),"native trace must reject a sampled calendar rewind");continuous.ambientFrameSamples[6].calendar_sample=oldClock;
  double oldPhase=continuous.ambientFrameSamples[6].phase;continuous.ambientFrameSamples[6].phase=.1;Need(continuous.AmbientProofValidateContinuity().Contains("pose moved backwards"),"native trace must reject reversed train pose");continuous.ambientFrameSamples[6].phase=oldPhase;
  continuous.ambientFrameSamples[6].draw_active=false;Need(continuous.AmbientProofValidateContinuity().Contains("blinked off"),"native trace must reject off/on inside an active authority interval");continuous.ambientFrameSamples[6].draw_active=true;
  continuous.ambientBodySamples[6].colored_alpha=0;Need(continuous.AmbientProofValidateContinuity().Contains("blinked out and back"),"actual GPU body hole must fail despite a correct clock");continuous.ambientBodySamples[6].colored_alpha=100;
  continuous.ambientBodySamples[6].observation_flags="different";Need(continuous.AmbientProofValidateContinuity().Contains("visibility changed"),"FOV changes cannot be disguised as stable render continuity");
  var pixels=new[]{new Color32(1,2,3,255),new Color32(0,0,0,0)};ulong saved=AmbientProofPixelHash(pixels);Need(saved==AmbientProofPixelHash(pixels),"pause hash must be stable");pixels[0]=new Color32(1,2,4,255);Need(saved!=AmbientProofPixelHash(pixels),"different GPU colors must alter pose fingerprint");pixels[0]=new Color32(1,2,3,0);Need(saved!=AmbientProofPixelHash(pixels),"alpha changes must alter pose fingerprint");
  Console.WriteLine("real-window target / midnight / offset / bounded rate / stable GPU fingerprint PASS");
 }
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "AmbientController.cs"), filepath.Join(dir, "AmbientController.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("ambient controller compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("ambient controller execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
