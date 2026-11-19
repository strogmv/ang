package gamepoc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/emitter"
)

func screenFixture(t *testing.T) (string, Compiled, *normalizer.GRPCDef) {
	t.Helper()
	root := t.TempDir()
	// A minimal SFNT with an explicit 600/1000 advance and cmap, no system font dependency.
	put := binary.BigEndian.PutUint16
	put32 := binary.BigEndian.PutUint32
	head := make([]byte, 20)
	put(head[18:], 1000)
	hhea := make([]byte, 36)
	put(hhea[34:], 1)
	hmtx := make([]byte, 4)
	put(hmtx, 600)
	cmap := make([]byte, 40)
	put(cmap[2:], 1)
	put(cmap[4:], 3)
	put(cmap[6:], 10)
	put32(cmap[8:], 12)
	put(cmap[12:], 12)
	put32(cmap[16:], 28)
	put32(cmap[24:], 1)
	put32(cmap[28:], 32)
	put32(cmap[32:], 65535)
	put32(cmap[36:], 1)
	tables := map[string][]byte{"head": head, "hhea": hhea, "hmtx": hmtx, "cmap": cmap}
	font := make([]byte, 12+len(tables)*16)
	put(font[4:], uint16(len(tables)))
	for i, k := range sortedScreenKeys(tables) {
		at := 12 + 16*i
		copy(font[at:], k)
		put32(font[at+8:], uint32(len(font)))
		put32(font[at+12:], uint32(len(tables[k])))
		font = append(font, tables[k]...)
	}
	if err := os.WriteFile(filepath.Join(root, "font.ttf"), font, 0600); err != nil {
		t.Fatal(err)
	}
	c := Compiled{GodotTheme: &GodotTheme{Target: ".", Skins: []GodotThemeSkin{{ID: "copper"}}, Fonts: []GodotThemeAsset{{File: "res://font.ttf"}}}, Localization: LocalizationCatalog{Locales: []string{"ru", "en"}, Messages: []LocalizationMessage{{Key: "ui.test.title", Texts: map[string]string{"ru": "Title", "en": "Title"}}, {Key: "ui.test.action", Texts: map[string]string{"ru": "Take", "en": "Take"}}}}}
	action := &ScreenAction{Command: "inventory_command", Params: map[string]ScreenValue{"item_id": {Path: "row.id"}, "epoch": {Path: "state.epoch"}, "quantity": {Value: float64(1)}}}
	button := ScreenControl{ID: "take", Kind: "button", Key: "ui.test.action", FontPX: 19, WidthPX: 400, Lines: 2, Action: action}
	list := ScreenControl{ID: "items", Kind: "list", FontPX: 19, WidthPX: 900, Lines: 8, Bind: map[string]ScreenValue{"source": {Path: "state.inventory.items"}}, Children: []ScreenControl{button}}
	screen := GodotScreen{ID: "bag", Chrome: "folk", Skin: "copper", Title: "ui.test.title", Layout: "page", Close: ScreenAction{Command: "host.close"}, Focus: []string{"items", "take"}, Controls: []ScreenControl{list}}
	c.GodotScreens = &GodotScreens{Screens: []GodotScreen{screen}}
	d := &normalizer.GRPCDef{Messages: []normalizer.GRPCMessage{
		{Name: "PocGameState", Fields: []normalizer.GRPCField{{Name: "epoch", Type: "string"}, {Name: "inventory", Type: "Inventory"}}},
		{Name: "Inventory", Fields: []normalizer.GRPCField{{Name: "items", Type: "Item", Repeated: true}}},
		{Name: "Item", Fields: []normalizer.GRPCField{{Name: "id", Type: "string"}, {Name: "name", Type: "string"}}},
		{Name: "ClientMessage", Oneofs: []normalizer.GRPCOneof{{Members: []normalizer.GRPCField{{Name: "inventory_command", Type: "Command"}}}}},
		{Name: "Command", Fields: []normalizer.GRPCField{{Name: "item_id", Type: "string"}, {Name: "epoch", Type: "string"}, {Name: "quantity", Type: "int32"}}},
	}}
	return root, c, d
}
func TestGodotScreensTypedDeterministic(t *testing.T) {
	root, c, d := screenFixture(t)
	a, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	b, err := compileGodotScreens(root, c, d)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatalf("nondeterministic screens: %v", err)
	}
	for _, piece := range []string{"state: C.PocGameState", "(row as C.Item).id", "state.inventory.items", "q.item_id = read(", "message.inventory_command = q"} {
		if !strings.Contains(string(a), piece) {
			t.Fatalf("missing typed binding %s", piece)
		}
	}
}

func TestGodotScreensDirectReaderDispatch(t *testing.T) {
	root, c, d := screenFixture(t)
	for index := 0; index < 300; index++ {
		c.GodotScreens.Screens[0].Controls = append(c.GodotScreens.Screens[0].Controls, ScreenControl{
			ID: fmt.Sprintf("label_%d", index), Kind: "label", FontPX: 19, WidthPX: 900, Lines: 1,
			Bind: map[string]ScreenValue{"text": {Path: "state.epoch"}},
		})
	}
	catalog, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	last := c.GodotScreens.Screens[0].Controls[300].Bind["text"].Ref
	if !strings.Contains(string(catalog), fmt.Sprintf("%q: _read_%s", last, last)) ||
		!strings.Contains(string(catalog), "_readers.get(id, Callable())") {
		t.Fatal("late bindings must dispatch directly to a typed reader")
	}
	bin := os.Getenv("ANG_GODOT_BIN")
	if bin == "" {
		t.Log("set ANG_GODOT_BIN for actual typed-reader execution")
		return
	}
	d.Package = "game.v1"
	d.Services = []normalizer.GRPCService{{Name: "API", Methods: []normalizer.GRPCMethod{
		{Name: "Call", Request: "ClientMessage", Response: "PocGameState", Stream: "none"},
	}}}
	for i := range d.Messages {
		for j := range d.Messages[i].Fields {
			d.Messages[i].Fields[j].Number = j + 1
		}
		for j := range d.Messages[i].Oneofs {
			d.Messages[i].Oneofs[j].Name = "msg"
			for k := range d.Messages[i].Oneofs[j].Members {
				d.Messages[i].Oneofs[j].Members[k].Number = len(d.Messages[i].Fields) + k + 1
			}
		}
	}
	contract, err := emitter.RenderGDScriptContract(d, "screen-readers.test")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"generated/screens/catalog.gd": catalog,
		"generated/game_contract.gd":   contract,
		"project.godot": []byte("config_version=5\n[application]\nconfig/name=\"Typed readers fixture\"\n" +
			"config/use_custom_user_dir=true\nconfig/custom_user_dir_name=\"phase6-reader-fixture\"\n"),
		"test.gd": []byte(fmt.Sprintf(`extends SceneTree
const C = preload("res://generated/game_contract.gd")
const Catalog = preload("res://generated/screens/catalog.gd")
func _initialize():
	var state = C.PocGameState.new()
	state.epoch = "current epoch"
	state.inventory = C.Inventory.new()
	var item = C.Item.new()
	item.id = "current item"
	state.inventory.items.append(item)
	var controls = Catalog.specification().screens[0].controls
	var source = controls[0].bind.source.ref
	assert(Catalog.read(source, state, null, {}) == state.inventory.items)
	assert(Catalog.read(%q, state, null, {}) == state.epoch)
	assert(Catalog.read(source, null, null, {}) == null)
	assert(Catalog.read("unknown", state, null, {}) == null)
	var command = Catalog.command(controls[0].children[0].action.ref, state, item, {}, "request")
	assert(command.inventory_command.item_id == item.id)
	assert(command.inventory_command.epoch == state.epoch)
	quit()
`, last)),
	}
	for _, path := range c.GodotScreens.Themes {
		files[strings.TrimPrefix(path, "res://")] = []byte("[gd_resource type=\"Theme\" format=3]\n[resource]\n")
	}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, bin, "--headless", "--path", root,
		"--audio-driver", "Dummy", "--script", "res://test.gd").CombinedOutput()
	if err != nil || strings.Contains(string(output), "ERROR:") {
		t.Fatalf("typed reader execution failed: %v\n%s", err, output)
	}
}
func TestGodotScreensNegative(t *testing.T) {
	cases := []struct {
		name, field string
		mutate      func(*Compiled)
	}{
		{"unknown path", "controls.items.bind.source", func(c *Compiled) {
			c.GodotScreens.Screens[0].Controls[0].Bind["source"] = ScreenValue{Path: "state.inventory.missing"}
		}},
		{"missing locale", "controls.take.text_key", func(c *Compiled) { delete(c.Localization.Messages[1].Texts, "en") }},
		{"missing text key", "controls.take.text_key", func(c *Compiled) { c.GodotScreens.Screens[0].Controls[0].Children[0].Key = "ui.test.absent" }},
		{"missing theme", "skin", func(c *Compiled) { c.GodotTheme = nil }},
		{"unknown skin", "skin", func(c *Compiled) { c.GodotScreens.Screens[0].Skin = "unknown" }},
		{"unknown chrome", "chrome", func(c *Compiled) { c.GodotScreens.Screens[0].Chrome = "unknown" }},
		{"action without command", "controls.take.action.command", func(c *Compiled) { c.GodotScreens.Screens[0].Controls[0].Children[0].Action.Command = "" }},
		{"unreachable control", "focus", func(c *Compiled) { c.GodotScreens.Screens[0].Focus = []string{"items"} }},
		{"text cannot fit", "controls.take.text_key", func(c *Compiled) { c.Localization.Messages[1].Texts["en"] = strings.Repeat("Wide", 200) }},
		{"quiet button cannot wrap", "controls.take.text_key", func(c *Compiled) { c.Localization.Messages[1].Texts["en"] = strings.Repeat("Wide ", 7) }},
		{"text below 12", "controls.take.text_key", func(c *Compiled) { c.GodotScreens.Screens[0].Controls[0].Children[0].FontPX = 11 }},
		{"unknown command field", "controls.take.action.params.item_typo", func(c *Compiled) {
			c.GodotScreens.Screens[0].Controls[0].Children[0].Action.Params["item_typo"] = ScreenValue{Path: "row.id"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, c, d := screenFixture(t)
			tc.mutate(&c)
			_, err := compileGodotScreens(root, c, d)
			if err == nil || !strings.Contains(err.Error(), "screen bag field "+tc.field) {
				t.Fatalf("wanted screen id and field %s; got %v", tc.field, err)
			}
		})
	}
}

func TestGodotBookComposition(t *testing.T) {
	root, c, d := screenFixture(t)
	c.GodotScreens.ChromeProfiles = map[string]ScreenChrome{
		"book": {FrameMargins: []float64{50, 142, 50, 80}, Padding: []float64{62, 48, 62, 48},
			OrnamentSize: []float64{62, 64}, TitlePX: 43, HeadingOutside: true},
	}
	c.GodotScreens.Screens[0].ChromeProfile = "book"
	c.GodotScreens.Screens[0].Controls = append(c.GodotScreens.Screens[0].Controls,
		ScreenControl{ID: "divider", Kind: "divider", FontPX: 19, WidthPX: 900, Lines: 8})
	if _, err := compileGodotScreens(root, c, d); err != nil {
		t.Fatal(err)
	}
	c.GodotScreens.Screens[0].ChromeProfile = "missing"
	if _, err := compileGodotScreens(root, c, d); err == nil ||
		!strings.Contains(err.Error(), "screen bag field chrome_profile") {
		t.Fatalf("unvalidated chrome: %v", err)
	}
	c.GodotScreens.Screens[0].ChromeProfile = "book"
	p := c.GodotScreens.ChromeProfiles["book"]
	p.TitlePX = 11
	c.GodotScreens.ChromeProfiles["book"] = p
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "title_px") {
		t.Fatalf("small heading: %v", err)
	}
}
func TestGodotCustomSelectionUsesContractRows(t *testing.T) {
	root, c, d := screenFixture(t)
	c.GodotScreens.Widgets = map[string]string{"atlas": "res://atlas.gd"}
	list := &c.GodotScreens.Screens[0].Controls[0]
	list.Kind = "custom"
	list.Widget = "atlas"
	list.ActionSource = "source"
	list.Children = nil
	c.GodotScreens.Screens[0].Focus = []string{"items", "take"}
	c.GodotScreens.Screens[0].Controls = append(c.GodotScreens.Screens[0].Controls,
		ScreenControl{ID: "take", Kind: "button", FontPX: 19, WidthPX: 620,
			Action: &ScreenAction{Command: "inventory_command", Params: map[string]ScreenValue{
				"item_id": {Path: "selected.items.id"}}}})
	out, err := compileGodotScreens(root, c, d)
	if err != nil || !bytes.Contains(out, []byte("selected.get(\"items\")")) {
		t.Fatalf("custom selection is not typed: %v", err)
	}
}

func TestGodotScreenGridVisibleRows(t *testing.T) {
	root, c, d := screenFixture(t)
	c.GodotScreens.Screens[0].Controls[0].Columns = 2
	c.GodotScreens.Screens[0].Controls[0].Mode = "choices"
	c.GodotScreens.Screens[0].Controls[0].Children = []ScreenControl{{ID: "name", Kind: "label", FontPX: 19, WidthPX: 400, Lines: 2, Bind: map[string]ScreenValue{"text": {Path: "row.name"}}}}
	c.GodotScreens.Screens[0].Focus = []string{"items"}
	c.GodotScreens.Screens[0].Controls[0].VisibleRows = 2
	output, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `\"visible_rows\":2`) {
		t.Fatal("visible rows lost from catalog")
	}
	for _, rows := range []int{-1, 10} {
		c.GodotScreens.Screens[0].Controls[0].VisibleRows = rows
		_, err = compileGodotScreens(root, c, d)
		if err == nil || !strings.Contains(err.Error(), "screen bag field controls.items.visible_rows") {
			t.Fatalf("invalid visible rows accepted: %v", err)
		}
	}
	c.GodotScreens.Screens[0].Controls[0].VisibleRows = 2
	c.GodotScreens.Screens[0].Controls[0].Columns = 0
	_, err = compileGodotScreens(root, c, d)
	if err == nil || !strings.Contains(err.Error(), "controls.items.visible_rows") {
		t.Fatalf("non-grid accepted: %v", err)
	}
}

// A title crest draws the name large and the subtitle under it smaller (line_px): the check measures each line at the
// size it is drawn, so a subtitle that only fits at its own size passes, and the same text at one size does not.
func TestScreenTextMeasuresEachLineAtItsOwnSize(t *testing.T) {
	root, c, _ := screenFixture(t)
	c.Localization.Messages = append(c.Localization.Messages, LocalizationMessage{Key: "ui.test.logo",
		Texts: map[string]string{"ru": "Futurocide\nSecond Creation", "en": "Futurocide\nSecond Creation"}})
	if err := screenTextSized(c, "ui.test.logo", 69, nil, 696, 2, root); err == nil || !strings.Contains(err.Error(), "3 > 2 lines") {
		t.Fatalf("one size for both lines must wrap the subtitle: %v", err)
	}
	if err := screenTextSized(c, "ui.test.logo", 69, []int{69, 38}, 696, 2, root); err != nil {
		t.Fatalf("each line at its own size must fit: %v", err)
	}
	if err := screenTextSized(c, "ui.test.logo", 69, []int{69, 60}, 696, 2, root); err == nil {
		t.Fatal("a subtitle too large for the room must still fail")
	}
	if err := screenTextSized(c, "ui.test.logo", 69, []int{69, 11}, 696, 2, root); err == nil || !strings.Contains(err.Error(), "below 12 px") {
		t.Fatalf("a line below 12 px must fail: %v", err)
	}
}

// Menus from the keyboard (owner 19.11): a held Esc goes back once (the main menu's Esc opens the exit confirmation,
// whose Esc goes back, so a repeat would toggle between them), and a way back gives the focus to the control the screen
// was left from (the main menu's card), not to the screen's first one. The game walks it with its menu tool
// (frames.py --keys); this guards the shared runtime's half.
func TestScreenEscGoesBackOnceAndBackKeepsTheFocus(t *testing.T) {
	navigation, err := sources.ReadFile("templates/godot_screen_navigation.gd.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	escape := strings.Index(string(navigation), "event.keycode == KEY_ESCAPE:")
	closing := strings.Index(string(navigation), "ui._close()")
	if escape < 0 || closing < escape || !strings.Contains(string(navigation)[escape:closing], "if not event.echo:") {
		t.Fatal("Esc must close the screen on the first press only, not on its repeats")
	}
	runtime, err := sources.ReadFile("templates/godot_screen_runtime.gd.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, piece := range []string{
		"func present(id: String, snapshot: C.PocGameState, context = null, requested_us := 0, resume := false)",
		"focus_memory[screen_id] = str(leaving.get_meta(\"focus_uid\", \"\"))",
		"resume_focus = str(focus_memory.get(id, \"\")) if resume else \"\"",
		"if rail[i].get_meta(\"focus_uid\", \"\") == wanted:",
		"fallback_focus = index < 0 or resumed",
	} {
		if !strings.Contains(string(runtime), piece) {
			t.Fatalf("a way back must give the focus back: missing %q", piece)
		}
	}
}
