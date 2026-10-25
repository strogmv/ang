package gamepoc

import (
	"strings"
	"testing"
)

func hudTemplate(t *testing.T, name string) string {
	t.Helper()
	data, err := sources.ReadFile("templates/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func TestHudUsesPortraitAndAuthoritativeStatus(t *testing.T) {
	s := hudTemplate(t, "hud.cs.tmpl")
	for _, want := range []string{"resource.art.hud.hero_portrait", "characterIdleTextures.TryGetValue", "state.PlayerName", "state.Inventory.CarriedMassG", "state.Inventory.CapacityMassG", "HudActivity()", `L("ui.hud.coverage_unknown")`, "Связь с игровым сервером потеряна"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing authoritative HUD contract %q", want)
		}
	}
	for _, bad := range []string{"11,4 кг", "В порядке · бронза", "connected?\"Сеть купола", "Сеть купола: подключена"} {
		if strings.Contains(s, bad) {
			t.Errorf("invented state %q", bad)
		}
	}
	if !strings.Contains(s, "new Rect(1024,31,238,92).Contains(mouse)") {
		t.Fatal("top-right controls must block world clicks through their whole footprint")
	}
}
func TestHudMapUsesOnlyPlayerObservations(t *testing.T) {
	s := sceneTemplateSection(t, "hud.cs.tmpl", "  void DrawHudLocalMap(){", "  void DrawHudJournal(){")
	for _, want := range []string{"var view=state.PlayerView", "ViewCell(view,x,y)!='0'", "seen=='0'", "view.Terrain[y][x]", "if(seen=='1')", "ViewCell(view,state.PlayerX,state.PlayerY)=='2'"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing observed-map guard %q", want)
		}
	}
	for _, bad := range []string{"resource.spatial", "CurrentSpatialLevel()", "state.Entities", "state.SceneAgents", "ObserverView()"} {
		if strings.Contains(s, bad) {
			t.Errorf("map may reveal non-player observations: %q", bad)
		}
	}
}
func TestHudPagesAndActionsRemainReal(t *testing.T) {
	s := hudTemplate(t, "hud.cs.tmpl")
	for _, want := range []string{"KeyCode.I", "KeyCode.C", "KeyCode.M", "KeyCode.J", "HudHasMap()", "HudHasJournal()", "state.Life.Events[i]", "DrawHudPage()", "DrawInventory()", "DrawDevelopment()", "DrawHudMap()", "DrawHudJournal()", "SendInventory(\"drop\",bag)", "Options(!state.Paused,1)"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing action/page %q", want)
		}
	}
	preview := sceneTemplateSection(t, "hud.cs.tmpl", "  void DrawArtPreviewHud(", "\n }\n}")
	if !strings.Contains(preview, "Художественная примерка") {
		t.Fatal("preview must be identified")
	}
	for _, bad := range []string{"state.Inventory", "HudMass()", "HudTime()", "DevelopmentView()", "connected", "Вещи\\n[I]"} {
		if strings.Contains(preview, bad) {
			t.Errorf("art preview invents gameplay UI %q", bad)
		}
	}
}
func TestHudInventoryPreservesCapabilitiesAndCommandIdentity(t *testing.T) {
	s := hudTemplate(t, "inventory.cs.tmpl")
	for _, want := range []string{"if(inventoryPending||state==null||state.Inventory==null||!CanSend()||loadingActive)return", "Sequence=state.Inventory.NextSequence", "ExpectedRevision=item==null?0:item.Revision", "selected.CanTake", "selected.CanBuy", "selected.CanEquip", "selected.CanUnequip", "selected.CanDrop", "selected.Carried", "store.Open", "state.Inventory.Balances", "state.Inventory.Entitlements", "inventorySelectedID=item.Id"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing inventory guard/state %q", want)
		}
	}
	for _, action := range []string{"take", "buy", "equip", "unequip", "drop", "put", "open"} {
		if !strings.Contains(s, "SendInventory(\""+action+"\"") {
			t.Errorf("lost inventory action %s", action)
		}
	}
	if strings.Contains(s, "GUI.enabled=true;") {
		t.Fatal("inventory must preserve outer enabled state")
	}
}
func TestHudDevelopmentUsesServerPracticeOffers(t *testing.T) {
	s := hudTemplate(t, "development.cs.tmpl")
	for _, want := range []string{"offer.CanStart&&!view.Pending&&practiceRequestId==\"\"&&CanSend()", "SendPractice(offer)", "skill.ValueMilli", "skill.Cap", "offer.Reason", "view.Events[i]", "view.RemainingTicks"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing practice guard/state %q", want)
		}
	}
}

func TestHudCompactLabelsKeepNativeTextWithinFrames(t *testing.T) {
	s := hudTemplate(t, "hud.cs.tmpl")
	for _, want := range []string{`new Rect(1160,94,102,29),"Отладка"`, `new Rect(119,740,245,18),HudActivity()`, `new Rect(119,758,245,26),HudMass()`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing native clipping regression guard %q", want)
		}
	}
	if !strings.Contains(s, "KeyCode.F3") {
		t.Fatal("compact debug label must preserve the keyboard shortcut")
	}
	development := hudTemplate(t, "development.cs.tmpl")
	if !strings.Contains(development, `((decimal)milli/1000).ToString("0.###")`) {
		t.Fatal("skill display must preserve milli precision without redundant zeros")
	}
}

func TestHudJournalKeepsGlobalEventsBehindDebugGuard(t *testing.T) {
	s := sceneTemplateSection(t, "hud.cs.tmpl", "  void DrawHudJournal(){", "  void DrawArtPreviewHud(")
	guard := strings.Index(s, "if(!hudDebugOpen){")
	stop := strings.Index(s, "return;")
	events := strings.Index(s, "state.Life.Events")
	debugTitle := strings.Index(s, "Отладочный журнал симуляции")
	if guard < 0 || stop < guard || debugTitle < stop || events < debugTitle {
		t.Fatal("global Life.Events must be reachable only after the non-debug early return")
	}
	ordinary := s[guard:stop]
	for _, want := range []string{"HudLocation()", "HudActivity()", "Записей героя пока нет"} {
		if !strings.Contains(ordinary, want) {
			t.Errorf("ordinary journal missing truthful current-state content %q", want)
		}
	}
	for _, bad := range []string{".Events", "foreach", "for(", "Regex", "Contains("} {
		if strings.Contains(ordinary, bad) {
			t.Errorf("ordinary journal must not enumerate/filter global telemetry: %q", bad)
		}
	}
}
