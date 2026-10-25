package gamepoc

import (
	"strings"
	"testing"
)

func TestRegionNativeProofAuditsActualKnownProjection(t *testing.T) {
	filter := sceneTemplateSection(t, "region_proof.cs.tmpl", "  static string RegionProofKnowledgeError(", "  static void RegionProofPixelChanges(")
	runPaintedTextureNative(t, "RegionProofKnowledge", `using System;using System.Collections.Generic;
class PocRegionDeck{public string Id;}class PocRegionNode{public string Id,DeckId,Knowledge,LevelId;public bool Current,CanTravel;}class PocRegionLink{public string FromNode,ToNode;}
class PocRegionMap{public string Id="map",CurrentNode="market",CurrentDeck="bronze";public List<PocRegionDeck> Decks=new List<PocRegionDeck>();public List<PocRegionNode> Nodes=new List<PocRegionNode>();public List<PocRegionLink> Links=new List<PocRegionLink>();}
class Proof{
`+filter+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 static PocRegionMap Map(){var m=new PocRegionMap();m.Decks.Add(new PocRegionDeck{Id="bronze"});m.Nodes.Add(new PocRegionNode{Id="market",DeckId="bronze",Knowledge="visited",LevelId="market.level",Current=true});m.Nodes.Add(new PocRegionNode{Id="station",DeckId="bronze",Knowledge="known",LevelId="station.level",CanTravel=true});m.Links.Add(new PocRegionLink{FromNode="market",ToNode="station"});return m;}
 public static void Main(){
  Need(RegionProofKnowledgeError(Map())==null,"valid known map rejected");
  var m=Map();m.Nodes.Add(new PocRegionNode{Id="hidden",DeckId="bronze",Knowledge="unknown"});Need(RegionProofKnowledgeError(m)!=null,"unknown node entered native player map");
  m=Map();m.Links[0].ToNode="hidden";Need(RegionProofKnowledgeError(m)!=null,"dangling link revealed hidden endpoint");
  m=Map();m.Nodes[1].LevelId="";Need(RegionProofKnowledgeError(m)!=null,"unbuilt landmark permitted travel");
  m=Map();m.Nodes[1].DeckId="private.deck";Need(RegionProofKnowledgeError(m)!=null,"absent deck accepted");
  m=Map();m.Decks.Add(new PocRegionDeck{Id="hidden.empty.deck"});Need(RegionProofKnowledgeError(m)!=null,"empty private deck leaked through selector");
  m=Map();m.Nodes[1].Id="market";Need(RegionProofKnowledgeError(m)!=null,"duplicate node accepted");
  m=Map();m.CurrentNode="station";Need(RegionProofKnowledgeError(m)!=null,"current node differs from actual hero");
  Console.WriteLine("known-only native map: unknown nodes/decks/endpoints and fake travel rejected PASS");
 }
}`)
}

func TestRegionNativeSelectionAndClippedViewAudit(t *testing.T) {
	audit := sceneTemplateSection(t, "region_proof.cs.tmpl", "  sealed class RegionSelectionAudit ", "  static string RegionProofKnowledgeError(")
	pixels := sceneTemplateSection(t, "region_proof.cs.tmpl", "  static void RegionProofPixelChanges(", "  [Serializable] sealed class RegionMapCapture ")
	runPaintedTextureNative(t, "RegionProofStationaryClip", `using System;
struct Vector2{public float x,y;public Vector2(float a,float b){x=a;y=b;}}
struct Rect{float x,y,w,h;public Rect(float a,float b,float c,float d){x=a;y=b;w=c;h=d;}public bool Contains(Vector2 p){return p.x>=x&&p.x<x+w&&p.y>=y&&p.y<y+h;}}
struct Color32{public byte r,g,b;public Color32(byte v){r=g=b=v;}}
class Proof{
`+audit+pixels+`
 static void Need(bool b,string why){if(!b)throw new Exception(why);}
 public static void Main(){
  var a=new RegionSelectionAudit("market",4,7,false);for(int tick=0;tick<20;tick++)Need(a.Still("market",4,7,0,false,false),"real tick alone invalidated unchanged manual hero");
  Need(!a.Still("market",5,7,0,false,false)&&!a.Still("market",4,7,1,false,false)&&!a.Still("market",4,7,0,true,false)&&!a.Still("station",4,7,0,false,false)&&!a.Still("market",4,7,0,false,true),"selection moved hero/path/automatic/travel without detection");
  int w=1280,h=800;var before=new Color32[w*h];var after=(Color32[])before.Clone();var canvas=new Rect(324,282,650,364);int inside,outside;
  for(int y=350;y<400;y++)for(int x=500;x<560;x++)after[(h-y-1)*w+x]=new Color32(255);
  RegionProofPixelChanges(before,after,w,h,canvas,out inside,out outside);Need(inside>200&&outside==0,"valid zoom/pan changed the adjacent inspector");
  for(int y=350;y<400;y++)for(int x=1000;x<1040;x++)after[(h-y-1)*w+x]=new Color32(255);
  RegionProofPixelChanges(before,after,w,h,canvas,out inside,out outside);Need(outside>16,"map escaped canvas into inspector without detection");
  after=(Color32[])before.Clone();for(int x=0;x<w;x++)after[(h-10)*w+x]=new Color32(255);RegionProofPixelChanges(before,after,w,h,canvas,out inside,out outside);Need(outside==0,"live diagnostics mistaken for map clipping");
  Console.WriteLine("unchanged hero over real ticks; actual screenshot canvas/inspector comparison detects spill PASS");
 }
}`)
}

func TestRegionNativeProofUsesOrdinaryUIAndExactStationExit(t *testing.T) {
	s := hudTemplate(t, "region_proof.cs.tmpl")
	for _, want := range []string{
		`"--poc-verify-region-map"`, `Path.IsPathRooted`, `NewGame(777,"life")`, `LifeCommand("goto",RegionProofMarket)`,
		`RegionSelectNode(state.Life.RegionMap,station)`, `node.LevelId==RegionProofStation`, `route.LocalExitId!="market_station"`,
		`state.Tick-regionProofStationaryTick>=8`, `RegionProofStill()`, `RegionZoomAt(hudRegionCanvas.center`,
		`SetHudMapMode("local")`, `SetHudMapMode("floor")`, `RegionProofLocale("en")`, `english.Name==regionProofPlaceNameRU`,
		`"region-map-ru.png"`, `"region-map-en.png"`, `"region-map-zoom-pan.png"`, `"region-station-arrival.png"`,
		`ScreenCapture.CaptureScreenshotAsTexture()`, `RegionProofPixelChanges(regionProofSelectedPixels`,
		`CopperRouteLegAudit(marker.EdgeId`, `WayfindingGo()`, `state.Life.LevelId!=RegionProofStation`, `ReferenceEquals(regionProofSession,session)`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing ordinary native region-map evidence: %s", want)
		}
	}
	for _, bad := range []string{`state.PlayerX=`, `state.PlayerY=`, `state.Life.LevelId=`, `state.Tick=`, `state.GameSeconds=`, `new PocGameState`, `RegionMap.Nodes.Add`, `PlayerPrefs.Set`} {
		if strings.Contains(strings.ReplaceAll(s, "==", "EQ"), bad) {
			t.Errorf("native proof fabricates authority or persists test locale: %s", bad)
		}
	}
	if strings.Count(s, "NewGame(") != 1 || strings.Count(s, "WayfindingGo()") != 1 {
		t.Fatal("proof must preserve one real session and issue exactly one explicit Go")
	}
	u := hudTemplate(t, "unity.cs.tmpl")
	for _, hook := range []string{`InitRegionMapProof()`, `verifyLife = verifyRegionMap ||`, `RegionMapProofDiagnostics(now)`, `RegionMapProofStep(now)`, `RegionMapProofReportJSON()`, `return regionProofDirectory`, `RestoreRegionMapProofLocale()`} {
		if !strings.Contains(u, hook) {
			t.Errorf("missing native region-map runner hook: %s", hook)
		}
	}
}
