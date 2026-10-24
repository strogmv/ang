package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKProofUsesSingleAuthoritativeSessionAndRealScreens(t *testing.T) {
	data, err := sources.ReadFile("templates/sdk_proof.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`NewGame(777,"life100")`, `LifeCommand("goto",sdkLevels[0])`, `LifeCommand("goto",sdkLevels[sdkLevelIndex])`,
		`"level.c7_market","level.poc_housing","level.poc_monorail","level.poc_work"`,
		`"inventory","development","map","journal"`, `state.Inventory!=null`, `DevelopmentView()!=null`, `HudHasMap()`, `HudHasJournal()`,
		`ScreenCapture.CaptureScreenshotAsTexture()`, `UnityEngine.Rendering.SplashScreen.isFinished`,
		`state.Epoch!=captureEpoch`, `ReferenceEquals(sdkSessionIdentity,session)`, `SdkProofDiagnostics(now)`, `now-sdkLogAt<10`, `capture_elapsed_seconds=clock.Elapsed.TotalSeconds-sdkCaptureStarted`, `!state.Paused`, `sdkViews.Count!=10||!defaults`,
		`SetHudMapMode("local")`, `SetHudMapMode("floor")`, `SetLocale("en")`, `SetLocale(sdkOriginalLocale)`, `panel-map-en.png`, `panel-map-local.png`, `state.Life.RegionMap.Name==sdkRussianRegionTitle`,
		`ResolveCharacterSheet(actor.Id,out key)`, `characterTextures.TryGetValue(key,out texture)`,
		`if(!ViewSees(actor.X,actor.Y))continue`, `default_binding=!characterBindings.ContainsKey(actor.Id)`,
		`content_hash=resource.content_hash`, `now>420`, `state.Tick<10`, `real_state_visual_capture_not_gui_button_input`,
		`state.Life.Activity!="Прибыл"`, `scene.proof_waypoints[0]`, `ViewCell(ObserverView(),sdkHousingX,sdkHousingY)=='0'`,
		`ClickMove(sdkHousingX,sdkHousingY)`, `SdkPoseReached(state.Life.LevelId,"level.poc_housing"`, `hero_x=state.PlayerX,hero_y=state.PlayerY`,
		`paintedCaches.TryGetValue(state.Life.LevelId,out cache)`, `draw.id=="char.hero"`, `ViewSees(state.PlayerX,state.PlayerY)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing SDK proof contract %s", want)
		}
	}
	if strings.Count(s, "NewGame(") != 1 {
		t.Fatal("SDK proof resets session")
	}
	if strings.Count(s, "yield return new WaitForEndOfFrame();") != 2 {
		t.Fatal("SDK proof must settle two rendered frames")
	}
	for _, forbidden := range []string{"new PocGameState", "state.PlayerX=", "state.PlayerY=", "state.SceneAgents.Add", "state.Inventory="} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("SDK proof fabricates state: %s", forbidden)
		}
	}
}

func TestSDKHousingPoseRequiresPhysicalArrivalNative(t *testing.T) {
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
	method := sceneTemplateSection(t, "sdk_proof.cs.tmpl", "  static bool SdkPoseReached(", "  void SdkBeginCapture(")
	harness := `using System;
class Proof {
` + method + `
 public static void Main(){
  if(!SdkPoseReached("housing","housing",false,false,0,"На месте",41,37,41,37))throw new Exception("authored physical housing viewpoint rejected");
  if(SdkPoseReached("housing","housing",false,false,0,"Идёт: дом",41,37,41,37)||SdkPoseReached("housing","housing",false,false,1,"На месте",41,37,41,37))throw new Exception("pending path mistaken for arrival");
  if(SdkPoseReached("station","housing",false,false,0,"На месте",41,37,41,37)||SdkPoseReached("housing","housing",true,false,0,"На месте",41,37,41,37)||SdkPoseReached("housing","housing",false,true,0,"На месте",41,37,41,37))throw new Exception("wrong room, transit or automatic routine accepted");
  if(SdkPoseReached("housing","housing",false,false,0,"На месте",3,2,41,37))throw new Exception("old hardcoded room coordinates accepted");
  Console.WriteLine("SDK authored housing viewpoint requires actual level, pose and completed manual movement: PASS");
 }
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "proof.cs"), filepath.Join(dir, "proof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("SDK pose compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("SDK pose execution: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
