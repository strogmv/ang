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
	contract := sceneTemplateSection(t, "location_sdk_overrides.cs.tmpl", " [Serializable] public sealed class AngelsDemonsLocationOverrideEntry ", " public sealed partial class AngelsDemonsPoc ")
	harness := `using System;using System.Collections.Generic;
public class GameObject {} public class ScriptableObject {public string name="Registry";}
public class CreateAssetMenuAttribute:Attribute {public string fileName,menuName;}
` + dto + contract + `
class Proof {
 static void Require(bool value,string why){if(!value)throw new Exception(why);}
 static PocLocationSDKBundle Bundle(){return new PocLocationSDKBundle{levels=new[]{new PocLocationSDKLevel{id="layout.a",instances=new[]{new PocLocationSDKInstance{id="shell",width=12,depth=8,rotation=0,at=new PocLocationSDKCell{x=3,y=5}}}},new PocLocationSDKLevel{id="layout.b",instances=new[]{new PocLocationSDKInstance{id="shell"}}}}};}
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
  int checkedPrefabs=0;LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)},entry=>checkedPrefabs++);Require(checkedPrefabs==1,"prefab component validation bypassed");
  Error("ui.sdk.override_unsupported",()=>LocationSDKOverrideContract.Validate(bundle,new[]{Registry(a)},entry=>{throw new LocationSDKOverrideError("ui.sdk.override_unsupported","layout.a :: shell","Collider");}));
  Require(LocationSDKOverrideContract.Validate(bundle,new AngelsDemonsLocationOverrideRegistry[0]).Count==0,"absent registry stopped ordinary SDK rendering");
 }
 static void Mapping(){
  foreach(int rotation in new[]{0,90,180,270})foreach(float floor in new[]{-7.5f,0f,18f}){bool quarter=rotation==90||rotation==270;var instance=new PocLocationSDKInstance{width=quarter?8:12,depth=quarter?12:8,rotation=rotation,at=new PocLocationSDKCell{x=3,y=5}};var pose=LocationSDKOverrideContract.Pose(instance,.5f,floor);double radians=pose.yaw_degrees*Math.PI/180;
   foreach(int x in new[]{0,3,11})foreach(int y in new[]{0,4,7}){int rx=x,ry=y;if(rotation==90){rx=7-y;ry=x;}else if(rotation==180){rx=11-x;ry=7-y;}else if(rotation==270){rx=y;ry=11-x;}double localX=(x+.5)*.5,localZ=(y+.5)*.5,worldX=pose.x+Math.Cos(radians)*localX+Math.Sin(radians)*localZ,worldZ=pose.z-Math.Sin(radians)*localX+Math.Cos(radians)*localZ;
    Require(Math.Abs(worldX-(3+rx+.5)*.5)<.00001&&Math.Abs(worldZ-(5+ry+.5)*.5)<.00001&&pose.y==floor,"prefab quarter-turn/pivot does not match CUE's rotated cells");
   }
  }
 }
 static void Pause(){float authored=.85f;float speed=LocationSDKOverrideContract.PlaybackSpeed(authored,false,true);Require(speed==authored,"visible running Animator speed lost");speed=LocationSDKOverrideContract.PlaybackSpeed(authored,true,true);Require(speed==0,"server pause did not freeze Animator");speed=LocationSDKOverrideContract.PlaybackSpeed(authored,false,false);Require(speed==0,"remembered architecture played live animation");speed=LocationSDKOverrideContract.PlaybackSpeed(authored,false,true);Require(speed==authored,"unpause did not restore authored Animator speed");Require(LocationSDKOverrideContract.PlaybackSpeed(-.5f,false,true)==-.5f,"authored reverse animation was normalized");}
 static void Main(string[] args){if(args[0]=="validation")Validation();else if(args[0]=="mapping")Mapping();else Pause();Console.WriteLine("override "+args[0]+" executable contract PASS");}
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "OverrideContract.cs"), filepath.Join(dir, "OverrideContract.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile exact override contract: %v\n%s", err, out)
	}
	for _, mode := range []string{"validation", "mapping", "pause"} {
		t.Run(mode, func(t *testing.T) {
			if out, err := exec.Command(mono, binary, mode).CombinedOutput(); err != nil {
				t.Fatalf("override %s: %v\n%s", mode, err, out)
			}
		})
	}
}
