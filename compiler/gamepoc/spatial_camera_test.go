package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpatialCameraAllLifeMapsShareScaleAndInput(t *testing.T) {
	s := hudTemplate(t, "spatial.cs.tmpl")
	for _, want := range []string{"bool LifeSpatialCamera(){return state!=null&&state.Life!=null&&state.Life.Enabled;}", "tw=52*Mathf.Clamp(spatialZoom", "return LifeSpatialCamera()?tw/52f:1f", "if(LifeSpatialCamera()&&e.type==EventType.ScrollWheel", "HandleLifeDoorInput(origin,tw,th)", "SpatialGridPoint(e.mousePosition,origin,tw,th)", "ViewCell(view,x,y)=='0'"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing universal Life camera contract %q", want)
		}
	}
	if strings.Contains(s, "LargeSpatialMap") || strings.Contains(s, "state.Width>20") {
		t.Fatal("camera scale cannot depend on map size")
	}
	game := sceneTemplateSection(t, "unity.cs.tmpl", "        void DrawGame()", "        void LoadSprites()")
	if !strings.Contains(game, "GetSpatialProjection(out origin,out tw,out th)") || !strings.Contains(game, "DrawLifeOverlay(origin,tw,th)") {
		t.Fatal("drawing/doors must share projection")
	}
}

// Execute the actual C# projection at map corners and zoom limits, including the
// original capture scenario. No Unity player or generated project is required.
func TestSpatialCameraProjectionNative(t *testing.T) {
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
	methods := sceneTemplateSection(t, "spatial.cs.tmpl", "  void GetSpatialProjection(", "  void SpatialCellBounds(")
	harness := `using System;
struct Vector2 {
 public float x,y; public Vector2(float a,float b){x=a;y=b;}
 public static Vector2 operator +(Vector2 a,Vector2 b){return new Vector2(a.x+b.x,a.y+b.y);}
 public static Vector2 operator -(Vector2 a,Vector2 b){return new Vector2(a.x-b.x,a.y-b.y);}
}
struct Rect {
 public float xMin,yMin,xMax,yMax;
 public Rect(float x,float y,float w,float h){xMin=x;yMin=y;xMax=x+w;yMax=y+h;}
 public Vector2 center {get{return new Vector2((xMin+xMax)/2,(yMin+yMax)/2);}}
}
static class Mathf {
 public static float Clamp(float v,float min,float max){return Math.Max(min,Math.Min(max,v));}
 public static float Max(float a,float b){return Math.Max(a,b);}
}
class State {public int Width,Height;}
class Proof {
 State state=new State(); Vector2 shownPosition; float spatialZoom; bool life=true;
 bool LifeSpatialCamera(){return life;}
 float SpatialZoomMin(){return .7f;} float SpatialZoomMax(){return 3.8f;}
 Rect SpatialViewport(){return new Rect(16,104,1248,584);}
` + methods + `
 static void Equal(float a,float b,string what){if(Math.Abs(a-b)>.002f)throw new Exception(what+": "+a+" != "+b);}
 void Run(){
  foreach(var size in new[]{new[]{48,32},new[]{20,10},new[]{16,8},new[]{1,1}}){
   state.Width=size[0];state.Height=size[1];
   foreach(float zoom in new[]{.7f,1f,2.8f,3.8f})foreach(var hero in new[]{new Vector2(0,0),new Vector2(size[0]-1,size[1]-1),new Vector2((size[0]-1)/2f,(size[1]-1)/2f)}){
    spatialZoom=zoom;shownPosition=hero;Vector2 origin;float tw,th;GetSpatialProjection(out origin,out tw,out th);
    Equal(tw,52*zoom,"consistent cell/character scale");
    foreach(var cell in new[]{new Vector2(0,0),new Vector2(size[0]-1,size[1]-1),hero}){
     var screen=origin+new Vector2((cell.x-cell.y)*tw/2+tw/2,(cell.x+cell.y)*th/2+th/2);
     var inverse=SpatialGridPoint(screen,origin,tw,th);
     Equal(inverse.x,cell.x+.5f,"inverse x");Equal(inverse.y,cell.y+.5f,"inverse y");
    }
    var feet=origin+new Vector2((hero.x-hero.y)*tw/2+tw/2,(hero.x+hero.y)*th/2+th/2);var viewport=SpatialViewport();
    if(feet.x<viewport.xMin||feet.x>viewport.xMax||feet.y<viewport.yMin||feet.y>viewport.yMax)throw new Exception("hero outside clamped viewport");
   }
  }
  Equal(SpatialCameraAxis(999,0,100,10,210),60,"small map centered");
  Equal(SpatialCameraAxis(999,0,1000,10,210),10,"near edge clamp");
  Equal(SpatialCameraAxis(-999,0,1000,10,210),-790,"far edge clamp");
  life=false;spatialZoom=3.8f;Vector2 legacy;float legacyW,legacyH;GetSpatialProjection(out legacy,out legacyW,out legacyH);
  Equal(legacy.x,370,"capture origin x");Equal(legacy.y,185,"capture origin y");Equal(legacyW,64,"capture tile width");Equal(legacyH,32,"capture tile height");
 }
 public static void Main(){new Proof().Run();Console.WriteLine("camera projection PASS");}
}`
	dir := t.TempDir()
	source := filepath.Join(dir, "CameraProof.cs")
	binary := filepath.Join(dir, "CameraProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile camera: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("camera: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
