package gamepoc

import (
	"strings"
	"testing"
)

func TestTrainFleetPaintedQueryKeepsBothTrainOwners(t *testing.T) {
	query := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static PocTrainMotionState[] TrainPaintedBoundTrains(", "  void UpdatePaintedTrainMotion(")
	find := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static PocTrainMotionBinding TrainBinding(", "  static Transform TrainChild(")
	runPaintedTextureNative(t, "TrainFleetOwners", `using System;using System.Collections.Generic;
class PocTrainMotionBinding {public string level_id,passage_id;public bool visible;}
class PocTrainMotionModule {public string id="car.01";public PocTrainMotionBinding[] bindings;}
class PocTrainMotionState {public string train_id,stock_id;public PocTrainMotionModule[] modules;}
class PocTrainMotionSnapshot {public PocTrainMotionState[] trains;}
class Proof {`+query+find+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static PocTrainMotionState Train(string id,string level,string passage){return new PocTrainMotionState{train_id=id,stock_id=id,modules=new[]{new PocTrainMotionModule{bindings=new[]{new PocTrainMotionBinding{level_id=level,passage_id=passage,visible=true}}}}};}
 public static void Main(){var passenger=Train("passenger","housing","lower");var freight=Train("freight","housing","lower");var station=Train("legacy","station","boarding");
 var snapshot=new PocTrainMotionSnapshot{trains=new[]{passenger,freight,station}};var bound=TrainPaintedBoundTrains(snapshot,"housing","lower");
 Need(bound.Length==2&&ReferenceEquals(bound[0],passenger)&&ReferenceEquals(bound[1],freight),"same canonical car ID collapsed independent train owners");
 passenger.modules[0].bindings[0].visible=false;bound=TrainPaintedBoundTrains(snapshot,"housing","lower");Need(bound.Length==1&&ReferenceEquals(bound[0],freight),"offscreen passenger hid or impersonated freight");
 Need(TrainPaintedBoundTrains(snapshot,"housing","boarding").Length==0,"fleet escaped matching physical passage");Need(TrainPaintedBoundTrains(null,"housing","lower").Length==0,"missing snapshot exposed cached train");Console.WriteLine("both independent train IDs with same canonical module IDs; correct passage and hidden owner PASS");
 }}`)
	adapter := hudTemplate(t, "train_motion_painted.cs.tmpl")
	for _, required := range []string{"foreach(var candidate in trainMotionSnapshot.trains)", "TrainPaintedObserveModules(scene,passage.passage,candidate,candidateLead,ObserverView());", "foreach(var candidate in boundTrains)", "TrainPaintedModuleObserved(scene,passage.passage,binding,candidate.speed_mps,candidateLead,ObserverView())"} {
		if !strings.Contains(adapter, required) {
			t.Errorf("missing per-train projection ownership %q", required)
		}
	}
	core := hudTemplate(t, "train_motion.cs.tmpl")
	if !strings.Contains(core, "trainMotionInstances.TryGetValue(train.train_id,out c)") || !strings.Contains(core, "trainMotionInstances[train.train_id]=c") {
		t.Fatal("fleet instance cache must be keyed by immutable train ID")
	}
}

func TestTrainFleetReviewRejectsOneTypeWrongTimeAndChangedSavedClock(t *testing.T) {
	hours := sceneTemplateSection(t, "train_fleet_proof.cs.tmpl", "  static bool FleetEveningHour(", "  static bool FleetForbiddenPhysics(")
	travel := sceneTemplateSection(t, "train_fleet_proof.cs.tmpl", "  static bool FleetTravelReady(", "  bool FleetLocationReady(")
	gates := sceneTemplateSection(t, "train_fleet_proof.cs.tmpl", "  static bool FleetTypesReady(", "  static bool FleetTravelReady(")
	clock := sceneTemplateSection(t, "train_fleet_proof.cs.tmpl", "  static bool FleetClockEquals(", "  void UpdateTrainFleetProof(")
	runPaintedTextureNative(t, "FleetReviewAuthority", `using System;using System.Collections.Generic;
class Proof {const string FleetPassenger="train.c7.passenger.01",FleetFreight="train.c7.freight.01";`+hours+gates+travel+clock+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){Need(!FleetEveningHour(18.99)&&FleetEveningHour(19)&&FleetEveningHour(23.99)&&!FleetEveningHour(24),"daylight/midnight fixture accepted as evening");
 var seen=new Dictionary<string,int>{{"market/"+FleetPassenger,100}};Need(!FleetTypesReady(seen,"market",5),"one moving passenger passed fleet review");seen["market/"+FleetFreight]=4;Need(!FleetTypesReady(seen,"market",5),"fleeting unverified second type passed");seen["market/"+FleetFreight]=5;Need(FleetTypesReady(seen,"market",5),"both stable IDs failed");Need(!FleetTypesReady(seen,"housing",5),"market evidence was reused as another location");
 Need(FleetClockEquals(new[]{7d,10d,12d,20d,22d},new[]{7d,10d,12d,20d,22d}),"saved state mismatch");Need(!FleetClockEquals(new[]{7d,10d,12d,20d,22d},new[]{7d,10d,12d,20.01d,22d}),"second train drift was not checked");Need(!FleetClockEquals(null,new[]{7d}),"missing saved fleet accepted");var first=new Dictionary<string,double>{{"market/"+FleetPassenger,0},{"market/"+FleetFreight,100}};var last=new Dictionary<string,double>{{"market/"+FleetPassenger,20},{"market/"+FleetFreight,100}};Need(!FleetTravelReady(first,last,"market",5),"stationary freight passed two-type movement proof");last["market/"+FleetFreight]=108;Need(FleetTravelReady(first,last,"market",5),"both independently travelled trains failed");Console.WriteLine("both train types, actual evening and strict both-train pause/load clock PASS");
 }}`)
	proof := hudTemplate(t, "train_fleet_proof.cs.tmpl")
	for _, required := range []string{"SaveOrLoad(\"slot3\",false)", "Options(true,1)", "ClickMove(101,70)", "state.Tick==fleetSavedTick", "foreach(var train in trainMotionSnapshot.trains)", "figures[i].forceRenderingOff=true", "FleetChangedPixels(baseline,FleetPixels(locationSDKTarget))", "lights[i].enabled=enabled[i]", "fleetFigurePixels.Length==3", "trainMotionSnapshot.game_seconds%86400"} {
		if !strings.Contains(proof, required) {
			t.Errorf("missing real fleet proof contract %q", required)
		}
	}
	if strings.Contains(proof, "Instantiate(") || strings.Contains(proof, "RenderSettings.") {
		t.Fatal("proof must inspect existing rendered passengers and actual evening, not clone bodies or fake lighting")
	}
}

func TestTrainFleetReviewReturnsFromActualEveningHome(t *testing.T) {
	approach := sceneTemplateSection(t, "train_fleet_proof.cs.tmpl", "  static string FleetApproachEdge(", "  static bool FleetEveningHour(")
	runPaintedTextureNative(t, "FleetEveningHomeApproach", `using System;
class Proof {`+approach+`
 static void Need(bool ok,string message){if(!ok)throw new Exception(message);}
 public static void Main(){
  Need(FleetApproachEdge("level.poc_housing")=="home_station","real21:00hero at home cannot start ordinary return");
  Need(FleetApproachEdge("level.poc_monorail")=="station_hub_walk","home return attempted a boarding or invented edge");
  Need(FleetApproachEdge("level.c7_transport_hub")=="hub_market_walk","hub return does not reach market");
  Need(FleetApproachEdge("level.c7_market")=="","already present hero needlessly leaves market");
  Need(FleetApproachEdge("unknown")==null,"unsupported fixture location silently accepted");
  Console.WriteLine("actual evening home -> ordinary station/hub/market approach, no teleport PASS");
 }}`)
	proof := hudTemplate(t, "train_fleet_proof.cs.tmpl")
	loadGate := sceneTemplateSection(t, "train_fleet_proof.cs.tmpl", "    case 1:", "    case 2:")
	if strings.Contains(loadGate, `LevelId!="level.c7_market"`) || strings.Contains(loadGate, "locationSDKFrameReady") {
		t.Fatal("paused evening at home must not wait for native market SDK before ordinary travel")
	}
	for _, required := range []string{`if(!state.Paused||!FleetEveningHour(`, `FleetClockState()==null`, `LifeCommand("enter",approach)`, `state.Life.LevelId==fleetApproachFrom`, `if(page!="game"||!locationSDKFrameReady||!TrainMotionRead())return;fleetStarted=true`, `FleetDiagnostic();`, `fleetDiagnosticAt=Time.unscaledTime+2`, `Path.Combine(fleetDir,"phases.jsonl")`} {
		if !strings.Contains(proof, required) {
			t.Errorf("missing bounded real-state approach/diagnostics gate %q", required)
		}
	}
}
