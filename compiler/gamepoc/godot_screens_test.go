package gamepoc

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
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
