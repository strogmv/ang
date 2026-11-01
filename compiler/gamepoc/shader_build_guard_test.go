package gamepoc

import (
	"strings"
	"testing"
)

func TestEditorShaderBuildGuardRejectsTargetCompilerErrorsNative(t *testing.T) {
	guard := sceneTemplateSection(t, "editor.cs.tmpl", "public static class PocShaderBuildGuard", "// Opt-in acceptance only:")
	runPaintedTextureNative(t, "ShaderBuildGuard", `using System;using System.Collections.Generic;using UnityEditor;using UnityEngine;
namespace UnityEngine {public class Shader {public string name;}}
namespace UnityEditor.Rendering {public enum ShaderCompilerMessageSeverity {Error,Warning}}
namespace UnityEditor {
 public struct ShaderMessage {public Rendering.ShaderCompilerMessageSeverity severity;public string message;public int line;}
 public static class AssetDatabase {
  public static readonly HashSet<string> folders=new HashSet<string>();public static readonly Dictionary<string,Shader> assets=new Dictionary<string,Shader>();
  public static bool IsValidFolder(string path){return folders.Contains(path);}
  public static string[] FindAssets(string filter,string[] dirs){if(filter!="t:Shader"||dirs.Length!=1)throw new Exception("bad shader filter");var result=new List<string>();foreach(var pair in assets)if(pair.Key.StartsWith(dirs[0]+"/"))result.Add(pair.Key);return result.ToArray();}
  public static string GUIDToAssetPath(string guid){return guid;}
  public static T LoadAssetAtPath<T>(string path)where T:class{return assets[path] as T;}
 }
 public static class ShaderUtil {public static readonly Dictionary<string,ShaderMessage[]> messages=new Dictionary<string,ShaderMessage[]>();public static ShaderMessage[] GetShaderMessages(Shader shader){ShaderMessage[] value;return messages.TryGetValue(shader.name,out value)?value:new ShaderMessage[0];}}
}
`+guard+`
class Proof {
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 static void Reject(string expected){try{PocShaderBuildGuard.RequireCompiledShaders();}catch(InvalidOperationException e){Need(e.Message.Contains(expected),"missing target compiler diagnostic: "+e.Message);return;}throw new Exception("bad shader accepted");}
 public static void Main(){
  PocShaderBuildGuard.RequireCompiledShaders();
  const string folder="Assets/Resources/Generated",path=folder+"/PocStorefront.shader";AssetDatabase.folders.Add(folder);AssetDatabase.assets[path]=new Shader{name="ANG/Storefront"};
  ShaderUtil.messages["ANG/Storefront"]=new[]{new ShaderMessage{severity=UnityEditor.Rendering.ShaderCompilerMessageSeverity.Warning,message="benign warning",line=2}};PocShaderBuildGuard.RequireCompiledShaders();
  ShaderUtil.messages["ANG/Storefront"]=new[]{new ShaderMessage{severity=UnityEditor.Rendering.ShaderCompilerMessageSeverity.Error,message="syntax error: unexpected token 'point' (on metal)",line=15}};
  Reject("PocStorefront.shader(15): syntax error: unexpected token 'point' (on metal)");
  ShaderUtil.messages.Clear();PocShaderBuildGuard.RequireCompiledShaders();AssetDatabase.assets[path]=null;Reject("shader import is missing");
  Console.WriteLine("target shader error rejects nominally successful build; warning allowed; missing import rejected PASS");
 }
}`)
	build := sceneTemplateSection(t, "editor.cs.tmpl", "    public static void BuildMac()", "public static class PocShaderBuildGuard")
	compile, validate, ready := strings.Index(build, "BuildPipeline.BuildPlayer("), strings.Index(build, "PocShaderBuildGuard.RequireCompiledShaders();"), strings.Index(build, `Debug.Log("POC_PLAYER_READY `)
	if compile < 0 || validate <= compile || ready <= validate {
		t.Fatal("target compiler diagnostics must be checked after BuildPlayer and before readiness")
	}
}
