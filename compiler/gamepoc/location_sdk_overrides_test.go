package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocationSDKOverrideIntegrationPreservesOwnershipAndAuthority(t *testing.T) {
	source, err := os.ReadFile("templates/location_sdk_overrides.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	for _, required := range []string{"public int schema_version=CurrentSchemaVersion", "Resources.LoadAll<AngelsDemonsLocationOverrideRegistry>(\"LocationOverrides\")", "LocationSDKOverrideContract.Key(level.id,instance.id)", "ValidateLocationSDKPrefab", "component is Animator", "component is Renderer", "child.gameObject.layer=LocationSDKLayer", "light.cullingMask=1<<LocationSDKLayer", "applyRootMotion=false", "PlaybackSpeed(visual.speeds[i],state.Paused,observed)", "ViewCell(view,cell.x,cell.y)=='2'", "particle.Pause(false)", "particle.Play(false)"} {
		if !strings.Contains(s, required) {
			t.Errorf("visual override seam missing %q", required)
		}
	}
	for _, forbidden := range []string{"state.PlayerX=", "state.PlayerY=", "state.Path.Add", "Time.timeScale", "AddComponent<Collider", "AddComponent<Rigidbody", "SendMoveTo", "File.Write", "AssetDatabase.CreateAsset"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("runtime visual override changed ownership or gameplay: %s", forbidden)
		}
	}
	renderer, err := os.ReadFile("templates/location_sdk.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	r := string(renderer)
	if strings.Index(r, "LoadLocationSDKOverrides();") > strings.Index(r, "locationSDKRoot=new GameObject") {
		t.Fatal("prefab validation must precede any scene instantiation")
	}
	for _, required := range []string{"locationSDKCameraData.renderPostProcessing=postProfile!=null&&locationSDKVolume!=null", "LocationSDKOverrideContract.PresentationProfile(locationSDKPresentationEntry,GraphicsFull)", "LocationSDKOverrideContract.PresentationAmbient(entry,locationSDKTime,brightness);var probe=new UnityEngine.Rendering.SphericalHarmonicsL2();probe.AddAmbientLight(ambient);", "locationSDKCameraData.volumeLayerMask=1<<LocationSDKLayer", "void ReleaseLocationSDK(){", "RenderSettings.ambientProbe=probe", "LocationSDKApplyPresentation(light);", "VolumeManager.instance.Update(locationSDKCamera.transform,1<<LocationSDKLayer);PocRendering.RenderCamera"} {
		if !strings.Contains(r, required) {
			t.Errorf("presentation/graphics seam missing %q", required)
		}
	}
	// Scene release still restores global ambient first, after only the optional
	// scan callback/material cleanup, which must precede destruction of SDK bodies.
	releaseMarker := "void ReleaseLocationSDK(){"
	if strings.Count(r, releaseMarker) != 1 {
		t.Fatal("expected one SDK release function")
	}
	releaseBody := r[strings.Index(r, releaseMarker)+len(releaseMarker):]
	ambientAt := strings.Index(releaseBody, "LocationSDKRestoreAmbient();")
	if ambientAt < 0 {
		t.Fatal("SDK release lost ambient restoration")
	}
	beforeAmbient := strings.TrimSpace(releaseBody[:ambientAt])
	if beforeAmbient != "" && beforeAmbient != "ReleaseMedicalScan();" {
		t.Fatal("SDK release must restore ambient before any scene destruction, after optional scan cleanup only")
	}
	for _, required := range []string{"visual.lights[i].shadows=GraphicsFull?visual.shadows[i]:LightShadows.None", "visual.lights[i].enabled=LocationSDKOverrideContract.LightEnabled(visual.lightQualities[i],GraphicsFull)", "ValidatePresentation(resource.location_sdk,registries)"} {
		if !strings.Contains(s, required) {
			t.Errorf("graphics quality light seam missing %q", required)
		}
	}
	// Painted background (doc 72): applied after the live camera bounds, bound to
	// structure/camera/fingerprint, static parts keep shadows only, never stretched.
	if strings.Index(r, "LocationSDKApplyBackground(level);") < strings.Index(r, "locationSDKCameraBounds.Encapsulate(renderer.bounds)") {
		t.Fatal("painted background must not change the live camera framing")
	}
	for _, required := range []string{"if(!closed)locationSDKBackgroundLights.Add(light)", "GraphicsFull?LocationSDKBackgroundMaxLights:LocationSDKBackgroundEconomyLights", "Shader.SetGlobalFloat(\"_PbSpecular\",GraphicsFull?1:0)", "Shader.SetGlobalFloat(\"_PbDaylight\",LocationSDKDaylightNow())", "if(entry.lit)locationSDKBackgroundOwned.Add(copy)"} {
		if !strings.Contains(s, required) {
			t.Errorf("relit background seam missing %q", required)
		}
	}
	if strings.Index(r, "LocationSDKUploadBackgroundLights(") < strings.Index(r, "UpdateLocationSDKOverrides(view)") || !strings.Contains(r, "LocationSDKLightActor(actor,feet)") {
		t.Fatal("lamps must be uploaded after Go storefront state is applied, and actors lit with them")
	}
	// demo-hub v2-light: a lens follows its bound light (emission x current / authored intensity), validated paths,
	// presentation only (MaterialPropertyBlock, no material copies, no scripts in artist prefabs).
	for _, required := range []string{"public string[] lens_paths;", "\"missing lens renderer: \"+lens", "k=Math.Max(k,l.intensity/lens.reference)", "lens.renderer.SetPropertyBlock(locationSDKLensBlock)"} {
		if !strings.Contains(s, required) {
			t.Errorf("lens binding seam missing %q", required)
		}
	}
	// demo-hub v2-light: user shaders read the graphics option from one global, set with the presentation.
	if !strings.Contains(r, "Shader.SetGlobalFloat(\"_AngGraphicsEconomy\",GraphicsFull?0f:1f)") {
		t.Fatal("graphics option global _AngGraphicsEconomy missing from the SDK presentation")
	}
	if !strings.Contains(r, "renderPostProcessing=postProfile!=null&&locationSDKVolume!=null&&LocationSDKBackgroundStatus!=\"active\"") {
		t.Fatal("painted background must not be graded twice by the post stack of either graphics option")
	}
	for _, required := range []string{"ValidateBackgrounds(resource.location_sdk,registries,ValidateLocationSDKBackgroundPrefab)", "BackgroundStale(level,entry,LocationSDKStaticFingerprint(statics))", "renderer.shadowCastingMode=UnityEngine.Rendering.ShadowCastingMode.ShadowsOnly", "material.shader.name!=\"ANG/LocationSDKDepthSprite\"", "foreach(var nodes in new[]{visual.goods,visual.closed,visual.working})", "live 3D architecture kept"} {
		if !strings.Contains(s, required) {
			t.Errorf("painted background seam missing %q", required)
		}
	}
	if !strings.Contains(r, "if(TryLocationSDKOverride(level,instance,bottom,out replacement))continue") || !strings.Contains(r, "UpdateLocationSDKOverrides(view)") || !strings.Contains(r, "locationSDKOverrideFailure,hudWrap") || !strings.Contains(r, "ReleaseLocationSDKOverrides();") {
		t.Fatal("renderer silently falls back, omits visible errors, skips pause/FOV or leaks override state")
	}
	editor, err := os.ReadFile("templates/location_sdk_overrides_editor.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	e := string(editor)
	if !strings.Contains(e, "Assets/Resources/LocationOverrides") || strings.Index(e, "if(File.Exists(Path)||File.Exists(Path+\".meta\"))") > strings.Index(e, "AssetDatabase.CreateAsset") || !strings.Contains(e, "if(existing!=null)") || !strings.Contains(e, "SaveAssetIfDirty(registry)") {
		t.Fatal("Editor helper must create once and preserve existing asset, metadata and unrelated dirty files")
	}
	for _, forbidden := range []string{"InitializeOnLoad", "DidReloadScripts", "File.WriteAllText", "DeleteAsset", "SaveAssets()", "EditorSceneManager"} {
		if strings.Contains(e, forbidden) {
			t.Errorf("registry menu must not auto-write or change scenes: %s", forbidden)
		}
	}
	proof, err := os.ReadFile("templates/location_sdk_proof.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proof), "IsLocationSDKOverrideTransform(filter.transform)") {
		t.Fatal("structural proof must not read imported prefab meshes or certify their animation geometry")
	}
}

// demo-hub v3-people: humanoid actors are presentation only and follow the interpolated Go position, the simulation
// clock (pause) and the distance walked; layouts without the authored bit keep the character sheets.
func TestLocationSDKHumanoidActorsFollowGoMotionAndClock(t *testing.T) {
	source, err := os.ReadFile("templates/location_sdk_humanoid.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	h := string(source)
	for _, required := range []string{"an.cullingMode=AnimatorCullingMode.AlwaysAnimate", "bool held=paused||(state!=null&&state.Paused)", "h.rate=held?0:", "LocationSDKHumanoidPhaseAt(curve,(float)((h.offset*stride+h.distance)%stride))", "an.speed=fading?h.rate*", "LocationSDKHumanoidHash(id)", "LocationSDKSampleWorld(navStart.Value,path,startNS,durationNS,nowNS+window", "Mathf.MoveTowardsAngle(h.yaw,target,cast.turn_degrees_per_second*", "r.shadowCastingMode=full?UnityEngine.Rendering.ShadowCastingMode.On:UnityEngine.Rendering.ShadowCastingMode.Off", "t.gameObject.layer=LocationSDKLayer", "locationSDKVisibleActors.Add(id)", "character sheets kept"} {
		if !strings.Contains(h, required) {
			t.Errorf("humanoid seam missing %q", required)
		}
	}
	for _, forbidden := range []string{"state.PlayerX=", "state.PlayerY=", "state.Path.Add", "SendMoveTo", "Time.timeScale", "AddComponent<Collider", "AddComponent<Rigidbody", "applyRootMotion=true", "File.Write"} {
		if strings.Contains(h, forbidden) {
			t.Errorf("humanoid presentation changed gameplay or ownership: %s", forbidden)
		}
	}
	renderer, err := os.ReadFile("templates/location_sdk.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	r := string(renderer)
	for _, required := range []string{"if(locationSDKHumanoidCast!=null){LocationSDKUpdateHumanoid(", "LocationSDKHumanoidVisibility();", "ReleaseLocationSDKHumanoids();", "return actors_authored&&actors!=null&&actors.presentation==\"humanoid\""} {
		if !strings.Contains(r, required) {
			t.Errorf("renderer humanoid seam missing %q", required)
		}
	}
	overrides, err := os.ReadFile("templates/location_sdk_overrides.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overrides), "ValidateActorCasts(registries,ValidateLocationSDKActorCast)") || !strings.Contains(string(overrides), "cast body needs a humanoid Animator with a controller") {
		t.Fatal("cast resolution or body validation missing from the registry load")
	}
}

func TestLocationSDKOverrideMappingValidationAndPauseNative(t *testing.T) {
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
	contract := sceneTemplateSection(t, "location_sdk_overrides.cs.tmpl", " [Serializable] public sealed class AngelsDemonsLocationLightBinding ", " public sealed partial class AngelsDemonsPoc ")
	harness := `using System;using System.Collections.Generic;
namespace UnityEngine.Rendering { public class VolumeProfile {} }
public class Material {} public class GameObject {} public class ScriptableObject {public string name="Registry";}
public struct Color {public float r,g,b,a;public Color(float r,float g,float b,float a=1){this.r=r;this.g=g;this.b=b;this.a=a;}public static Color black{get{return new Color(0,0,0,1);}}public static Color clear{get{return new Color(0,0,0,0);}}public static Color operator*(Color c,float k){return new Color(c.r*k,c.g*k,c.b*k,c.a*k);}public static bool operator==(Color x,Color y){return Math.Abs(x.r-y.r)<1e-5f&&Math.Abs(x.g-y.g)<1e-5f&&Math.Abs(x.b-y.b)<1e-5f&&Math.Abs(x.a-y.a)<1e-5f;}public static bool operator!=(Color x,Color y){return !(x==y);}public override bool Equals(object o){return o is Color&&this==(Color)o;}public override int GetHashCode(){return 0;}}
public class SerializeReference:Attribute {}
public class CreateAssetMenuAttribute:Attribute {public string fileName,menuName;}
` + dto + contract + `
class Proof {
 static void Require(bool value,string why){if(!value)throw new Exception(why);}
 static PocLocationSDKBundle Bundle(){return new PocLocationSDKBundle{surfaces=new[]{new PocLocationSDKSurface{id="surface.floor"}},levels=new[]{new PocLocationSDKLevel{id="layout.a",views=new[]{new PocLocationSDKView{time_id="day"},new PocLocationSDKView{time_id="night"}},instances=new[]{new PocLocationSDKInstance{id="shell",width=12,depth=8,rotation=0,at=new PocLocationSDKCell{x=3,y=5}}}},new PocLocationSDKLevel{id="layout.b",instances=new[]{new PocLocationSDKInstance{id="shell"}}}}};}
 static AngelsDemonsLocationOverrideEntry Entry(string layout="layout.a",string instance="shell"){return new AngelsDemonsLocationOverrideEntry{layout_id=layout,instance_id=instance,prefab=new GameObject()};}
 static AngelsDemonsLocationOverrideRegistry Registry(params AngelsDemonsLocationOverrideEntry[] entries){return new AngelsDemonsLocationOverrideRegistry{entries=entries};}
 static void Error(string key,Action run){try{run();throw new Exception("expected rejection "+key);}catch(LocationSDKOverrideError error){Require(error.localization_key==key,"wrong visible registry error: "+error.localization_key);}}
 static void Validation(){
  var bundle=Bundle();var a=Entry();var b=Entry("layout.b");var registry=Registry(a,b);var result=LocationSDKOverrideContract.Validate(bundle,new[]{registry});Require(result.Count==2&&ReferenceEquals(result[LocationSDKOverrideContract.Key("layout.a","shell")],a),"same instance ID in different layouts was conflated");
  Require(bundle.levels[0].instances[0].width==12&&bundle.levels[0].instances[0].at.x==3&&registry.entries[0]==a,"validation mutated authored physics or assets");
  Require(LocationSDKOverrideContract.Key("a/b","c")!=LocationSDKOverrideContract.Key("a","b/c"),"stable key aliases IDs containing slashes");
  Error("ui.sdk.override_unknown_layout",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(Entry("wrong"))}));
  Error("ui.sdk.override_unknown_layout",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(Entry("LAYOUT.A"))}));
  Error("ui.sdk.override_unknown_instance",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(Entry("layout.a","missing"))}));
  Error("ui.sdk.override_duplicate",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a),Registry(Entry())}));
  var missing=Entry();missing.prefab=null;Error("ui.sdk.override_missing_prefab",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(missing)}));
  var future=Registry(a);future.schema_version=2;Error("ui.sdk.override_registry",()=>LocationSDKOverrideContract.Validate(bundle,new[]{future}));
  var binding=new AngelsDemonsLocationLightBinding{path="fixture/lamp",active_time_ids=new[]{"night"}};a.light_bindings=new[]{binding};LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});
  binding.active_time_ids=new[]{"missing"};Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));binding.active_time_ids=new[]{"night"};
  a.light_bindings=new[]{binding,binding};Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));a.light_bindings=null;
  Require(LocationSDKOverrideContract.LightFactor(.64f,"night",new[]{"night"})==1,"night fixture was dimmed with ambient fill");
  Require(LocationSDKOverrideContract.LightFactor(1,"day",new[]{"night"})==0,"night-only fixture stayed on by day");
  Require(LocationSDKOverrideContract.LightFactor(.64f,"night",null)==.64f,"legacy light brightness changed");
  bundle.levels[0].camera=new PocLocationSDKCamera(); // actual JsonUtility legacy materialization
  a.baked_camera=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=30,azimuth_degrees=45};LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});
  a.baked_camera.azimuth_degrees=20;Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));
  bundle.levels[0].camera_authored=true;bundle.levels[0].camera=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=35,azimuth_degrees=20};a.baked_camera.elevation_degrees=35;bundle.levels[0].camera.hero_view_height_m=15;LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});
  foreach(float angle in new[]{float.NaN,float.PositiveInfinity,0f,35.01f}){a.baked_camera.elevation_degrees=angle;Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));}a.baked_camera.elevation_degrees=35;
  foreach(int rotation in new[]{90,180,270}){bundle.levels[0].instances[0].rotation=rotation;Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));}bundle.levels[0].instances[0].rotation=0;
  a.baked_camera.projection="perspective";Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));a.baked_camera=null;LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});
  int checkedPrefabs=0;LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)},entry=>checkedPrefabs++);Require(checkedPrefabs==1,"prefab component validation bypassed");
  Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)},entry=>{throw new LocationSDKOverrideError("ui.sdk.override_unsupported","layout.a :: shell","Collider");}));
  Require(LocationSDKOverrideContract.Validate(bundle,new AngelsDemonsLocationOverrideRegistry[0]).Count==0,"absent registry stopped ordinary SDK rendering");
  var chosen=new Material();var surfaceEntry=new AngelsDemonsLocationSurfaceOverrideEntry{layout_id="layout.a",surface_id="surface.floor",material=chosen};var surfaceRegistry=Registry();surfaceRegistry.surface_entries=new[]{surfaceEntry};
  var materials=LocationSDKOverrideContract.ValidateSurfaces(bundle,new[]{surfaceRegistry});Require(materials.Count==1&&ReferenceEquals(materials[LocationSDKOverrideContract.Key("layout.a","surface.floor")],chosen),"authored surface material was not preserved");
  surfaceEntry.surface_id="missing";Error("ui.sdk.override_unknown_surface",()=>LocationSDKOverrideContract.ValidateSurfaces(bundle,new[]{surfaceRegistry}));surfaceEntry.surface_id="surface.floor";
  surfaceEntry.material=null;Error("ui.sdk.override_missing_material",()=>LocationSDKOverrideContract.ValidateSurfaces(bundle,new[]{surfaceRegistry}));surfaceEntry.material=chosen;
  Error("ui.sdk.override_duplicate",()=>LocationSDKOverrideContract.ValidateSurfaces(bundle,new[]{surfaceRegistry,surfaceRegistry}));
  var quality=new AngelsDemonsLocationLightBinding{path="fixture/bra",active_time_ids=new[]{"day","night"},min_quality="full"};a.light_bindings=new[]{quality};LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});
  quality.min_quality="economy";LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});quality.min_quality="";LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)});
  quality.min_quality="ultra";Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)}));a.light_bindings=null;
  Require(LocationSDKOverrideContract.LightEnabled("full",true)&&!LocationSDKOverrideContract.LightEnabled("full",false)&&LocationSDKOverrideContract.LightEnabled(null,false)&&LocationSDKOverrideContract.LightEnabled("economy",false),"economy graphics dropped an essential light or kept a supplementary one");
  var mood=new AngelsDemonsLocationPresentationEntry{layout_id="layout.a",ambient_color=new Color(.1f,.12f,.15f),reflection_intensity=.2f};var presentationRegistry=Registry();presentationRegistry.presentation_entries=new[]{mood};
  var presentations=LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry});Require(presentations.Count==1&&ReferenceEquals(presentations["layout.a"],mood),"layout presentation was not preserved");
  Require(LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{Registry()}).Count==0,"absent presentation changed ordinary SDK lighting");
  Error("ui.sdk.override_duplicate",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry,presentationRegistry}));
  mood.layout_id="missing";Error("ui.sdk.override_unknown_layout",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.layout_id="layout.a";
  mood.reflection_intensity=float.NaN;Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.reflection_intensity=.2f;
  mood.ambient_color=new Color(-.1f,0,0);Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.ambient_color=new Color(2,0,0);Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.ambient_color=new Color(.1f,.12f,.15f);
  // Lookdev light profile (lookdev-parity-v1): exact ambient per view time, economy post profile.
  Require(LocationSDKOverrideContract.PresentationAmbient(mood,"night",.5f)==new Color(.05f,.06f,.075f,1),"ambient without ambient_by_time must stay ambient_color x brightness");
  mood.ambient_by_time=new[]{new AngelsDemonsLocationTimeAmbient{time_id="night",color=new Color(.09f,.11f,.15f)}};LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry});
  Require(LocationSDKOverrideContract.PresentationAmbient(mood,"night",.5f)==new Color(.09f,.11f,.15f,1)&&LocationSDKOverrideContract.PresentationAmbient(mood,"day",1f)==new Color(.1f,.12f,.15f,1),"ambient_by_time must give the exact colour of its time and fall back otherwise");
  mood.ambient_by_time[0].time_id="dusk";Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.ambient_by_time[0].time_id="night";
  mood.ambient_by_time[0].color=new Color(1.5f,0,0);Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.ambient_by_time[0].color=new Color(.09f,.11f,.15f);
  mood.ambient_by_time=new[]{mood.ambient_by_time[0],new AngelsDemonsLocationTimeAmbient{time_id="night",color=Color.black}};Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));
  mood.ambient_by_time=new AngelsDemonsLocationTimeAmbient[]{null};Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.ValidatePresentation(bundle,new[]{presentationRegistry}));mood.ambient_by_time=null;
  Require(LocationSDKOverrideContract.PresentationProfile(mood,true)==null&&LocationSDKOverrideContract.PresentationProfile(mood,false)==null&&LocationSDKOverrideContract.PresentationProfile(null,true)==null,"no profile must mean no post at either quality");

 }
 static void Mapping(){
  foreach(int rotation in new[]{0,90,180,270})foreach(float floor in new[]{-7.5f,0f,18f}){bool quarter=rotation==90||rotation==270;var instance=new PocLocationSDKInstance{width=quarter?8:12,depth=quarter?12:8,rotation=rotation,at=new PocLocationSDKCell{x=3,y=5}};var pose=LocationSDKOverrideContract.Pose(instance,.5f,floor);double radians=pose.yaw_degrees*Math.PI/180;
   foreach(int x in new[]{0,3,11})foreach(int y in new[]{0,4,7}){int rx=x,ry=y;if(rotation==90){rx=7-y;ry=x;}else if(rotation==180){rx=11-x;ry=7-y;}else if(rotation==270){rx=y;ry=11-x;}double localX=(x+.5)*.5,localZ=(y+.5)*.5,worldX=pose.x+Math.Cos(radians)*localX+Math.Sin(radians)*localZ,worldZ=pose.z-Math.Sin(radians)*localX+Math.Cos(radians)*localZ;
    Require(Math.Abs(worldX-(3+rx+.5)*.5)<.00001&&Math.Abs(worldZ-(5+ry+.5)*.5)<.00001&&pose.y==floor,"prefab quarter-turn/pivot does not match CUE's rotated cells");
   }
  }
 }
 static void Pause(){float authored=.85f;float speed=LocationSDKOverrideContract.PlaybackSpeed(authored,false,true);Require(speed==authored,"visible running Animator speed lost");speed=LocationSDKOverrideContract.PlaybackSpeed(authored,true,true);Require(speed==0,"server pause did not freeze Animator");speed=LocationSDKOverrideContract.PlaybackSpeed(authored,false,false);Require(speed==0,"remembered architecture played live animation");speed=LocationSDKOverrideContract.PlaybackSpeed(authored,false,true);Require(speed==authored,"unpause did not restore authored Animator speed");Require(LocationSDKOverrideContract.PlaybackSpeed(-.5f,false,true)==-.5f,"authored reverse animation was normalized");}
 static void Background(){
  var bundle=Bundle();var level=bundle.levels[0];level.structural_hash="s1";level.initial_layout_known=true;level.camera_authored=true;level.camera=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=35,azimuth_degrees=37,hero_view_height_m=15};
  var e=new AngelsDemonsLocationBackgroundEntry{layout_id="layout.a",bake_id="bake.v1",structural_hash="s1",static_fingerprint="f1",baked_camera=new PocLocationSDKCamera{projection="orthographic",elevation_degrees=35,azimuth_degrees=37},prefab=new GameObject()};
  var reg=Registry();reg.background_entries=new[]{e};Func<object> run=()=>LocationSDKOverrideContract.ValidateBackgrounds(bundle,new[]{reg});
  var found=LocationSDKOverrideContract.ValidateBackgrounds(bundle,new[]{reg});Require(found.Count==1&&ReferenceEquals(found["layout.a"],e),"background entry was not preserved");
  Require(LocationSDKOverrideContract.ValidateBackgrounds(bundle,new[]{Registry()}).Count==0,"absent background changed live 3D");
  int checkedPrefabs=0;LocationSDKOverrideContract.ValidateBackgrounds(bundle,new[]{reg},x=>checkedPrefabs++);Require(checkedPrefabs==1,"background prefab validation bypassed");
  Error("ui.sdk.override_duplicate",()=>LocationSDKOverrideContract.ValidateBackgrounds(bundle,new[]{reg,reg}));
  e.layout_id="missing";Error("ui.sdk.override_unknown_layout",()=>run());e.layout_id="layout.a";
  e.prefab=null;Error("ui.sdk.override_missing_prefab",()=>run());e.prefab=new GameObject();
  e.static_fingerprint="";Error("ui.sdk.override_unsupported",()=>run());e.static_fingerprint="f1";
  e.bake_id=null;Error("ui.sdk.override_unsupported",()=>run());e.bake_id="bake.v1";
  e.structural_hash="";Error("ui.sdk.override_unsupported",()=>run());e.structural_hash="s1";
  var cam=e.baked_camera;e.baked_camera=null;Error("ui.sdk.override_unsupported",()=>run());e.baked_camera=cam;
  cam.projection="perspective";Error("ui.sdk.override_unsupported",()=>run());cam.projection="orthographic";
  cam.elevation_degrees=float.NaN;Error("ui.sdk.override_unsupported",()=>run());cam.elevation_degrees=35;
  e.live_instance_ids=new[]{"shell"};run();e.live_instance_ids=new[]{"missing"};Error("ui.sdk.override_unknown_instance",()=>run());e.live_instance_ids=new string[]{null};Error("ui.sdk.override_unknown_instance",()=>run());e.live_instance_ids=null;
  Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")==null,"matching bake rejected");
  Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f2")=="static geometry fingerprint differs","changed static geometry accepted");
  level.structural_hash="s2";Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")=="structural hash differs","changed structure accepted");level.structural_hash="s1";
  level.camera.azimuth_degrees=45;Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")=="camera differs","changed camera accepted");level.camera.azimuth_degrees=37;
  level.initial_layout_known=false;Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")=="layout is not public architecture","private layout painted");level.initial_layout_known=true;
  level.camera_authored=false;level.camera=new PocLocationSDKCamera();Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")=="camera differs","35/37 bake accepted for legacy 30/45 view");
  cam.elevation_degrees=30;cam.azimuth_degrees=45;Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")==null,"legacy default camera binding rejected");
  level.camera=new PocLocationSDKCamera{projection="orthographic"};Require(LocationSDKOverrideContract.BackgroundStale(level,e,"f1")=="invalid layout camera","unmarked camera accepted");
  string a=LocationSDKOverrideContract.StaticFingerprint(new[]{"x|m|4|1|0,0,0|1,1,1","y|n|8|2|0,0,0|2,2,2"}),b=LocationSDKOverrideContract.StaticFingerprint(new[]{"y|n|8|2|0,0,0|2,2,2","x|m|4|1|0,0,0|1,1,1"});
  Require(a==b&&a.Length==64,"fingerprint depends on renderer order");
  Require(a!=LocationSDKOverrideContract.StaticFingerprint(new[]{"x|m|4|1|0,0,0|1,1,2","y|n|8|2|0,0,0|2,2,2"}),"1 mm static change not detected");
  Require(a!=LocationSDKOverrideContract.StaticFingerprint(new[]{"x|m|4|1|0,0,0|1,1,1"}),"removed static part not detected");
 }
 static void Daylight(){
  var d=new PocLocationSDKDaylight{source="passive_lightguides",sunrise_second=21600,sunset_second=79200,twilight_seconds=3600,day_brightness=1,night_brightness=.64f};
  Require(LocationSDKOverrideContract.DaylightFactor(d,12*3600)==1,"noon light guide not full");
  Require(LocationSDKOverrideContract.DaylightFactor(d,2*3600)==0&&LocationSDKOverrideContract.DaylightFactor(d,23*3600)==0,"night light guide not dark");
  float dusk=LocationSDKOverrideContract.DaylightFactor(d,22*3600);Require(dusk>.45f&&dusk<.55f,"sunset is not mid-twilight");
  Require(LocationSDKOverrideContract.DaylightFactor(d,22*3600-600)>dusk&&LocationSDKOverrideContract.DaylightFactor(d,22*3600+600)<dusk,"twilight is not monotonic");
  Require(LocationSDKOverrideContract.DaylightFactor(d,86400+12*3600)==1,"calendar does not wrap days");
  Require(LocationSDKOverrideContract.DaylightFactor(null,12*3600)==0&&LocationSDKOverrideContract.DaylightFactor(new PocLocationSDKDaylight(),12*3600)==0,"absent profile made light");
 }
 // demo-hub v3-people: humanoid casts are resolved by ID with measured, finite speeds; sprite layouts stay sprites.
 static AngelsDemonsLocationActorCast Cast(string id="cast.people.v1"){return new AngelsDemonsLocationActorCast{cast_id=id,body_prefab=new GameObject(),walk_clip_speed_mps=1.05f,walk_cycle_seconds=1.07f,idle_cycle_seconds=2.5f,tints=new[]{new Color(.3f,.4f,.5f)}};}
 static void Casts(){
  var cast=Cast();var reg=Registry();reg.actor_casts=new[]{cast};Func<object> run=()=>LocationSDKOverrideContract.ValidateActorCasts(new[]{reg});
  var found=LocationSDKOverrideContract.ValidateActorCasts(new[]{reg});Require(found.Count==1&&ReferenceEquals(found["cast.people.v1"],cast),"cast was not preserved");
  Require(LocationSDKOverrideContract.ValidateActorCasts(new[]{Registry()}).Count==0,"absent cast changed sprites");
  int checkedPrefabs=0;LocationSDKOverrideContract.ValidateActorCasts(new[]{reg},x=>checkedPrefabs++);Require(checkedPrefabs==1,"cast body validation bypassed");
  Error("ui.sdk.override_duplicate",()=>LocationSDKOverrideContract.ValidateActorCasts(new[]{reg,reg}));
  cast.body_prefab=null;Error("ui.sdk.override_missing_prefab",()=>run());cast.body_prefab=new GameObject();
  foreach(float bad in new[]{0f,-1f,float.NaN,float.PositiveInfinity}){cast.walk_clip_speed_mps=bad;Error("ui.sdk.override_unsupported",()=>run());}cast.walk_clip_speed_mps=1.05f;
  cast.walk_cycle_seconds=0;Error("ui.sdk.override_unsupported",()=>run());cast.walk_cycle_seconds=1.07f;
  cast.walk_state="";Error("ui.sdk.override_unsupported",()=>run());cast.walk_state="walk";
  cast.tints=new Color[0];Error("ui.sdk.override_unsupported",()=>run());cast.tints=new[]{new Color(.3f,.4f,.5f)};
  cast.stairs_state="stairs";Error("ui.sdk.override_unsupported",()=>run());cast.stairs_clip_speed_mps=.9f;cast.stairs_cycle_seconds=1.2f;run();
  cast.course_window_seconds=5;Error("ui.sdk.override_unsupported",()=>run());cast.course_window_seconds=.35f;
  cast.cast_id=null;Error("ui.sdk.override_unsupported",()=>run());cast.cast_id="cast.people.v1";
  cast.walk_distance_by_phase=new[]{0f,.4f,.9f,1.4f};run();cast.walk_distance_by_phase=new[]{0f,.5f,.4f,1.4f};Error("ui.sdk.override_unsupported",()=>run());
  cast.walk_distance_by_phase=new[]{.1f,.5f,1.4f};Error("ui.sdk.override_unsupported",()=>run());cast.walk_distance_by_phase=new[]{0f,float.NaN};Error("ui.sdk.override_unsupported",()=>run());cast.walk_distance_by_phase=null;
  var level=new PocLocationSDKLevel{actors=new PocLocationSDKActors{presentation="humanoid",cast="cast.people.v1"}};
  Require(level.HumanoidCast()==null,"cast used without the authored bit (JsonUtility empty object)");level.actors_authored=true;Require(level.HumanoidCast()=="cast.people.v1","authored humanoid cast lost");
  level.actors.presentation="sprite";Require(level.HumanoidCast()==null,"sprite layout drew humanoids");
 }
 static void Main(string[] args){if(args[0]=="casts")Casts();else if(args[0]=="daylight")Daylight();else if(args[0]=="validation")Validation();else if(args[0]=="mapping")Mapping();else if(args[0]=="background")Background();else Pause();Console.WriteLine("override "+args[0]+" executable contract PASS");}
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "OverrideContract.cs"), filepath.Join(dir, "OverrideContract.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile exact override contract: %v\n%s", err, out)
	}
	for _, mode := range []string{"validation", "mapping", "pause", "background", "daylight", "casts"} {
		t.Run(mode, func(t *testing.T) {
			if out, err := exec.Command(mono, binary, mode).CombinedOutput(); err != nil {
				t.Fatalf("override %s: %v\n%s", mode, err, out)
			}
		})
	}
}
