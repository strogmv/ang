package gamepoc

import (
	"github.com/strogmv/ang/compiler/emitter"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Optional real-schema compiler artifact for a source-only Unity API check.
// The output is scratch evidence, never a hand-edited generated project file.
func TestSanitaryClientProtoContract(t *testing.T) {
	dir := os.Getenv("ANG_SANITARY_API_DIR")
	if dir == "" {
		t.Skip("set scratch directory for a complete C# API compile")
	}
	_, def, err := readCompiled(filepath.Join(os.Getenv("ANG_MEDICAL_PROJECT"), "server"))
	if err != nil {
		t.Fatal(err)
	}
	body := emitter.RenderGRPCProto(def, "angelsdemons/server")
	if !strings.Contains(body, "sanitary_services_json = 93;") {
		t.Fatal("sanitary snapshot must retain field 93")
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "game.proto"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSanitaryClientPhysicalPhasesNative(t *testing.T) {
	dto := sceneTemplateSection(t, "sanitary_services.cs.tmpl", " [Serializable] public sealed class PocSanitaryPose", " public sealed partial class AngelsDemonsPoc")
	math := sceneTemplateSection(t, "sanitary_services.cs.tmpl", "  static double SanitaryClock(", "  bool SanitaryRead(")
	runPaintedTextureNative(t, "SanitaryPhysicalPhases", `using System;
`+dto+`
class Proof {`+math+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static void Near(double a,double b,string why){Need(Math.Abs(a-b)<.000001,why+" actual="+a+" expected="+b);}
 public static void Main(){
  Near(SanitaryClock(1000000000L,2000000000d,true,250,4),1000000000,"pause advanced use");
  Near(SanitaryClock(1000000000L,2000000000d,false,250,1),1250000000,"extrapolation escaped horizon");
  Near(SanitaryClock(1000000000L,2000000000d,false,250,4),2000000000,"speed omitted");
  Near(SanitaryClock(1000000000L,500000000d,false,250,1),1000000000,"load rewound pose");
  var u=new PocSanitaryUnit{phase="entry",phase_start_ns=1000000000L,phase_end_ns=3000000000L,
   actor_pose=new PocSanitaryPose(),entry_pose=new PocSanitaryPose{x=3.75,z=23.75,yaw_degrees=270},inside_pose=new PocSanitaryPose{x=2.5,z=23.625,yaw_degrees=90}};
  var p=SanitaryPatientPose(u,1800000000d);Near(p.x,3.125,"entry has no continuous physical travel");Near(p.yaw_degrees,270,"turn before arrival");
  p=SanitaryPatientPose(u,2800000000d);Near(p.x,2.5,"turn drifted inside");Near(p.yaw_degrees,180,"in-place turn is wrong");
  p=SanitaryPatientPose(u,5000000000d);Near(p.x,2.5,"late pose escaped cabin");Near(p.yaw_degrees,90,"use orientation wrong");
  foreach(var phase in new[]{"sealing","using","opening_exit"}){u.phase=phase;Need(SanitaryPatientPose(u,5000000000d)==u.inside_pose,"interior phase escaped cabin");}
  u.phase="exit";p=SanitaryPatientPose(u,2000000000d);Near(p.x,3.125,"exit motion lost");Need(p.yaw_degrees>80&&p.yaw_degrees<90,"off-centre WC exit walks backwards");
  u.entry_pose=new PocSanitaryPose{x=4.75,z=21.75,yaw_degrees=180};u.inside_pose=new PocSanitaryPose{x=4.6,z=20.95,yaw_degrees=180};
  p=SanitaryPatientPose(u,2000000000d);Need(p.yaw_degrees>0&&p.yaw_degrees<15,"shower exit must face hall");
  u.phase="approach";Need(SanitaryPatientPose(u,2000000000d)==null,"service stole common-entry navigation");
  u.phase="using";u.actor_pose=null;Need(SanitaryPatientPose(u,2000000000d)==null,"reconstructed hidden NPC cabin pose");
  u.door=new PocSanitaryDoor{start_ns=1000000000L,end_ns=1600000000L,from=0,to=1};Near(SanitaryDoorProgress(u,1300000000d),.5,"door ignores saved physical phase");Near(SanitaryDoorProgress(u,2000000000d),1,"door exceeded stop");
  u.door.from=.7;u.door.to=0;Near(SanitaryDoorProgress(u,1300000000d),.35,"saved partial closing jumps");u.door=null;u.door_progress=.4;Near(SanitaryDoorProgress(u,0),.4,"static saved door lost");
  Console.WriteLine("saved clock, shared-entry navigation, entry/turn/exit, private pose and bifold phase PASS");
 }
}`)
}

func TestSanitaryClientAuthorityAndSameBody(t *testing.T) {
	core := hudTemplate(t, "sanitary_services.cs.tmpl")
	panel := hudTemplate(t, "sanitary_services_panel.cs.tmpl")
	for _, forbidden := range []string{"Instantiate(", "new GameObject(", "Time.deltaTime", "Time.unscaledDeltaTime", "Needs[", "BalanceMinor="} {
		if strings.Contains(core, forbidden) || strings.Contains(panel, forbidden) {
			t.Errorf("sanitary presentation owns forbidden operation %q", forbidden)
		}
	}
	for _, need := range []string{"sanitarySnapshot.epoch==state.Epoch", "sanitarySnapshot.level_id==state.Life.LevelId", "u.actor_pose==null", "u.occupant_id==id", "v.doorRest*Quaternion.AngleAxis(-90*p", "v.foldRest*Quaternion.AngleAxis(180*p", "v.flow.gameObject.SetActive(u.flowing"} {
		if !strings.Contains(core, need) {
			t.Errorf("missing authoritative/private visual guard %q", need)
		}
	}
	for _, need := range []string{`SanitarySend("sanitary_request",u.id)`, `SanitarySend("sanitary_cancel",u.id)`, `u.request_enabled`, `u.request_reason`, `Guid.NewGuid()`} {
		if !strings.Contains(panel, need) {
			t.Errorf("missing ordinary service UI operation %q", need)
		}
	}
	hero := hudTemplate(t, "hero_body.cs.tmpl")
	hum := hudTemplate(t, "location_sdk_humanoid.cs.tmpl")
	if !strings.Contains(hero, "if(SanitaryHeroFeet(out feet,out heading))return true;") || !strings.Contains(hero, "if(SanitaryHeroBusy()){hbSteering=hbFollowing=hbPress=false;return;}") || !strings.Contains(hum, "bool sanitary=SanitaryActorPose(id,out sanitaryFeet,out sanitaryYaw)") {
		t.Fatal("service must retain existing hero and common humanoid ownership")
	}
	proof := hudTemplate(t, "sanitary_services_proof.cs.tmpl")
	if strings.Contains(proof, "GetInstanceID(") {
		t.Fatal("Unity 6.6 obsolete instance ID used")
	}
	for _, need := range []string{`SaveOrLoad("slot3",false)`, `SaveOrLoad("slot2",true)`, `SaveOrLoad("slot2",false)`, `sanitaryReviewWaterBefore==25000`, `sanitaryReviewSavedWater`, `sanitaryReviewSavedNeed`, `previous!=h.go`, `copies!=1`, `GetEntityId().ToString()`, `CaptureScreenshotAsTexture()`} {
		if !strings.Contains(proof, need) {
			t.Errorf("native proof lacks actual outcome/identity check %q", need)
		}
	}
}
