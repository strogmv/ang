package gamepoc

import "testing"

func TestCombatUICategoriesPreserveAuthoritativeOptionsNative(t *testing.T) {
	selection := sceneTemplateSection(t, "combat_panel.cs.tmpl", "  List<PocCombatOption> CombatDisplayRows(){", "  bool CombatHeroTurn()")
	runPaintedTextureNative(t, "CombatUICategories", `using System;using System.Collections.Generic;
class PocCombatOption {public string Action,Group,Mode,Id;public bool Enabled;public int Ap;public string Reason;}
struct Vector2 {public static Vector2 zero;}
class Proof {
 string combatUiGroup="";int combatSelected,combatScrollSelected;Vector2 combatScroll;bool active=true;
 List<PocCombatOption> options=new List<PocCombatOption>();
 bool HeroCombatActive(){return active;}List<PocCombatOption> CombatRows(){return options;}
`+selection+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var p=new Proof();var attack=new PocCombatOption{Action="attack",Group="attack",Enabled=false,Ap=7,Reason="Out of reach"};var weapon=new PocCombatOption{Action="equip",Group="weapon",Enabled=true,Ap=2};var move=new PocCombatOption{Action="disengage",Group="move",Enabled=false,Reason="Break distance first"};var care=new PocCombatOption{Action="aid",Group="care",Enabled=false,Reason="Under threat"};var end=new PocCombatOption{Action="end",Group="turn",Enabled=true};p.options.AddRange(new[]{attack,weapon,move,care,end});
 Need(p.CombatDisplayRows().Count==4,"all actions lost an option or duplicated the fixed end button");
 p.CombatUiSelectGroup("attack");var rows=p.CombatDisplayRows();Need(rows.Count==2&&object.ReferenceEquals(rows[0],attack)&&object.ReferenceEquals(rows[1],weapon),"attack/weapon filter changed server options");Need(!rows[0].Enabled&&rows[0].Ap==7&&rows[0].Reason=="Out of reach","client changed authoritative refusal or AP");
 p.CombatUiSelectGroup("move");Need(p.CombatDisplayRows().Count==1&&object.ReferenceEquals(p.CombatDisplayRows()[0],move),"exit action hidden");
 p.CombatUiSelectGroup("care");Need(p.CombatDisplayRows().Count==1&&p.CombatDisplayRows()[0].Reason=="Under threat","care refusal hidden");
 p.CombatUiSelectGroup("stance");Need(p.CombatDisplayRows().Count==0,"empty category invents an action");Need(p.CombatRows().Contains(end),"category filter removed global end turn");
 p.active=false;p.options=new List<PocCombatOption>{care};Need(p.CombatDisplayRows().Count==1,"post-combat care filtered by stale category");
 Console.WriteLine("server options preserved; category/empty/exit/end-turn/post-combat-care PASS");
 }
}`)
}

func TestCombatUIHitAreasFollowVisiblePanelsNative(t *testing.T) {
	areas := sceneTemplateSection(t, "combat_panel.cs.tmpl", "  internal static readonly Rect CombatPanelFrame", "  string combatUiGroup")
	blocking := sceneTemplateSection(t, "combat_panel.cs.tmpl", "  bool CombatUiBlocks", "  internal Rect CombatPanelRect")
	runPaintedTextureNative(t, "CombatUIHitAreas", `using System;
struct Vector2 {public float x,y;public Vector2(float a,float b){x=a;y=b;}}
struct Rect {public float x,y,width,height;public Rect(float a,float b,float w,float h){x=a;y=b;width=w;height=h;}public bool Contains(Vector2 p){return p.x>=x&&p.x<x+width&&p.y>=y&&p.y<y+height;}}
class Proof {
 bool active=true,combatDetails,combatPanelOpen;string warning="";
 bool HeroCombatActive(){return active;}string CombatUiWarning(){return warning;}
`+areas+blocking+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var p=new Proof();Need(p.CombatUiBlocks(new Vector2(1100,740)),"end-turn clicks leak to floor");Need(p.CombatUiBlocks(new Vector2(1000,380)),"target/body-part clicks leak to floor");Need(!p.CombatUiBlocks(new Vector2(80,300)),"closed health panel eats world input");p.combatDetails=true;Need(p.CombatUiBlocks(new Vector2(80,300)),"open health panel leaks clicks");p.combatDetails=false;p.warning="Real own wound";Need(p.CombatUiBlocks(new Vector2(500,540)),"danger banner leaks clicks");p.warning="";Need(!p.CombatUiBlocks(new Vector2(500,540)),"absent warning blocks floor");p.active=false;Need(!p.CombatUiBlocks(new Vector2(1100,740)),"ended combat still blocks lower screen");p.combatPanelOpen=true;Need(p.CombatUiBlocks(new Vector2(1000,300)),"post-combat care leaks clicks");Console.WriteLine("visible HUD blocks floor; absent HUD releases floor PASS");}
}`)
}

func TestCombatUIHidesOnlyExactMedicalImplementationSuffixNative(t *testing.T) {
	label := sceneTemplateSection(t, "combat_panel.cs.tmpl", "  static string CombatUiActionLabel", "  void CombatPanelStyles")
	runPaintedTextureNative(t, "CombatUILabel", `using System;
class PocCombatOption {public string Group,TargetId,Label;}
class Proof {`+label+`
 static void Need(bool ok,string why){if(!ok)throw new Exception(why);}
 public static void Main(){var o=new PocCombatOption{Group="care",TargetId="w1",Label="Pressure · thigh (w1)"};Need(CombatUiActionLabel(o)=="Pressure · thigh","medical implementation ID leaked");Need(o.Label=="Pressure · thigh (w1)"&&o.TargetId=="w1","authoritative option mutated");o.Label="Pressure (left)";Need(CombatUiActionLabel(o)==o.Label,"legitimate localized parentheses removed");o.Group="attack";o.Label="Tool (w1)";Need(CombatUiActionLabel(o)==o.Label,"non-care item name altered");Console.WriteLine("presentation hides exact wound suffix; command and other labels preserved PASS");}
}`)
}
