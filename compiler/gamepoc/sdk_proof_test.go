package gamepoc

import (
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
