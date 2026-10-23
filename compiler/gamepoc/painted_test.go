package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaintedGameplayUsesAuthoritativePositionsAndVisibility(t *testing.T) {
	s := hudTemplate(t, "painted.cs.tmpl")
	for _, want := range []string{"foreach(var actor in SceneActors())", "if(!ViewSees(actor.X,actor.Y))continue", "SampleNPC(actor)", "ViewSees(state.PlayerX,state.PlayerY)", "\"char.hero\",shownPosition", "ViewCell(view,x,y)=='0'", "ClickMove(x,y)", "LifeCommand(\"enter\",marker.EdgeId)", "state.Life.Inspections", "obj.Fragments", "HudBlocksWorldInput", "state.Paused||state.Life.Transit", "view.ActorId!=\"char.hero\""} {
		if !strings.Contains(s, want) {
			t.Errorf("missing authoritative painted contract %q", want)
		}
	}
	draw := sceneTemplateSection(t, "painted.cs.tmpl", "  bool DrawPaintedGame(){", "\n }\n}")
	for _, bad := range []string{"scene.actors", "artScene.layers", "DrawGameHud", "DrawSprite(", "glyphSprites", "state.PlayerX=", "SceneAgents.Add"} {
		if strings.Contains(draw, bad) {
			t.Errorf("painted rendering bypasses server authority or old-renderer isolation: %q", bad)
		}
	}
	if strings.Index(draw, "paintedDraws.Sort(paintedComparison)") > strings.Index(draw, "foreach(var draw in paintedDraws)") {
		t.Fatal("draw ordering must precede rendering")
	}
	if strings.Index(draw, "cache.fog,ScaleMode") > strings.Index(draw, "foreach(var draw in paintedDraws)") {
		t.Fatal("ground fog must precede whole visible actor sprites")
	}
	if !strings.Contains(s, "ShadePaintedChangedPixels(cut.pixels,cut.shadedPixels,cache.fogPixels,cache.fogChanged,cut.fogSamples,cache.fogDirty,cache.fog.width)") || strings.Contains(draw, "draw.overlay") {
		t.Fatal("foreground must sample the completed global field without an independent overlay")
	}
}
func TestPaintedCachesRasterizationOutsideFrameLoop(t *testing.T) {
	s := hudTemplate(t, "painted.cs.tmpl")
	for _, want := range []string{"paintedCaches.TryGetValue", "const int maxWidth=512", "cache.fogCells", "if(!changed)return", "static readonly Comparison<PaintedDraw>", "paintedDraws.Clear()", "texture.Apply(true,false)"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing bounded/cache contract %q", want)
		}
	}
	draw := sceneTemplateSection(t, "painted.cs.tmpl", "  bool DrawPaintedGame(){", "\n }\n}")
	for _, bad := range []string{"GetPixels", "new Texture2D", "new List", "PaintedPolygonContains"} {
		if strings.Contains(draw, bad) {
			t.Errorf("frame loop rasterization/allocation %q", bad)
		}
	}
}
func TestPaintedAffineAndPolygonNative(t *testing.T) {
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
	math := sceneTemplateSection(t, "painted.cs.tmpl", "  static Vector2 PaintedPoint(", "  PocPaintedScene CurrentPaintedScene()")
	fit := sceneTemplateSection(t, "painted.cs.tmpl", "  static Rect PaintedFit(", "  void QueuePaintedActor(")
	harness := `using System;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public static Vector2 zero{get{return new Vector2(0,0);}}}
struct Rect {public float x,y,width,height;public Rect(float a,float b,float c,float d){x=a;y=b;width=c;height=d;}public float xMin{get{return x;}}public float yMin{get{return y;}}public float xMax{get{return x+width;}}public float yMax{get{return y+height;}}public Vector2 center{get{return new Vector2(x+width/2,y+height/2);}}public bool Overlaps(Rect b){return xMin<b.xMax&&xMax>b.xMin&&yMin<b.yMax&&yMax>b.yMin;}}
static class Mathf {public static float Abs(float v){return Math.Abs(v);}public static float Min(float a,float b){return Math.Min(a,b);}public static float Clamp(float v,float min,float max){return Math.Max(min,Math.Min(max,v));}}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
class PocPaintedScene {public int width=2048,height=1143,view_width;public PocPaintedPoint origin,axis_x,axis_y;}
class Proof {
` + math + fit + `
 static void Equal(float a,float b){if(Math.Abs(a-b)>.001f)throw new Exception("not equal: "+a+" "+b);}
 public static void Main(){
  foreach(var axes in new[]{new[]{45f,20f,-35f,23f},new[]{-22f,9f,45f,12f},new[]{80f,0f,0f,80f}}){
   var s=new PocPaintedScene{origin=new PocPaintedPoint(870,220),axis_x=new PocPaintedPoint(axes[0],axes[1]),axis_y=new PocPaintedPoint(axes[2],axes[3])};
   foreach(var p in new[]{new Vector2(0,0),new Vector2(17,9),new Vector2(8.25f,3.5f),new Vector2(-1,2)}){
    Vector2 grid;if(!PaintedInverse(s,PaintedPoint(s,p),out grid))throw new Exception("inverse failed");Equal(grid.x,p.x);Equal(grid.y,p.y);
    var f=PaintedFit(s,new Rect(16,104,1248,584));float scale=f.width/s.width;var target=PaintedTarget(new Rect(100,200,60,80),f,scale);Equal((target.x-f.x)/scale,100);Equal((target.y-f.y)/scale,200);
   }
  }
  Vector2 invalid;var degenerate=new PocPaintedScene{origin=new PocPaintedPoint(0,0),axis_x=new PocPaintedPoint(1,1),axis_y=new PocPaintedPoint(2,2)};if(PaintedInverse(degenerate,new Vector2(5,5),out invalid))throw new Exception("singular projection accepted");
  var polygon=new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(10,0),new PocPaintedPoint(10,10),new PocPaintedPoint(5,5),new PocPaintedPoint(0,10)};
  if(!PaintedPolygonContains(polygon,5,2)||PaintedPolygonContains(polygon,5,9)||PaintedPolygonContains(polygon,20,5))throw new Exception("concave polygon mask mismatch");
  var viewport=new Rect(16,104,1248,584);
  var wide=new PocPaintedScene{width=4096,height=2286,view_width=2048,origin=new PocPaintedPoint(400,100),axis_x=new PocPaintedPoint(48,26),axis_y=new PocPaintedPoint(-48,26)};
  var left=PaintedCameraFit(wide,viewport,new Vector2(0,0));Equal(left.xMin,viewport.xMin);Equal(left.yMin,viewport.yMin);
  var right=PaintedCameraFit(wide,viewport,new Vector2(4096,2286));Equal(right.xMax,viewport.xMax);Equal(right.yMax,viewport.yMax);
  var center=PaintedCameraFit(wide,viewport,new Vector2(2048,1143));var focusOnScreen=PaintedCanvasToScreen(new Vector2(2048,1143),center,center.width/wide.width);Equal(focusOnScreen.x,viewport.center.x);Equal(focusOnScreen.y,viewport.center.y);
  if(Math.Abs(center.x-left.x)<1||Math.Abs(center.y-left.y)<1)throw new Exception("camera did not pan");
  // A larger world must preserve the same person size at fixed view_width.
  var small=new PocPaintedScene{width=1000,height=600,view_width=2048};var centeredSmall=PaintedCameraFit(small,viewport,new Vector2(100,400));Equal(centeredSmall.center.x,viewport.center.x);Equal(centeredSmall.center.y,viewport.center.y);Equal(centeredSmall.width/small.width,center.width/wide.width);
  small.view_width=0;var legacy=PaintedFit(small,viewport);var fallback=PaintedCameraFit(small,viewport,new Vector2(-9999,9999));Equal(legacy.x,fallback.x);Equal(legacy.y,fallback.y);Equal(legacy.width,fallback.width);Equal(legacy.height,fallback.height);
  foreach(var focus in new[]{new Vector2(1072,1036),new Vector2(1600,1200),new Vector2(0,0),new Vector2(4096,2286)}){
   var camera=PaintedCameraFit(wide,viewport,focus);float scale=camera.width/wide.width;var grid=new Vector2(26,10);var canvas=PaintedPoint(wide,grid);var screen=PaintedCanvasToScreen(canvas,camera,scale);Vector2 roundtrip;
   if(!PaintedInverse(wide,PaintedScreenToCanvas(screen,camera,scale),out roundtrip))throw new Exception("panned pointer inverse failed");Equal(roundtrip.x,grid.x);Equal(roundtrip.y,grid.y);
   var visible=PaintedVisibleCanvas(viewport,camera,scale);var outside=new Rect(visible.xMax+1,visible.y,30,200);var partial=new Rect(visible.xMax-5,visible.y+10,30,100);
   if(visible.Overlaps(outside)||!visible.Overlaps(partial))throw new Exception("panned sprite culling lost partial or retained outside sprite");
  }
  Console.WriteLine("painted affine + polygon + follow camera + panned pointer + culling PASS");
 }
}`
	dir := t.TempDir()
	source := filepath.Join(dir, "PaintedProof.cs")
	binary := filepath.Join(dir, "PaintedProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile painted proof: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("painted proof: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}

func TestPaintedNativeProofUsesSharedInputAndRealGoState(t *testing.T) {
	s := hudTemplate(t, "painted_proof.cs.tmpl")
	for _, want := range []string{`"--poc-verify-painted"`, `Path.IsPathRooted`, `NewGame(777,"life")`, `LifeCommand("goto",paintedProofScene.level_id)`, `state.Life.Activity!="Прибыл"`, `SendPaintedScreenPointer(paintedProofScene,screen,viewport,fit,scale)`, `paintedProofScene.proof_waypoints[0]`, `paintedProofScene.proof_waypoints[1]`, `paintedProofScene.proof_blocked`, `if(moveAccepted)`, `SaveOrLoad("slot3",true)`, `SaveOrLoad("slot3",false)`, `state.Tick!=savedTick`, `state.Epoch==savedEpoch`, `paintedCameraPanned`, `camera_x=camera.x`, `PaintedInventoryFingerprint()!=paintedSavedInventory`, `paintedInventoryRestored=true`, `authoritative actor cell overlap`, `Event.current.type!=EventType.Repaint`, `ScreenCapture.CaptureScreenshotAsTexture()`, `SplashScreen.isFinished`, `animation_verified=false`, `foreach(var actor in SceneActors())`, `hidden NPC rendered`, `rendered actor has no authoritative identity`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing native painted proof contract %q", want)
		}
	}
	for _, bad := range []string{"ClickMove(3,", "SceneAgents.Add", "new PocAgentState", "ViewCell(view,x,y)=", `scenario="life100"`, "File.Copy("} {
		if strings.Contains(s, bad) {
			t.Errorf("proof invents state, hardcodes map or copies artwork: %q", bad)
		}
	}
	input := sceneTemplateSection(t, "painted.cs.tmpl", "  void HandlePaintedInput(", "  void DrawPaintedKnownDetails(")
	if !strings.Contains(input, "SendPaintedScreenPointer(scene,e.mousePosition,viewport,fit,scale)") || !strings.Contains(input, "PaintedScreenToCanvas(screen,fit,scale)") || !strings.Contains(input, "!viewport.Contains(screen)") || !strings.Contains(input, "HudBlocksWorldInput(screen)") || !strings.Contains(input, "return ClickMove(x,y)") {
		t.Fatal("proof and normal pointer must share the inverse command path")
	}
}

func TestPaintedPublicArchitectureNeverPromotesUnknownPlayableFloor(t *testing.T) {
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
	body := sceneTemplateSection(t, "painted.cs.tmpl", "  static Color32 PaintedFogColor(", "  static Rect PaintedFit(")
	harness := `using System;
struct Color32 {public byte a;public Color32(byte r,byte g,byte b,byte alpha){a=alpha;}}
class PaintedCache {public bool knownArchitecture; public bool[] publicShell; public char[] visibility;}
class Proof {
` + body + `
 static void Alpha(PaintedCache c,int cell,int alpha){if(PaintedFogColor(c,cell).a!=alpha)throw new Exception("fog leak or overmask at "+cell);}
 public static void Main(){
  var c=new PaintedCache{knownArchitecture=true,publicShell=new[]{false,true,false,false},visibility=new[]{'0','0','1','2'}};
  Alpha(c,0,255);Alpha(c,1,0);Alpha(c,2,145);Alpha(c,3,0);Alpha(c,-1,0);Alpha(c,-2,0);
  c.knownArchitecture=false;Alpha(c,0,255);Alpha(c,1,255);Alpha(c,-1,145);Alpha(c,-2,0);
  Console.WriteLine("painted public shell preserves unknown playable floor PASS");
 }
}`
	dir := t.TempDir()
	source := filepath.Join(dir, "FogProof.cs")
	binary := filepath.Join(dir, "FogProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile fog proof: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("fog proof: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}

func TestPaintedFogFeatherPrivacyAndForegroundSeamsNative(t *testing.T) {
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
	body := sceneTemplateSection(t, "painted.cs.tmpl", "  static void SoftenPaintedFog(", "  static Color32 PaintedFogColor(")
	allocationBody := body[:strings.Index(body, "  static PaintedFogSamples BuildPaintedFogPixels(")] + body[strings.Index(body, "  static bool ShadePaintedChangedPixels("):]
	for _, bad := range []string{"new Color32[", "new byte[", "new int[", "new List", "GetPixels"} {
		if strings.Contains(allocationBody, bad) {
			t.Fatalf("visibility update allocates or reads back source pixels: %s", bad)
		}
	}
	harness := `using System;
struct Color32 {public byte r,g,b,a;public Color32(byte red,byte green,byte blue,byte alpha){r=red;g=green;b=blue;a=alpha;}}
struct PaintedFogAxis {public int a,b,weight;}
struct PaintedFogPixel {public int source,a;public ushort row,wx,wy;public byte column;}
struct PaintedFogBucket {public int offset,count,left,bottom,right,top;}
class PaintedFogSamples {public PaintedFogPixel[] pixels;public PaintedFogBucket[] buckets;}
class Proof {
` + body + `
 static void Equal(Color32 a,Color32 b){if(a.r!=b.r||a.g!=b.g||a.b!=b.b||a.a!=b.a)throw new Exception("background/foreground fog sample mismatch");}
 public static void Main(){
  const int w=21,h=13;var raw=new Color32[w*h];var scratch=new Color32[w*h];var field=new Color32[w*h];var distance=new byte[w*h];
  for(int y=0;y<h;y++)for(int x=0;x<w;x++)raw[y*w+x]=x>=12?new Color32(18,23,25,145):new Color32(0,0,0,0);
  for(int y=4;y<=8;y++)for(int x=0;x<=4;x++)raw[y*w+x]=new Color32(8,13,16,255);
  SoftenPaintedFog(raw,scratch,field,distance,w,h);
  if(field[10].a<=0||field[10].a>=145||field[13].a<=0||field[13].a>=145)throw new Exception("remembered/visible edge was not softened");
  for(int y=0;y<h;y++)for(int x=0;x<w;x++)if(raw[y*w+x].a==255){
   if(field[y*w+x].a!=255)throw new Exception("unknown pixel became translucent");
   foreach(float dy in new[]{-.499f,0,.499f})foreach(float dx in new[]{-.499f,0,.499f}){
    var sample=SamplePaintedFog(field,PaintedFogSample((x+.5f+dx)/w,w,1),PaintedFogSample((y+.5f+dy)/h,h,w));
    if(sample.a!=255)throw new Exception("bilinear filtering exposed unknown texel edge");
    Equal(CompositePaintedFog(new Color32(250,240,230,255),sample),new Color32(8,13,16,255));
   }
  }
  const int sw=42,sh=26,cw=17,ch=9,left=7,bottom=5;var source=new Color32[sw*sh];var whole=new Color32[source.Length];var cut=new Color32[cw*ch];var shadedCut=new Color32[cut.Length];
  var allX=new PaintedFogAxis[sw];var allY=new PaintedFogAxis[sh];var cutX=new PaintedFogAxis[cw];var cutY=new PaintedFogAxis[ch];
  for(int x=0;x<sw;x++)allX[x]=PaintedFogSample((x+.5f)/sw,w,1);for(int y=0;y<sh;y++)allY[y]=PaintedFogSample((y+.5f)/sh,h,w);
  for(int y=0;y<sh;y++)for(int x=0;x<sw;x++)source[y*sw+x]=new Color32((byte)(80+x*3),(byte)(60+y*5),120,(byte)((x+y)%7==0?0:(x+y)%7==1?127:255));
  // A cropped plate uses a top-left canvas rectangle; its pixels are bottom-up.
  float canvasW=420,canvasH=260,rectX=left*10,rectY=canvasH-(bottom+ch)*10,rectW=cw*10,rectH=ch*10;
  for(int x=0;x<cw;x++)cutX[x]=PaintedFogSample((rectX+(x+.5f)*rectW/cw)/canvasW,w,1);
  for(int y=0;y<ch;y++)cutY[y]=PaintedFogSample(1f-(rectY+(ch-y-.5f)*rectH/ch)/canvasH,h,w);
  for(int y=0;y<ch;y++)for(int x=0;x<cw;x++)cut[y*cw+x]=source[(y+bottom)*sw+x+left];
  ShadePaintedPixels(source,whole,field,allX,allY);ShadePaintedPixels(cut,shadedCut,field,cutX,cutY);
  for(int y=0;y<ch;y++)for(int x=0;x<cw;x++){Equal(shadedCut[y*cw+x],whole[(y+bottom)*sw+x+left]);if(shadedCut[y*cw+x].a!=cut[y*cw+x].a)throw new Exception("foreground polygon alpha changed");}
  var previous=new Color32[field.Length];var dirty=new int[(w+1)*(h+1)];var changedPixels=new byte[field.Length];var sparse=BuildPaintedFogPixels(cut,cutX,cutY,w,h);var fast=(Color32[])cut.Clone();
  if(sparse.pixels.Length>=cut.Length)throw new Exception("transparent polygon pixels were not excluded from sparse cache");
  BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);ShadePaintedChangedPixels(cut,fast,field,changedPixels,sparse,dirty,w);
  for(int i=0;i<cut.Length;i++)Equal(fast[i],shadedCut[i]);
  Array.Copy(field,previous,field.Length);
  if(BuildPaintedFogDirty(previous,field,dirty,w,h)!=0||PaintedFogRegionChanged(dirty,w,0,0,w-1,h-1))throw new Exception("unchanged field needlessly refreshes foreground");
  var changed=field[6*w+10];changed.a=(byte)(changed.a==255?254:255);field[6*w+10]=changed;
  if(BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels)!=1||!PaintedFogRegionChanged(dirty,w,9,5,11,7)||PaintedFogRegionChanged(dirty,w,0,0,8,4))throw new Exception("dirty-field crop skip missed or invented a change");
  ShadePaintedChangedPixels(cut,fast,field,changedPixels,sparse,dirty,w);ShadePaintedPixels(cut,shadedCut,field,cutX,cutY);for(int i=0;i<cut.Length;i++)Equal(fast[i],shadedCut[i]);
  Array.Copy(field,previous,field.Length);BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);if(ShadePaintedChangedPixels(cut,fast,field,changedPixels,sparse,dirty,w))throw new Exception("unchanged field requests a texture upload");
  var outside=field[0];outside.a=(byte)(outside.a==255?254:255);field[0]=outside;BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);if(ShadePaintedChangedPixels(cut,fast,field,changedPixels,sparse,dirty,w))throw new Exception("field change outside polygon crop requests an upload");
  var allSparse=BuildPaintedFogPixels(source,allX,allY,w,h);var fastWhole=(Color32[])source.Clone();Array.Clear(previous,0,previous.Length);BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);ShadePaintedChangedPixels(source,fastWhole,field,changedPixels,allSparse,dirty,w);
  Array.Copy(field,previous,field.Length);var border=field[6*w+16];border.a=(byte)(border.a==255?0:255);field[6*w+16]=border;BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);ShadePaintedChangedPixels(source,fastWhole,field,changedPixels,allSparse,dirty,w);ShadePaintedPixels(source,whole,field,allX,allY);for(int i=0;i<source.Length;i++)Equal(fastWhole[i],whole[i]);
  var counter=typeof(GC).GetMethod("GetAllocatedBytesForCurrentThread",Type.EmptyTypes);
  if(counter!=null){var bytes=(Func<long>)Delegate.CreateDelegate(typeof(Func<long>),counter);SoftenPaintedFog(raw,scratch,field,distance,w,h);BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);ShadePaintedChangedPixels(cut,fast,field,changedPixels,sparse,dirty,w);long before=bytes();for(int i=0;i<100;i++){SoftenPaintedFog(raw,scratch,field,distance,w,h);BuildPaintedFogDirty(previous,field,dirty,w,h,changedPixels);ShadePaintedChangedPixels(cut,fast,field,changedPixels,sparse,dirty,w);}long allocated=bytes()-before;if(allocated!=0)throw new Exception("fog update allocated "+allocated+" bytes");}
  Console.WriteLine("fog feather: opaque unknown bilinear edges, shared-field crop seams, preserved polygon alpha, cached buffers PASS");
 }
}`
	dir := t.TempDir()
	source := filepath.Join(dir, "FeatherProof.cs")
	binary := filepath.Join(dir, "FeatherProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile feather proof: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("feather proof: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}

func TestPaintedControlledHeroCutawayNative(t *testing.T) {
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
	types := sceneTemplateSection(t, "painted.cs.tmpl", "  struct PaintedOpaqueRun {", "  readonly List<PaintedFragment> paintedFragmentsA=")
	cold := sceneTemplateSection(t, "painted.cs.tmpl", "  static Rect[] BuildPaintedOpaqueRects(", "  void UpdatePaintedFog(")
	math := sceneTemplateSection(t, "painted.cs.tmpl", "  static bool PaintedLowerEdgeAtX(", "  void DrawPaintedFragment(")
	draw := sceneTemplateSection(t, "painted.cs.tmpl", "  void DrawPaintedFragment(", "  void HandlePaintedInput(")
	compare := sceneTemplateSection(t, "painted.cs.tmpl", "  static int ComparePaintedDraw(", "  static Vector2 PaintedPoint(")
	for _, bad := range []string{"new Texture2D", "SetPixels", "GetPixels", "new List", "paintedDraws.Add", "SceneActors(", "state.", "cut.texture"} {
		if strings.Contains(math+draw, bad) {
			t.Fatalf("sprite clipping mutates masks or replays foreground over prior actors: %s", bad)
		}
	}
	harness := `using System;using System.Collections.Generic;
struct Color32 {public byte a;public Color32(byte alpha){a=alpha;}}
struct Rect {public float x,y,width,height;public Rect(float left,float top,float w,float h){x=left;y=top;width=w;height=h;}public float xMin{get{return x;}}public float xMax{get{return x+width;}}public float yMin{get{return y;}}public float yMax{get{return y+height;}}public bool Overlaps(Rect b){return xMin<b.xMax&&xMax>b.xMin&&yMin<b.yMax&&yMax>b.yMin;}}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float xx,float yy){x=xx;y=yy;}}
struct PaintedDraw {public string id;public int kind;public float depth;}
class Proof {
` + types + cold + math + compare + `
 static bool Contains(Rect r,float x,float y){return r.width>0&&r.height>0&&x>=r.xMin&&x<r.xMax&&y>=r.yMin&&y<r.yMax;}
 static List<PaintedFragment> Partition(Rect actor,Rect[] masks,bool hero,List<PaintedFragment> a,List<PaintedFragment> b){a.Clear();AddPaintedFragment(a,actor,1);int work=0;foreach(var mask in masks){if(!mask.Overlaps(actor))continue;if(!ClipPaintedFragments(a,b,mask,hero,ref work,PaintedClipWorkLimit))throw new Exception("normal fixture exceeded fragment bounds");var swap=a;a=b;b=swap;}return a;}
 static float OpacityAt(List<PaintedFragment> fragments,float x,float y){float result=0;int coverage=0;foreach(var f in fragments)if(Contains(f.rect,x,y)){coverage++;result+=f.opacity;}if(coverage>1)throw new Exception("fragment replay overlaps");return result;}
 static void UV(Rect full,Rect region,float x,float y){var uv=PaintedRegionUV(full,new Rect(.1f,.2f,.7f,.6f),region);float a=.1f+(x-full.x)*.7f/full.width,b=uv.x+(x-region.x)*uv.width/region.width,c=.2f+(1-(y-full.y)/full.height)*.6f,d=uv.y+(1-(y-region.y)/region.height)*uv.height;if(Math.Abs(a-b)>.00001f||Math.Abs(c-d)>.00001f)throw new Exception("clipping changed sprite UV");}
 public static void Main(){
  var polygon=new[]{new PocPaintedPoint(0,0),new PocPaintedPoint(100,0),new PocPaintedPoint(100,150),new PocPaintedPoint(0,50)};float lower;
  if(!PaintedLowerEdgeAtX(polygon,20,out lower)||Math.Abs(lower-70)>.001f)throw new Exception("local slanted base not interpolated");
  if(PaintedMaskInFrontOfFoot(polygon,20,70)||PaintedMaskInFrontOfFoot(polygon,-1,5)||!PaintedMaskInFrontOfFoot(polygon,40,80)||!PaintedMaskInFrontOfFoot(polygon,20,50))throw new Exception("side/front/rear classification broken");
  if(!PaintedHeroCutawayEligible("char.hero",1,"char.hero",true)||PaintedHeroCutawayEligible("npc",1,"char.hero",true)||PaintedHeroCutawayEligible("char.hero",1,"npc",true)||PaintedHeroCutawayEligible("char.hero",1,"char.hero",false))throw new Exception("hidden or other actor enabled hero reveal");
  const int width=100,height=150;var pixels=new Color32[width*height];for(int y=0;y<height;y++)for(int x=0;x<width;x++)pixels[(height-y-1)*width+x]=new Color32((byte)(y+.5f<50+x+.5f?255:0));
  var masks=BuildPaintedOpaqueRects(pixels,width,height,new Rect(0,0,width,height));
  for(int y=0;y<height;y++)for(int x=0;x<width;x++){int hits=0;foreach(var m in masks)if(Contains(m,x+.5f,y+.5f))hits++;if(hits!=(pixels[(height-y-1)*width+x].a==0?0:1))throw new Exception("cold silhouette runs differ from source alpha");}
  var a=new List<PaintedFragment>(PaintedFragmentLimit);var b=new List<PaintedFragment>(PaintedFragmentLimit);var actorFront=new Rect(0,20,40,50);var actorBehind=new Rect(20,30,40,50);
  if(ComparePaintedDraw(new PaintedDraw{id="front",kind=1,depth=70},new PaintedDraw{id="behind",kind=1,depth=80})>=0)throw new Exception("ordinary feet ordering changed");
  var behind=Partition(actorBehind,masks,false,a,b);if(OpacityAt(behind,30,50)!=0)throw new Exception("rear actor painted through facade");
  // Front A is drawn first. Behind B has a later foot depth and an overlapping
  // screen rectangle, but contributes no pixel in the wall-covered overlap.
  float frontA=1,afterB=OpacityAt(behind,30,50);if(frontA*(1-afterB)!=1)throw new Exception("behind B erased front A");
  var other=new Rect(130,20,20,50);var unchanged=Partition(other,masks,false,a,b);if(unchanged.Count!=1||OpacityAt(unchanged,140,40)!=1)throw new Exception("nonintersecting NPC modified");
  var revealed=Partition(actorBehind,masks,true,a,b);float head=OpacityAt(revealed,30,40);if(head<.45f||head>.60f)throw new Exception("rear hero must be distinct translucent local silhouette");
  for(float y=30.37f;y<80;y+=1.37f)for(float x=20.29f;x<60;x+=1.19f){float value=OpacityAt(revealed,x,y);if(value<=0||value>1.00001f)throw new Exception("hero lost a fragment");foreach(var f in revealed)if(Contains(f.rect,x,y))UV(actorBehind,f.rect,x,y);}
  int work=0;var smallA=new List<PaintedFragment>(1);var smallB=new List<PaintedFragment>(1);AddPaintedFragment(smallA,new Rect(0,0,20,20),1);if(ClipPaintedFragments(smallA,smallB,new Rect(5,5,5,5),false,ref work,PaintedClipWorkLimit))throw new Exception("fragment capacity was allowed to grow");work=0;if(ClipPaintedFragments(smallA,smallB,new Rect(5,5,5,5),false,ref work,0)||work!=0)throw new Exception("work budget exceeded");
  var counter=typeof(GC).GetMethod("GetAllocatedBytesForCurrentThread",Type.EmptyTypes);if(counter!=null){var bytes=(Func<long>)Delegate.CreateDelegate(typeof(Func<long>),counter);Partition(actorBehind,masks,true,a,b);long before=bytes();for(int i=0;i<100;i++)Partition(actorBehind,masks,true,a,b);if(bytes()-before!=0)throw new Exception("sprite clipping allocated on frame path");}
  Console.WriteLine("local slanted base; front/rear/side; cached source-alpha runs; front A not erased by behind B; normal feet order; controlled 55% rear silhouette; hidden/NPC gates; bounded work and zero frame allocations PASS");
 }
}`
	dir := t.TempDir()
	source := filepath.Join(dir, "OcclusionProof.cs")
	binary := filepath.Join(dir, "OcclusionProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile occlusion proof: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("occlusion proof: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
