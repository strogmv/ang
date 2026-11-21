package gamepoc

import (
	"os"
	"regexp"
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

func chipHudCompiled() Compiled {
	below := 0.5
	h := ChipHud{
		ItemSlots: 5,
		Feed: ChipHudFeed{Lines: 2, HitKey: "ui.hud.k", MissKey: "ui.hud.k", ActKey: "ui.hud.k", OtherKey: "ui.hud.k",
			Events: map[string]string{"attack": "ui.hud.k"}, Modes: map[string]string{"single": "ui.hud.k"}},
		Quick: []ChipHudQuick{{Definition: "item.patch", KindKey: "ui.hud.k", IdleKey: "ui.hud.k"}},
		Needs: []ChipHudNeed{{ID: "food", BrainIndex: 0, TextKey: "ui.hud.k"}},
		Seen:  []ChipHudLevel{{Below: &below, TextKey: "ui.hud.k"}, {TextKey: "ui.hud.k"}},
		Noise: ChipHudNoise{StillKey: "ui.hud.k", QuietKey: "ui.hud.k", WalkKey: "ui.hud.k", RunKey: "ui.hud.k"},
		Functions: []ChipHudFunction{
			{ID: "medmonitor", Icon: "pulse", NameKey: "ui.hud.k", TextKey: "ui.hud.k", Unlock: "start"},
			{ID: "dome_map", Icon: "dome_map", NameKey: "ui.hud.k", TextKey: "ui.hud.k", Unlock: "locked", ReasonKey: "ui.hud.k", LaterKey: "ui.hud.k"},
		},
		Link: ChipHudLink{Default: "dome", Levels: map[string]string{"level.a": "weak"},
			States: map[string]ChipHudLinkState{"dome": {TextKey: "ui.hud.k", Bars: 3}, "weak": {TextKey: "ui.hud.k", Bars: 1}}},
	}
	c := Compiled{Hud: &h}
	c.Inventory.Definitions = map[string]inv.Definition{"item.patch": {}}
	c.Localization.Messages = []LocalizationMessage{{Key: "ui.hud.k", Texts: map[string]string{"ru": "к", "en": "k"}}}
	return c
}

// The chip HUD's block: texts in every locale, real items once each, the brain's needs, climbing levels, open or locked
// functions with their reasons, link states that exist.
func TestChipHudValidates(t *testing.T) {
	if err := validateChipHud(chipHudCompiled()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ChipHud){
		"feed lines":   func(h *ChipHud) { h.Feed.Lines = 0 },
		"item slots":   func(h *ChipHud) { h.ItemSlots = 0 },
		"missing text": func(h *ChipHud) { h.Feed.HitKey = "ui.hud.none" },
		"unknown item": func(h *ChipHud) { h.Quick[0].Definition = "item.none" },
		"twice":        func(h *ChipHud) { h.Quick = append(h.Quick, h.Quick[0]) },
		"brain index":  func(h *ChipHud) { h.Needs[0].BrainIndex = 5 },
		"seen bound":   func(h *ChipHud) { h.Seen = h.Seen[:1] },
		"unlock":       func(h *ChipHud) { h.Functions[0].Unlock = "story" },
		"open reason":  func(h *ChipHud) { h.Functions[0].ReasonKey = "ui.hud.k" },
		"locked later": func(h *ChipHud) { h.Functions[1].LaterKey = "" },
		"function id":  func(h *ChipHud) { h.Functions[1].ID = h.Functions[0].ID },
		"link default": func(h *ChipHud) { h.Link.Default = "none" },
		"link level":   func(h *ChipHud) { h.Link.Levels["level.b"] = "none" },
		"link bars":    func(h *ChipHud) { h.Link.States["dome"] = ChipHudLinkState{TextKey: "ui.hud.k", Bars: 5} },
	} {
		c := chipHudCompiled()
		change(c.Hud)
		if err := validateChipHud(c); err == nil {
			t.Fatalf("%s: want a failure", name)
		}
	}
}

// The server reads the HUD's block only to show: the template writes no state but the ПРЕДМЕТЫ slots the tick keeps
// (state.HudSlots, review 21.11), and a fighter the hero has not seen is only «?» (no id).
func TestChipHudTemplateOnlyShows(t *testing.T) {
	raw, err := os.ReadFile("templates/chip_hud.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, forbidden := range []string{"r.state.Combat.Actors[", "r.state.Life.Actors[", "= append(r.state", "r.state.Inventory.Items["} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("chip_hud template writes or indexes state: %s", forbidden)
		}
	}
	for _, m := range regexp.MustCompile(`r\.state\.([A-Za-z.]+)(?:\s*,\s*_)?\s*=[^=]`).FindAllStringSubmatch(src, -1) {
		if m[1] != "HudSlots" {
			t.Fatalf("chip_hud template writes state.%s", m[1])
		}
	}
	if !strings.Contains(src, "row.Unknown = true") || strings.Contains(src, "row.Unknown = true\n\t\t\trow.Id") {
		t.Fatal("an unseen fighter must be sent without his id")
	}
}
