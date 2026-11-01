package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"github.com/strogmv/ang/compiler/locationsdk"
)

// demo-hub v11-text: every slot is resolved and measured in every locale with the font metrics Unity draws with.
func worldTextFixture(t *testing.T) (string, Compiled) {
	t.Helper()
	unity := t.TempDir()
	dir := filepath.Join(unity, "Assets", "Resources", "WorldText")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	glyphs := []map[string]any{{"c": " ", "adv": 0.25}}
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789·.ГОРЯЧЕКАШЙ" {
		glyphs = append(glyphs, map[string]any{"c": string(r), "adv": 0.5})
	}
	for _, style := range []string{"stencil", "chalk"} {
		raw, _ := json.Marshal(map[string]any{"schema": "ang.world_text.font.v1", "style": style, "family": "Test", "metrics": map[string]any{"cap_height": 0.7}, "glyphs": glyphs})
		if err := os.WriteFile(filepath.Join(dir, style+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	strips := &locationsdk.TextStrips{Count: 16, PitchMM: 305, WidthMM: 296}
	level := locationsdk.RuntimeLevel{ID: "hub",
		Vistas: []locationsdk.Vista{{ID: "vista.a", SignKey: "ui.corridor"}},
		TextSlots: []locationsdk.TextSlot{
			{ID: "strips", Face: locationsdk.South, WidthMM: 4871, HeightMM: 290, MarginMM: 30, Style: "stencil", Surface: "pvc", Ink: "#000000", MinGlyphMM: 150, MaxLines: 1, Layout: "strips_auto", Strips: strips, Key: "sign.strips", Reason: "r"},
			{ID: "number", Face: locationsdk.South, WidthMM: 600, HeightMM: 320, MarginMM: 25, Style: "stencil", Surface: "steel", Ink: "#FFFFFF", MinGlyphMM: 200, MaxLines: 1, Layout: "rows", Key: "sign.number", Args: map[string]string{"number": "6"}, Reason: "r"},
			{ID: "board", Face: locationsdk.South, WidthMM: 720, HeightMM: 550, MarginMM: 40, Style: "chalk", Surface: "board", Ink: "#FFFFFF", MinGlyphMM: 75, MaxLines: 4, Layout: "rows",
				PriceList: &locationsdk.TextPriceList{Store: "shop/display", LineKey: "sign.line", FooterKey: "sign.footer", Currency: "CRD", Rows: []locationsdk.TextPriceRow{{Item: "item.porridge", NameKey: "sign.porridge"}}}, Reason: "r"},
		}}
	msg := func(key, ru, en string) LocalizationMessage {
		return LocalizationMessage{Key: key, Context: "test", Texts: map[string]string{"ru": ru, "en": en}}
	}
	c := Compiled{
		LocationSDK: &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{level}},
		WorldText: WorldTextCatalog{Fonts: []WorldTextFontRef{{Style: "stencil", Metrics: "WorldText/stencil", TrackingEM: 0.06, LinePitch: 1.4}, {Style: "chalk", Metrics: "WorldText/chalk", TrackingEM: 0.02, LinePitch: 1.35}},
			VistaSign: &WorldTextVistaSign{Style: "stencil", Surface: "steel", Ink: "#000000", WidthMM: 2700, HeightMM: 600, MarginMM: 60, MinGlyphMM: 150}},
		Localization: LocalizationCatalog{DefaultLocale: "ru", FallbackLocale: "en", Locales: []string{"ru", "en"}, Messages: []LocalizationMessage{
			msg("sign.strips", "ГОРЯЧЕЕ · КАША · ЧАЙ", "HOT · PORRIDGE · TEA"), msg("sign.number", "{number}", "{number}"),
			msg("sign.line", "{item} {price}", "{item} {price}"), msg("sign.footer", "{currency}", "{currency}"), msg("sign.porridge", "КАША", "PORRIDGE"),
			msg("ui.corridor", "ГОРЯЧЕЕ", "HOT"),
		}},
		Inventory: inv.Model{
			Definitions: map[string]inv.Definition{"item.porridge": {ID: "item.porridge", PriceUnit: "CRD", PriceNumerator: 300, PriceDenominator: 1}},
			Currencies:  map[string]inv.Currency{"CRD": {Code: "CRD", Scale: 100}},
			Sources:     []inv.Source{{ID: "s", Definition: "item.porridge", Quantity: 10, ForSale: true, Location: inv.Location{Kind: "store", Store: "shop/display"}}},
		},
	}
	return unity, c
}

func TestWorldTextFitsEveryLocaleWithServerPrices(t *testing.T) {
	unity, c := worldTextFixture(t)
	out, err := compileWorldText(unity, c)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]WorldTextSlotDrawn{}
	for _, s := range out.Slots {
		got[s.ID] = s
	}
	if len(got) != 4 || got["vista.a.sign"].Vista != "vista.a" {
		t.Fatalf("slots %v", got)
	}
	for _, x := range got["strips"].Texts {
		if x.Mode != "per_strip" || x.GlyphMM < 150 || x.FirstStrip != 0 {
			t.Fatalf("16 glyphs on 16 strips: %+v", x)
		}
	}
	if n := got["number"].Texts[0]; n.Lines[0] != "6" || n.GlyphMM != 270 {
		t.Fatalf("gate number: %+v", n)
	}
	if b := got["board"].Texts[1]; strings.Join(b.Lines, "|") != "PORRIDGE 3|CRD" || b.Locale != "en" {
		t.Fatalf("price board must show the server price in units: %+v", b)
	}
}

func TestWorldTextRejectsTextsThatDoNotFit(t *testing.T) {
	for name, change := range map[string]func(*Compiled){
		"longer than the strips: flow under the minimum": func(c *Compiled) {
			c.Localization.Messages[0].Texts["en"] = "HOT · PORRIDGE · TEA · HOT · PORRIDGE · TEA · HOT · PORRIDGE · TEA"
		},
		"glyph missing in the font": func(c *Compiled) { c.Localization.Messages[0].Texts["en"] = "HOT · PORRIDGE · TEA!" },
		"locale missing":            func(c *Compiled) { delete(c.Localization.Messages[0].Texts, "en") },
		"argument missing":          func(c *Compiled) { c.LocationSDK.Levels[0].TextSlots[1].Args = nil },
		"item not sold there":       func(c *Compiled) { c.Inventory.Sources[0].Location.Store = "other" },
		"other currency":            func(c *Compiled) { c.LocationSDK.Levels[0].TextSlots[2].PriceList.Currency = "SLT" },
		"vista plate too small":     func(c *Compiled) { c.WorldText.VistaSign.WidthMM = 500 },
	} {
		unity, c := worldTextFixture(t)
		change(&c)
		if _, err := compileWorldText(unity, c); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	unity, c := worldTextFixture(t)
	c.Localization.Messages[0].Texts["en"] = "HOT PORRIDGE · HOT TEA · VITAMINS"
	out, err := compileWorldText(unity, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range out.Slots {
		if s.ID == "strips" && (s.Texts[1].Mode != "flow" || s.Texts[0].Mode != "per_strip") {
			t.Fatalf("strips_auto must choose per locale: %+v", s.Texts)
		}
	}
}
