package gamepoc

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/emitter"
)

func TestWayfindingLanesMergeClassTrunksAndNeverBridgeUnknownFloorNative(t *testing.T) {
	methods := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  static Color WayfindingColor(", "  Mesh WayfindingMesh(")
	stroke := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  sealed class WayfindingStroke ", "  ulong wayfindingHash;")
	// Keep the actual geometry/cache implementation; only unrelated wire and
	// Unity primitives are supplied by this executable deterministic harness.
	runPaintedTextureNative(t, "WayfindingGeometry", `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float xx,float yy){x=xx;y=yy;}public static Vector2 zero{get{return new Vector2();}}public float sqrMagnitude{get{return x*x+y*y;}}public Vector2 normalized{get{float n=(float)Math.Sqrt(sqrMagnitude);return n>0?this*(1/n):zero;}}public static Vector2 operator+(Vector2 a,Vector2 b){return new Vector2(a.x+b.x,a.y+b.y);}public static Vector2 operator-(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}public static Vector2 operator*(Vector2 a,float b){return new Vector2(a.x*b,a.y*b);}}
struct Color {public float r,g,b,a;public Color(float rr,float gg,float bb){r=rr;g=gg;b=bb;a=1;}public Color(float rr,float gg,float bb,float aa){r=rr;g=gg;b=bb;a=aa;}}
static class ColorUtility {public static bool TryParseHtmlString(string s,out Color c){c=new Color(.4f,.6f,.7f);return s!=null;}}
static class Mathf {public static float Sqrt(float f){return (float)Math.Sqrt(f);}public static float Min(float a,float b){return Math.Min(a,b);}public static int Clamp(int a,int b,int c){return Math.Max(b,Math.Min(c,a));}}
class PocPoint {public int X,Y;public PocPoint(int x,int y){X=x;Y=y;}}
class PocWayfindingGuide {public string Id,Color;public List<PocPoint> Path=new List<PocPoint>();}
class PocWayfindingDestination {public string Id,Color;public int X,Y,Number;}
class PocWayfindingView {public string LevelId="public",SelectedDestinationId="",SelectedColor="#445566";public List<PocWayfindingGuide> Guides=new List<PocWayfindingGuide>();public List<PocPoint> Path=new List<PocPoint>();public List<PocWayfindingDestination> Destinations=new List<PocWayfindingDestination>();}
class Proof {
`+strings.Replace(stroke, "  ulong wayfindingHash;", "", 1)+`
 ulong wayfindingHash;int wayfindingGuideSegments,wayfindingSelectedSegments;bool wayfindingProofHideSelected;PocWayfindingView view=new PocWayfindingView();HashSet<string> hidden=new HashSet<string>();
 PocWayfindingView WayfindingView(){return view;}object ObserverView(){return null;}char ViewCell(object o,int x,int y){return hidden.Contains(x+","+y)?'0':'1';}
`+methods+`
 static void Need(bool v,string why){if(!v)throw new Exception(why);}
 static PocWayfindingGuide Guide(string id,string color,bool reverse=false){var g=new PocWayfindingGuide{Id=id,Color=color};for(int x=0;x<4;x++)g.Path.Add(new PocPoint(reverse?3-x:x,0));return g;}
 public static void Main(){var p=new Proof();p.view.Guides.Add(Guide("shop.a","#AA9977"));p.view.Guides.Add(Guide("shop.b","#aa9977",true));p.PrepareWayfinding();Need(p.wayfindingGuideSegments==3,"same-hue shop trunk was duplicated into parallel lines");
 p.view.Guides.Add(Guide("medical","#6688CC"));p.PrepareWayfinding();Need(p.wayfindingGuideSegments==6,"different classes lost distinct parallel lanes");bool low=false,high=false;foreach(var s in p.wayfindingStrokes){if(s.a.y<-.01f)low=true;if(s.a.y>.01f)high=true;}Need(low&&high,"shared corridor classes lie on top of each other");
 float pixelsPerUnit=2496f/(float)Math.Sqrt(48*48+26*26)*.9375f;Need(2*WayfindingGuideHalfWidth(2)*pixelsPerUnit>=3&&2*.065f*pixelsPerUnit>=5,"reference 1920px floor strokes are below readable width");for(int lanes=2;lanes<=24;lanes++){float spacing=WayfindingLaneSpacing(lanes),half=WayfindingGuideHalfWidth(lanes);Need(2*half<spacing,"thicker parallel guide lanes merged");Need((lanes-1)*spacing/2+half<.5f,"guide stroke left known cell clearance");}var subdued=WayfindingColor("#AA9977",false);var bright=WayfindingColor("#AA9977",true);Need(bright.r>subdued.r&&bright.g>subdued.g&&bright.b>subdued.b&&bright.a==1,"selected tint has no raised luminance");
 var a=WayfindingLane(0,0,1,0,0,3);var b=WayfindingLane(1,0,0,0,0,3);Need((a-b).sqrMagnitude<.000001f,"opposite route direction swapped a lane");Need(Math.Abs(a.y)<.35f,"lane left the supporting cell envelope");
 int count=p.wayfindingStrokes.Count;var first=p.wayfindingStrokes[0];p.PrepareWayfinding();Need(p.wayfindingStrokes.Count==count&&Object.ReferenceEquals(first,p.wayfindingStrokes[0]),"unchanged guides rebuild frame geometry");
 p.hidden.Add("1,0");p.PrepareWayfinding();foreach(var s in p.wayfindingStrokes)Need(!(s.ax==1&&s.ay==0)&&!(s.bx==1&&s.by==0),"guide bridged an unknown floor cell");
 p.view.Destinations.Add(new PocWayfindingDestination{Id="medical",Color="#445566",X=3,Y=0,Number=7});p.PrepareWayfinding();Need(p.wayfindingNumbers["medical"]==7,"authored room number lost");p.view.Destinations.Add(new PocWayfindingDestination{Id="earlier.discovery",Color="#775544",X=0,Y=0,Number=2});p.PrepareWayfinding();Need(p.wayfindingNumbers["medical"]==7&&p.wayfindingNumbers["earlier.discovery"]==2,"new discovery renumbered an existing room");
 p.view.Path.Add(new PocPoint(2,0));p.view.Path.Add(new PocPoint(3,0));p.PrepareWayfinding();Need(p.wayfindingSelectedSegments==1,"bright selection path was omitted");p.wayfindingProofHideSelected=true;p.PrepareWayfinding();Need(p.wayfindingSelectedSegments==0,"native floor contribution control did not hide selected geometry");
 Console.WriteLine("shared class trunks, parallel opposing lanes, cache reuse, unknown-floor clipping and selected geometry PASS");}
}`)
}

func TestWayfindingSelectionAndExplicitGoAreDifferentCommandsNative(t *testing.T) {
	commands := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  bool WayfindingSelect(", "  Rect WayfindingPanel(")
	runPaintedTextureNative(t, "WayfindingIntent", `using System;using System.Collections.Generic;
class View {public string SelectedDestinationId="",Status="ready";public bool Reachable=true;}
class State {public bool Paused,Manual=true;public int X=7,Y=9;public List<int> Path=new List<int>();}
class Proof {State state=new State();View view=new View();string wayfindingRequested;bool wayfindingPending;List<string> wayfindingChoices=new List<string>();List<string> sent=new List<string>();bool CanSend(){return true;}View WayfindingView(){return view;}void LifeCommand(string action,string destination=""){sent.Add(action+"/"+destination);}
`+commands+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}public static void Main(){var p=new Proof();Need(p.WayfindingSelect("room.medical")&&p.sent[0]=="route_preview/room.medical","selection is not read-only preview request");Need(p.state.X==7&&p.state.Y==9&&p.state.Manual&&p.state.Path.Count==0,"selection touched hero state");Need(!p.WayfindingGo()&&p.sent.Count==1,"Go used a stale pending selection");p.view.SelectedDestinationId="room.medical";p.wayfindingPending=false;p.state.Paused=true;Need(!p.WayfindingGo(),"paused Go dispatched movement");p.state.Paused=false;Need(p.WayfindingGo()&&p.sent[1]=="route_go/room.medical","explicit Go did not repeat selected stable ID");p.WayfindingClear();Need(p.sent[2]=="route_clear/"&&p.state.X==7&&p.state.Path.Count==0,"clear mutated or moved hero");Console.WriteLine("selection/Go/clear separation, stale response and pause guards PASS");}}
`)
}

func TestWayfindingProofUsesTemporaryRussianLocaleNative(t *testing.T) {
	methods := sceneTemplateSection(t, "wayfinding_proof.cs.tmpl", "  void BeginWayfindingProofLocale(", "  void InitWayfindingProof(")
	runPaintedTextureNative(t, "WayfindingProofLocale", `using System;
static class PlayerPrefs{public static string saved="en";public static int writes;public static void SetString(string key,string value){saved=value;writes++;}}
class Proof{string localePreference="en",wayfindingProofOriginalLocale;int sent;string lastWire;string CurrentLocale(){return localePreference;}string ValidClientLocale(string value){return value=="ru"?"ru":"en";}void SendLocalePreference(){sent++;lastWire=CurrentLocale();}void SetLocale(string value){localePreference=value;PlayerPrefs.SetString("locale",value);}
`+methods+`
 static void Need(bool v,string why){if(!v)throw new Exception(why);}public static void Main(){var p=new Proof();p.BeginWayfindingProofLocale();Need(p.CurrentLocale()=="ru"&&p.lastWire=="ru"&&p.sent==1,"proof UI and server locale differ");Need(PlayerPrefs.saved=="en"&&PlayerPrefs.writes==0,"proof overwrote player language preference");p.RestoreWayfindingProofLocale();p.RestoreWayfindingProofLocale();Need(p.CurrentLocale()=="en"&&PlayerPrefs.saved=="en"&&PlayerPrefs.writes==0,"temporary language did not restore cleanly");Console.WriteLine("proof UI+wire RU, original locale restored, zero persistent preference writes PASS");}}
`)
	exit := sceneTemplateSection(t, "unity.cs.tmpl", "        void ExitProof(bool ok)", "        void Quit()")
	if !strings.Contains(exit, "RestoreWayfindingProofLocale()") || strings.Index(exit, "RestoreWayfindingProofLocale()") > strings.Index(exit, "DisposeHost()") {
		t.Fatal("proof must restore temporary locale after captures and before closing its session")
	}
}

func TestWayfindingCanvasResourcesReusedAcrossRouteUpdatesNative(t *testing.T) {
	draw := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  void DrawPaintedWayfinding(", "  void UpdateSDKWayfinding(")
	release := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  void ReleaseWayfindingCanvas(", "  void ReleaseSDKWayfinding(")
	runPaintedTextureNative(t, "WayfindingCanvasReuse", `using System;using System.Collections.Generic;
struct Vector2{public float x,y;public Vector2(float xx,float yy){x=xx;y=yy;}public static Vector2 operator-(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}}
struct Vector3{public float x,y,z;public Vector3(float xx,float yy,float zz){x=xx;y=yy;z=zz;}}
struct Rect{public float x,y,width,height;public Vector2 position{get{return new Vector2(x,y);}set{x=value.x;y=value.y;}}}
struct Color{public static Color clear,white;}
struct Matrix4x4{public static Matrix4x4 identity;}
enum FilterMode{Bilinear}enum RenderTextureFormat{ARGB32}enum ScaleMode{StretchToFill}enum ColorSpace{Linear,Gamma}
static class QualitySettings{public static ColorSpace activeColorSpace=ColorSpace.Gamma;}
class Mesh{public static int made;public Mesh(){made++;}}
class Shader{public static Shader Find(string s){return new Shader();}}
class Material{public static int made;public Material(Shader s){made++;}public bool SetPass(int i){return true;}}
class RenderTexture{public static RenderTexture active;public static int made,released;public int width,height,antiAliasing;public FilterMode filterMode;public RenderTexture(int w,int h,int d,RenderTextureFormat f){width=w;height=h;made++;}public void Create(){}public void Release(){released++;}}
static class GL{public static bool sRGBWrite=true;public static int pushes;public static void PushMatrix(){pushes++;}public static void PopMatrix(){pushes--;}public static void Clear(bool a,bool b,Color c){}public static void LoadPixelMatrix(int a,int b,int c,int d){}}
static class Graphics{public static int draws;public static bool fail;public static void DrawMeshNow(Mesh m,Matrix4x4 matrix){draws++;if(fail)throw new Exception("GPU failure");}}
static class GUI{public static Color color;public static void DrawTexture(Rect r,RenderTexture t,ScaleMode s,bool a){}}
static class Debug{public static int errors;public static void LogError(string s){errors++;}}
class PocPaintedScene{public string level_id="market";public int width=2560,height=1441;}
class Proof{List<int> wayfindingStrokes=new List<int>{1};ulong wayfindingHash=1,wayfindingCanvasHash;string wayfindingCanvasScene,wayfindingRenderFailure;RenderTexture wayfindingCanvas;Mesh wayfindingCanvasMesh;Material wayfindingCanvasMaterial;int destroyed;void Destroy(object o){destroyed++;}void PrepareWayfinding(){}Vector2 PaintedPoint(PocPaintedScene s,Vector2 p){return p;}Mesh WayfindingMesh(Func<Vector2,Vector3> p,Mesh reuse=null){return reuse??new Mesh();}
`+draw+release+`
 static void Need(bool v,string why){if(!v)throw new Exception(why);}public static void Main(){var p=new Proof();var scene=new PocPaintedScene();var sentinel=new RenderTexture(1,1,0,RenderTextureFormat.ARGB32);RenderTexture.active=sentinel;int initial=RenderTexture.made;
 p.DrawPaintedWayfinding(scene,new Rect(),new Rect());var texture=p.wayfindingCanvas;var mesh=p.wayfindingCanvasMesh;var material=p.wayfindingCanvasMaterial;
 for(int i=0;i<100;i++){p.wayfindingHash++;p.DrawPaintedWayfinding(scene,new Rect(),new Rect());Need(Object.ReferenceEquals(texture,p.wayfindingCanvas)&&Object.ReferenceEquals(mesh,p.wayfindingCanvasMesh)&&Object.ReferenceEquals(material,p.wayfindingCanvasMaterial),"route update recreated retained GPU resources");}
 Need(RenderTexture.made==initial+1&&Material.made==1&&Mesh.made==1&&p.destroyed==0,"route churn allocated/destroyed scene-size targets");int draws=Graphics.draws;for(int i=0;i<100;i++)p.DrawPaintedWayfinding(scene,new Rect(),new Rect());Need(Graphics.draws==draws,"unchanged route rasterized on repaint");
 Graphics.fail=true;p.wayfindingHash++;p.DrawPaintedWayfinding(scene,new Rect(),new Rect());draws=Graphics.draws;for(int i=0;i<10;i++)p.DrawPaintedWayfinding(scene,new Rect(),new Rect());Need(Debug.errors==1&&Graphics.draws==draws&&RenderTexture.active==sentinel&&GL.sRGBWrite&&GL.pushes==0,"failed draw retried or corrupted active target/color/matrix state");
 Graphics.fail=false;scene.width++;p.DrawPaintedWayfinding(scene,new Rect(),new Rect());Need(RenderTexture.made==initial+2&&RenderTexture.released==1&&p.destroyed==3&&p.wayfindingRenderFailure==null,"actual canvas resize did not replace only its old resources");p.ReleaseWayfindingCanvas();Need(RenderTexture.released==2&&p.destroyed==6,"final scene cleanup leaked retained resources");Console.WriteLine("100 route updates+warm frames: RT/material/mesh retained; resize/error/cleanup bounded and GPU state restored PASS");}}
`)
}

func TestWayfindingRejectedPreviewAcknowledgementIsCorrelatedNative(t *testing.T) {
	view := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  PocWayfindingView WayfindingView(", "  bool WayfindingSelect(")
	runPaintedTextureNative(t, "WayfindingPreviewAck", `using System;using System.Collections.Generic;
class PocWayfindingView{public string LevelId="market",SelectedDestinationId="",RejectedDestinationId="",Status="none";}
class Life{public bool Enabled=true,Transit;public string LevelId="market";public PocWayfindingView Wayfinding=new PocWayfindingView();}
class State{public string Epoch="epoch";public Life Life=new Life();}class Observer{public string ActorId="char.hero";}
class Proof{State state=new State();string wayfindingEpoch="epoch",wayfindingRequested="B",wayfindingChoiceLevel="market";bool wayfindingPending=true;List<string> wayfindingChoices=new List<string>();Observer ObserverView(){return new Observer();}
`+view+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}public static void Main(){var p=new Proof();p.WayfindingView();Need(p.wayfindingPending,"old none snapshot cleared a pending request");p.state.Life.Wayfinding.Status="unknown";p.state.Life.Wayfinding.RejectedDestinationId="A";p.WayfindingView();Need(p.wayfindingPending,"stale rejection A acknowledged pending B");p.state.Life.Wayfinding.RejectedDestinationId="B";p.WayfindingView();Need(!p.wayfindingPending,"matching rejected preview left pending latched");p.wayfindingPending=true;p.state.Life.Wayfinding.RejectedDestinationId="";p.state.Life.Wayfinding.SelectedDestinationId="B";p.WayfindingView();Need(!p.wayfindingPending,"matching selection did not acknowledge request");p.wayfindingChoices.Add("old.room");p.wayfindingChoiceLevel="old.level";p.WayfindingView();Need(p.wayfindingChoices.Count==0,"old doorway popup survived a physical level change");Console.WriteLine("read-only preview: stale none/unknown ignored, exact rejection/selection acknowledged, level popup invalidated PASS");}}
`)
}

func TestWayfindingSharedDoorRequiresExplicitDestinationChoiceNative(t *testing.T) {
	pointer := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  bool SendWayfindingPointer(", "  void DrawWayfindingChoices(")
	commands := sceneTemplateSection(t, "wayfinding.cs.tmpl", "  bool WayfindingSelect(", "  Rect WayfindingPanel(")
	runPaintedTextureNative(t, "WayfindingSharedDoor", `using System;using System.Collections.Generic;
struct Vector2{public float x,y;public Vector2(float xx,float yy){x=xx;y=yy;}public static Vector2 zero{get{return new Vector2();}}public float sqrMagnitude{get{return x*x+y*y;}}public static Vector2 operator-(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}}
class PocWayfindingDestination{public string Id;public int X,Y;}class PocWayfindingView{public string LevelId="market",SelectedDestinationId="",Status="ready";public bool Reachable=true;public List<PocWayfindingDestination> Destinations=new List<PocWayfindingDestination>();}class State{public bool Paused;public int X=3,Y=4;public List<int> Path=new List<int>();}
class Proof{State state=new State();PocWayfindingView view=new PocWayfindingView();string wayfindingRequested,wayfindingChoiceLevel;bool wayfindingPending;Vector2 wayfindingChoiceAt,wayfindingChoiceScroll;List<string> wayfindingChoices=new List<string>();List<string> sent=new List<string>();PocWayfindingView WayfindingView(){return view;}bool CanSend(){return true;}bool HudBlocksWorldInput(Vector2 p){return false;}bool WayfindingMarkerScreen(PocWayfindingDestination d,out Vector2 p){p=new Vector2(d.X*20,d.Y*20);return true;}void LifeCommand(string a,string d=""){sent.Add(a+"/"+d);}
`+commands+pointer+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}public static void Main(){var p=new Proof();p.view.Destinations.Add(new PocWayfindingDestination{Id="walk.free",X=5,Y=6});p.view.Destinations.Add(new PocWayfindingDestination{Id="rail.paid",X=5,Y=6});Need(p.SendWayfindingPointer(new Vector2(100,120))&&p.wayfindingChoices.Count==2&&p.sent.Count==0,"shared door arbitrarily chose first route or moved on popup open");Need(!p.WayfindingChoose("hidden.destination")&&p.sent.Count==0,"popup fabricated an unknown choice");Need(p.WayfindingChoose("rail.paid")&&p.sent.Count==1&&p.sent[0]=="route_preview/rail.paid"&&p.state.X==3&&p.state.Y==4&&p.state.Path.Count==0,"specific shared-door choice did not remain read-only");Need(p.wayfindingChoices.Count==0,"chosen popup did not close");p.SendWayfindingPointer(new Vector2(100,120));Need(p.SendWayfindingPointer(new Vector2(800,700))&&p.wayfindingChoices.Count==0&&p.sent.Count==1,"outside dismissal fell through to movement");Console.WriteLine("co-located walk/rail: popup opens without order, authorized choice previews only, dismissal consumes click PASS");}}
`)
}

func TestWayfindingCompositionAndNativeProofContracts(t *testing.T) {
	painted := hudTemplate(t, "painted.cs.tmpl")
	floor := strings.Index(painted, "DrawPaintedWayfinding(scene,fit,viewport)")
	foreground := strings.Index(painted, "foreach(var draw in paintedDraws)if(draw.kind==2)")
	actors := strings.Index(painted, "foreach(var draw in paintedDraws)if(draw.kind!=2)")
	if floor < 0 || floor >= foreground || foreground >= actors {
		t.Fatal("floor guidance may cover occluding architecture or actor sprites")
	}
	region := hudTemplate(t, "region.cs.tmpl")
	if strings.Contains(region, `LifeCommand("goto",node.LevelId)`) || !strings.Contains(region, `WayfindingSelect(nodes[i].LevelId)`) || !strings.Contains(region, `WayfindingGo()`) || !strings.Contains(region, `L("ui.wayfinding.show_floor")`) {
		t.Fatal("regional selection must preview, preserve the first local leg and await explicit Go")
	}
	proof := hudTemplate(t, "wayfinding_proof.cs.tmpl")
	for _, required := range []string{`"--poc-verify-wayfinding"`, `SendWayfindingPointer(pointer)`, `WayfindingProofStill()`, `state.Tick-wayfindingProofStartTick<8`, `WayfindingGo()`, `moved>ticks`, `selected_world_difference_pixels<50`, `wayfindingGuideSegments<1||wayfindingSelectedSegments<2`, `"selected-route.png"`} {
		if !strings.Contains(proof, required) {
			t.Error("missing physical/native wayfinding check: " + required)
		}
	}
}

// This compiles exact runtime templates with protocol messages emitted from the
// current CUE into a disposable directory. It does not regenerate the project or
// launch Unity. CI without the book or Unity keeps the deterministic tests above.
func TestWayfindingCSharpTemplatesCompileWithCurrentProtocol(t *testing.T) {
	project := os.Getenv("ANG_WAYFINDING_PROJECT")
	if project == "" {
		t.Skip("set ANG_WAYFINDING_PROJECT for exact CUE/Unity source compile")
	}
	editors, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app")
	if len(editors) == 0 {
		t.Skip("Unity assemblies unavailable")
	}
	api, err := cuePackage(filepath.Join(project, "cue/api"))
	if err != nil {
		t.Fatal(err)
	}
	def, err := normalizer.New().ExtractGRPC(api)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	proto := emitter.GRPCProtoPath(def)
	path := filepath.Join(tmp, proto)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(emitter.RenderGRPCProto(def, "angelsdemons/server")), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("protoc", "-I", tmp, "--csharp_out="+tmp, proto).CombinedOutput(); err != nil {
		t.Fatalf("current protocol: %v\n%s", err, output)
	}
	editor := editors[len(editors)-1]
	mono := filepath.Join(editor, "Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	args := []string{csc, "/nologo", "/target:library", "/out:" + filepath.Join(tmp, "Wayfinding.dll")}
	refs, _ := filepath.Glob(filepath.Join(editor, "Contents/Resources/Scripting/Managed/UnityEngine/UnityEngine*.dll"))
	refs = append(refs, filepath.Join(filepath.Dir(mono), "../lib/mono/4.8-api/Facades/netstandard.dll"), filepath.Join(filepath.Dir(mono), "../lib/mono/unityjit-macos/System.Net.Http.dll"), filepath.Join(filepath.Dir(mono), "../lib/mono/unityjit-macos/Facades/System.Memory.dll"))
	book := filepath.Dir(project)
	transportRefs, _ := filepath.Glob(filepath.Join(book, "client-unity/Library/ScriptAssemblies/Cysharp*.dll"))
	refs = append(refs, transportRefs...)
	if err := filepath.WalkDir(filepath.Join(book, "client-unity/Assets/Plugins/PocDependencies"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".dll") {
			refs = append(refs, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, reference := range refs {
		args = append(args, "/r:"+reference)
	}
	entries, err := sources.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".cs.tmpl") || strings.Contains(name, "editor") {
			continue
		}
		data, _ := sources.ReadFile("templates/" + name)
		dest := filepath.Join(tmp, strings.TrimSuffix(name, ".tmpl"))
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, dest)
	}
	if err := filepath.WalkDir(filepath.Join(book, "client-unity/Assets/Generated/AngelsDemons"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".cs") && !strings.Contains(path, string(filepath.Separator)+"Messages"+string(filepath.Separator)) {
			args = append(args, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	protocol, _ := filepath.Glob(filepath.Join(tmp, "*.cs"))
	for _, path := range protocol {
		if strings.HasSuffix(path, "Game.cs") {
			args = append(args, path)
		}
	}
	if output, err := exec.Command(mono, args...).CombinedOutput(); err != nil {
		t.Fatalf("exact runtime C# templates: %v\n%s", err, output)
	}
}
