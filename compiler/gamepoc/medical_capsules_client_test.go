package gamepoc

import (
	"strings"
	"testing"
)

func TestMedicalClientPhysicalPhasesNative(t *testing.T) {
	dto := sceneTemplateSection(t, "medical_capsules.cs.tmpl", " [Serializable] public sealed class PocMedicalPose", " public sealed partial class AngelsDemonsPoc")
	math := sceneTemplateSection(t, "medical_capsules.cs.tmpl", "  static double MedicalClock(", "  bool MedicalRead(")
	runPaintedTextureNative(t, "MedicalPhysicalPhases", `using System;
`+dto+`
class Proof {`+math+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static void Near(double a,double b,string why){Need(Math.Abs(a-b)<.000001,why+" actual="+a+" expected="+b);}
 public static void Main(){
  Near(MedicalClock(1000000000L,1200000000d,true,250,1),1000000000,"pause advanced a clinical phase");
  Near(MedicalClock(1000000000L,2000000000d,false,250,1),1250000000,"extrapolation escaped physical horizon");
  Near(MedicalClock(1000000000L,2000000000d,false,250,4),2000000000,"physical speed multiplier was ignored");
  Near(MedicalClock(1000000000L,500000000d,false,250,1),1000000000,"stale presentation rewound saved service");
  var u=new PocMedicalUnit{phase="entry",phase_start_ns=1000000000L,phase_end_ns=3000000000L,
   scan_pose=new PocMedicalPose{x=4.75,y=0,z=33.25,yaw_degrees=270},inside_pose=new PocMedicalPose{x=3,y=0,z=33.25,yaw_degrees=90}};
  var p=MedicalPatientPose(u,1000000000d);Near(p.x,4.75,"entry teleported at start");Near(p.yaw_degrees,270,"entry did not face walking direction");
  p=MedicalPatientPose(u,1750000000d);Near(p.x,3.875,"entry distance not sampled over first 75 percent");Near(p.yaw_degrees,270,"entry rotated while walking");
  u.patient_pose=new PocMedicalPose{x=4.5,y=.02,z=33.2,yaw_degrees=265};p=MedicalPresentedPose(u,1000000000d,1000000000d);Near(p.x,4.5,"published patient pose was ignored at server sample");Near(p.y,.02,"server support height was discarded");Near(p.yaw_degrees,265,"published yaw was ignored");p=MedicalPresentedPose(u,1000000000d,1750000000d);Near(p.x,3.625,"sub-snapshot motion lost authoritative anchor");Near(u.inside_pose.x,3,"sampling mutated server inside pose");u.patient_pose=null;
  p=MedicalPatientPose(u,2500000000d);Near(p.x,3,"entry not inside before turn");Near(p.yaw_degrees,270,"turn started before arrival");
  p=MedicalPatientPose(u,2750000000d);Near(p.x,3,"patient slid during turn");Near(p.yaw_degrees,180,"in-place turn not halfway");
  p=MedicalPatientPose(u,9000000000d);Near(p.x,3,"late sample escaped capsule");Near(p.yaw_degrees,90,"entry turn not complete");
  u.phase="treatment";p=MedicalPatientPose(u,1500000000d);Near(p.x,3,"treatment is not inside");Near(p.yaw_degrees,90,"treatment did not face room");
  u.phase="exit";p=MedicalPatientPose(u,2000000000d);Near(p.x,3.875,"exit did not walk toward room");Near(p.yaw_degrees,90,"exit walked backwards");
  u.exit_blocked=true;p=MedicalPatientPose(u,5000000000d);Near(p.x,3,"blocked exit put body on occupied anchor");u.exit_blocked=false;
  u.phase="scan";p=MedicalPatientPose(u,2000000000d);Near(p.x,4.75,"scanner moved before consent");
  u.phase="offer";Need(MedicalPatientPose(u,2000000000d)==u.scan_pose,"offer detached patient from public floor");
  u.phase="approach";Need(MedicalPatientPose(u,2000000000d)==null,"service stole ordinary path approach");
  u.door_start_ns=1000000000L;u.door_end_ns=1600000000L;u.door_from=0;u.door_to=1;
  Near(MedicalDoorProgress(u,1300000000d),.5,"opening door does not share saved physical phase");Near(MedicalDoorProgress(u,2000000000d),1,"opening door exceeded stroke");
  u.door_from=.7;u.door_to=0;Near(MedicalDoorProgress(u,1300000000d),.35,"closing did not preserve saved partial stroke");
  u.door_start_ns=u.door_end_ns=0;u.door_progress=.4;Near(MedicalDoorProgress(u,1300000000d),.4,"zero-duration saved door lost authoritative progress");
  u.phase="scan";u.phase_start_ns=1000000000L;u.phase_end_ns=4000000000L;
  Near(MedicalPhaseProgress(u.phase_start_ns,u.phase_end_ns,MedicalClock(1750000000L,3500000000d,true,250,1)),.25,"paused scan band advanced down patient");
  Near(MedicalPhaseProgress(u.phase_start_ns,u.phase_end_ns,1000000000d),0,"full scan lost its top");Near(MedicalPhaseProgress(u.phase_start_ns,u.phase_end_ns,4000000000d),1,"full scan lost its bottom");
  Console.WriteLine("saved physical clock, consent boundary, entry/turn/exit, blocked exit and reversible door phase PASS");
 }
}`)
}

func TestMedicalClientExitWaitsForSavedOpeningNative(t *testing.T) {
	dto := sceneTemplateSection(t, "medical_capsules.cs.tmpl", " [Serializable] public sealed class PocMedicalPose", " public sealed partial class AngelsDemonsPoc")
	math := sceneTemplateSection(t, "medical_capsules.cs.tmpl", "  static double MedicalClock(", "  bool MedicalRead(")
	runPaintedTextureNative(t, "MedicalExitOpeningInterlock", `using System;`+dto+`class Proof {`+math+`
 static void Near(double a,double b,string why){if(Math.Abs(a-b)>1e-7)throw new Exception(why+" actual="+a+" expected="+b);}
 public static void Main(){var u=new PocMedicalUnit{phase="exit",phase_start_ns=1000000000,phase_end_ns=3000000000,door_start_ns=1000000000,door_end_ns=1600000000,door_from=0,door_to=1,inside_pose=new PocMedicalPose{x=3,yaw_degrees=90},scan_pose=new PocMedicalPose{x=4.75,yaw_degrees=270}};
 Near(MedicalPatientPose(u,1300000000).x,3,"actual body advanced while leaves opened");Near(MedicalPatientPose(u,1600000000).x,3,"body moved at opening endpoint");Near(MedicalPatientPose(u,2300000000).x,3.875,"remaining phase was not rescaled");Near(MedicalPatientPose(u,3000000000).x,4.75,"exit deadline changed");
 u.patient_pose=new PocMedicalPose{x=3,yaw_degrees=90};Near(MedicalPresentedPose(u,1300000000,1500000000).x,3,"bounded extrapolation crossed closed leaves");Near(MedicalPresentedPose(u,1300000000,2300000000).x,3.875,"published pose anchor lost after opening");
 Near(MedicalPatientPose(u,MedicalClock(1300000000,2300000000,true,250,1)).x,3,"paused saved opening advanced");u.door_from=1;Near(MedicalExitProgress(u,2000000000),.5,"already-open door waited again");u.door_from=0;u.phase_start_ns=4000000000;u.phase_end_ns=6000000000;Near(MedicalExitProgress(u,5000000000),.5,"blocked retry reused old opening");u.exit_blocked=true;Near(MedicalPatientPose(u,6000000000).x,3,"blocked retry moved");Console.WriteLine("saved opening interlock, unchanged deadline, authoritative anchor, pause/load and already-open retry PASS");}
}`)
}

func TestMedicalClientUsesExistingBodyAndPrivateOffer(t *testing.T) {
	core := hudTemplate(t, "medical_capsules.cs.tmpl")
	panel := hudTemplate(t, "medical_capsules_panel.cs.tmpl")
	for _, forbidden := range []string{"Instantiate(", "new GameObject(", "Time.deltaTime", "Time.unscaledDeltaTime", "wounds.StartCare", "BalanceMinor="} {
		if strings.Contains(core, forbidden) || strings.Contains(panel, forbidden) {
			t.Errorf("medical presentation owns forbidden simulation/body operation %q", forbidden)
		}
	}
	if !strings.Contains(core, "medicalSnapshot.epoch==state.Epoch") || !strings.Contains(core, "unit.patient_id==id") || !strings.Contains(core, "visual.door.localRotation=visual.doorRest*") {
		t.Fatal("medical actor/motion authority is not bound to current snapshot")
	}
	for _, need := range []string{"MedicalOfferId=offer", `MedicalSend("medical_accept",unit.id,hero.offer_id)`, `if(own){`, `hero.quote`, `unit.request_enabled`} {
		if !strings.Contains(panel, need) {
			t.Errorf("missing real private treatment integration %q", need)
		}
	}
	hero := hudTemplate(t, "hero_body.cs.tmpl")
	if !strings.Contains(hero, "if(MedicalHeroFeet(out feet,out heading))return true;") || !strings.Contains(hero, "if(!string.IsNullOrEmpty(a.ServiceAttachment))continue;") || !strings.Contains(hero, "if(MedicalHeroBusy()){hbSteering=hbFollowing=hbPress=false;return;}") {
		t.Fatal("attached actor must keep body/camera ownership and leave floor occupancy; service must block steer without sending stop")
	}
	hum := hudTemplate(t, "location_sdk_humanoid.cs.tmpl")
	if !strings.Contains(hum, "bool medical=MedicalActorPose(id,out medicalFeet,out medicalYaw)") || !strings.Contains(hum, "if(medical){h.yaw=medicalYaw;h.hasYaw=true;}") || !strings.Contains(hum, "if(!medical&&!crouch&&!h.walking&&LocationSDKTalkGesture") {
		t.Fatal("medical pose must use the ordinary humanoid and walk sampler while suppressing stale conversations")
	}
	sdk := hudTemplate(t, "location_sdk.cs.tmpl")
	if !strings.Contains(sdk, "UpdateTrainMotionSDK();UpdateMedicalCapsuleSDK();") || strings.Index(sdk, "UpdateMedicalCapsuleSDK();") > strings.Index(sdk, "LocationSDKRenderNow();") {
		t.Fatal("clinical motion must be posed before the real world render")
	}
	proof := hudTemplate(t, "medical_capsules_proof.cs.tmpl")
	if strings.Contains(proof, ".GetInstanceID(") || !strings.Contains(proof, "object.ReferenceEquals(previous,h.go)") || !strings.Contains(proof, "Dictionary<string,GameObject> medicalReviewBodies") {
		t.Fatal("native medical ownership must compare actual object references, without obsolete instance IDs or hash identities")
	}
	for _, need := range []string{`SaveOrLoad("slot3",false)`, `MedicalSend("medical_accept",offer.unit_id,offer.offer_id)`, `SaveOrLoad("slot2",true)`, `SaveOrLoad("slot2",false)`, "medicalReviewBalanceBefore-medicalReviewBalanceAfter==medicalReviewQuotePrice", "copies!=1", "unit.queue_count>0", `instance.id=="wall.med.1"`} {
		if !strings.Contains(proof, need) {
			t.Errorf("native review lacks concrete clinical/ownership gate %q", need)
		}
	}
}

func TestMedicalNativeReviewStartsBeforeMotionAndShowsOffer(t *testing.T) {
	present := sceneTemplateSection(t, "medical_capsules_proof.cs.tmpl", "  void MedicalReviewOfferPresented(", "  void UpdateMedicalCapsuleProof(")
	runPaintedTextureNative(t, "MedicalRenderedOfferGate", `using System;
enum EventType{Layout,Repaint}class Event {public static Event current=new Event();public EventType type;}static class Time{public static float unscaledTime;}
class Proof {bool medicalReviewStarted,medicalReviewDone;string medicalReviewOfferID;double medicalReviewOfferAt=-1;`+present+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var p=new Proof();Event.current.type=EventType.Repaint;Time.unscaledTime=10;p.MedicalReviewOfferPresented("a","offer1");Need(p.medicalReviewOfferAt<0,"ordinary UI starts native acceptance clock");p.medicalReviewStarted=true;Event.current.type=EventType.Layout;p.MedicalReviewOfferPresented("a","offer1");Need(p.medicalReviewOfferAt<0,"unrendered offer starts acceptance clock");Event.current.type=EventType.Repaint;p.MedicalReviewOfferPresented("a","offer1");Need(p.medicalReviewOfferAt==10,"first rendered offer lost");Time.unscaledTime=10.9f;p.MedicalReviewOfferPresented("a","offer1");Need(Time.unscaledTime-p.medicalReviewOfferAt<1,"offer accepted before a rendered second");Time.unscaledTime=11.1f;p.MedicalReviewOfferPresented("a","offer1");Need(Time.unscaledTime-p.medicalReviewOfferAt>=1,"continuously rendered offer cannot satisfy visible hold");p.MedicalReviewOfferPresented("a","offer2");Need(p.medicalReviewOfferAt==Time.unscaledTime,"replacement quote inherited old visible hold");Console.WriteLine("only actual repaint starts offer hold; new quote resets it; ordinary UI unaffected PASS");}
}`)
	fixture := sceneTemplateSection(t, "medical_capsules_test.go.tmpl", "func TestMedicalNativeFixture(", "func TestMedicalScanDoesNotStopActualBleeding(")
	request := strings.Index(fixture, `Action: "medical_request"`)
	pause := strings.Index(fixture, "r.state.Paused = true")
	save := strings.Index(fixture, "r.writeSave(")
	if request < 0 || pause < request || save < pause || !strings.Contains(fixture, "!loaded.Paused") {
		t.Fatal("native fixture must save a verified paused ordinary request before loading can advance scan")
	}
	proof := hudTemplate(t, "medical_capsules_proof.cs.tmpl")
	for _, need := range []string{`Options(false,1);MedicalReviewNoteAdd("SDK ready;`, `Time.unscaledTime-medicalReviewOfferAt<1`, `medicalClosedUnit=offer.unit_id;medicalSelectedUnit=null;medicalReviewPhase=3`, `medicalReviewRenderedTick!=state.Tick`, `medicalReviewExit=false;Options(false,1)`, `SaveOrLoad("slot1",true)`} {
		if !strings.Contains(proof, need) {
			t.Errorf("native review readiness/offer or preserved load/exit gate missing %q", need)
		}
	}
	panel := hudTemplate(t, "medical_capsules_panel.cs.tmpl")
	for _, need := range []string{`MedicalReviewOfferPresented(unit.id,hero.offer_id);`, `own.unit_id!=medicalClosedUnit`, `if(medicalClosedUnit==unit.id)return;`} {
		if !strings.Contains(panel, need) {
			t.Errorf("rendered offer and persistent panel closure missing %q", need)
		}
	}
}

func TestMedicalScanDrawsOnlyActualSelectedPatientSubmeshesNative(t *testing.T) {
	eligible := sceneTemplateSection(t, "medical_scan.cs.tmpl", "  static bool MedicalScanEligible(", "  sealed class MedicalScanPass:")
	draw := sceneTemplateSection(t, "medical_scan.cs.tmpl", "  void MedicalScanDraw(", "  void MedicalScanRemove(")
	runPaintedTextureNative(t, "MedicalActualSurfaceDraw", `using System;using System.Collections.Generic;
class PocMedicalUnit {public string id,patient_id,level_id,phase;public long phase_start_ns,phase_end_ns;}
class GameObject {public bool activeInHierarchy=true;}class Material {}class Mesh {public int subMeshCount=2;}class Renderer {}class SkinnedMeshRenderer:Renderer {public bool enabled=true,forceRenderingOff;public GameObject gameObject=new GameObject();public Mesh sharedMesh=new Mesh();}
class RasterCommandBuffer {public readonly List<Renderer> drawn=new List<Renderer>();public readonly List<int> submeshes=new List<int>();public void DrawRenderer(Renderer renderer,Material material,int submesh,int pass){drawn.Add(renderer);submeshes.Add(submesh);}}
static class Time {public static int frameCount=4;}class Life {public string LevelId="market";}class State {public Life Life=new Life();}
class MedicalScanSurface {public string unit,patient;public GameObject body=new GameObject();public SkinnedMeshRenderer[] renderers;public Material material=new Material();public int drawnFrame=-1,drawCalls;}
class Proof {readonly Dictionary<string,MedicalScanSurface> medicalScanSurfaces=new Dictionary<string,MedicalScanSurface>();readonly List<PocMedicalUnit> units=new List<PocMedicalUnit>();State state=new State();string medicalScanHiddenUnit;bool MedicalRead(){return true;}PocMedicalUnit MedicalPatientUnit(string patient){return units.Find(u=>u.patient_id==patient);}`+eligible+draw+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var p=new Proof();var unit=new PocMedicalUnit{id="a",patient_id="hero",level_id="market",phase="scan",phase_start_ns=100,phase_end_ns=300};p.units.Add(unit);var real=new SkinnedMeshRenderer();var hidden=new SkinnedMeshRenderer{enabled=false};p.medicalScanSurfaces["a"]=new MedicalScanSurface{unit="a",patient="hero",renderers=new[]{real,hidden}};var cmd=new RasterCommandBuffer();
 p.MedicalScanDraw(cmd);Need(cmd.drawn.Count==2&&Object.ReferenceEquals(cmd.drawn[0],real)&&Object.ReferenceEquals(cmd.drawn[1],real),"scan substituted a mesh/body or drew another renderer");Need(cmd.submeshes[0]==0&&cmd.submeshes[1]==1,"scan skipped actual patient submesh");Need(p.medicalScanSurfaces["a"].drawCalls==2,"executed draw audit disagrees with commands");
 p.medicalScanHiddenUnit="a";p.MedicalScanDraw(cmd);Need(cmd.drawn.Count==2,"pixel comparison did not isolate the chosen patient");p.medicalScanHiddenUnit=null;unit.phase="offer";p.MedicalScanDraw(cmd);Need(cmd.drawn.Count==2,"offer/treatment retained laser draw");unit.phase="scan";unit.patient_id="npc";p.MedicalScanDraw(cmd);Need(cmd.drawn.Count==2,"stale body inherited another patient's scan");unit.patient_id="hero";unit.level_id="elsewhere";p.MedicalScanDraw(cmd);Need(cmd.drawn.Count==2,"scan escaped its SDK level");unit.level_id="market";unit.phase_end_ns=100;p.MedicalScanDraw(cmd);Need(cmd.drawn.Count==2,"invalid saved phase acquired surface scan");Console.WriteLine("actual selected renderer/submeshes; no body clone, wrong patient, non-scan phase or foreign level PASS");}
}`)
}

func TestMedicalScanReleasesOverlayWithoutChangingPatientNative(t *testing.T) {
	release := sceneTemplateSection(t, "medical_scan.cs.tmpl", "  void MedicalScanRemove(", "  void UpdateMedicalScanSDK(")
	runPaintedTextureNative(t, "MedicalSurfaceRelease", `using System;using System.Collections.Generic;
static class RenderPipelineManager {public static event Action beginCameraRendering;public static bool Listening{get{return beginCameraRendering!=null;}}}
class Material {public bool destroyed;}class Body {public bool active=true;}class MedicalScanSurface {public Material material=new Material();public Body body=new Body();}
class Proof {readonly Dictionary<string,MedicalScanSurface> medicalScanSurfaces=new Dictionary<string,MedicalScanSurface>();readonly HashSet<string> medicalScanSeen=new HashSet<string>();bool medicalScanSubscribed;void MedicalScanBeginCamera(){}static void Destroy(Material material){material.destroyed=true;}`+release+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var p=new Proof();var a=new MedicalScanSurface();var b=new MedicalScanSurface();p.medicalScanSurfaces["a"]=a;p.medicalScanSurfaces["b"]=b;p.medicalScanSeen.Add("a");RenderPipelineManager.beginCameraRendering+=p.MedicalScanBeginCamera;p.medicalScanSubscribed=true;p.MedicalScanRemove("a");Need(a.material.destroyed&&a.body.active&&!b.material.destroyed,"ending one scan touched body or other patient's overlay");p.ReleaseMedicalScan();Need(b.material.destroyed&&b.body.active&&p.medicalScanSurfaces.Count==0&&p.medicalScanSeen.Count==0&&!RenderPipelineManager.Listening,"SDK release retained renderer callback/material or destroyed body");p.ReleaseMedicalScan();Console.WriteLine("per-patient release and SDK unsubscribe preserve original bodies PASS");}
}`)
	core := hudTemplate(t, "medical_capsules.cs.tmpl")
	scan := hudTemplate(t, "medical_scan.cs.tmpl")
	shader := hudTemplate(t, "medical_scan.shader.tmpl")
	for _, forbidden := range []string{"Instantiate(", "new GameObject(", "BakeMesh(", "sharedMaterials=", "Time.deltaTime", "Time.unscaledDeltaTime"} {
		if strings.Contains(scan, forbidden) {
			t.Errorf("surface scan owns forbidden body/material/simulation operation %q", forbidden)
		}
	}
	for _, need := range []string{"camera!=locationSDKCamera", "RenderPassEvent.BeforeRenderingTransparents", "SetRenderAttachmentDepth(resources.activeDepthTexture,AccessFlags.Read)", "commands.DrawRenderer(renderer,surface.material,submesh,0)", "GetComponentsInChildren<SkinnedMeshRenderer>(true)", "if(!medicalScanSeen.Contains(id))removed.Add(id)"} {
		if !strings.Contains(scan, need) {
			t.Errorf("actual surface/depth/release contract missing %q", need)
		}
	}
	if !strings.Contains(core, "foreach(var ghost in line.GetComponentsInChildren<Renderer>(true))ghost.enabled=false;") || !strings.Contains(core, "UpdateMedicalScanSDK();") || !strings.Contains(core, "lightsocket.scan_projector.") {
		t.Fatal("surface scan must replace ghost geometry and follow the actual projector and existing server-height driver")
	}
	proof := hudTemplate(t, "medical_capsules_proof.cs.tmpl")
	for _, need := range []string{"MedicalScanExecuted(unit.id,out scanDraws)", "medicalScanHiddenUnit=unitID", "medicalScanHiddenUnit=null", "medicalReviewScanMinProgress<=.1", "medicalReviewScanMaxProgress>=.9", "Math.Abs(medicalReviewScanSeconds-3)", "medicalReviewScanMaxPixels>5", "medicalReviewScanReleased"} {
		if !strings.Contains(proof, need) {
			t.Errorf("native scan must prove actual draw, complete phase, pixels and release: %q", need)
		}
	}
	for _, need := range []string{"ZTest LEqual ZWrite Off", "Blend One One", "fwidth(input.positionWS.y)", "TransformObjectToWorldNormal", "dot(normalize(input.normalWS),toProjector)", "clip(facing-.03)"} {
		if !strings.Contains(shader, need) {
			t.Errorf("physical surface scan shader missing %q", need)
		}
	}
}

func TestMedicalNativeReviewRequiresBothNamedNPCsAndQueueDrain(t *testing.T) {
	proof := hudTemplate(t, "medical_capsules_proof.cs.tmpl")
	for _, need := range []string{`MedicalReviewNPCIDs={"char.market_granny","char.market_resident_01"}`, `medicalReviewNPCTreated.Count==2`, `medicalReviewNPCCompleted.Count==2`, `MedicalPatientUnit(id)==null`, `unit.queue_count!=0||!string.IsNullOrEmpty(unit.patient_id)`, `ClickMove(11,66)`, `medicalReviewScanPointCleared&&medicalReviewPayment`, `medicalReviewQueueCompleted&&medicalReviewNPCCompleted.Count==2`, `SaveOrLoad("slot1",true)`} {
		if !strings.Contains(proof, need) {
			t.Errorf("native queue completion lacks %q", need)
		}
	}
}
