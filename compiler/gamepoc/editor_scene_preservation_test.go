package gamepoc

import (
	"strings"
	"testing"
)

func TestEditorBootstrapPreservesAuthoredScenesNative(t *testing.T) {
	helper := sceneTemplateSection(t, "editor.cs.tmpl", "public static class PocSceneSetup", "// Preserve the original sheet grid.")
	runPaintedTextureNative(t, "ScenePreservation", `using System;using System.IO;using System.Collections.Generic;using UnityEditor;using UnityEditor.SceneManagement;using UnityEngine;using UnityEngine.SceneManagement;using AngelsDemons.Poc;
namespace UnityEngine {
 public class Component {public bool enabled=true;public GameObject gameObject;}
 public class GameObject {
  public string name;public bool activeInHierarchy=true;public readonly List<Component> components=new List<Component>();
  public GameObject(string n):this(n,true){}public GameObject(string n,bool attach){name=n;if(attach)SceneManager.Attach(this);}
  public T AddComponent<T>()where T:Component,new(){var c=new T{gameObject=this};components.Add(c);return c;}
  public T[] GetComponentsInChildren<T>(bool inactive)where T:class{var result=new List<T>();foreach(var c in components)if(c is T)result.Add(c as T);return result.ToArray();}
  public GameObject Clone(){var g=new GameObject(name,false){activeInHierarchy=activeInHierarchy};foreach(var c in components){var copy=g.AddComponent<AngelsDemonsPoc>();copy.enabled=c.enabled;}return g;}
 }
}
namespace AngelsDemons.Poc {public class AngelsDemonsPoc:Component {}}
namespace UnityEngine.SceneManagement {
 public class Record {public string path="",name="Untitled";public bool loaded=true,dirty,closed,preview;public List<GameObject> roots=new List<GameObject>();public Record Clone(){var r=new Record{path=path,name=name,dirty=dirty,preview=preview};foreach(var g in roots)r.roots.Add(g.Clone());return r;}}
 public struct Scene {public int id;public static bool operator==(Scene a,Scene b){return a.id==b.id;}public static bool operator!=(Scene a,Scene b){return a.id!=b.id;}public override bool Equals(object other){return other is Scene&&((Scene)other).id==id;}public override int GetHashCode(){return id;}public bool IsValid(){return SceneManager.records.ContainsKey(id)&&!SceneManager.records[id].closed;}public bool isLoaded{get{return IsValid()&&SceneManager.records[id].loaded;}}public bool isDirty{get{return IsValid()&&SceneManager.records[id].dirty;}}public string path{get{return IsValid()?SceneManager.records[id].path:"";}}public string name{get{return IsValid()?SceneManager.records[id].name:"";}}public GameObject[] GetRootGameObjects(){return SceneManager.records[id].roots.ToArray();}}
 public static class SceneManager {
  public static int next,active;public static bool refuseActivation;public static Dictionary<int,Record> records=new Dictionary<int,Record>();public static List<int> open=new List<int>();public static int sceneCount{get{return open.Count;}}
  public static Scene Make(Record record,bool preview){int id=++next;record.preview=preview;records[id]=record;if(!preview)open.Add(id);return new Scene{id=id};}
  public static Scene GetSceneByPath(string path){foreach(int id in open)if(records[id].path==path)return new Scene{id=id};return new Scene();}
  public static Scene GetActiveScene(){return new Scene{id=active};}public static bool SetActiveScene(Scene s){if(!s.isLoaded||refuseActivation||active==s.id)return false;active=s.id;return true;}
  public static Scene GetSceneAt(int i){return new Scene{id=open[i]};}public static void Attach(GameObject g){var r=records[active];r.roots.Add(g);r.dirty=true;}
 }
}
namespace UnityEditor {
 public class EditorBuildSettingsScene {public string path;public bool enabled;public EditorBuildSettingsScene(string p,bool e){path=p;enabled=e;}}
 public static class EditorBuildSettings {public static int writes;static EditorBuildSettingsScene[] value=new EditorBuildSettingsScene[0];public static EditorBuildSettingsScene[] scenes{get{return value;}set{writes++;EditorBuildSettings.value=value;}}}
}
namespace UnityEditor.SceneManagement {
 public enum NewSceneSetup {EmptyScene}public enum NewSceneMode {Single,Additive}
 public static class EditorSceneManager {
  public static int saves,closed,previews;public static bool failSave,leaveNewInactive;public static readonly Dictionary<string,Record> disk=new Dictionary<string,Record>();
  public static Scene NewScene(NewSceneSetup setup,NewSceneMode mode){if(mode==NewSceneMode.Additive){foreach(int id in SceneManager.open)if(SceneManager.records[id].path=="")throw new InvalidOperationException("Unity refuses additive creation beside an untitled scene");}if(mode==NewSceneMode.Single){foreach(int id in SceneManager.open)SceneManager.records[id].closed=true;SceneManager.open.Clear();}var scene=SceneManager.Make(new Record(),false);if(!leaveNewInactive)SceneManager.SetActiveScene(scene);return scene;}
  public static bool SaveScene(Scene s,string path){saves++;if(failSave)return false;var r=SceneManager.records[s.id];r.path=path;r.name=Path.GetFileNameWithoutExtension(path);r.dirty=false;disk[path]=r.Clone();string text="";foreach(var root in r.roots)text+=root.name+"|"+root.components.Count+";";Directory.CreateDirectory(Path.GetDirectoryName(path));File.WriteAllText(path,text);return true;}
  public static Scene OpenPreviewScene(string path){previews++;var scene=SceneManager.Make(disk[path].Clone(),true);SceneManager.SetActiveScene(scene);return scene;}
  public static void ClosePreviewScene(Scene s){SceneManager.records[s.id].closed=true;}
  public static bool CloseScene(Scene s,bool remove){closed++;SceneManager.open.Remove(s.id);SceneManager.records[s.id].closed=true;return true;}
 }
}
`+helper+`
class Proof {
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 static void Reject(Action action,string why){bool rejected=false;try{action();}catch(InvalidOperationException){rejected=true;}Need(rejected,why);}
 public static void Main(){
  var manual=EditorSceneManager.NewScene(NewSceneSetup.EmptyScene,NewSceneMode.Additive);var marker=new GameObject("Authored marker");EditorSceneManager.SaveScene(manual,"Assets/Manual/Main.unity");string manualDisk=File.ReadAllText(manual.path);marker.name="Unsaved animation staging";SceneManager.records[manual.id].dirty=true;
  var custom=new[]{new EditorBuildSettingsScene("Assets/Manual/Main.unity",true),new EditorBuildSettingsScene("Assets/Scenes/Poc.unity",false)};EditorBuildSettings.scenes=custom;int writes=EditorBuildSettings.writes;
  Need(PocSceneSetup.EnsureBootstrapScene(),"absent bootstrap not created");Need(SceneManager.sceneCount==1&&SceneManager.GetActiveScene().id==manual.id&&manual.isLoaded&&manual.isDirty&&marker.name=="Unsaved animation staging"&&File.ReadAllText(manual.path)==manualDisk,"bootstrap creation closed/saved/changed manual dirty scene");PocSceneSetup.InitializeBuildSettings(true);Need(Object.ReferenceEquals(EditorBuildSettings.scenes,custom)&&EditorBuildSettings.writes==writes,"custom build order/enabled list reset");
  EditorSceneManager.leaveNewInactive=true;Need(PocSceneSetup.EnsureBootstrapScene("Assets/Proof/NeedsActivation.unity")&&SceneManager.GetActiveScene()==manual&&manual.isDirty,"new inactive scene was not activated/restored");SceneManager.refuseActivation=true;int beforeFailureSaves=EditorSceneManager.saves;Reject(()=>PocSceneSetup.EnsureBootstrapScene("Assets/Proof/NoActivation.unity"),"genuine activation failure was ignored");Need(EditorSceneManager.saves==beforeFailureSaves&&SceneManager.sceneCount==1&&SceneManager.GetActiveScene()==manual&&manual.isDirty&&!File.Exists("Assets/Proof/NoActivation.unity"),"activation failure modified manual scene or saved to wrong scene");SceneManager.refuseActivation=false;EditorSceneManager.leaveNewInactive=false;
  File.WriteAllText(PocSceneSetup.BootstrapPath+".meta","manual-guid");string bootstrap=File.ReadAllText(PocSceneSetup.BootstrapPath);int saves=EditorSceneManager.saves;
  Need(!PocSceneSetup.EnsureBootstrapScene()&&EditorSceneManager.saves==saves&&File.ReadAllText(PocSceneSetup.BootstrapPath)==bootstrap&&File.ReadAllText(PocSceneSetup.BootstrapPath+".meta")=="manual-guid","existing bootstrap or GUID overwritten");Need(SceneManager.GetActiveScene().id==manual.id&&manual.isDirty&&SceneManager.sceneCount==1,"preview inspection changed open scene setup");
  var open=SceneManager.Make(EditorSceneManager.disk[PocSceneSetup.BootstrapPath].Clone(),false);SceneManager.SetActiveScene(open);var art=new GameObject("Handmade animated prop");EditorSceneManager.SaveScene(open,PocSceneSetup.BootstrapPath);string artDisk=File.ReadAllText(PocSceneSetup.BootstrapPath);art.name="Unsaved keyframe edit";SceneManager.records[open.id].dirty=true;SceneManager.SetActiveScene(manual);saves=EditorSceneManager.saves;int closed=EditorSceneManager.closed,previews=EditorSceneManager.previews;
  Need(!PocSceneSetup.EnsureBootstrapScene()&&EditorSceneManager.saves==saves&&EditorSceneManager.closed==closed&&EditorSceneManager.previews==previews&&open.isLoaded&&open.isDirty&&art.name=="Unsaved keyframe edit"&&File.ReadAllText(open.path)==artDisk,"loaded authored bootstrap was replaced/saved/reopened");
  var untitled=EditorSceneManager.NewScene(NewSceneSetup.EmptyScene,NewSceneMode.Additive);var unsaved=new GameObject("Unsaved untitled scene work");SceneManager.records[untitled.id].dirty=true;saves=EditorSceneManager.saves;closed=EditorSceneManager.closed;int count=SceneManager.sceneCount;
  Need(!PocSceneSetup.EnsureBootstrapScene()&&untitled.isDirty&&unsaved.name=="Unsaved untitled scene work"&&SceneManager.GetActiveScene().id==untitled.id,"existing bootstrap preparation disrupted dirty untitled work");
  foreach(bool dirty in new[]{true,false}){SceneManager.records[untitled.id].dirty=dirty;bool rejected=false;try{PocSceneSetup.EnsureBootstrapScene("Assets/UntitledGuard/Missing.unity");}catch(InvalidOperationException error){rejected=error.Message.Contains("Save the untitled scene explicitly");}Need(rejected&&EditorSceneManager.saves==saves&&EditorSceneManager.closed==closed&&SceneManager.sceneCount==count&&untitled.isLoaded&&untitled.isDirty==dirty&&SceneManager.GetActiveScene().id==untitled.id&&!Directory.Exists("Assets/UntitledGuard"),"missing bootstrap+untitled must reject before file/scene mutation, for both dirty and clean untitled");}
  EditorSceneManager.CloseScene(untitled,true);SceneManager.SetActiveScene(manual);
  foreach(string mode in new[]{"missing host","duplicate host","disabled host"}){
   string path="Assets/Proof/"+mode.Replace(' ','_')+".unity";var bad=EditorSceneManager.NewScene(NewSceneSetup.EmptyScene,NewSceneMode.Additive);var go=new GameObject("Custom incompatible scene");if(mode!="missing host"){var host=go.AddComponent<AngelsDemonsPoc>();if(mode=="disabled host")host.enabled=false;else go.AddComponent<AngelsDemonsPoc>();}EditorSceneManager.SaveScene(bad,path);EditorSceneManager.CloseScene(bad,true);SceneManager.SetActiveScene(manual);string before=File.ReadAllText(path);saves=EditorSceneManager.saves;Reject(()=>PocSceneSetup.EnsureBootstrapScene(path),"incompatible bootstrap accepted: "+mode);Need(File.ReadAllText(path)==before&&EditorSceneManager.saves==saves&&SceneManager.GetActiveScene().id==manual.id,"incompatible bootstrap rewritten or scene context changed");
  }
  saves=EditorSceneManager.saves;closed=EditorSceneManager.closed;Reject(()=>PocSceneSetup.BuildScenes(),"build silently accepted dirty open scenes");Need(EditorSceneManager.saves==saves&&EditorSceneManager.closed==closed&&manual.isDirty&&open.isDirty,"dirty build rejection saved/closed manual work");
  SceneManager.records[manual.id].dirty=false;SceneManager.records[open.id].dirty=false;Directory.CreateDirectory("Assets/Manual");File.WriteAllText("Assets/Manual/Outro.unity","authored outro");EditorBuildSettings.scenes=new[]{new EditorBuildSettingsScene("Assets/Manual/Main.unity",true),new EditorBuildSettingsScene("Assets/Scenes/Poc.unity",false),new EditorBuildSettingsScene("Assets/Manual/Outro.unity",true)};var paths=PocSceneSetup.BuildScenes();Need(paths.Length==2&&paths[0]=="Assets/Manual/Main.unity"&&paths[1]=="Assets/Manual/Outro.unity","builder forced single Poc scene or changed enabled order");
  EditorBuildSettings.scenes=new EditorBuildSettingsScene[0];writes=EditorBuildSettings.writes;PocSceneSetup.InitializeBuildSettings(false);Need(EditorBuildSettings.writes==writes&&EditorBuildSettings.scenes.Length==0,"intentionally empty existing build settings overwritten");Reject(()=>PocSceneSetup.BuildScenes(),"empty build list silently repaired");PocSceneSetup.InitializeBuildSettings(true);Need(EditorBuildSettings.scenes.Length==1&&EditorBuildSettings.scenes[0].path==PocSceneSetup.BootstrapPath&&EditorBuildSettings.scenes[0].enabled,"genuinely new setup missing bootstrap build default");
  SceneManager.records[manual.id].dirty=true;SceneManager.SetActiveScene(manual);EditorSceneManager.failSave=true;Reject(()=>PocSceneSetup.EnsureBootstrapScene("Assets/Proof/Failed.unity"),"failed initial save reported success");Need(SceneManager.GetActiveScene().id==manual.id&&manual.isDirty&&SceneManager.sceneCount==2,"failed save lost preexisting open scenes");
  Console.WriteLine("bootstrap once; authored/open/dirty scenes+GUIDs/build lists preserved; incompatible+dirty build rejected PASS");
 }
}`)
}

func TestEditorPrepareDoesNotSaveManualAssetsOrForceBuildScene(t *testing.T) {
	prepare := sceneTemplateSection(t, "editor.cs.tmpl", "    public static void Prepare()", "    static void PreserveSDKShaders()")
	for _, bad := range []string{"AssetDatabase.SaveAssets(", "NewSceneMode.Single", "SaveOpenScenes(", "SaveCurrentModifiedScenesIfUserWantsTo(", "EditorBuildSettings.scenes ="} {
		if strings.Contains(prepare, bad) {
			t.Errorf("Prepare may destroy or persist manual work: %s", bad)
		}
	}
	for _, need := range []string{"PocSceneSetup.EnsureBootstrapScene()", "PocSceneSetup.InitializeBuildSettings(created)", "if(created&&initialBuildList)", "EditorApplication.isPlayingOrWillChangePlaymode"} {
		if !strings.Contains(prepare, need) {
			t.Errorf("Prepare preservation contract missing %s", need)
		}
	}
	shader := sceneTemplateSection(t, "editor.cs.tmpl", "    static void PreserveSDKShaders()", "    public static void Verify()")
	if !strings.Contains(shader, "SaveAssetIfDirty(assets[0])") || strings.Contains(shader, "SaveAssets()") {
		t.Fatal("shader setup must save only its own modified GraphicsSettings object")
	}
	build := sceneTemplateSection(t, "editor.cs.tmpl", "    public static void BuildMac()", "// Only this new bootstrap")
	if !strings.Contains(build, "BuildPipeline.BuildPlayer(PocSceneSetup.BuildScenes(),") {
		t.Fatal("BuildMac ignores authored enabled build scenes")
	}
}

func TestEditorScenePreservationProofIsOptInAndDoesNotBuild(t *testing.T) {
	proof := sceneTemplateSection(t, "editor.cs.tmpl", "public static class PocScenePreservationProof", "// Only this new bootstrap")
	for _, need := range []string{"!Application.isBatchMode", `GetEnvironmentVariable("ANG_SCENE_PROOF_STAGING")!="1"`, `GetEnvironmentVariable("ANG_SCENE_PROOF_DIR")`, "manual_animation_preserved", "EditorUtility.IsDirty(clip)", "EditorUtility.IsDirty(material)", "Digest(PocSceneSetup.BootstrapPath", "PocBuild.Prepare();PocBuild.Prepare();", "EditorBuildSettings.scenes=originalBuild", "scene.IsValid()&&scene.isLoaded", "AssetDatabase.DeleteAsset(folder)", "report.unity_errors==0", "originalActive.isDirty||originalActive.GetRootGameObjects().Length!=0", "EditorSceneManager.GetSceneManagerSetup()", "if(originalSetup.Length==0)EditorSceneManager.NewScene(NewSceneSetup.EmptyScene,NewSceneMode.Single)", "report.startup_scene_restored"} {
		if !strings.Contains(proof, need) {
			t.Errorf("staging preservation acceptance missing %s", need)
		}
	}
	for _, forbidden := range []string{"BuildPipeline.", "PocBuild.BuildMac(", "AssetDatabase.SaveAssets("} {
		if strings.Contains(proof, forbidden) {
			t.Errorf("opt-in preservation proof must not build/save arbitrary work: %s", forbidden)
		}
	}
}
