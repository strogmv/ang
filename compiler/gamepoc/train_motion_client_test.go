package gamepoc

import (
	"strings"
	"testing"
)

func TestTrainMotionPhysicalClockAndWheelSamplingNative(t *testing.T) {
	lead := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static double TrainMotionLead(", "  static Vector3 TrainMotionDirection(")
	wheels := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static double TrainWheelDegrees(", "  static PocTrainMotionBinding TrainBinding(")
	runPaintedTextureNative(t, "TrainMotionClock", `using System;
class PocTrainMotionSnapshot {public string clock_domain="simulation";public long sample_sim_ns=10000000000L;public double clock_seconds=10;}
class PocTrainMotionState {public double next_event_seconds=11;}
class Proof {`+lead+wheels+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static void Near(double actual,double expected,string why){Need(Math.Abs(actual-expected)<.00001,why+" actual="+actual);}
 public static void Main(){var snapshot=new PocTrainMotionSnapshot();var train=new PocTrainMotionState();
 Near(TrainMotionLead(snapshot,train,10500000000d,false,.2),.2,"presentation exceeded configured horizon");
 Near(TrainMotionLead(snapshot,train,10500000000d,true,1),0,"paused train advanced");
 Near(TrainMotionLead(snapshot,train,9000000000d,false,1),0,"stale sample rewound train");
 train.next_event_seconds=10.05;Need(TrainMotionLead(snapshot,train,11000000000d,false,2)<.05,"presentation crossed the server event");
 Near(TrainMotionHorizon(250,4),1,"speed multiplier must scale the physical lead horizon");Near(TrainMotionHorizon(250,1),.25,"ordinary horizon changed");Near(TrainMotionHorizon(-1,4),0,"negative horizon was not bounded");
 snapshot.clock_domain="calendar";Near(TrainMotionLead(snapshot,train,11000000000d,false,2),0,"calendar acceleration drove physical train");
 Near(TrainWheelDegrees(.32*Math.PI,.32),180,"load wheel lost no-slip travel/radius");Near(TrainWheelDegrees(.2*Math.PI*2,.2),0,"guide wheel did not complete one revolution");Near(TrainWheelDegrees(double.NaN,.32),0,"nonfinite travel reached rotations");
 Console.WriteLine("physical clock / pause / horizon / event boundary / wheels PASS");
 }} `)
}

func TestTrainMotionPaintedPathExtendsWithoutClockOrGeometryStretchNative(t *testing.T) {
	path := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static Vector2 TrainPaintedPoint(", "  static double TrainPaintedPixelsPerMetre(")
	runPaintedTextureNative(t, "TrainMotionProjection", `using System;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public static Vector2 zero=>new Vector2(0,0);}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
class Proof {`+path+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static void At(Vector2 p,float x,float y){Need(Math.Abs(p.x-x)<.0001&&Math.Abs(p.y-y)<.0001,"bad projected point "+p.x+","+p.y);}
 public static void Main(){var p=new[]{new PocPaintedPoint(0,10),new PocPaintedPoint(30,10),new PocPaintedPoint(30,50)};
 At(TrainPaintedPoint(p,0),0,10);At(TrainPaintedPoint(p,3d/7),30,10);At(TrainPaintedPoint(p,1),30,50);At(TrainPaintedPoint(p,-1d/7),-10,10);At(TrainPaintedPoint(p,8d/7),30,60);
 Need(TrainPaintedPathLength(p)==70,"path length changed its authored metric scale");At(TrainPaintedPoint(null,.5),0,0);Console.WriteLine("anchor follows authored polyline; trailing cars can extend beyond endpoint PASS");
 }} `)
}

func TestTrainMotionPaintedMetricScaleAndFixedDatumNative(t *testing.T) {
	metric := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static Vector2 TrainPaintedPoint(", "  static void TrainPaintedBindMaterial(")
	runPaintedTextureNative(t, "TrainPaintedMetres", `using System;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public static Vector2 zero=>new Vector2(0,0);}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
class PocPaintedScene {public PocPaintedPoint axis_x,axis_y;}
class PocPaintedRailMetric {public double cell_meters,track_m,path_progress;}
class PocAmbientPassage {public PocPaintedPoint[] path;public PocPaintedRailMetric metric_rail;}
class Proof {`+metric+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 static void Near(double actual,double expected,string why){Need(Math.Abs(actual-expected)<.003,why+" actual="+actual+" expected="+expected);}
 static double Length(Vector2 a,Vector2 b){return Math.Sqrt(Math.Pow(a.x-b.x,2)+Math.Pow(a.y-b.y,2));}
 static Vector2 At(PocPaintedScene s,PocAmbientPassage p,double metres){return TrainPaintedPoint(p.path,TrainPaintedMetricProgress(s,p,metres));}
 public static void Main(){
 var station=new PocPaintedScene{axis_x=new PocPaintedPoint(61.244f,-3.982f),axis_y=new PocPaintedPoint(9.187f,42.878f)};
 var rail=new PocAmbientPassage{path=new[]{new PocPaintedPoint(-2000,1077.9907893721f),new PocPaintedPoint(3267.751196172249f,729.0017717331f)},metric_rail=new PocPaintedRailMetric{cell_meters=1,track_m=29.9,path_progress=.5}};
 Near(TrainPaintedPixelsPerMetre(station,rail)*13.6,834.52584,"station car length not calibrated to source floor metres");
 var datum=At(station,rail,29.9);Near(datum.x,633.875598,"station fixed berth datum X");Near(datum.y,903.49628,"station fixed berth datum Y");Near(Length(At(station,rail,29.9+13.6),datum),834.52584,"station physical car stretched in placement");Near(Length(At(station,rail,29.9+3),datum),184.08658,"three metres/second became canvas-window speed");
 var hub=new PocPaintedScene{axis_x=new PocPaintedPoint(24,26),axis_y=new PocPaintedPoint(-42,10)};
 var freight=new PocAmbientPassage{path=new[]{new PocPaintedPoint(-500,1008.936f),new PocPaintedPoint(2172,481.329f)},metric_rail=new PocPaintedRailMetric{cell_meters=1,track_m=29.9,path_progress=.5}};
 Near(TrainPaintedPixelsPerMetre(hub,freight)*13.6,599.77836,"hub car length not calibrated to original floor metres");var f=At(hub,freight,29.9);Near(f.x,836,"hub fixed lower-rail datum X");Near(f.y,745.1325,"hub fixed lower-rail datum Y");Near(Length(At(hub,freight,43.5),f),599.77836,"hub physical car stretched in placement");Near(Length(At(hub,freight,32.9),f),132.30405,"hub physical speed changed");
 var housing=new PocPaintedScene{axis_x=new PocPaintedPoint(31.35f,23.51f),axis_y=new PocPaintedPoint(-31.35f,13.06f)};
 var lower=new PocAmbientPassage{path=new[]{new PocPaintedPoint(-500,1325),new PocPaintedPoint(2172,431)},metric_rail=new PocPaintedRailMetric{cell_meters=1,track_m=60,path_progress=.5}};
 Near(TrainPaintedPixelsPerMetre(housing,lower)*13.6,482.21123,"housing rail direction must use both floor axes");var h=At(housing,lower,60);Near(h.x,836,"housing fixed datum X");Near(h.y,878,"housing fixed datum Y");Near(Length(At(housing,lower,73.6),h),482.21123,"housing physical car length changed");Near(Length(At(housing,lower,63),h),106.37012,"housing physical speed changed");
 lower.metric_rail.cell_meters=.5;Near(TrainPaintedPixelsPerMetre(housing,lower)*13.6,964.42246,"logical half-metre cell lost physical conversion");lower.metric_rail=null;bool refused=false;try{TrainPaintedPixelsPerMetre(housing,lower);}catch(InvalidOperationException){refused=true;}Need(refused,"missing live metric datum silently invented a scale");Console.WriteLine("13.6m hub599.78px/housing482.21px (legacy station projection regression834.53px); fixed anchors, metres/sec, cell scale and visibility independence PASS");
 }} `)
	adapter := hudTemplate(t, "train_motion_painted.cs.tmpl")
	if strings.Contains(adapter, "route_start_m") || strings.Contains(adapter, "route_end_m") || !strings.Contains(adapter, "Vector3.Dot(trainPaintedAnchor,TrainMotionDirection(anchorBinding.yaw_degrees))") {
		t.Fatal("painted metric position/scale must use physical server pose and floor calibration, never a visibility interval")
	}
}

func TestTrainMotionNativeHooksAndSingleAuthority(t *testing.T) {
	core := hudTemplate(t, "train_motion.cs.tmpl")
	for _, need := range []string{"state.Life.TrainMotionJson", "trainMotionSnapshot.epoch==state.Epoch", "train.prefab_resource", "module.node.SetPositionAndRotation", "bogies=new[]{east,west}", "TrainWheelDegrees(travel,wheel.radius)", "foreach(var door in module.doors)door.node.localPosition=door.rest"} {
		if !strings.Contains(core, need) {
			t.Errorf("missing live train integration %q", need)
		}
	}
	if strings.Contains(core, "Time.deltaTime") || strings.Contains(core, "Time.unscaledDeltaTime") {
		t.Error("local accumulated timer drives train")
	}
	sdk := hudTemplate(t, "location_sdk.cs.tmpl")
	if strings.Index(sdk, "UpdateTrainMotionSDK();") > strings.Index(sdk, "LocationSDKRenderNow();") || !strings.Contains(sdk, "UpdateTrainMotionSDK();") {
		t.Error("train pose must be applied before the world render")
	}
	painted := hudTemplate(t, "painted.cs.tmpl")
	if !strings.Contains(painted, "DrawPaintedAmbient(scene,fit,scale,viewport,visibleCanvas);DrawPaintedTrainMotion(scene,fit,scale,viewport,visibleCanvas);GUI.DrawTexture(localFit,cache.fog") {
		t.Error("live projection must use observer clip and precede ground fog")
	}
}

func TestTrainMotionPaintedBindsFullObserverProjectionNative(t *testing.T) {
	bind := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static void TrainPaintedBindMaterial(", "  void TrainPaintedEnsure(")
	runPaintedTextureNative(t, "TrainPaintedObserverBinding", `using System;using System.Collections.Generic;
struct Vector4 {public float x,y,z,w;public Vector4(float a,float b,float c,float d){x=a;y=b;z=c;w=d;}public static Vector4 zero=>new Vector4(0,0,0,0);}
struct Rect {public float x,y,width,height;public Rect(float a,float b,float c,float d){x=a;y=b;width=c;height=d;}}
class Texture2D {}class RenderTexture {}
class Material {public readonly Dictionary<string,object> values=new Dictionary<string,object>();public void SetTexture(string k,object v){values[k]=v;}public void SetFloat(string k,float v){values[k]=v;}public void SetVector(string k,Vector4 v){values[k]=v;}}
class PaintedAmbientCache {public Texture2D mask=new Texture2D(),observation=new Texture2D();public byte[] observationPixels=new byte[7];public Rect bounds=new Rect(350,420,1322,521);}
class Proof {`+bind+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var material=new Material();var passage=new PaintedAmbientCache();var target=new RenderTexture();
 TrainPaintedBindMaterial(material,passage,target);
 Need(Object.ReferenceEquals(material.values["_MaskTex"],passage.mask),"clip mask was not explicitly retained");Need(Object.ReferenceEquals(material.values["_ObservationTex"],passage.observation),"current observer texture was not explicitly retained");Need((float)material.values["_ObservationWidth"]==7,"nearest platform lookup has zero/wrong width");
 var layer=(Vector4)material.values["_LayerRect"];Need(layer.x==350&&layer.y==420&&layer.z==1322&&layer.w==521,"CBUFFER-only canvas bounds were lost through material clone");
 Need(Object.ReferenceEquals(material.values["_TrainTex"],target),"actual 3D target is not sampled");var uv=(Vector4)material.values["_TrainUV"];Need(uv.x==0&&uv.y==0&&uv.z==1&&uv.w==1,"offscreen UV was not bound");Need((float)material.values["_TrackEnabled"]==0,"train layer duplicates underlying track");Need((float)material.values["_WholeModuleObservation"]==1,"live 3D cars retained per-pixel observation slicing");Console.WriteLine("explicit actual RT, layer bounds and unchanged observer/mask binding PASS");
 }} `)
}

func TestTrainMotionExternalAmbientPreservesOtherRendererPropertiesNative(t *testing.T) {
	apply := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static void TrainMotionApplyAmbient(", "  void TrainMotionUpdateAmbient(")
	runPaintedTextureNative(t, "TrainExternalAmbient", `using System;
namespace UnityEngine.Rendering {public struct SphericalHarmonicsL2 {public float day;}public enum LightProbeUsage {BlendProbes,CustomProvided}}
class MaterialPropertyBlock {public float untouched;public UnityEngine.Rendering.SphericalHarmonicsL2[] probe;public void CopySHCoefficientArraysFrom(UnityEngine.Rendering.SphericalHarmonicsL2[] value){probe=(UnityEngine.Rendering.SphericalHarmonicsL2[])value.Clone();}}
class Renderer {public int lightmapIndex=7;public UnityEngine.Rendering.LightProbeUsage lightProbeUsage;public float existing=.73f;public MaterialPropertyBlock assigned;public void GetPropertyBlock(MaterialPropertyBlock block){block.untouched=existing;}public void SetPropertyBlock(MaterialPropertyBlock block){assigned=block;}}
class Proof {`+apply+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var renderer=new Renderer();var block=new MaterialPropertyBlock();var source=new[]{new UnityEngine.Rendering.SphericalHarmonicsL2{day=.4f}};
 TrainMotionApplyAmbient(renderer,source,block);Need(renderer.lightProbeUsage==UnityEngine.Rendering.LightProbeUsage.CustomProvided&&renderer.lightmapIndex==-1,"interior lightmap/probe remains owner of train");Need(renderer.assigned.untouched==.73f,"ambient assignment erased another visual property");Need(renderer.assigned.probe[0].day==.4f,"location ambient not copied into actual renderer block");source[0].day=.1f;TrainMotionApplyAmbient(renderer,source,block);Need(renderer.assigned.probe[0].day==.1f,"day/night ambient did not update");Console.WriteLine("external receiver uses location SH and preserves existing property block PASS");
 }} `)
	core := hudTemplate(t, "train_motion.cs.tmpl")
	for _, need := range []string{"go.AddComponent<TrainMotionLightingReceiver>()", "TrainMotionUpdateInstances(level,locationSDKRoot.transform,LocationSDKLayer);TrainMotionUpdateAmbient();", "var ambient=RenderSettings.ambientProbe", "train.ambient.Equals(ambient)"} {
		if !strings.Contains(core, need) {
			t.Errorf("missing external ambient owner integration %q", need)
		}
	}
}

func TestTrainMotionMatchesExplicitPaintedPassageNative(t *testing.T) {
	find := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static PocTrainMotionBinding TrainBinding(", "  static Transform TrainChild(")
	runPaintedTextureNative(t, "TrainPassageIdentity", `using System;
class PocTrainMotionBinding {public string level_id,passage_id;}
class PocTrainMotionModule {public PocTrainMotionBinding[] bindings;}
class Proof {`+find+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var freight=new PocTrainMotionBinding{level_id="level.c7_transport_hub",passage_id="hub.lower_freight_rail"};var other=new PocTrainMotionBinding{level_id="level.c7_transport_hub",passage_id="another.public.rail"};var housing=new PocTrainMotionBinding{level_id="level.poc_housing",passage_id="housing.lower_rail"};var module=new PocTrainMotionModule{bindings=new[]{other,freight,housing}};
 Need(Object.ReferenceEquals(TrainBinding(module,"level.c7_transport_hub","hub.lower_freight_rail"),freight),"first passage was chosen instead of explicit physical rail");Need(TrainBinding(module,"level.c7_transport_hub","station.back_rail")==null,"whole-level match suppressed another rail");Need(TrainBinding(module,"level.poc_monorail","station.back_rail")==null,"56m stock claims 28m passenger boarding station");Need(Object.ReferenceEquals(TrainBinding(module,"level.poc_housing","housing.lower_rail"),housing),"housing rail binding lost");Console.WriteLine("exact passage identity; independent rail and legacy boarding retained PASS");
 }} `)
	adapter := hudTemplate(t, "train_motion_painted.cs.tmpl")
	if !strings.Contains(adapter, "TrainBinding(module,scene.level_id,candidate.passage.id)") {
		t.Fatal("live compositor must choose the server's exact rail passage")
	}
	ambient := hudTemplate(t, "painted_ambient.cs.tmpl")
	if !strings.Contains(ambient, "!TrainMotionReplacesPainted(scene.level_id,c.passage.id)") || !strings.Contains(ambient, "bool moving=!external&&PaintedPassageRenderPhase(") {
		t.Fatal("external motion must avoid legacy calendar and replace only the explicitly bound passage")
	}
}

func TestTrainMotionExternalPassageNeverLoadsPlaceholderPNGNative(t *testing.T) {
	selectSource := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static bool PaintedExternalTrain(", "  static void BlitPaintedAmbient(")
	load := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "    if(!PaintedExternalTrain(passage))", "    // External passages")
	runPaintedTextureNative(t, "TrainExternalNoPNG", `using System;
struct Vector4 {public Vector4(float a,float b,float c,float d){}}
class Rect {public float x,y,width=1,height=1;}
class Texture {}class Material {public int textures;public void SetTexture(string k,Texture value){textures++;}public void SetVector(string k,Vector4 value){}}
class PocAmbientPassage {public string motion_source,image;public object source_rect;public float shear_y,feet_x;}
class Proof {static int loads;`+selectSource+`
 static Texture ArtSceneTexture(string path){loads++;if(path==null)throw new Exception("external passage tried to load a placeholder image");return new Texture();}static Rect ArtSceneUV(object rect,Texture image){return new Rect();}
 static void Prepare(PocAmbientPassage passage,Material material){`+load+`}
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var material=new Material();Prepare(new PocAmbientPassage{motion_source="train_motion"},material);Need(loads==0&&material.textures==0,"external rail requires fake PNG");Prepare(new PocAmbientPassage{image="legacy.28m"},material);Need(loads==1&&material.textures==1,"default legacy source changed");Prepare(new PocAmbientPassage{motion_source="calendar",image="legacy.28m"},material);Need(loads==2,"explicit calendar source lost");Console.WriteLine("external aperture has no placeholder PNG; legacy calendar PNG path unchanged PASS");}
}`)
}

func TestTrainMotionLegacyStationNeverSuppressedNative(t *testing.T) {
	find := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static PocTrainMotionBinding TrainBinding(", "  static Transform TrainChild(")
	replace := sceneTemplateSection(t, "train_motion.cs.tmpl", "  bool TrainMotionReplacesPainted(", "  void TrainMotionFail(")
	runPaintedTextureNative(t, "TrainLegacyPassengerIndependence", `using System;using System.Collections.Generic;
class GameObject {}static class Resources {public static bool available=true;public static T Load<T>(string path)where T:class{return available?new GameObject() as T:null;}}
class PocTrainMotionBinding {public string level_id,passage_id;public bool visible;}
class PocTrainMotionModule {public PocTrainMotionBinding[] bindings;}
class PocTrainMotionState {public string prefab_resource="TrainMotion/MarketTrainV1";public PocTrainMotionModule[] modules;}
class PocTrainMotionSnapshot {public PocTrainMotionState[] trains;}
class Proof {PocTrainMotionSnapshot trainMotionSnapshot;readonly Dictionary<string,GameObject> trainMotionPrefabs=new Dictionary<string,GameObject>();int failures;bool TrainMotionRead(){return trainMotionSnapshot!=null;}void TrainMotionFail(string why){failures++;}`+find+replace+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var p=new Proof();p.trainMotionSnapshot=new PocTrainMotionSnapshot{trains=new[]{new PocTrainMotionState{modules=new[]{new PocTrainMotionModule{bindings=new[]{new PocTrainMotionBinding{level_id="level.c7_transport_hub",passage_id="hub.lower_freight_rail",visible=false},new PocTrainMotionBinding{level_id="level.poc_housing",passage_id="housing.lower_rail",visible=true}}}}}}};
 Need(!p.TrainMotionReplacesPainted("level.poc_monorail","station.back_rail"),"legacy 28m station boarding train was suppressed");Need(!p.TrainMotionReplacesPainted("level.c7_transport_hub","another.public.rail"),"unrelated same-level rail was suppressed");Need(p.TrainMotionReplacesPainted("level.c7_transport_hub","hub.lower_freight_rail"),"bound lower rail reverted to a calendar train when physical stock was offscreen");Need(p.TrainMotionReplacesPainted("level.poc_housing","housing.lower_rail"),"housing exact binding failed");Need(!p.TrainMotionReplacesPainted("level.c7_market",null),"native market suppressed unnamed painted passage");p.trainMotionPrefabs.Clear();Resources.available=false;Need(!p.TrainMotionReplacesPainted("level.poc_housing","housing.lower_rail")&&p.failures==1,"missing prefab silently passed replacement ownership");Console.WriteLine("actual suppression: exact lower rails only, legacy station and missing-resource failure preserved PASS");}
}`)
}

func TestTrainMotionRouteProofUsesOrdinaryWalkingViaStation(t *testing.T) {
	proof := hudTemplate(t, "train_motion_proof.cs.tmpl")
	for _, need := range []string{`"market_hub_walk":"hub_station_walk"`, `LifeCommand("enter","station_home")`, `"level.c7_transport_hub":"level.poc_housing"`, `trainRouteLeg==1?12:18,trainRouteLeg==1?22:25`, `if(trainReviewPhase==23)`, `trainRoutePixels<100`} {
		if !strings.Contains(proof, need) {
			t.Errorf("connected lower-rail native proof missing %q", need)
		}
	}
	if strings.Contains(proof, `"market_station"`) || strings.Contains(proof, `Options(false,4)`) || strings.Contains(proof, `LifeCommand("goto"`) {
		t.Error("native train observation must use ordinary walking edges and normal Life speed")
	}
}

func TestTrainMotionWholeCarObservationFiltersEachModuleNative(t *testing.T) {
	observe := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static bool TrainPaintedModuleObserved(", "  // Extrapolate only the canvas anchor.")
	metric := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static Vector2 TrainPaintedPoint(", "  static void TrainPaintedBindMaterial(")
	nearest := sceneTemplateSection(t, "painted_ambient.cs.tmpl", "  static int PaintedNearestObservation(", "  // End pure functions;")
	point := sceneTemplateSection(t, "painted.cs.tmpl", "  static Vector2 PaintedPoint(", "  static bool PaintedInverse(")
	position := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static Vector3 TrainMotionDirection(", "  static Quaternion TrainMotionRotation(")
	find := sceneTemplateSection(t, "train_motion.cs.tmpl", "  static PocTrainMotionBinding TrainBinding(", "  static Transform TrainChild(")
	viewCell := sceneTemplateSection(t, "visibility.cs.tmpl", "        static char ViewCell(", "        bool ViewSees(")
	runPaintedTextureNative(t, "TrainRigidCarObservation", `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public static Vector2 zero=>new Vector2(0,0);}
struct Vector3 {public float x,y,z;public Vector3(float a,float b,float c){x=a;y=b;z=c;}public static Vector3 operator+(Vector3 a,Vector3 b){return new Vector3(a.x+b.x,a.y+b.y,a.z+b.z);}public static Vector3 operator*(Vector3 a,float b){return new Vector3(a.x*b,a.y*b,a.z*b);}public static float Dot(Vector3 a,Vector3 b){return a.x*b.x+a.y*b.y+a.z*b.z;}}
static class Mathf {public static int RoundToInt(float value){return(int)Math.Round(value);}}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
class PocPaintedScene {public string level_id="housing";public PocPaintedPoint origin=new PocPaintedPoint(0,0),axis_x=new PocPaintedPoint(100,0),axis_y=new PocPaintedPoint(0,100);}
class PocPaintedRailMetric {public double cell_meters=1,track_m,path_progress;}
class PocAmbientPassage {public string id="lower";public PocPaintedPoint[] path,observation_cells;public PocPaintedRailMetric metric_rail=new PocPaintedRailMetric();}
class PocTrainMotionBinding {public string level_id="housing",passage_id="lower";public bool visible=true;public double x,y,z,yaw_degrees=90;}
class PocTrainMotionModule {public string id;public PocTrainMotionBinding[] bindings;}
class PocTrainMotionState {public string train_id="one";public double speed_mps=1;public PocTrainMotionModule[] modules;}
class PocObserverView {public List<string> Cells=new List<string>();}
class GameObject {public bool active=true;public void SetActive(bool value){active=value;}}class Node {public GameObject gameObject=new GameObject();}class TrainModule {public Node node=new Node();}class TrainInstance {public readonly Dictionary<string,TrainModule> modules=new Dictionary<string,TrainModule>();}
class Proof {readonly Dictionary<string,TrainInstance> trainMotionInstances=new Dictionary<string,TrainInstance>();`+observe+metric+nearest+point+position+find+viewCell+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var scene=new PocPaintedScene();var rail=new PocAmbientPassage{path=new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(10000,0)},observation_cells=new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(14,0),new PocPaintedPoint(28,0),new PocPaintedPoint(42,0)}};var cells=new char[43];for(int i=0;i<cells.Length;i++)cells[i]='0';cells[0]='2';cells[14]='1';cells[42]='2';var observer=new PocObserverView();observer.Cells.Add(new string(cells));var train=new PocTrainMotionState{modules=new PocTrainMotionModule[4]};var p=new Proof();var consist=new TrainInstance();p.trainMotionInstances["one"]=consist;
 for(int i=0;i<4;i++){string id="car"+i;train.modules[i]=new PocTrainMotionModule{id=id,bindings=new[]{new PocTrainMotionBinding{x=14*i}}};consist.modules[id]=new TrainModule();}
 p.TrainPaintedObserveModules(scene,rail,train,0,observer);Need(consist.modules["car0"].node.gameObject.active&&consist.modules["car3"].node.gameObject.active,"observed rigid car centres must show their whole module");Need(!consist.modules["car1"].node.gameObject.active&&!consist.modules["car2"].node.gameObject.active,"one observed car exposed remembered/unknown cars in the consist");
 train.modules[3].bindings[0].visible=false;p.TrainPaintedObserveModules(scene,rail,train,0,observer);Need(!consist.modules["car3"].node.gameObject.active,"client observation resurrected a server-hidden car");
 observer.Cells[0]=new string('1',43);p.TrainPaintedObserveModules(scene,rail,train,0,observer);foreach(var car in consist.modules.Values)Need(!car.node.gameObject.active,"remembered architecture exposed a moving train");
 observer.Cells[0]=new string(cells);var near=new PocTrainMotionBinding{x=6.9};Need(TrainPaintedModuleObserved(scene,rail,near,1,0,observer),"centre before boundary should be observed");Need(!TrainPaintedModuleObserved(scene,rail,near,1,.2,observer),"bounded rendered centre crossed into an unobserved rail segment without hiding the whole car");Need(!TrainPaintedModuleObserved(scene,rail,near,1,0,null),"missing current observer exposed a car");Console.WriteLine("four independent rigid-car decisions; current observed vs remembered/unknown, server-hidden and metric centre boundary PASS");}
}`)
	adapter := hudTemplate(t, "train_motion_painted.cs.tmpl")
	if !strings.Contains(adapter, "TrainPaintedObserveModules(scene,passage.passage,shownTrain,lead,ObserverView());") {
		t.Fatal("current observer must filter each actual module before the offscreen camera render")
	}
	if strings.Index(adapter, "TrainPaintedObserveModules(scene,passage.passage") > strings.Index(adapter, "PocRendering.RenderCamera(trainPaintedCamera,trainPaintedTarget)") {
		t.Fatal("module observation applied after the train was rendered")
	}
}

func TestTrainMotionWholeCarShaderPreservesPhysicalClipAndLegacyDefault(t *testing.T) {
	shader := hudTemplate(t, "painted_passage.shader.tmpl")
	for _, need := range []string{`_WholeModuleObservation("Whole rigid module already observed",Float)=0`, `if(_WholeModuleObservation<.5){float observation=`, `train.a*=SAMPLE_TEXTURE2D(_ObservationTex`, `return half4(rgb,alpha*clip.a);`} {
		if !strings.Contains(shader, need) {
			t.Errorf("legacy observation or unconditional physical foreground clip lost %q", need)
		}
	}
	ambient := hudTemplate(t, "painted_ambient.cs.tmpl")
	if !strings.Contains(ambient, `material.SetFloat("_WholeModuleObservation",0);`) {
		t.Fatal("legacy calendar PNG must retain per-pixel observer alpha")
	}
	bind := sceneTemplateSection(t, "train_motion_painted.cs.tmpl", "  static void TrainPaintedBindMaterial(", "  void TrainPaintedEnsure(")
	if !strings.Contains(bind, `material.SetFloat("_WholeModuleObservation",1);`) || !strings.Contains(bind, `material.SetTexture("_MaskTex",passage.mask)`) {
		t.Fatal("whole-car pass must bypass only observation slicing and retain the physical foreground mask")
	}
}
