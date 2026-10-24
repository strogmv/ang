package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocationSDKRendererUsesAuthoritativeMovementAndPublicGeometry(t *testing.T) {
	source, err := os.ReadFile("templates/location_sdk.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	for _, required := range []string{
		"level.id==state.Life.LevelId", "LocationSDKMatchesServer(level,state.Width,state.Height,state.Terrain)",
		"!resource.location_sdk.public_architecture||!level.initial_layout_known", "ViewCell(view,floor.at.x,floor.at.y)=='0'",
		"ViewSees(actor.X,actor.Y)", "ViewSees(state.PlayerX,state.PlayerY)", "LocationSDKHeroHeightM=1.8f",
		"instance.height*meters", "LocationSDKExtrusion(instance.footprint,meters", "LocationSDKCellCenter(position.x,locationSDKLevel.cell_meters)",
		"HudBlocksWorldInput(e.mousePosition)", "LocationSDKToolbar(viewport).Contains(e.mousePosition)", "ClickMove(x,y)",
		"locationSDKMain.cullingMask=locationSDKMainMask", "locationSDKTarget.Release()", "Destroy(actor.material)",
		"public PocLocationSDKPath[] reserved_paths", "foreach(var path in level.reserved_paths)LocationSDKBuildGuide(path,meters,bottom)",
	} {
		if !strings.Contains(s, required) {
			t.Errorf("SDK renderer contract missing %q", required)
		}
	}
	guides := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  void LocationSDKBuildGuide(", "  void LocationSDKEnsureTarget(")
	if !strings.Contains(guides, "path.points[i-1]") || !strings.Contains(guides, "path.points[i]") || strings.Contains(guides, "path.cells") || strings.Contains(guides, "ClickMove") {
		t.Fatal("procedural paint must follow ordered authored centreline, not expanded traffic cells or new movement rules")
	}
	for _, forbidden := range []string{"GetPixels", "GetPixel(", "GetRawTextureData", "ReadPixels", "CreatePrimitive", "state.PlayerX=", "state.PlayerY=", "state.Path.Add", "session.SendNewGame", "RotateAroundPivot", "instance.rotation=="} {
		if strings.Contains(s, forbidden) {
			t.Errorf("SDK renderer must not read imported CPU pixels, build cell cubes, mutate simulation or rotate compiled footprints again: %s", forbidden)
		}
	}
}

func TestLocationSDKRendererGeometryAndScaleNative(t *testing.T) {
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
	shapeTypes := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  sealed class LocationSDKRect ", "  sealed class LocationSDKActor ")
	math := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  static long LocationSDKCellKey(", "  sealed class LocationSDKMeshBuilder ")
	if strings.Contains(math, "for(int i=result.Count") || !strings.Contains(math, "previous.TryGetValue(key,out rect)") {
		t.Fatal("top span merging must use bounded previous-row lookup, not scan all prior rectangles")
	}
	dto := sceneTemplateSection(t, "location_sdk.cs.tmpl", " [Serializable] public sealed class PocLocationSDKCell ", " public sealed partial class AngelsDemonsPoc ")
	dto = strings.Replace(dto, "class PocLocationSDKCell {public int x,y;}", "class PocLocationSDKCell {public int x,y;public PocLocationSDKCell(int xx,int yy){x=xx;y=yy;}}", 1)
	constants := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  const int LocationSDKLayer=", "  const float LocationSDKHeroHeightM=")
	harness := `using System;using System.Collections.Generic;` + dto + `class Proof {
` + constants + shapeTypes + math + `
 static void Require(bool value,string reason){if(!value)throw new Exception(reason);}
 static void Rectangles(){
  var cells=new List<PocLocationSDKCell>();for(int y=20;y<22;y++)for(int x=10;x<14;x++)cells.Add(new PocLocationSDKCell(x,y));
  var rects=LocationSDKTopRectangles(cells.ToArray());var edges=LocationSDKBoundaryRuns(cells.ToArray());
  Require(rects.Count==1&&rects[0].x==10&&rects[0].y==20&&rects[0].w==4&&rects[0].h==2,"global 4x2 module was recentered, rotated or made into cell cubes");
  Require(edges.Count==4,"internal material seams became vertical faces");int perimeter=0;foreach(var edge in edges)perimeter+=edge.length;Require(perimeter==12,"boundary length changed");
  cells.Add(new PocLocationSDKCell(10,20));Require(LocationSDKBoundaryRuns(cells.ToArray()).Count==4,"duplicate physical cell changed the boundary");
 }
 static void Holes(){
  var cells=new List<PocLocationSDKCell>();for(int y=0;y<3;y++)for(int x=0;x<3;x++)if(x!=1||y!=1)cells.Add(new PocLocationSDKCell(x,y));
  int area=0;foreach(var rect in LocationSDKTopRectangles(cells.ToArray())){area+=rect.w*rect.h;Require(!(rect.x<=1&&rect.y<=1&&rect.x+rect.w>1&&rect.y+rect.h>1),"empty shell interior covered by a top face");}
  Require(area==8&&LocationSDKBoundaryRuns(cells.ToArray()).Count==8,"shell hole or inner boundary disappeared");
  var l=new[]{new PocLocationSDKCell(5,6),new PocLocationSDKCell(6,6),new PocLocationSDKCell(5,7)};
  Require(LocationSDKTopRectangles(l).Count==2&&LocationSDKBoundaryRuns(l).Count==6,"L-shaped footprint was replaced with its bounding box");
 }
 static void Scale(){
  Require(Math.Abs(LocationSDKCellCenter(0,.5f)-.25f)<.00001f&&Math.Abs(LocationSDKCellCenter(3,.5f)-1.75f)<.00001f,"actor pose is not the cell centre in metres");
  foreach(float body in new[]{48f,96f,200f})Require(Math.Abs(LocationSDKActorPixelScale(body,1.8f)*body-1.8f)<.00001f,"different sprite crops changed reference body height");
  foreach(float bad in new[]{0f,-1f,float.NaN,float.PositiveInfinity})Require(LocationSDKActorPixelScale(bad,1.8f)==0&&LocationSDKActorPixelScale(96,bad)==0,"invalid atlas metric entered mesh");
 }
 static void MetricUV(){
  Require(Math.Abs(LocationSDKTextureCoordinate(6,750)-8)<.0001,"750mm non-grid surface repeat changed physical top scale");
  Require(Math.Abs(LocationSDKTextureCoordinate(.5f,2000)-.25f)<.0001,"half-metre cell was treated as a full metre in top UV");
  Require(Math.Abs(LocationSDKSideU(6,9,0,2000)-3)<.0001&&Math.Abs(LocationSDKSideU(6,9,1,2000)-4.5f)<.0001,"side horizontal UV uses wrong physical axis");
  Require(Math.Abs(LocationSDKSideV(10.5f,7.5f,3000)-1)<.0001&&Math.Abs(LocationSDKSideV(-4.5f,-7.5f,3000)-1)<.0001,"translated deck height changed wall texture repeat");
  Require(Math.Abs(LocationSDKTextureCoordinate(-.75f,750)+1)<.0001,"negative metric repeat stopped wrapping");
 }
 static void Ground(){
  int x,y;Require(LocationSDKGroundCell(.25f,1.75f,.5f,4,5,out x,out y)&&x==0&&y==3,"half-metre inverse pick wrong");
  Require(LocationSDKGroundCell(.5f,1f,.5f,4,5,out x,out y)&&x==1&&y==2,"exact edge pick wrong");
  foreach(float bad in new[]{-0.001f,2f,float.NaN,float.PositiveInfinity})Require(!LocationSDKGroundCell(bad,1,.5f,4,5,out x,out y),"outside/nonfinite pointer entered level");
  Require(!LocationSDKGroundCell(.1f,.1f,float.NaN,4,5,out x,out y),"nonfinite cell size accepted");
 }
 static void Checkerboard(){
  var cells=new List<PocLocationSDKCell>();for(int y=0;y<256;y++)for(int x=0;x<256;x++)if((x+y)%2==0)cells.Add(new PocLocationSDKCell(x,y));
  var rects=LocationSDKTopRectangles(cells.ToArray());Require(rects.Count==32768,"checkerboard holes merged into occupied cells");foreach(var rect in rects)Require(rect.w==1&&rect.h==1&&(rect.x+rect.y)%2==0,"checkerboard geometry changed");
 }
 static void Camera(){
  foreach(int width in new[]{32,256,2048})foreach(int depth in new[]{32,256,2048})foreach(float height in new[]{0f,3f,1000f})foreach(float elevation in new[]{-500000f,0f,500000f})foreach(float aspect in new[]{.7f,2.1f}){
   float distance,near,far;LocationSDKCameraRange(width,depth,.5f,height,out distance,out near,out far);float fit=LocationSDKCameraFit(width,depth,.5f,height,aspect);
   foreach(double x in new[]{0d,width*.5})foreach(double z in new[]{0d,depth*.5})foreach(double y in new[]{(double)elevation-.07,elevation+(double)height}){
    double dx=x-width*.25,dz=z-depth*.25,dy=y-elevation-height*.28;
    double along=distance-(dx*.612372435695794+dy*.5+dz*.612372435695794),sx=(dx-dz)*.707106781186548,sy=-(dx+dz)*.353553390593274+dy*.866025403784439;
    Require(along>near&&along<far,"allowed long/tall or translated scene crosses camera near/far planes");
    Require(Math.Abs(sx)<=fit*aspect&&Math.Abs(sy)<=fit,"default orthographic fit crops whole-layout corners");
   }
  }
 }
 static void Budget(){
  var level=new PocLocationSDKLevel{width=2048,depth=32,floor=new PocLocationSDKFloor[65536]};Require(LocationSDKWithinBudget(level),"64k supported-cell limit rejected");level.width++;Require(!LocationSDKWithinBudget(level),"oversized runtime area admitted");level.width--;
  level.instances=new[]{new PocLocationSDKInstance{footprint=new[]{new PocLocationSDKCell(1,1)}}};Require(!LocationSDKWithinBudget(level),"material cell expansion exceeds floor budget");
  level=new PocLocationSDKLevel{width=32,depth=32,floor=new PocLocationSDKFloor[1],instances=new PocLocationSDKInstance[513]};Require(!LocationSDKWithinBudget(level),"unbounded instance definitions admitted");level.instances=null;
  level.barriers=new PocLocationSDKBarrier[1024];Require(LocationSDKWithinBudget(level),"fixed 1024 parts budget rejected");level.barriers=new PocLocationSDKBarrier[1025];Require(!LocationSDKWithinBudget(level),"unbounded barrier meshes/materials admitted");level.barriers=null;
  level.reserved_paths=new[]{new PocLocationSDKPath{points=new PocLocationSDKCell[16384]}};Require(LocationSDKWithinBudget(level),"guide budget boundary rejected");level.reserved_paths[0].points=new PocLocationSDKCell[16385];Require(!LocationSDKWithinBudget(level),"unbounded guide point expansion admitted");level.reserved_paths=null;
  var decoration=new PocLocationSDKDecoration{id="a",width=256,depth=256,height=1};level.views=new[]{new PocLocationSDKView{decorations=new[]{decoration}},new PocLocationSDKView{decorations=new[]{decoration}}};Require(!LocationSDKWithinBudget(level),"visual attachment cells not bounded");decoration.width=1;decoration.depth=1;Require(LocationSDKWithinBudget(level),"same binding in independent time views counted twice");
 }
 static void Main(string[] args){if(args[0]=="rectangles")Rectangles();else if(args[0]=="holes")Holes();else if(args[0]=="scale")Scale();else if(args[0]=="uv")MetricUV();else if(args[0]=="checkerboard")Checkerboard();else if(args[0]=="camera")Camera();else if(args[0]=="budget")Budget();else Ground();Console.WriteLine(args[0]+" SDK geometry PASS");}
}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "SDKRendererProof.cs"), filepath.Join(dir, "SDKRendererProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile exact SDK geometry helpers: %v\n%s", err, out)
	}
	for _, mode := range []string{"rectangles", "holes", "scale", "uv", "ground", "checkerboard", "camera", "budget"} {
		t.Run(mode, func(t *testing.T) {
			if out, err := exec.Command(mono, binary, mode).CombinedOutput(); err != nil {
				t.Fatalf("SDK geometry %s: %v\n%s", mode, err, out)
			}
		})
	}
}

func TestLocationSDKSurfaceSamplingAndDepthAwareActorShaders(t *testing.T) {
	source, err := os.ReadFile("templates/location_sdk.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	for _, required := range []string{"PocLocationSDKSurface[] surfaces", "default_floor_surface", "surface_id", "mesh.SetUVs(0,uv)", "LocationSDKSideV(bottom,deckMetres,sideRepeat.y)", "locationSDKMaterialCache.TryGetValue", "texture=Resources.Load<Texture2D>(path)", "texture.wrapMode=TextureWrapMode.Repeat", "texture.filterMode=FilterMode.Trilinear", "(surface==null?diagnostic:Color.white)", "Shader.Find(\"ANG/LocationSDKActor\")"} {
		if !strings.Contains(s, required) {
			t.Errorf("metric surface/actor renderer missing %q", required)
		}
	}
	textureLoader := sceneTemplateSection(t, "location_sdk.cs.tmpl", "  Texture2D LocationSDKSurfaceTexture(", "  Material LocationSDKMaterial(")
	if strings.Contains(textureLoader, "ArtSceneTexture") || strings.Contains(textureLoader, "Clamp") || strings.Contains(textureLoader, "GetPixels") {
		t.Fatal("surface sampling must keep independent GPU-only repeat imports")
	}
	surface, err := os.ReadFile("templates/location_sdk_surface.shader.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"Shader \"ANG/LocationSDKSurface\"", "tex2D(_MainTex,i.uv)*_Color", "ZWrite On", "color.a=1"} {
		if !strings.Contains(string(surface), required) {
			t.Errorf("opaque metric surface shader missing %q", required)
		}
	}
	actor, err := os.ReadFile("templates/location_sdk_actor.shader.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	a := string(actor)
	for _, required := range []string{"Shader \"ANG/LocationSDKActor\"", "ZTest Greater", "ZTest LEqual", "ZWrite Off", "clip(body.a-.02)", "_OccludedTint.a*body.a*_Color.a", "return body*_Color", ".24,.23)"} {
		if !strings.Contains(a, required) {
			t.Errorf("depth-aware atlas shader missing %q", required)
		}
	}
	if strings.Contains(a, "ZTest Always") || strings.Contains(a, "ZWrite On") {
		t.Fatal("controlled sprite must not become a full always-on-top body or replace opaque depth")
	}
}

func TestLocationSDKProofUsesRealPointerStateAndWorldReadback(t *testing.T) {
	data, err := os.ReadFile("templates/location_sdk_proof.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, required := range []string{"NewGame(777,expected.scenario)", "SendLocationSDKScreenPointer(screen,SpatialViewport())", "state.LastMoveSeq!=pendingMove", "if(moveAccepted||!LocationSDKProofSettled()", "SaveOrLoad(\"slot3\",true)", "SaveOrLoad(\"slot3\",false)", "state.GameSeconds!=locationSDKProofSavedSeconds", "new WaitForEndOfFrame()", "RenderTexture.active=locationSDKTarget", "original SDK hero sprite made no actual GPU contribution", "presentation changed SDK structural mesh", "requiredCaptures+=level.views.Length", "locationSDKProofCaptures.Count!=requiredCaptures"} {
		if !strings.Contains(s, required) {
			t.Errorf("native SDK acceptance missing %s", required)
		}
	}
	for _, forbidden := range []string{"state.PlayerX=", "state.PlayerY=", "state.Tick=", "state.GameSeconds=", "state.Path.Add", "Application.isBatchMode){FinishProof(true", "DateTime.Now"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("native SDK acceptance must not fabricate a gameplay state: %s", forbidden)
		}
	}
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
	body := sceneTemplateSection(t, "location_sdk_proof.cs.tmpl", "  static bool LocationSDKProofTarget(", "  bool LocationSDKProofCanPick(")
	harness := `using System;class Proof {` + body + `
 static void Require(bool value,string why){if(!value)throw new Exception(why);}
 static void Main(){int x,y;
  Require(LocationSDKProofTarget(new[]{"........."},new[]{"222222222"},0,0,6,(a,b)=>true,out x,out y)&&x==6&&y==0,"free target lacks a real connected six-cell route");
  Require(LocationSDKProofTarget(new[]{"........."},new[]{"222222222"},0,0,6,(a,b)=>a<5,out x,out y)&&x==4,"HUD/viewport-rejected point still selected");
  Require(!LocationSDKProofTarget(new[]{"........."},new[]{"222000222"},0,0,6,(a,b)=>true,out x,out y),"crossed unknown floor to a distant target");
  Require(!LocationSDKProofTarget(new[]{"...#....."},new[]{"222222222"},0,0,6,(a,b)=>true,out x,out y),"crossed blocked geometry into another island");
  Require(!LocationSDKProofTarget(new[]{"........."},new[]{"022222222"},0,0,6,(a,b)=>true,out x,out y),"unknown player origin admitted");
  Require(!LocationSDKProofTarget(new[]{"........."},new[]{"22"},0,0,6,(a,b)=>true,out x,out y),"malformed observer row admitted");
  Require(!LocationSDKProofTarget(new[]{"........."},new[]{"222222222"},0,0,6,null,out x,out y),"missing camera eligibility admitted");
  Console.WriteLine("SDK proof known-floor target/obstacle/viewport contract PASS");
 }}`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "SDKPointerProof.cs"), filepath.Join(dir, "SDKPointerProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source).CombinedOutput(); err != nil {
		t.Fatalf("compile exact SDK proof target selection: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("SDK native target selection: %v\n%s", err, out)
	}
}
