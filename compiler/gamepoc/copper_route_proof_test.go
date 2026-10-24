package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopperRouteProofUsesDiscoveredDoorsAndOneRealSession(t *testing.T) {
	s := hudTemplate(t, "copper_route_proof.cs.tmpl")
	for _, want := range []string{
		`"--poc-verify-copper-route"`, `Path.IsPathRooted`, `NewGame(777,"life100")`,
		`LifeCommand("goto",copperRouteLevels[0])`, `LifeCommand("enter",exit.EdgeId)`,
		`foreach(var marker in state.Life.Markers)`, `LifeDoorVisible(marker)`, `ClickMove(x,y)`,
		`"work_service_stair","service_stair_copper","copper_service_stair","service_stair_work"`,
		`state.Life.SecondsPerTick!=1`, `LifeCommand("rate","",1)`,
		`copperRouteSaveRestored=true`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing physical route contract %q", want)
		}
	}
	for _, want := range []string{
		`SaveOrLoad("slot3",true)`, `SaveOrLoad("slot3",false)`, `state.Epoch!=copperRouteSavedEpoch`,
		`state.Tick==savedTick`, `state.GameSeconds==copperRouteSavedSeconds`, `state.PlayerX==copperRouteSavedX`,
		`CopperRouteInventory()!=copperRouteSavedInventory`, `ReferenceEquals(copperRouteSession,session)`,
		`CopperRouteIdleActivity(state.Life.Activity)`, `state.Life.Activity=="Прибыл"`, `stable.Observe(ready,`, `stable_frames=stable.stable_frames`,
		`ScreenCapture.CaptureScreenshotAsTexture()`, `SplashScreen.isFinished`, `copperRouteCaptures.Count!=10`,
		`scene.proof_waypoints[0]`, `CopperRouteObservedStep(terrain,cells`, `"ward-explored.png"`, `ward_explored=copperRouteWardExplored`,
		`courtyard move used an unknown or non-floor path cell`, `copperRouteWardObservedSteps==0`,
		`paintedCaches.TryGetValue(level,out cache)`, `draw.id=="char.hero"`, `ViewSees(state.PlayerX,state.PlayerY)`,
		`CopperRouteWorldPixelsVisible(capture.GetPixels32(),capture.width,capture.height)`, `world_pixels_visible=worldVisible`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing route capture/save evidence %q", want)
		}
	}
	if strings.Count(s, "NewGame(") != 1 || strings.Count(s, `LifeCommand("goto",`) != 1 {
		t.Fatal("route may bootstrap once, but its six legs must use physical discovered exits")
	}
	for _, forbidden := range []string{`state.PlayerX=`, `state.PlayerY=`, `state.Life.LevelId=`, `new PocGameState`, `SceneAgents.Add`, `scene.navigation`, `File.Copy(`} {
		// Equality is an observation, not a state assignment.
		if strings.Contains(strings.ReplaceAll(s, "==", "EQ"), forbidden) {
			t.Errorf("route proof fabricates state or knows hidden navigation: %s", forbidden)
		}
	}
	unity := hudTemplate(t, "unity.cs.tmpl")
	for _, want := range []string{`InitCopperRouteProof();`, `verifyLife = verifyCopperRoute ||`, `if (verifyCopperRoute) CopperRouteProofDiagnostics(now);`, `if (verifyCopperRoute) { CopperRouteProofStep(now); return; }`, `CopperRouteProofReportJSON()`, `if (verifyCopperRoute) return copperRouteProofDirectory;`} {
		if !strings.Contains(unity, want) {
			t.Errorf("route proof is not connected to the native runtime: %s", want)
		}
	}
}

func TestCopperRouteRejectsBlackWorldEvenWithVisibleHUDNative(t *testing.T) {
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
	method := sceneTemplateSection(t, "copper_route_proof.cs.tmpl", "  static bool CopperRouteWorldPixelsVisible(", "  void CopperRouteBeginCapture(")
	harness := `using System;
struct Color32 {public byte r,g,b,a;public Color32(byte red,byte green,byte blue,byte alpha){r=red;g=green;b=blue;a=alpha;}}
class Proof {
` + method + `
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 public static void Main(){
  const int w=320,h=200;var p=new Color32[w*h];for(int i=0;i<p.Length;i++)p[i]=new Color32(0,0,0,255);
  Need(!CopperRouteWorldPixelsVisible(p,w,h),"black world falsely passed because scene/hero exist");
  for(int y=0;y<h;y++)for(int x=0;x<w;x++)if(y<h*18/100||y>=h*80/100||x<w*15/100||x>=w*85/100)p[y*w+x]=new Color32(240,220,180,255);
  Need(!CopperRouteWorldPixelsVisible(p,w,h),"bright HUD around black game plane counted as rendered world");
  for(int y=h*18/100;y<h*80/100;y++)for(int x=w*15/100;x<w*85/100;x++)p[y*w+x]=new Color32(8,13,16,255);
  Need(!CopperRouteWorldPixelsVisible(p,w,h),"uniform unknown-space fog accepted as architecture");
  p[100*w+160]=new Color32(255,255,255,255);Need(!CopperRouteWorldPixelsVisible(p,w,h),"single bright noise pixel accepted");
  for(int y=80;y<112;y++)for(int x=128;x<192;x++)p[y*w+x]=new Color32(70,65,52,255);
  Need(CopperRouteWorldPixelsVisible(p,w,h),"partially visible dim room or actor amid real fog rejected");
  for(int i=0;i<p.Length;i++)p[i]=new Color32(255,255,255,0);
  Need(!CopperRouteWorldPixelsVisible(p,w,h),"transparent pixel colors accepted as rendered world");
  Need(!CopperRouteWorldPixelsVisible(null,w,h)&&!CopperRouteWorldPixelsVisible(p,w+1,h)&&!CopperRouteWorldPixelsVisible(p,0,h),"bad screenshot geometry accepted");
  Console.WriteLine("actual screenshot plane excludes HUD-only/black/unknown-fog/noise/transparent output and accepts dim observed content: PASS");
 }
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "proof.cs"), filepath.Join(dir, "proof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("pixel guard compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("pixel guard execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}

func TestCopperRouteTimingAndObservedExplorationNative(t *testing.T) {
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
	methods := sceneTemplateSection(t, "copper_route_proof.cs.tmpl", "  [Serializable] sealed class CopperRouteLegRecord", "  readonly string[] copperRouteLevels")
	harness := `using System;
using System.Collections.Generic;
class Proof {
` + methods + `
 static void Good(string error){if(error!=null)throw new Exception(error);}
 static void Bad(string error,string expected){if(error==null||!error.Contains(expected))throw new Exception("wrong rejection: "+error+" expected "+expected);}
 static CopperRouteLegAudit Leg(bool multiple=true){return new CopperRouteLegAudit("edge","a","b",100,1000,multiple);}
 public static void Main(){
  var stair=Leg();Good(stair.Observe("a",false,100,1000,0));Good(stair.Observe("a",true,101,1001,30));Good(stair.Observe("a",true,101,1001,30));Good(stair.Observe("a",true,111,1011,20));Good(stair.Observe("a",true,121,1021,10));Good(stair.Observe("b",false,131,1031,0));
  if(!stair.record.completed||stair.record.distinct_transit_samples!=3||!stair.record.remaining_decreased||stair.record.arrival_game_seconds-stair.record.first_transit_seconds!=30)throw new Exception("real timed stair flight not retained");
  var teleport=Leg();Bad(teleport.Observe("b",false,101,1001,0),"without observed physical transit");
  var instant=Leg();Good(instant.Observe("a",true,101,1001,30));Bad(instant.Observe("b",false,102,1002,0),"before the advertised travel time");
  var missing=Leg();Good(missing.Observe("a",true,101,1001,30));Bad(missing.Observe("b",false,131,1031,0),"distinct countdown");
  var rail=Leg(false);Good(rail.Observe("a",true,101,1001,300));Good(rail.Observe("b",false,111,1301,0));if(!rail.record.completed)throw new Exception("ordinary accelerated rail rejected");
  var wrong=Leg();Bad(wrong.Observe("c",false,101,1001,0),"unexpected level");
  var rewind=Leg();Bad(rewind.Observe("a",false,99,1000,0),"clock moved backwards");
  var reset=Leg();Good(reset.Observe("a",true,101,1001,30));Bad(reset.Observe("a",true,102,1002,31),"remaining travel time increased");
  var vanished=Leg();Good(vanished.Observe("a",true,101,1001,30));Bad(vanished.Observe("a",false,102,1002,0),"disappeared");
  var empty=Leg();Bad(empty.Observe("a",true,101,1001,0),"without positive travel time");
  var jumped=new CopperRouteLegAudit("edge","a","b",100,1000,true,0,0,10,0);Bad(jumped.Observe("a",true,101,1001,30,10,0),"approach jumped");
  var wrongDoor=new CopperRouteLegAudit("edge","a","b",100,1000,true,0,0,1,0);Bad(wrongDoor.Observe("a",true,101,1001,30,0,0),"away from its discovered exit");
  var walked=new CopperRouteLegAudit("edge","a","b",100,1000,true,0,0,3,0);Good(walked.Observe("a",false,101,1001,0,1,0));Good(walked.Observe("a",false,102,1002,0,2,0));Good(walked.Observe("a",true,103,1003,30,3,0));Good(walked.Observe("a",true,113,1013,20,3,0));Good(walked.Observe("b",false,133,1033,0,99,99));if(walked.record.approach_position_samples!=3||!walked.record.completed)throw new Exception("physical approach or per-level arrival lost");
  int x,y;var attempted=new HashSet<int>();
  // The visible far cell must never be reached through an unknown middle cell.
  if(CopperRouteExploreCell(new[]{".?."},new[]{"202"},0,0,attempted,false,out x,out y))throw new Exception("exploration routed through hidden floor");
  if(!CopperRouteExploreCell(new[]{"#####","#..?#","#####"},new[]{"22222","22202","22222"},1,1,attempted,true,out x,out y)||x!=2||y!=1)throw new Exception("known connected frontier not chosen");
  attempted.Add(7);if(CopperRouteExploreCell(new[]{"#####","#..?#","#####"},new[]{"22222","22202","22222"},1,1,attempted,true,out x,out y))throw new Exception("failed frontier selected repeatedly");
  attempted.Clear();if(!CopperRouteExploreCell(new[]{"#####","#...#","#####"},new[]{"22222","22222","22222"},1,1,attempted,false,out x,out y)||x==1&&y==1)throw new Exception("ward save/load cannot select a distinct known floor cell");
  if(CopperRouteExploreCell(new[]{"...",".."},new[]{"222","22"},0,0,attempted,false,out x,out y))throw new Exception("ragged observed view accepted");
  if(CopperRouteObservedStep(new[]{".?."},new[]{"202"},0,0,2,0,out x,out y))throw new Exception("visible waypoint connected through unknown shortcut");
  if(CopperRouteObservedStep(new[]{"..."},new[]{"220"},0,0,2,0,out x,out y))throw new Exception("authored unknown waypoint granted movement authority");
  if(!CopperRouteObservedStep(new[]{".....",".???.","#####"},new[]{"22222","20002","22222"},0,1,4,1,out x,out y)||x!=0||y!=0)throw new Exception("observed detour did not yield its adjacent first step");
  if(!CopperRouteObservedStep(new[]{"..."},new[]{"222"},1,0,1,0,out x,out y)||x!=1||y!=0)throw new Exception("observed waypoint arrival not recognized");
  if(CopperRouteObservedStep(new[]{"..."},new[]{"222"},0,0,5,0,out x,out y))throw new Exception("out-of-bounds courtyard waypoint accepted");
  attempted.Clear();if(!CopperRouteExploreCell(new[]{"#######","#.....#","#?????#","#######"},new[]{"2222222","2222222","2000002","2222222"},3,1,attempted,true,out x,out y,5,2)||x!=5||y!=1)throw new Exception("observed frontier ignored the authored goal direction");
  if(CopperRouteIdleActivity("Идёт: Рынок Коридора 7")||CopperRouteIdleActivity("Ждёт проход")||!CopperRouteIdleActivity("Прибыл")||!CopperRouteIdleActivity("На месте"))throw new Exception("pending goto acknowledgement mistaken for arrival");
  var capture=new CopperRouteCaptureLatch();if(capture.Observe(false,"e","market",4,1000,1,1)||capture.Observe(true,"e","market",4,1000,1,1)||capture.Observe(true,"e","market",4,1000,1,1))throw new Exception("capture did not wait three paused render frames");
  if(!capture.Observe(true,"e","market",4,1000,1,1)||capture.stable_frames!=3)throw new Exception("stable paused scene never captured");
  if(capture.Observe(true,"e","market",5,1000,1,1)||capture.Observe(true,"e","market",5,1001,1,1)||capture.Observe(true,"new","market",5,1001,1,1)||capture.Observe(true,"new","work",5,1001,1,1)||capture.Observe(true,"new","work",5,1001,2,1))throw new Exception("capture ignored changing authoritative scene tuple");
  if(capture.Observe(false,"new","work",5,1001,2,1)||capture.Observe(true,"new","work",5,1001,2,1)||capture.Observe(true,"new","work",5,1001,2,1)||!capture.Observe(true,"new","work",5,1001,2,1))throw new Exception("invalid intermediate frame did not reset capture stability");
  Console.WriteLine("timed forward/return route audit, no teleport, countdown samples, clock/level guards, observed-only discovery and save disturbance: PASS");
 }
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "proof.cs"), filepath.Join(dir, "proof.exe")
	if err := os.WriteFile(source, []byte(harness), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("copper-route compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("copper-route execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
