package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocationSDKCameraUsesOneProjectionForBoundsActorsAndPointer(t *testing.T) {
	data, err := os.ReadFile("templates/location_sdk.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{
		"public PocLocationSDKCamera camera;", "LocationSDKValidCamera(authoredCamera)",
		"LocationSDKCameraBasis(authoredCamera", "locationSDKCameraBounds.Encapsulate(renderer.bounds)",
		"locationSDKFocus=locationSDKCameraBounds.center", "locationSDKCameraBounds.extents",
		"locationSDKFocus+locationSDKCameraOffset*locationSDKDistance",
		"locationSDKCamera.transform.LookAt(locationSDKFocus)",
		"Vector3 right=locationSDKImageRight", "direction.x*locationSDKImageRight.x+direction.y*locationSDKImageRight.z",
		"locationSDKCamera.ViewportPointToRay", "locationSDKCamera.WorldToViewportPoint",
		"if(camera==null)locationSDKFit=LocationSDKCameraFit(",
		"locationSDKFollowHero=false;locationSDKFocus+=before-after", "heroHeight>0&&!locationSDKCameraZoomInitialized",
		"LocationSDKCellCenter(shownPosition.x", "lo=locationSDKZoomOrigin*.55f,hi=locationSDKZoomOrigin*3f",
		// Movement v2 (document 86): a story frame zooms between 3/4 and 4/3 of its authored height (12 m -> 9..16 m).
		"lo=locationSDKZoomOrigin*.75f;hi=locationSDKZoomOrigin*4f/3f",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("shared camera contract missing %q", required)
		}
	}
	proof, err := os.ReadFile("templates/location_sdk_proof.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"camera_json_verified=locationSDKProofCameraJSONVerified", "locationSDKProofCameraJSONVerified=LocationSDKProofCameraJSON()", "JsonUtility.FromJson<PocLocationSDKLevel>"} {
		if !strings.Contains(string(proof), required) {
			t.Fatalf("native JsonUtility regression not enforced: %s", required)
		}
	}
	actor := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  void LocationSDKUpdateActor(", "  // URP invokes IMGUI")
	if strings.Contains(actor, "new Vector3(.7071068f") {
		t.Fatal("actor billboard still fixes the old azimuth")
	}
}

func TestLocationSDKAuthoredCameraProjectionNative(t *testing.T) {
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
	dto := sceneTemplateSection(t, "location_sdk.cs.tmpl", " [Serializable] public sealed class PocLocationSDKCell ", " public sealed partial class AngelsDemonsPoc ")
	helpers := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  static bool LocationSDKValidCamera(", "  static bool LocationSDKWithinBudget(")
	harness := `using System;` + dto + `class Proof {` + helpers + `
 static void Require(bool ok,string why){if(!ok)throw new Exception(why);}
 static void Main(){
  var legacy=new PocLocationSDKLevel();Require(legacy.GetCamera()==null,"missing legacy camera lost default");
  legacy.camera=new PocLocationSDKCamera();Require(legacy.GetCamera()==null,"JsonUtility empty-object materialization lost default");
  legacy.camera_authored=true;Require(!LocationSDKValidCamera(legacy.GetCamera()),"explicit empty authored camera accepted");
  legacy.camera=null;bool rejected=false;try{legacy.GetCamera();}catch(InvalidOperationException){rejected=true;}Require(rejected,"authored null camera accepted");
  legacy.camera_authored=false;legacy.camera=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=35,azimuth_degrees=20};rejected=false;try{legacy.GetCamera();}catch(InvalidOperationException){rejected=true;}Require(rejected,"unmarked nonempty camera silently accepted");
  legacy.camera_authored=true;Require(LocationSDKValidCamera(legacy.GetCamera()),"marked valid camera rejected");
  float x,y,z,rx,rz;LocationSDKCameraBasis(null,out x,out y,out z,out rx,out rz);
  Require(x==.61237246f&&y==.5f&&z==.61237246f&&rx==.7071068f&&rz==-.7071068f,"legacy camera basis changed");
  Require(LocationSDKValidCamera(null),"absent camera rejected");
  foreach(float e in new[]{15f,30f,75f})foreach(float a in new[]{5f,20f,45f,85f}){
   var camera=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=e,azimuth_degrees=a};
   Require(LocationSDKValidCamera(camera),"supported camera rejected");LocationSDKCameraBasis(camera,out x,out y,out z,out rx,out rz);
   double er=e*Math.PI/180,ar=a*Math.PI/180;
   Require(Math.Abs(x*x+y*y+z*z-1)<.000001&&Math.Abs(rx*rx+rz*rz-1)<.000001&&Math.Abs(x*rx+z*rz)<.000001,"camera/right basis not orthonormal");
   Require(Math.Abs(Math.Atan2(x,z)*180/Math.PI-a)<.00001&&Math.Abs(Math.Asin(y)*180/Math.PI-e)<.00001,"authored degrees changed axis convention");
   foreach(float hx in new[]{.25f,18f,1024f})foreach(float hy in new[]{.9f,10f,1000f})foreach(float hz in new[]{.25f,13.5f,1024f})foreach(float aspect in new[]{.1f,.7f,2.1f,5f}){
    float distance,near,far;float fit=LocationSDKProjectedFit(hx,hy,hz,e,a,aspect,out distance,out near,out far);
    foreach(int ix in new[]{-1,1})foreach(int iy in new[]{-1,1})foreach(int iz in new[]{-1,1}){
     double dx=ix*(double)hx,dy=iy*(double)hy,dz=iz*(double)hz;
     double sx=dx*Math.Cos(ar)-dz*Math.Sin(ar),sy=-dx*Math.Sin(er)*Math.Sin(ar)+dy*Math.Cos(er)-dz*Math.Sin(er)*Math.Cos(ar);
     double depth=distance-dx*x-dy*y-dz*z;
     Require(Math.Abs(sx)<fit*aspect&&Math.Abs(sy)<fit,"authored camera crops bounds corner");Require(depth>near&&depth<far,"authored camera clips depth");
    }
   }
   // Project an arbitrary ground point and recover it by intersecting its
   // orthographic ray with the floor. This also covers nonzero floor datums.
   foreach(double elevation in new[]{-500000d,0d,500000d}){
    double px=12.25,pz=6.75,py=elevation,cx=9,cy=elevation+2,cz=7;
    double ux=-Math.Sin(er)*Math.Sin(ar),uy=Math.Cos(er),uz=-Math.Sin(er)*Math.Cos(ar);
    double sx=(px-cx)*rx+(pz-cz)*rz,sy=(px-cx)*ux+(py-cy)*uy+(pz-cz)*uz;
    double ox=cx+rx*sx+ux*sy+x*100,oy=cy+uy*sy+y*100,oz=cz+rz*sx+uz*sy+z*100;
    double travel=(oy-elevation)/y;
    Require(Math.Abs(ox-x*travel-px)<.00001&&Math.Abs(oz-z*travel-pz)<.00001,"ground pointer roundtrip drift");
   }
  }
  foreach(float bad in new[]{float.NaN,float.PositiveInfinity,float.NegativeInfinity,0f,90f}){
   Require(!LocationSDKValidCamera(new PocLocationSDKCamera{projection="orthographic",elevation_degrees=bad,azimuth_degrees=20}),"invalid elevation accepted");
   Require(!LocationSDKValidCamera(new PocLocationSDKCamera{projection="orthographic",elevation_degrees=30,azimuth_degrees=bad}),"invalid azimuth accepted");
  }
  Require(!LocationSDKValidCamera(new PocLocationSDKCamera{projection="perspective",elevation_degrees=30,azimuth_degrees=20}),"unsupported projection accepted");
  var framing=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=35,azimuth_degrees=37,hero_view_height_m=15};
  Require(LocationSDKValidCamera(framing)&&LocationSDKHeroViewHeight(framing,true)==15&&LocationSDKHeroViewHeight(framing,false)==0&&LocationSDKHeroViewHeight(null,true)==0,"story framing leaked into workbench/legacy");
  foreach(float fit in new[]{3f,30f,300f}){float initial=fit/(15f*.5f);Require(Math.Abs(LocationSDKCameraViewSize(fit,15,initial,initial)*2-15)<.0001f,"initial camera height depends on layout size");Require(Math.Abs(LocationSDKCameraViewSize(fit*2,15,initial,initial)*2-15)<.0001f,"aspect/fit recalculation resets story framing");Require(Math.Abs(LocationSDKCameraViewSize(fit,15,initial*2,initial)*2-7.5f)<.0001f,"manual zoom lost");Require(LocationSDKCameraViewSize(fit,0,2,1)==fit/2,"legacy/workbench fit changed");}
  foreach(float bad in new[]{-1f,5.99f,60.01f,float.NaN,float.PositiveInfinity}){framing.hero_view_height_m=bad;Require(!LocationSDKValidCamera(framing),"invalid story framing accepted");}
  Console.WriteLine("authored camera basis, bounds, clipping, ground projection and story framing PASS");
 }}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "Camera.cs"), filepath.Join(dir, "Camera.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile exact camera helpers: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("camera contract: %v\n%s", err, out)
	}
}
