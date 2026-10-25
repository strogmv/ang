package gamepoc

import (
	"regexp"
	"strings"
	"testing"
)

func TestPaintedCharacterMotionUsesAcceptedPathAndFootAnchorsNative(t *testing.T) {
	pose := sceneTemplateSection(t, "painted_motion.cs.tmpl", "  struct PaintedMotionPose", "  readonly Dictionary")
	math := sceneTemplateSection(t, "painted_motion.cs.tmpl", "  static PaintedMotionPose PaintedMotionSample", "  // End pure motion functions.")
	runPaintedTextureNative(t, "PaintedCharacterMotion", `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public float sqrMagnitude{get{return x*x+y*y;}}}
class PocPoint {public int X,Y;public PocPoint(int x,int y){X=x;Y=y;}}
class PocPaintedPoint {public float x,y;public PocPaintedPoint(float a,float b){x=a;y=b;}}
class PocPaintedScene {public PocPaintedPoint axis_x,axis_y;}
class Proof {`+pose+math+`
 static void Need(bool b,string s){if(!b)throw new Exception(s);}
 static void Equal(double a,double b,string why){Need(Math.Abs(a-b)<.00001,why);}
 public static void Main(){
  var start=new Vector2(8,5);var path=new[]{new PocPoint(9,5),new PocPoint(9,6)};long begin=7000000000,step=400000000;
  Need(!PaintedMotionSample(start,path,begin,step,begin-1).walking,"future accepted step walks early");
  var first=PaintedMotionSample(start,path,begin,step,begin+step*.25);Need(first.walking&&first.direction.x==1&&first.direction.y==0,"accepted first segment/facing lost");
  var turn=PaintedMotionSample(start,path,begin,step,begin+step*1.25);Need(turn.walking&&turn.direction.x==0&&turn.direction.y==1,"accepted corner does not change facing");
  Need(!PaintedMotionSample(start,path,begin,step,begin+step*2).walking,"completed path keeps walking");
  Need(!PaintedMotionSample(start,new PocPoint[0],begin,step,begin).walking&&!PaintedMotionSample(start,path,begin,0,begin).walking,"empty/zero-duration step animates");
  Need(!PaintedMotionSample(start,new[]{new PocPoint(8,5)},begin,step,begin).walking,"stationary accepted cell animates a stride");
  var scene=new PocPaintedScene{axis_x=new PocPaintedPoint(40,0),axis_y=new PocPaintedPoint(0,20)};var rows=new[]{0,1,2,3,4,5,6,7};
  Need(PaintedMotionRow(scene,new Vector2(1,0),rows,0)==6,"rightward sheet facing wrong");Need(PaintedMotionRow(scene,new Vector2(-1,0),rows,0)==2,"leftward sheet facing wrong");Need(PaintedMotionRow(scene,new Vector2(0,1),rows,0)==0,"downward sheet facing wrong");Need(PaintedMotionRow(scene,new Vector2(0,-1),rows,0)==4,"upward sheet facing wrong");
  scene.axis_x=new PocPaintedPoint(-40,0);Need(PaintedMotionRow(scene,new Vector2(1,0),rows,0)==2,"mirrored scene ignores actual affine");Need(PaintedMotionRow(scene,new Vector2(0,0),rows,3)==3,"idle turns back to a default facing");
  int[] frames={1,2,3,2};var seen=new HashSet<int>();for(int i=0;i<80;i++){double ns=begin+i*step/40d;seen.Add(PaintedMotionFrame(frames,ns,step,.25));Need(PaintedMotionFrame(frames,ns,step,.25)==PaintedMotionFrame(frames,ns*2,step*2,.25),"cadence detached from accepted step duration");}Need(seen.Count==3,"walk atlas never advances");
  double frozen=begin+step*.35;int held=PaintedMotionFrame(frames,frozen,step,.73);float breath=PaintedIdleLift(frozen,.73,200);for(int i=0;i<100;i++){Need(PaintedMotionFrame(frames,frozen,step,.73)==held,"paused stride changes");Equal(PaintedIdleLift(frozen,.73,200),breath,"paused idle changes");}
  float low=10,high=-10;for(int i=0;i<1000;i++){float lift=PaintedIdleLift(i*10000000d,.31,200);Need(lift>=0&&lift<=1.40001,"idle deformation exceeds 0.7% body height");low=Math.Min(low,lift);high=Math.Max(high,lift);}Need(high-low>1.3,"idle has no breathing range");Need(Math.Abs(PaintedIdleLift(0,.05,200)-PaintedIdleLift(0,.7,200))>.2,"residents breathe in lockstep");
  Console.WriteLine("accepted path timing, all affine facings, stride/speed parity, pause hold, bounded asynchronous breathing PASS");
 }} `)
	draw := sceneTemplateSection(t, "painted.cs.tmpl", "  bool TryPaintedActorDraw(", "  static bool PaintedLowerEdgeAtX(")
	for _, want := range []string{"string.IsNullOrEmpty(binding.animation_sheet)", "PaintedActorMotion(id)", "PaintedMotionRow(scene,motion.direction", "PaintedMotionFrame(sheet.walk,motion.sampleNS,motion.stepNS", "feetX=w-feetX", "at.y-feetY*factor-lift", "depth=at.y,footX=at.x", "walking=animated&&motion.walking"} {
		if !strings.Contains(draw, want) {
			t.Errorf("missing grounded motion contract %q", want)
		}
	}
	motion := hudTemplate(t, "painted_motion.cs.tmpl")
	for _, bad := range []string{"Time.time", "Time.deltaTime", "DateTime", "new Texture2D", "GetPixels", "state.PlayerX=", "SceneAgents.Add", "ClickMove("} {
		if strings.Contains(motion, bad) {
			t.Errorf("presentation must not invent time, movement, pixels or actors: %q", bad)
		}
	}
	if !strings.Contains(motion, "actor.MotionPaused?actor.MotionSampleNs:renderedSimNS") || !strings.Contains(motion, "if(ViewSees(actor.X,actor.Y))paintedMotionActors") {
		t.Fatal("motion cache must share actual paused sample and current visibility")
	}
	if !strings.Contains(motion, "rest.y+rest.height*.56f") || !strings.Contains(motion, "DrawPaintedSpriteRegion(draw,bottom,PaintedRegionUV(rest,draw.uv,bottom)") {
		t.Fatal("idle animation must leave lower body/feet untransformed")
	}
}

func TestPaintedAnimationSheetRequiresExplicitKnownIdentity(t *testing.T) {
	c := paintedInspectionFixture(t)
	if len(c.Art.Painted[0].Actors) == 0 {
		t.Fatal("fixture needs a painted actor")
	}
	c.Art.Painted[0].Actors[0].AnimationSheet = "not-a-known-sheet"
	if err := checkPaintedScenes(c); err == nil || !strings.Contains(err.Error(), "unknown animation sheet") {
		t.Fatalf("unknown sheet accepted: %v", err)
	}
	c.Art.Characters.Sheets = map[string]CharacterSheet{"specific-identity": {}}
	c.Art.Painted[0].Actors[0].AnimationSheet = "specific-identity"
	if err := checkPaintedScenes(c); err != nil {
		t.Fatal(err)
	}
}

func TestPaintedBreathingLeavesSolesAndForegroundClipsFixedNative(t *testing.T) {
	render := sceneTemplateSection(t, "painted_motion.cs.tmpl", "  void DrawPaintedSpriteRegion(", "\n }\n}")
	uv := sceneTemplateSection(t, "painted.cs.tmpl", "  static Rect PaintedRegionUV(", "  static bool AddPaintedFragment(")
	runPaintedTextureNative(t, "PaintedBreathingClip", `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public static Vector2 operator -(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}}
struct Rect {public float x,y,width,height;public Rect(float a,float b,float w,float h){x=a;y=b;width=w;height=h;}public float xMin{get{return x;}}public float xMax{get{return x+width;}}public float yMin{get{return y;}}public float yMax{get{return y+height;}}public Vector2 position{get{return new Vector2(x,y);}set{x=value.x;y=value.y;}}}
struct Color {public float a;}
class Texture2D{}
struct PaintedDraw {public Rect rect,uv;public float idleLift;public Color tint;public Texture2D texture;}
static class GUI {public struct Quad{public Rect rect,uv;}public static Color color;public static List<Quad> calls=new List<Quad>();public static void DrawTextureWithTexCoords(Rect r,Texture2D t,Rect uv,bool alpha){calls.Add(new Quad{rect=r,uv=uv});}}
class Proof {
 static Rect PaintedIntersect(Rect a,Rect b){float x=Math.Max(a.x,b.x),y=Math.Max(a.y,b.y);return new Rect(x,y,Math.Max(0,Math.Min(a.xMax,b.xMax)-x),Math.Max(0,Math.Min(a.yMax,b.yMax)-y));}
 static Rect PaintedTarget(Rect r,Rect fit,float s){return new Rect(fit.x+r.x*s,fit.y+r.y*s,r.width*s,r.height*s);}
`+uv+render+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 static void Same(Rect a,Rect b,string why){Need(Math.Abs(a.x-b.x)<.0001f&&Math.Abs(a.y-b.y)<.0001f&&Math.Abs(a.width-b.width)<.0001f&&Math.Abs(a.height-b.height)<.0001f,why);}
 public static void Main(){
  var p=new Proof();var rest=new Rect(100,200,80,200);var fit=new Rect(20,30,1280,800);var viewport=new Rect(0,0,1280,800);var lowerClip=new Rect(112,330,45,65);var fullClip=new Rect(90,190,120,220);
  foreach(float flip in new[]{1f,-1f})foreach(float lift in new[]{.01f,.7f,1.4f}){
   var draw=new PaintedDraw{rect=new Rect(rest.x,rest.y-lift,rest.width,rest.height+lift),uv=new Rect(flip<0?.7f:.3f,.25f,.4f*flip,.5f),idleLift=lift,tint=new Color{a=1},texture=new Texture2D()};
   GUI.calls.Clear();p.DrawPaintedBreathingFragment(draw,lowerClip,.55f,fit,1,viewport);Need(GUI.calls.Count==1,"stationary feet were split/deformed");Same(GUI.calls[0].rect,PaintedTarget(lowerClip,fit,1),"lower-body screen pixels moved");Same(GUI.calls[0].uv,PaintedRegionUV(rest,draw.uv,lowerClip),"lower-body source UV changed");
   GUI.calls.Clear();p.DrawPaintedBreathingFragment(draw,fullClip,1,fit,1,viewport);Need(GUI.calls.Count==2,"body needs exactly two joined regions");var a=GUI.calls[0];var b=GUI.calls[1];Need(Math.Abs(a.rect.yMax-b.rect.y)<.0001,"visible seam gap/overlap at fixed hips");Need(Math.Abs(b.rect.yMax-(rest.yMax+fit.y))<.0001,"sole moved while breathing");Need(Math.Abs(a.uv.y-b.uv.yMax)<.0001,"UV seam torn at hips");
   var clipped=new Rect(115,295,25,40);GUI.calls.Clear();p.DrawPaintedBreathingFragment(draw,clipped,.55f,fit,1,viewport);foreach(var q in GUI.calls){var expected=PaintedTarget(clipped,fit,1);Need(q.rect.x>=expected.x-.001&&q.rect.y>=expected.y-.001&&q.rect.xMax<=expected.xMax+.001&&q.rect.yMax<=expected.yMax+.001,"breathing revealed body outside foreground-allowed fragment");}
  }
  Console.WriteLine("stationary sole/lower UV unchanged; two seamless upper/lower regions; mirrored atlas and foreground fragment clips PASS");
 }} `)
}

func TestPaintedCharacterMotionProofUsesRealActors(t *testing.T) {
	s := hudTemplate(t, "painted_motion_proof.cs.tmpl")
	for _, want := range []string{`"--poc-verify-character-motion"`, `NewGame(777,"life100")`, `SampleNPC(actor)`, `ViewSees(actor.X,actor.Y)`, `ScreenCapture.CaptureScreenshotAsTexture()`, `characterMotionWalkCaptures==2`, `Options(true,1)`, `characterMotionPausedSignature!=CharacterMotionSignature()`, `CharacterMotionApproach(now)`, `ViewCell(view,x,y)=='0'`, `view.Terrain[y][x]!='.'`, `ClickMove(x,y)`, `accepted_path_samples=characterMotionPathSamples`, `walking_pose_samples=characterMotionWalkingSamples`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing native motion evidence %q", want)
		}
	}
	for _, bad := range []string{"new PocAgentState", "SceneAgents.Add", "state.PlayerX=", "File.Copy(", "Time.time"} {
		if strings.Contains(strings.ReplaceAll(s, "==", "EQ"), bad) {
			t.Errorf("motion proof fabricates a rendered result: %s", bad)
		}
	}
}

func TestPaintedProofCaptureDeclarationsCompileNative(t *testing.T) {
	// These templates share one C# partial class. Compile their exact nested DTO
	// declarations and coroutine signatures together, so a type/member collision
	// cannot hide behind the narrower pure-math harnesses.
	var declarations strings.Builder
	types := regexp.MustCompile(`(?m)^  \[Serializable\] sealed class [^\n]+`)
	methods := regexp.MustCompile(`(?m)^  (IEnumerator [^\n{]+)\{`)
	for _, name := range []string{"painted_motion_proof.cs.tmpl", "station_berth_proof.cs.tmpl"} {
		s := hudTemplate(t, name)
		for _, declaration := range types.FindAllString(s, -1) {
			declarations.WriteString(declaration + "\n")
		}
		for _, signature := range methods.FindAllStringSubmatch(s, -1) {
			declarations.WriteString(signature[1] + "{yield break;}\n")
		}
	}
	runPaintedTextureNative(t, "PaintedProofNames", "using System;using System.Collections;class CopperRouteLegRecord{} class Proof {"+declarations.String()+` public static void Main(){Console.WriteLine("shared partial-class proof DTO/coroutine names compile PASS");}}`)
}
