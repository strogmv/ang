package gamepoc

import (
	"strings"
	"testing"
)

func TestSanitaryCutawayPublicOnlyAndRestoreNative(t *testing.T) {
	counterFields := sceneTemplateSection(t, "sanitary_services_proof.cs.tmpl", "  int sanitaryReviewPrivateMaterials", "  readonly Dictionary<string,GameObject> sanitaryReviewBodies")
	counter := sceneTemplateSection(t, "sanitary_services_proof.cs.tmpl", "  bool SanitaryReviewCutawayPassed()", "  void SanitaryProofAfterFrame()")
	helper := sceneTemplateSection(t, "sanitary_services.cs.tmpl", "  Transform sanitaryCutawayInstance;", "  SanitaryVisual SanitaryVisualFor(")
	engine := sceneTemplateSection(t, "location_sdk_multideck.cs.tmpl", "  void LocationSDKUpdateInstanceCutaway(", "  // Ease of the dithered fade:")
	runPaintedTextureNative(t, "SanitaryPublicCutaway", `using System;using System.Collections.Generic;
namespace UnityEngine.Rendering {public enum ShadowCastingMode{Off,On}}
class Vector3 {
 public float x,y,z;public Vector3(float a=0,float b=0,float c=0){x=a;y=b;z=c;}
 public static Vector3 up=new Vector3(0,1,0);public float sqrMagnitude{get{return x*x+y*y+z*z;}}
 public Vector3 normalized{get{float n=(float)Math.Sqrt(sqrMagnitude);return new Vector3(x/n,y/n,z/n);}}
 public static Vector3 operator +(Vector3 a,Vector3 b){return new Vector3(a.x+b.x,a.y+b.y,a.z+b.z);}
 public static Vector3 operator -(Vector3 a,Vector3 b){return new Vector3(a.x-b.x,a.y-b.y,a.z-b.z);}
 public static Vector3 operator *(Vector3 a,float n){return new Vector3(a.x*n,a.y*n,a.z*n);}
 public static float Dot(Vector3 a,Vector3 b){return a.x*b.x+a.y*b.y+a.z*b.z;}
}
class Ray{public Vector3 origin,direction;public Ray(Vector3 p,Vector3 d){origin=p;direction=d;}}
struct Bounds {
 public Vector3 min,max;public Bounds(Vector3 lo,Vector3 hi){min=lo;max=hi;}
 public bool Contains(Vector3 p){return p.x>=min.x&&p.x<=max.x&&p.y>=min.y&&p.y<=max.y&&p.z>=min.z&&p.z<=max.z;}
 public void Encapsulate(Bounds b){min=new Vector3(Math.Min(min.x,b.min.x),Math.Min(min.y,b.min.y),Math.Min(min.z,b.min.z));max=new Vector3(Math.Max(max.x,b.max.x),Math.Max(max.y,b.max.y),Math.Max(max.z,b.max.z));}
 public bool IntersectRay(Ray ray){float d;return IntersectRay(ray,out d);}
 public bool IntersectRay(Ray ray,out float distance){float lo=0,hi=float.PositiveInfinity;float[] p={ray.origin.x,ray.origin.y,ray.origin.z},v={ray.direction.x,ray.direction.y,ray.direction.z},a={min.x,min.y,min.z},b={max.x,max.y,max.z};
  for(int i=0;i<3;i++){if(Math.Abs(v[i])<.000001){if(p[i]<a[i]||p[i]>b[i]){distance=0;return false;}}else{float u=(a[i]-p[i])/v[i],w=(b[i]-p[i])/v[i];lo=Math.Max(lo,Math.Min(u,w));hi=Math.Min(hi,Math.Max(u,w));if(hi<lo){distance=0;return false;}}}distance=lo;return true;}
}
class Transform {
 public string name;public GameObject gameObject;public Transform parent;public Vector3 position=new Vector3();public List<Transform> children=new List<Transform>();public List<Renderer> renderers=new List<Renderer>();
 public bool IsChildOf(Transform p){for(var n=this;n!=null;n=n.parent)if(n==p)return true;return false;}
 public Vector3 TransformDirection(Vector3 v){return v;}public Vector3 TransformPoint(Vector3 v){return v+position;}
 public T[] GetComponentsInChildren<T>(bool include){var values=new List<T>();if(typeof(T)==typeof(Transform))values.Add((T)(object)this);if(typeof(T)==typeof(Renderer))foreach(var r in renderers)values.Add((T)(object)r);foreach(var c in children)values.AddRange(c.GetComponentsInChildren<T>(include));return values.ToArray();}
}
class GameObject {
 public Transform transform;public bool activeInHierarchy=true;public GameObject(string n){transform=new Transform{name=n,gameObject=this};}
 public T[] GetComponentsInChildren<T>(bool include){return transform.GetComponentsInChildren<T>(include);}
}
class Shader{public string name="Opaque";}
class Material{public Shader shader=new Shader();}
class Renderer{public Transform transform;public Bounds bounds;public bool enabled=true;public UnityEngine.Rendering.ShadowCastingMode shadowCastingMode=UnityEngine.Rendering.ShadowCastingMode.On;public Material[] sharedMaterials;public Material sharedMaterial{get{return sharedMaterials[0];}}}
class Camera{public Transform transform=new GameObject("camera").transform;}
static class Time{public static float unscaledTime=42;}
class Group{public string policy;public string[] instances;}
class Navigation{public Group[] cutaway_groups=new Group[0];}
class Override{public string instance;public GameObject root;}
class Unit{public string instance_id;}
class Snapshot{public Unit[] units;}
class LocationSDKFadeRestore{public Material[] materials;public UnityEngine.Rendering.ShadowCastingMode shadows;}
class Proof {
 Navigation locationSDKNav=new Navigation();GameObject locationSDKRoot=new GameObject("SDK");Camera locationSDKCamera=new Camera();
 Vector3 locationSDKCameraOffset=new Vector3(.493f,.574f,.654f).normalized;
 List<Override> locationSDKOverrideVisuals=new List<Override>();Dictionary<GameObject,Bounds> locationSDKInstanceBounds=new Dictionary<GameObject,Bounds>();
 HashSet<GameObject> locationSDKInstanceFaded=new HashSet<GameObject>();Dictionary<Renderer,LocationSDKFadeRestore> locationSDKFadeOriginal=new Dictionary<Renderer,LocationSDKFadeRestore>();
 List<Renderer> locationSDKInstanceHiddenText=new List<Renderer>();Dictionary<Renderer,float> locationSDKInstanceFadeStart=new Dictionary<Renderer,float>();
 Snapshot sanitarySnapshot=new Snapshot();bool readable=true;string failure;Material dither=new Material();
 bool SanitaryRead(){return readable;}void SanitaryFail(string detail){failure=detail;}Material LocationSDKInstanceDitherFor(Material source){return dither;}
`+helper+engine+counterFields+counter+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static Transform Add(Transform parent,string name){var t=new GameObject(name).transform;t.parent=parent;parent.children.Add(t);return t;}
 static Renderer Mesh(Transform t,Vector3 lo,Vector3 hi){var r=new Renderer{transform=t,bounds=new Bounds(lo,hi),sharedMaterials=new[]{new Material()}};t.renderers.Add(r);return r;}
 static void MustReject(Action act,string why){bool rejected=false;try{act();}catch(InvalidOperationException){rejected=true;}Need(rejected,why);}
 public static void Main(){
  var p=new Proof();p.dither.shader.name="ANG/LocationSDKInstanceDither";p.locationSDKCamera.transform.position=new Vector3(30,30,55);var block=Add(p.locationSDKRoot.transform,"sanitary.block.v1");
  var front=Add(block,"shell.cutaway.front");var roof=Add(block,"shell.cutaway.corridor.roof");
  var frontMesh=Mesh(front,new Vector3(5.4f,0,20.5f),new Vector3(5.5f,2.4f,28));var roofMesh=Mesh(roof,new Vector3(3.5f,2.3f,21.5f),new Vector3(5.44f,2.4f,28));
  var privateRoots=new List<Transform>();var privateMeshes=new List<Renderer>();var originals=new List<Material[]>();
  foreach(var name in new[]{"unit.wc.0","unit.wc.1","unit.wc.2","unit.wc.3","unit.shower.6"}){var t=Add(block,name);privateRoots.Add(t);var r=Mesh(t,new Vector3(1.5f,0,20.5f),new Vector3(3.5f,2.4f,28));privateMeshes.Add(r);originals.Add(r.sharedMaterials);}
  var originalFront=frontMesh.sharedMaterials;var originalRoof=roofMesh.sharedMaterials;
  p.locationSDKOverrideVisuals.Add(new Override{instance="wall.sanitary",root=block.gameObject});var units=new[]{new Unit{instance_id="wall.sanitary"}};p.sanitarySnapshot.units=units;
  var inside=new Vector3(4.25f,0,24);p.LocationSDKUpdateInstanceCutaway(inside);
  p.SanitaryReviewCutaway();Need(!p.SanitaryReviewCutawayPassed()&&p.sanitaryReviewPrivateDither==0&&p.sanitaryReviewPrivateMaterials==5,"native counters accepted unrestored shell or lost actual private slots");
  Need(p.failure==null&&p.locationSDKInstanceFaded.Count==2,"PUBLIC shell not selected for actual camera obstruction");
  Need(frontMesh.sharedMaterials[0]==p.dither&&roofMesh.sharedMaterials[0]==p.dither,"existing SDK material switch was not used");
  Need(p.locationSDKInstanceFadeStart[roofMesh]==42,"existing SDK fade clock was not initialized");
  for(int i=0;i<privateMeshes.Count;i++)Need(privateMeshes[i].sharedMaterials==originals[i]&&privateMeshes[i].enabled&&privateMeshes[i].shadowCastingMode==UnityEngine.Rendering.ShadowCastingMode.On,"private cabin leaked into dissolve");
  Need(p.sanitarySnapshot.units==units&&units[0].instance_id=="wall.sanitary","camera presentation changed server DTO");
  p.LocationSDKUpdateInstanceCutaway(new Vector3(7,0,24));Need(p.locationSDKInstanceFaded.Count==0&&frontMesh.sharedMaterials==originalFront&&roofMesh.sharedMaterials==originalRoof,"exterior retained cutaway/material replacement");
  p.SanitaryReviewCutaway();Need(p.SanitaryReviewCutawayPassed(),"actual native PUBLIC fade/restore counters did not pass");p.SanitaryReviewCutaway();foreach(var row in p.sanitaryReviewPublic)Need(row.faded_samples==1&&row.restored_transitions==1,"restore counted unchanged frames instead of transitions");
  privateMeshes[0].sharedMaterials=new[]{p.dither};p.SanitaryReviewCutaway();Need(p.sanitaryReviewPrivateDither==1&&!p.SanitaryReviewCutawayPassed(),"actual PRIVATE dither did not fail native gate");privateMeshes[0].sharedMaterials=originals[0];
  Need(frontMesh.shadowCastingMode==UnityEngine.Rendering.ShadowCastingMode.On&&p.locationSDKInstanceFadeStart.Count==0,"restoration lost shadows or animation state");
  p.LocationSDKUpdateInstanceCutaway(inside);p.readable=false;p.LocationSDKUpdateInstanceCutaway(inside);Need(p.locationSDKInstanceFaded.Count==0&&roofMesh.sharedMaterials==originalRoof,"lost snapshot left faded shell");p.readable=true;
  p.locationSDKCamera.transform.position=new Vector3(4.26f,1.71f,24.01f);p.LocationSDKUpdateInstanceCutaway(inside);Need(p.locationSDKInstanceFaded.Count==0,"geometry beyond camera incorrectly faded");
  p.locationSDKCamera.transform.position=new Vector3(30,30,55);p.LocationSDKUpdateInstanceCutaway(inside);p.sanitarySnapshot.units=new Unit[0];p.LocationSDKUpdateInstanceCutaway(inside);Need(p.locationSDKInstanceFaded.Count==0,"empty public view failed to restore shell");
  var wc=privateRoots[0];block.children.Remove(wc);front.children.Add(wc);wc.parent=front;MustReject(()=>SanitaryPublicCutawayRoots(block),"private descendant allowed");front.children.Remove(wc);block.children.Add(wc);wc.parent=block;
  block.children.Remove(front);wc.children.Add(front);front.parent=wc;MustReject(()=>SanitaryPublicCutawayRoots(block),"public selection inside private cabin allowed");wc.children.Remove(front);block.children.Add(front);front.parent=block;
  var duplicate=Add(block,"shell.cutaway.front");MustReject(()=>SanitaryPublicCutawayRoots(block),"duplicate public node allowed");block.children.Remove(duplicate);
  roof.name="shell.cutaway.wrong";MustReject(()=>SanitaryPublicCutawayRoots(block),"missing node silently accepted");
  Console.WriteLine("actual SDK dissolve and exact restore, bounded hero-camera rays, PRIVATE hierarchy exclusion and unchanged DTO PASS");
 }
}`)
}

func TestSanitaryCutawayPresentationOnly(t *testing.T) {
	s := sceneTemplateSection(t, "sanitary_services.cs.tmpl", "  Transform sanitaryCutawayInstance;", "  SanitaryVisual SanitaryVisualFor(")
	for _, forbidden := range []string{"SetActive(", "session.", "SendLifeCommand", "SanitarySend(", "state.", "Time.", "unit.wc.\"}"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("cutaway owns forbidden simulation/visibility operation %q", forbidden)
		}
	}
	engine := hudTemplate(t, "location_sdk_multideck.cs.tmpl")
	if !strings.Contains(engine, "InstanceDitherCoverage=.38f,InstanceDitherSeconds=.3f") || strings.Count(engine, "SanitaryAppendPublicCutaway(feet,members,fade);") != 1 {
		t.Fatal("sanitary cutaway must reuse one existing SDK dissolve/restore pass")
	}
}

func TestSanitaryWCCycleProofGateNative(t *testing.T) {
	gate := sceneTemplateSection(t, "sanitary_services_proof.cs.tmpl", "  bool SanitaryReviewWCPassed()", "  bool SanitaryReviewNodeInViewport(")
	runPaintedTextureNative(t, "SanitaryWCGate", `using System;
class Row{public int restored_transitions=2;}
class Proof{
 bool sanitaryReviewWCEntry=true,sanitaryReviewWCExit=true,sanitaryReviewWCFlush=true;
 long sanitaryReviewWCDuration=240,sanitaryReviewWCUse=240;int sanitaryReviewWCDoorSamples=3,sanitaryReviewWCFoldSamples=3;
 int[] sanitaryReviewWCRestoreBefore=new[]{1,1};Row[] sanitaryReviewPublic=new[]{new Row(),new Row()};
`+gate+`
 static void Need(bool b,string m){if(!b)throw new Exception(m);}
 public static void Main(){var p=new Proof();Need(p.SanitaryReviewWCPassed(),"complete WC evidence rejected");
 p.sanitaryReviewWCEntry=false;Need(!p.SanitaryReviewWCPassed(),"entry not required");p.sanitaryReviewWCEntry=true;
 p.sanitaryReviewWCExit=false;Need(!p.SanitaryReviewWCPassed(),"exit not required");p.sanitaryReviewWCExit=true;
 p.sanitaryReviewWCFlush=false;Need(!p.SanitaryReviewWCPassed(),"actual 3 L flush not required");p.sanitaryReviewWCFlush=true;
 p.sanitaryReviewWCUse=239;Need(!p.SanitaryReviewWCPassed(),"partial use accepted");p.sanitaryReviewWCUse=240;
 p.sanitaryReviewWCDoorSamples=0;Need(!p.SanitaryReviewWCPassed(),"primary door viewport motion not required");p.sanitaryReviewWCDoorSamples=3;
 p.sanitaryReviewWCFoldSamples=0;Need(!p.SanitaryReviewWCPassed(),"fold viewport motion not required");p.sanitaryReviewWCFoldSamples=3;
 p.sanitaryReviewPublic[0].restored_transitions=1;Need(!p.SanitaryReviewWCPassed(),"old shower restoration accepted as WC restoration");p.sanitaryReviewPublic[0].restored_transitions=2;
 p.sanitaryReviewPublic[1].restored_transitions=1;Need(!p.SanitaryReviewWCPassed(),"second PUBLIC root not restored");
 Console.WriteLine("WC proof requires physical entry/exit, whole service duration, 3 L flush, both moving visible panels and new PUBLIC restoration PASS");}
}`)
}
