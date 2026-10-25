package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestRegionUIUsesOnlyChipKnowledgeAndServerTravel(t *testing.T) {
	s := hudTemplate(t, "region.cs.tmpl")
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			t.Fatal("UI template contains text outside the localization catalog")
		}
	}
	for _, want := range []string{
		"state.Life.RegionMap", "DrawHudLocalMap()", `hudMapFloor=true`,
		`L("ui.region.memory_caption")`,
		`node.Knowledge=="known"||node.Knowledge=="visited"`,
		`node.CanTravel&&!node.Current&&!string.IsNullOrEmpty(node.LevelId)`,
		`WayfindingSelect(node.LevelId)`, `WayfindingGo()`, `GUI.BeginGroup(hudRegionCanvas)`,
		`RegionHandleMapInput(nodes,bounds)`, `RegionKnownDecks(map)`,
		`RegionClipLine(canvas,ref from,ref to)`, `RegionClippedFill`, `RegionFocusNode(map,current)`, `RegionSelectNode`, `RegionPlane`,
		`L("ui.region.landmark")`, `L("ui.region.tab_floor")`, `L("ui.region.tab_local")`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing region UI contract %q", want)
		}
	}
	for _, bad := range []string{"Resources.Load", "resource.", "state.Entities", "state.SceneAgents", "planning/", "teleport", "ETA", "GUI.matrix", "RotateAroundPivot"} {
		if strings.Contains(s, bad) {
			t.Errorf("region UI may fabricate information or load developer state: %q", bad)
		}
	}
}

// Compile the complete presentation template against real Unity assemblies, then
// execute its actual filtering, bounds, focus-zoom and travel guards under Mono.
// Protocol/HUD stubs provide only the declared wire fields and unrelated hooks.
func TestRegionUINativeProjectionKnowledgeAndTravel(t *testing.T) {
	editors, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app")
	if len(editors) == 0 {
		t.Skip("Unity editor assemblies unavailable; wire/privacy contracts still tested")
	}
	editor := editors[len(editors)-1]
	mono := filepath.Join(editor, "Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	managed := filepath.Join(editor, "Contents/Resources/Scripting/Managed/UnityEngine")
	core := filepath.Join(managed, "UnityEngine.CoreModule.dll")
	imgui := filepath.Join(managed, "UnityEngine.IMGUIModule.dll")
	textRendering := filepath.Join(managed, "UnityEngine.TextRenderingModule.dll")
	facade := filepath.Join(filepath.Dir(mono), "../lib/mono/4.8-api/Facades/netstandard.dll")
	for _, path := range []string{mono, csc, core, imgui, textRendering, facade} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("Unity compile dependency unavailable: %s", path)
		}
	}
	harness := `
namespace AngelsDemons.Game.V1 {
 public class PocRegionMap {public string Id="",Name="",CurrentNode="",CurrentDeck="",SelectedDestinationId="";public System.Collections.Generic.List<string> SelectedLocalLinkIds=new System.Collections.Generic.List<string>();public System.Collections.Generic.List<PocRegionDeck> Decks=new System.Collections.Generic.List<PocRegionDeck>();public System.Collections.Generic.List<PocRegionNode> Nodes=new System.Collections.Generic.List<PocRegionNode>();public System.Collections.Generic.List<PocRegionLink> Links=new System.Collections.Generic.List<PocRegionLink>();}
 public class PocRegionDeck {public string Id="",Name="";public double ElevationM;}
 public class PocRegionNode {public string Id="",Name="",DeckId="",Knowledge="",LevelId="",Description="",Kind="",Label="";public double X,Y,Width,Height;public bool Current,CanTravel;}
 public class PocRegionLink {public string Id="",FromNode="",ToNode="",Mode="";}
}
namespace AngelsDemons.Poc {
 public sealed partial class AngelsDemonsPoc {
  class State {public Life Life=new Life();public bool Paused;}
  class Life {public AngelsDemons.Game.V1.PocRegionMap RegionMap;}
  State state;bool loadingActive;string page="game",message="";
  class Guidance {public string SelectedDestinationId;public bool Reachable;}bool wayfindingPending;Guidance WayfindingView(){return null;}bool WayfindingSelect(string id){return false;}void WayfindingClear(){}bool WayfindingGo(){return false;}
  UnityEngine.Color hudBrass,hudPaper;UnityEngine.GUIStyle hudSmall,hudBody,hudWrap,hudHeading;
  readonly System.Collections.Generic.List<UnityEngine.Rect> regionProofFills=new System.Collections.Generic.List<UnityEngine.Rect>();
  bool CanSend(){return false;}void Fill(UnityEngine.Rect r,UnityEngine.Color c){regionProofFills.Add(r);}void HudFrame(UnityEngine.Rect r,bool brass=true){}bool HudButton(UnityEngine.Rect r,string label,bool enabled=true){return false;}
  void DrawHudLocalMap(){}void LifeCommand(string action,string destination){}
  static readonly System.Collections.Generic.Dictionary<string,string> regionEnglish=new System.Collections.Generic.Dictionary<string,string> {
   {"ui.region.tab_floor","Floor"},
   {"ui.region.selected_object","Selected place"},
   {"ui.region.preview_route","Preview route"},
   {"ui.region.tab_local","Location"},
   {"ui.region.link_monorail","Monorail"},
   {"ui.region.link_stairs","Stairs"},
   {"ui.region.link_lift","Lift"},
   {"ui.region.link_corridor","Passage"},
   {"ui.region.select_place","Select a place on the map."},
   {"ui.region.status_here","You are here"},
   {"ui.region.status_visited","Previously visited"},
   {"ui.region.status_known","Known from information"},
   {"ui.region.go_here","Go here"},
   {"ui.region.route_notice","Route: {name}"},
   {"ui.region.travel_paused","Travel is paused"},
   {"ui.region.travel_known_connections","Via known connections"},
   {"ui.region.current_place","Current place"},
   {"ui.region.landmark","Map landmark"},
   {"ui.region.map_title","Floor map"},
   {"ui.region.memory_caption","Local chip memory \u00b7 known connections"},
   {"ui.region.empty_map","No floor map is stored in the chip yet."},
   {"ui.region.elevation","Elevation {elevation} m"},
   {"ui.region.deck_lower","Lower"},
   {"ui.region.deck_higher","Higher"},
   {"ui.region.legend","Light mark: you \u00b7 brass: selected \u00b7 grey: memory"},
   {"ui.region.fit_all","Fit map"},
   {"ui.region.focus_hero","Find me"},
   {"ui.region.controls_hint","Wheel: zoom \u00b7 drag: pan"},
   {"ui.region.connection","{mode} \u2192 {name}"},
  };
  string L(string key){string value;if(!regionEnglish.TryGetValue(key,out value))throw new System.Exception("Missing localization key "+key);return value;}

  static void Require(bool ok,string reason){if(!ok)throw new System.Exception(reason);}
  static void Near(float a,float b,string reason){Require(System.Math.Abs(a-b)<.002,reason+": "+a+" / "+b);}
  static AngelsDemons.Game.V1.PocRegionNode Node(string id,string knowledge,string deck="low") {return new AngelsDemons.Game.V1.PocRegionNode {Id=id,Name=id,DeckId=deck,Knowledge=knowledge,X=20,Y=30,Width=12,Height=8};}
  public static void Main(){
   var known=Node("market","known");var visited=Node("home","visited");visited.X=110;visited.Y=80;
   var unknown=Node("hidden","unknown","secret");unknown.X=90000;unknown.CanTravel=true;unknown.LevelId="level.secret";
   var malformed=Node("invalid","known");malformed.X=double.NaN;
   Require(!RegionNodeTravelAllowed(known),"landmark cannot travel");known.CanTravel=true;
   Require(!RegionNodeTravelAllowed(known),"destination ID is required");known.LevelId="level.c7_market";
   Require(RegionNodeTravelAllowed(known),"known destination capability accepted");known.Current=true;
   Require(!RegionNodeTravelAllowed(known),"current node is not a new trip");known.Current=false;
   Require(!RegionNodeTravelAllowed(unknown),"unknown destination rejected even if capability is malformed");
   var map=new AngelsDemons.Game.V1.PocRegionMap {Id="region",CurrentNode="home",CurrentDeck="low"};
   map.Decks.Add(new AngelsDemons.Game.V1.PocRegionDeck {Id="empty",ElevationM=0});map.Decks.Add(new AngelsDemons.Game.V1.PocRegionDeck {Id="secret",ElevationM=100});map.Decks.Add(new AngelsDemons.Game.V1.PocRegionDeck {Id="low",Name="Residential deck",ElevationM=18});
   map.Nodes.Add(known);map.Nodes.Add(visited);map.Nodes.Add(unknown);map.Nodes.Add(malformed);
   Require(RegionKnownDecks(map).Count==1&&RegionKnownDecks(map)[0].Id=="low","empty/unknown-only decks are absent");
   Require(RegionFindNode(map,"hidden")==null,"unknown node cannot be selected");
   var nodes=RegionDeckNodes(map,"low");Require(nodes.Count==2,"unknown and nonfinite nodes cannot enter projection");
   var bounds=RegionModelBounds(nodes);Require(bounds.width<130&&bounds.height<70,"unknown coordinates do not affect fit");
   var canvas=new UnityEngine.Rect(324,282,650,340);float zoom=1;var pan=new UnityEngine.Vector2(18,-12);
   var before=RegionMapPoint(known.X,known.Y,bounds,canvas,zoom,pan);
   RegionZoomAt(before,canvas,1.7f,ref zoom,ref pan);var after=RegionMapPoint(known.X,known.Y,bounds,canvas,zoom,pan);
   Near(before.x,after.x,"cursor anchor X");Near(before.y,after.y,"cursor anchor Y");
   RegionZoomAt(canvas.center,canvas,100,ref zoom,ref pan);Near(zoom,4.5f,"maximum zoom");
   RegionZoomAt(canvas.center,canvas,.0001f,ref zoom,ref pan);Near(zoom,.65f,"minimum zoom");
   var global=RegionMapPoint(visited.X,visited.Y,bounds,canvas,zoom,pan);var local=RegionMapPoint(visited.X,visited.Y,bounds,new UnityEngine.Rect(0,0,canvas.width,canvas.height),zoom,pan);
   Near(global.x,local.x+canvas.x,"clip-group/input X agreement");Near(global.y,local.y+canvas.y,"clip-group/input Y agreement");
   map.Links.Add(new AngelsDemons.Game.V1.PocRegionLink {FromNode="market",ToNode="hidden",Mode="lift"});
   var ui=new AngelsDemonsPoc();ui.RegionPrepareConnections(nodes);ui.regionConnectionRoutes["probe"]=new System.Collections.Generic.List<UnityEngine.Vector2>();ui.RegionPrepareConnections(nodes);Require(ui.regionConnectionRoutes.Count==1,"unchanged known topology rebuilt its presentation routes");known.X+=1;ui.RegionPrepareConnections(nodes);Require(ui.regionConnectionRoutes.Count==0,"modified known geometry retained stale presentation routes");known.X-=1;Require(!ui.RegionNodeDescription(map,known).Contains("hidden"),"dangling/unknown connection is not described");
   var clip=new UnityEngine.Rect(0,0,596,326);var strokes=new[]{
    new[]{new UnityEngine.Vector2(-100,80),new UnityEngine.Vector2(700,80)},
    new[]{new UnityEngine.Vector2(40,-100),new UnityEngine.Vector2(40,500)},
    new[]{new UnityEngine.Vector2(-100,-100),new UnityEngine.Vector2(700,500)},
    new[]{new UnityEngine.Vector2(0,0),new UnityEngine.Vector2(596,326)},
    new[]{new UnityEngine.Vector2(-10000000,-5000000),new UnityEngine.Vector2(10000000,5000000)}
   };
   foreach(var stroke in strokes){ui.regionProofFills.Clear();ui.RegionLine(clip,stroke[0],stroke[1],new UnityEngine.Color(1,1,1),3);Require(ui.regionProofFills.Count>0&&ui.regionProofFills.Count<=400,"bounded stroke rendering cost after clipping");foreach(var fill in ui.regionProofFills)Require(fill.width>0&&fill.height>0&&fill.xMin>=clip.xMin&&fill.yMin>=clip.yMin&&fill.xMax<=clip.xMax+.001f&&fill.yMax<=clip.yMax+.001f,"every submitted fill stays inside the canvas");}
   ui.regionProofFills.Clear();ui.RegionLine(clip,new UnityEngine.Vector2(-50,-50),new UnityEngine.Vector2(-1,-1),new UnityEngine.Color(),3);Require(ui.regionProofFills.Count==0,"fully offscreen lines cannot reach another HUD panel");
   ui.RegionLine(clip,new UnityEngine.Vector2(float.NaN,0),new UnityEngine.Vector2(10,10),new UnityEngine.Color(),3);Require(ui.regionProofFills.Count==0,"nonfinite lines cannot submit fill rectangles");
   Require(ui.RegionLinkName("monorail")=="Monorail"&&ui.RegionLinkName("stairs")=="Stairs"&&ui.RegionLinkName("lift")=="Lift"&&ui.RegionLinkName("corridor")=="Passage","connection modes use localized terms");
   Require(ui.RegionRouteNotice("Market")=="Route: Market","localized route notice preserves the destination placeholder");
   Require(ui.RegionElevationLabel(18)=="Elevation +18 m"&&ui.RegionElevationLabel(-6)=="Elevation -6 m","localized elevation and unit preserve the signed height");
   map.Links.Add(new AngelsDemons.Game.V1.PocRegionLink {FromNode="market",ToNode="home",Mode="monorail"});
   Require(ui.RegionNodeDescription(map,known).Contains("Monorail \u2192 home"),"localized connection format contains the known localized place name");
   ui.SetHudMapMode("local");Require(!ui.hudMapFloor,"local tab reachable");ui.OpenHudMap();Require(ui.hudMapFloor&&ui.page=="map"&&ui.hudRegionFollowCurrent,"M opens floor by default");
   ui.RegionRefreshSelection(map,RegionKnownDecks(map));Require(ui.hudRegionDeckID=="low"&&ui.hudRegionNodeID=="home","current place selected from wire only");
   Near(ui.hudRegionZoom,1,"opening view shows all known destinations");
   ui.RegionFocusNode(map,visited);
   var focused=RegionMapPoint(visited.X,visited.Y,RegionModelBounds(RegionDeckNodes(map,"low")),canvas,ui.hudRegionZoom,ui.hudRegionPan);Near(focused.x,canvas.center.x,"current place centered X");Near(focused.y,canvas.center.y,"current place centered Y");
   var work=Node("work","known");work.X=1120;work.Y=140;work.LevelId="level.poc_work";map.Nodes.Add(work);ui.RegionFocusNode(map,visited);
   UnityEngine.Rect edge;string arrow;Require(RegionEdgeLabel(work,RegionNodeRect(work,RegionModelBounds(RegionDeckNodes(map,"low")),canvas,ui.hudRegionZoom,ui.hudRegionPan),canvas,out edge,out arrow)&&arrow=="\u2192"&&canvas.Contains(edge.center),"distant B7 connection remains discoverable at the focused view edge");
   Require(!RegionEdgeLabel(unknown,new UnityEngine.Rect(2000,500,28,19),canvas,out edge,out arrow),"unknown remote places cannot get edge labels");
   ui.RegionResetView();Near(ui.hudRegionZoom,1,"fit-all restores overview");foreach(var n in RegionDeckNodes(map,"low"))Require(canvas.Contains(RegionMapPoint(n.X,n.Y,RegionModelBounds(RegionDeckNodes(map,"low")),canvas,ui.hudRegionZoom,ui.hudRegionPan)),"overview includes the remote B7 destination");
   System.Console.WriteLine("region UI projection / knowledge / travel PASS");
  }
 }
}
`
	dir := t.TempDir()
	source := filepath.Join(dir, "RegionProof.cs")
	output := filepath.Join(dir, "RegionProof.exe")
	if err := os.WriteFile(source, []byte(hudTemplate(t, "region.cs.tmpl")+harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/r:"+facade, "/r:"+core, "/r:"+imgui, "/r:"+textRendering, "/out:"+output, source).CombinedOutput(); err != nil {
		t.Fatalf("compile full region template against Unity: %v\n%s", err, out)
	}
	cmd := exec.Command(mono, output)
	cmd.Env = append(os.Environ(), "MONO_PATH="+managed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("execute region presentation helpers: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
