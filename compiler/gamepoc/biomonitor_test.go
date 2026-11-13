package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func biomonitorInt(v int) *int    { return &v }
func biomonitorBool(v bool) *bool { return &v }

// biomonitorTable is the owner's rule of 13.11 as the game's CUE writes it: whole, wounded, critical, dead.
func biomonitorTable() *BiomonitorCatalog {
	return &BiomonitorCatalog{Segments: 5, OffColour: "#14181B", Tiers: []BiomonitorTier{
		{ID: "dead", States: []string{"dead"}, Segments: biomonitorInt(0), Colour: "#000000", Note: "dead"},
		{ID: "critical", States: []string{"confused", "down"}, Segments: biomonitorInt(1), Colour: "#FF3020", Energy: 3, BlinkHz: 1.5, Note: "critical"},
		{ID: "wounded", States: []string{"ok", "weak"}, Wounded: biomonitorBool(true), Colour: "#FFB040", Energy: 2, SeriousMax: biomonitorInt(3),
			SegmentsByLoss: []BiomonitorLossRow{{BelowPermille: biomonitorInt(100), Segments: 4}, {BelowPermille: biomonitorInt(200), Segments: 3}, {Segments: 2}}, Note: "wounded"},
		{ID: "whole", States: []string{"ok"}, Wounded: biomonitorBool(false), Segments: biomonitorInt(5), Colour: "#B8FFDA", Energy: 1.5, Note: "whole"},
	}}
}

func TestBiomonitorTableAcceptedAndProjected(t *testing.T) {
	c := biomonitorTable()
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(clientBiomonitor(c))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"off_colour":"#14181B","segments":5,"tiers":[{"blink_hz":0,"colour":"#000000","energy":0,"id":"dead"},` +
		`{"blink_hz":1.5,"colour":"#FF3020","energy":3,"id":"critical"},{"blink_hz":0,"colour":"#FFB040","energy":2,"id":"wounded"},` +
		`{"blink_hz":0,"colour":"#B8FFDA","energy":1.5,"id":"whole"}]}`
	if string(raw) != want {
		t.Fatalf("client table:\n got %s\nwant %s", raw, want)
	}
	// Who is shown: only people bound to a look with a biomonitor row, sorted.
	a := Art{Looks: map[string]Look{"look.a": {Biomonitor: &LookBiomonitor{Surface: "glow_health"}}, "look.b": {}},
		PersonLooks: map[string]string{"char.z": "look.a", "char.b": "look.b", "char.a": "look.a"}}
	if got := strings.Join(biomonitorWearers(a), ","); got != "char.a,char.z" {
		t.Fatalf("wearers %s", got)
	}
}

func TestBiomonitorTableRejected(t *testing.T) {
	cases := map[string]func(c *BiomonitorCatalog){
		"segments_zero":   func(c *BiomonitorCatalog) { c.Segments = 0 },
		"segments_many":   func(c *BiomonitorCatalog) { c.Segments = 9 },
		"off_colour":      func(c *BiomonitorCatalog) { c.OffColour = "dark" },
		"no_tiers":        func(c *BiomonitorCatalog) { c.Tiers = nil },
		"duplicate_id":    func(c *BiomonitorCatalog) { c.Tiers[1].ID = "dead" },
		"bad_id":          func(c *BiomonitorCatalog) { c.Tiers[1].ID = "Critical" },
		"unknown_state":   func(c *BiomonitorCatalog) { c.Tiers[1].States = []string{"dying"} },
		"no_states":       func(c *BiomonitorCatalog) { c.Tiers[0].States = nil },
		"both_segments":   func(c *BiomonitorCatalog) { c.Tiers[2].Segments = biomonitorInt(3) },
		"no_segments":     func(c *BiomonitorCatalog) { c.Tiers[0].Segments = nil },
		"over_row":        func(c *BiomonitorCatalog) { c.Tiers[3].Segments = biomonitorInt(6) },
		"loss_over_row":   func(c *BiomonitorCatalog) { c.Tiers[2].SegmentsByLoss[0].Segments = 6 },
		"loss_open_early": func(c *BiomonitorCatalog) { c.Tiers[2].SegmentsByLoss[0].BelowPermille = nil },
		"loss_closed_end": func(c *BiomonitorCatalog) { c.Tiers[2].SegmentsByLoss[2].BelowPermille = biomonitorInt(900) },
		"loss_falls":      func(c *BiomonitorCatalog) { c.Tiers[2].SegmentsByLoss[1].BelowPermille = biomonitorInt(50) },
		"serious_over":    func(c *BiomonitorCatalog) { c.Tiers[2].SeriousMax = biomonitorInt(6) },
		"colour":          func(c *BiomonitorCatalog) { c.Tiers[3].Colour = "#B8FFD" },
		"energy":          func(c *BiomonitorCatalog) { c.Tiers[3].Energy = 17 },
		"blink":           func(c *BiomonitorCatalog) { c.Tiers[1].BlinkHz = 9 },
		"no_weak":         func(c *BiomonitorCatalog) { c.Tiers[2].States = []string{"ok"} },
		"no_whole":        func(c *BiomonitorCatalog) { c.Tiers = c.Tiers[:3] },
		"no_wounded_ok":   func(c *BiomonitorCatalog) { c.Tiers[2].States = []string{"weak"} },
		"no_dead":         func(c *BiomonitorCatalog) { c.Tiers = c.Tiers[1:] },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			c := biomonitorTable()
			spoil(c)
			if err := c.validate(); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
