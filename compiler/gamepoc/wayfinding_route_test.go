package gamepoc

import (
	"strings"
	"testing"
)

func TestWayfindingPaintProgressUsesPhysicalPositionAndHoldsPauseNative(t *testing.T) {
	code := sceneTemplateSection(t, "wayfinding_route.cs.tmpl", "  static Vector2 WayfindingRouteMetric(", "  WayfindingRouteCursor wayfindingRoute;")
	runPaintedTextureNative(t, "PaintProgress", `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public float sqrMagnitude{get{return x*x+y*y;}}public static Vector2 operator+(Vector2 a,Vector2 b){return new Vector2(a.x+b.x,a.y+b.y);}public static Vector2 operator-(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}public static Vector2 operator*(Vector2 a,float t){return new Vector2(a.x*t,a.y*t);}}
class PocWayfindingShapePoint {public bool Junction;public double X,Y;public PocWayfindingShapePoint(double a,double b){X=a;Y=b;}}
class PocWayfindingGuide {public string Id;public bool Authored;public double HalfWidthU,HalfWidthV;public PocWayfindingShapePoint AxisU,AxisV;public List<PocWayfindingShapePoint> Shape=new List<PocWayfindingShapePoint>();}
class PocWayfindingView {public string SelectedDestinationId,SelectedGuideId;public List<PocWayfindingGuide> Guides=new List<PocWayfindingGuide>();}
class Proof {`+code+`
 static void Need(bool v,string why){if(!v)throw new Exception(why);}static void Near(float got,float want,string why){Need(Math.Abs(got-want)<.0001,why+" got="+got+" want="+want);}
 public static void Main(){
  var guide=new PocWayfindingGuide{Id="local.paint",Authored=true,AxisU=new PocWayfindingShapePoint(1,0),AxisV=new PocWayfindingShapePoint(.6,.8)};
  var cursor=new WayfindingRouteCursor(guide,new List<Vector2>{new Vector2(0,0),new Vector2(4,0),new Vector2(5.8f,2.4f)});Near(cursor.Length,7,"arc length must use physical floor axes, not skewed navigation chord");
  Need(cursor.Sample(new Vector2(1,.1f),true),"paused initial selection must show its own projected preview");float begin=cursor.origin;Need(begin>.9f&&begin<1,"wrong initial projected origin");
  for(int i=0;i<1000;i++)Need(!cursor.Sample(new Vector2(1,.1f),false)&&cursor.progress==begin,"stationary selection progressed from elapsed frames");
  Need(cursor.Sample(new Vector2(2,0),false),"actual movement did not update progress");Near(cursor.progress,2,"first physical straight");
  for(int i=0;i<1000;i++)Need(!cursor.Sample(new Vector2(3,0),true)&&cursor.progress==2,"paused progress moved despite authoritative freeze");
  cursor.Sample(new Vector2(4,0),false);cursor.Sample(new Vector2(4.6f,.8f),false);Near(cursor.progress,5,"progress cut straight across a physical corner");
  cursor.Sample(new Vector2(15,15),false);Need(!cursor.onRoute&&cursor.progress==5,"off-route actor invented a connector or new progress");
  cursor.Sample(new Vector2(5.2f,1.6f),false);Near(cursor.progress,6,"nearby real rejoin lost route progress");cursor.Sample(new Vector2(4.6f,.8f),false);Near(cursor.progress,5,"backtracking pretended the player kept approaching");
  cursor.Sample(new Vector2(5.8f,2.4f),false);Near(cursor.progress,7,"terminal clamp did not reach exact authored endpoint");
  var outside=new WayfindingRouteCursor(guide,cursor.points);outside.Sample(new Vector2(40,40),false);Need(outside.origin==0&&outside.progress==0&&!outside.onRoute,"off-route initial selection must expose the intended paint without an imaginary hero connector");
  var hairpin=new WayfindingRouteCursor(null,new List<Vector2>{new Vector2(0,0),new Vector2(10,0),new Vector2(10,.5f),new Vector2(0,.5f)});hairpin.Sample(new Vector2(2,0),false);hairpin.Sample(new Vector2(2,.4f),false);Need(hairpin.progress==2&&!hairpin.onRoute,"nearby distant hairpin arm caused false progress jump");
  var view=new PocWayfindingView{SelectedDestinationId="region.work",SelectedGuideId=guide.Id};guide.Shape.Add(new PocWayfindingShapePoint(0,0));guide.Shape.Add(new PocWayfindingShapePoint(4,0));view.Guides.Add(guide);Need(Object.ReferenceEquals(WayfindingSelectedGuide(view),guide),"regional request lost explicit published local guide");guide.Shape.Clear();Need(WayfindingSelectedGuide(view)==null&&WayfindingHasAuthored(view),"hidden authored shape fell back to walking path");view.SelectedGuideId="PRIVATE";Need(WayfindingSelectedGuide(view)==null,"selection fabricated missing local guide");
  Console.WriteLine("physical arc/corner, stationary preview, pause, real movement, off-route hold/rejoin, backtracking, terminal and hairpin ambiguity PASS");
 }} `)
	sample := sceneTemplateSection(t, "wayfinding_route.cs.tmpl", "   public bool Sample(", "  WayfindingRouteCursor wayfindingRoute;")
	for _, forbidden := range []string{"new ", "Time.", "DateTime", "renderedSimNS", "state.Path"} {
		if strings.Contains(sample, forbidden) {
			t.Fatalf("progress must depend only on accepted position without per-frame allocation: %s", forbidden)
		}
	}
	shader := hudTemplate(t, "wayfinding.shader.tmpl")
	for _, required := range []string{"float2 route:TEXCOORD0", "clip(i.route.x-_RouteProgress.x", "_RouteProgress.y+_RouteProgress.z"} {
		if !strings.Contains(shader, required) {
			t.Fatalf("selected ribbon lost physical arc shader contract %s", required)
		}
	}
	if strings.Contains(shader, "_Time") {
		t.Fatal("route progress must not animate independently of the hero")
	}
}

func TestWayfindingNativeProofRejectsNonCoincidentSelectedRibbon(t *testing.T) {
	code := sceneTemplateSection(t, "wayfinding_proof.cs.tmpl", "  static bool WayfindingProofMatchesPaint(", "  Texture2D WayfindingProofReadLayer(")
	runPaintedTextureNative(t, "PaintCoincidence", `using System;using System.Collections.Generic;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}public float sqrMagnitude{get{return x*x+y*y;}}public static Vector2 operator-(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}}
class WayfindingStroke {public bool selected,joinedSides;public float half;public Vector2 a,b,startSide,endSide;}
class Proof {`+code+`
 static void Need(bool value,string why){if(!value)throw new Exception(why);}
 public static void Main(){var paint=new WayfindingStroke{a=new Vector2(2,2),b=new Vector2(8,2),startSide=new Vector2(.03f,.04f),endSide=new Vector2(.07f,.04f),half=.055f,joinedSides=true};var chosen=new WayfindingStroke{a=paint.a,b=paint.b,startSide=paint.startSide,endSide=paint.endSide,half=paint.half,joinedSides=true,selected=true};var all=new List<WayfindingStroke>{paint,chosen};Need(WayfindingProofMatchesPaint(chosen,all),"exact paint ribbon rejected");chosen.b=new Vector2(8,2.01f);Need(!WayfindingProofMatchesPaint(chosen,all),"deviating A* chord passed");chosen.b=paint.b;chosen.half=.065f;Need(!WayfindingProofMatchesPaint(chosen,all),"wider selected stripe leaves paint");chosen.half=paint.half;chosen.endSide=new Vector2(.04f,.07f);Need(!WayfindingProofMatchesPaint(chosen,all),"different physical corner join passed");chosen.endSide=paint.endSide;paint.selected=true;Need(!WayfindingProofMatchesPaint(chosen,all),"selected geometry was used as evidence of permanent paint");Console.WriteLine("native pixel-capture guard rejects chord, width, physical-join and selected-as-base substitutions PASS");}}
`)
	proof := hudTemplate(t, "wayfinding_proof.cs.tmpl")
	for _, want := range []string{"WayfindingProofMatchesPaint(line,wayfindingStrokes)", "WayfindingProofProgressFrame(\"route-progress.png\",false)", "wayfindingCanvasMaterial.GetVector(\"_RouteProgress\")", "AmbientProofPixelHash(after.GetPixels32())!=pixels", "ReferenceEquals(wayfindingCanvasMesh,wayfindingProofProgressMesh)"} {
		if !strings.Contains(proof, want) {
			t.Fatalf("missing real-frame acceptance of paint-following progress %s", want)
		}
	}
}
