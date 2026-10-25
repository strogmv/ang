package gamepoc

import (
	"strings"
	"testing"
)

func TestTransportHubFareObserverRejectsEarlyAndRepeatedDebitNative(t *testing.T) {
	audit := sceneTemplateSection(t, "transport_hub_proof.cs.tmpl", "  sealed class HubFareAudit ", "  [Serializable] sealed class HubCaptureRecord ")
	runPaintedTextureNative(t, "TransportHubFare", `using System;
class Proof {
`+audit+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 public static void Main(){
  var a=new HubFareAudit(1000,100);
  for(int i=0;i<100;i++)Need(a.Observe(false,1000)==null&&!a.boarded,"waiting/selection treated as boarding");
  Need(a.Observe(false,900)!=null&&!a.boarded,"early debit accepted");
  Need(a.Observe(true,1000)!=null&&!a.boarded,"free arrival accepted as paid boarding");
  Need(a.Observe(true,900)==null&&a.boarded,"single real boarding debit rejected");
  for(int i=0;i<100;i++)Need(a.Observe(true,900)==null,"paused/saved arrival repeated a fare");
  Need(a.Observe(false,900)==null,"arrival requires another charge");
  Need(a.Observe(true,800)!=null&&a.Observe(false,800)!=null,"double debit passed arrival/load");
  Need(a.Observe(false,1000)!=null,"refund incorrectly accepted after paid boarding");
  Console.WriteLine("real boarding only, waiting unchanged, saved paid arrival retained, second debit rejected PASS");
 }
}`)
}

func TestTransportHubProofHasPhysicalRoundtripAndNoSyntheticState(t *testing.T) {
	s := hudTemplate(t, "transport_hub_proof.cs.tmpl")
	for _, required := range []string{
		`"--poc-verify-transport-hub"`, `Path.IsPathRooted`, `NewGame(777,"life")`,
		`HubEnter("station_hub_walk",HubLevel)`, `HubEnter("hub_station_rail",HubStation)`,
		`foreach(var marker in state.Life.Markers)`, `LifeDoorVisible(marker)`, `ClickMove(x,y)`,
		`new CopperRouteLegAudit`, `ViewCell(v,at.X,at.Y)=='0'`,
		`"hub-passenger-lift.png"`, `"hub-freight-lift.png"`, `"hub-copper-gate.png"`, `"station-paid-return.png"`,
		`d.id=="char.poc_hub_guard"`, `capture.GetPixels32()`, `paintedCaches.TryGetValue`,
		`exit.TravelSeconds`, `hubLeg.record.completed`, `post-arrival save was not physically perturbed`, `SaveOrLoad("slot3",true)`, `SaveOrLoad("slot3",false)`,
		`state.Epoch!=hubSavedEpoch`, `state.Tick==hubSavedTick`, `state.GameSeconds==hubSavedSeconds`,
		`state.Life.TravelRemainingSeconds==0`, `state.Life.LevelId==HubStation`, `CopperRouteInventory()!=hubSavedInventory`,
		`HubBalance()!=hubBefore-100`, `hubFare.paid_samples<2`, `ReferenceEquals(hubSession,session)`,
	} {
		if !strings.Contains(s, required) {
			t.Errorf("missing hub native acceptance evidence %q", required)
		}
	}
	if strings.Count(s, "NewGame(") != 1 || strings.Count(s, `LifeCommand("goto",`) != 1 {
		t.Fatal("hub roundtrip must preserve one ordinary session and use explicit physical edges")
	}
	for _, bad := range []string{`state.PlayerX=`, `state.PlayerY=`, `state.Life.LevelId=`, `state.GameSeconds=`, `state.Tick=`, `new PocGameState`, `SceneAgents.Add`, `scene.navigation`} {
		if strings.Contains(strings.ReplaceAll(s, "==", "EQ"), bad) {
			t.Errorf("synthetic proof state forbidden: %q", bad)
		}
	}
	u := hudTemplate(t, "unity.cs.tmpl")
	for _, required := range []string{`InitTransportHubProof()`, `verifyLife = verifyRegionMap || verifyTransportHub ||`, `TransportHubProofDiagnostics(now)`, `TransportHubProofStep(now)`, `TransportHubProofReportJSON()`, `return transportHubProofDirectory`} {
		if !strings.Contains(u, required) {
			t.Errorf("missing ordinary proof lifecycle hook %q", required)
		}
	}
}
