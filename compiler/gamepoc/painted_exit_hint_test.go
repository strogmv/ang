package gamepoc

import "testing"

func TestPaintedSharedDoorHoverMatchesExplicitDestinationChoiceNative(t *testing.T) {
	helper := sceneTemplateSection(t, "painted.cs.tmpl", "  string PaintedKnownExitHint(", "  static bool PaintedInspectionFinite(")
	runPaintedTextureNative(t, "PaintedExitHint", `using System;using System.Collections.Generic;
class PocLifeMarker{public int X,Y;public string Name;public bool visible=true;}
class Life{public List<PocLifeMarker> Markers=new List<PocLifeMarker>();}
class State{public Life Life=new Life();}
class Proof{State state=new State();bool LifeDoorVisible(PocLifeMarker m){return m.visible;}string L(string key,params string[] args){return key+(args.Length>0?":"+args[0]:"");}
`+helper+`
static void Need(bool value,string why){if(!value)throw new Exception(why);}
public static void Main(){var p=new Proof();var station=new PocLifeMarker{X=3,Y=4,Name="Station"};var hub=new PocLifeMarker{X=3,Y=4,Name="Hub"};p.state.Life.Markers.Add(station);p.state.Life.Markers.Add(hub);
Need(p.PaintedKnownExitHint(3,4)=="ui.wayfinding.choose","shared doorway misleadingly names its first exit");p.state.Life.Markers.Reverse();Need(p.PaintedKnownExitHint(3,4)=="ui.wayfinding.choose","shared hint depends on marker order");hub.visible=false;Need(p.PaintedKnownExitHint(3,4)=="ui.exit.enter_hint:Station","unique visible exit lost its actual name");station.visible=false;Need(p.PaintedKnownExitHint(3,4)==null,"unknown doorway disclosed");station.visible=true;hub.visible=true;hub.X=8;Need(p.PaintedKnownExitHint(3,4)=="ui.exit.enter_hint:Station"&&p.PaintedKnownExitHint(8,4)=="ui.exit.enter_hint:Hub","distinct physical exits were merged");Need(p.PaintedKnownExitHint(0,0)==null,"empty floor invented an exit");Console.WriteLine("shared choice matches hover, order independent, unique names and hidden marker guard PASS");}}
`)
}
