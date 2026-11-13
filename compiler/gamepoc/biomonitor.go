package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"

	"cuelang.org/go/cue"
)

// BiomonitorCatalog is world.biomonitor (owner 13.11, doc 09): the armour's biomonitor shows the wound state of its
// bearer on the plates — a row of segments, how many burn and in what colour. One table for every armour with such a
// monitor: a look only names the surface of its row (world.art.looks[].biomonitor). The generated server reads each seen
// wearer's body in the wound model (templates/biomonitor.go.tmpl) and sends the tier and the lit segments; the client
// takes the tier's colour, brightness and blinking from the same table (presentation art.biomonitor).
type BiomonitorCatalog struct {
	Segments  int              `json:"segments"`
	OffColour string           `json:"off_colour"`
	Tiers     []BiomonitorTier `json:"tiers"`
	// Derived here from world.art.person_looks, never authored: who wears a look with a biomonitor (sorted).
	Wearers []string        `json:"wearers"`
	Sources json.RawMessage `json:"sources,omitempty"`
}

// BiomonitorTier: the first tier whose states hold the body's wound state (ok | weak | confused | down | dead) and, when
// Wounded is set, whose «wounded» matches (a wound, a condition, a state other than ok or any blood lost) is shown.
// Segments: a fixed count, or by the blood lost (the first row whose BelowPermille is above the loss; a row without a
// bound takes the rest); SeriousMax caps it while the wound model calls the body serious.
type BiomonitorTier struct {
	ID             string              `json:"id"`
	States         []string            `json:"states"`
	Wounded        *bool               `json:"wounded,omitempty"`
	Segments       *int                `json:"segments,omitempty"`
	SegmentsByLoss []BiomonitorLossRow `json:"segments_by_loss,omitempty"`
	SeriousMax     *int                `json:"serious_max,omitempty"`
	Colour         string              `json:"colour"`
	Energy         float64             `json:"energy"`
	BlinkHz        float64             `json:"blink_hz"`
	Note           string              `json:"note"`
}

type BiomonitorLossRow struct {
	BelowPermille *int `json:"below_permille,omitempty"`
	Segments      int  `json:"segments"`
}

var (
	biomonitorID     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	biomonitorColour = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	// The wound model's body states (wounds runtime Status.State), in the order a body falls through them.
	biomonitorStates = []string{"ok", "weak", "confused", "down", "dead"}
)

func readBiomonitor(world cue.Value) (*BiomonitorCatalog, error) {
	v := world.LookupPath(cue.ParsePath("biomonitor"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.biomonitor: %w", err)
	}
	var c BiomonitorCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("world.biomonitor: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("world.biomonitor: %w", err)
	}
	c.Sources = nil // provenance stays in CUE; the server needs the table only
	return &c, nil
}

// match reports whether a tier shows a body in the given state with the given «wounded».
func (t BiomonitorTier) match(state string, wounded bool) bool {
	in := false
	for _, s := range t.States {
		in = in || s == state
	}
	return in && (t.Wounded == nil || *t.Wounded == wounded)
}

// validate: sane numbers and colours, unique tiers, and a table that shows every body the wound model can have: for
// each state (and, in ok, wounded or not) some tier matches — the server never meets a body the table cannot show.
func (c *BiomonitorCatalog) validate() error {
	if c.Segments < 1 || c.Segments > 8 {
		return fmt.Errorf("1..8 segments in a row")
	}
	if !biomonitorColour.MatchString(c.OffColour) {
		return fmt.Errorf("off_colour must be #RRGGBB")
	}
	if len(c.Tiers) == 0 {
		return fmt.Errorf("a table needs its tiers")
	}
	known := map[string]bool{}
	for _, s := range biomonitorStates {
		known[s] = true
	}
	ids := map[string]bool{}
	for i, t := range c.Tiers {
		if !biomonitorID.MatchString(t.ID) || ids[t.ID] {
			return fmt.Errorf("tier %d: a unique lower-case id required", i)
		}
		ids[t.ID] = true
		if len(t.States) == 0 {
			return fmt.Errorf("tier %s: no body states", t.ID)
		}
		for _, s := range t.States {
			if !known[s] {
				return fmt.Errorf("tier %s: %q is not a state of the wound model (%v)", t.ID, s, biomonitorStates)
			}
		}
		if (t.Segments == nil) == (len(t.SegmentsByLoss) == 0) {
			return fmt.Errorf("tier %s: segments or segments_by_loss, exactly one", t.ID)
		}
		if t.Segments != nil && (*t.Segments < 0 || *t.Segments > c.Segments) {
			return fmt.Errorf("tier %s: segments within 0..%d", t.ID, c.Segments)
		}
		last := 0
		for k, row := range t.SegmentsByLoss {
			if row.Segments < 0 || row.Segments > c.Segments {
				return fmt.Errorf("tier %s: loss row %d: segments within 0..%d", t.ID, k, c.Segments)
			}
			open := row.BelowPermille == nil
			if open != (k == len(t.SegmentsByLoss)-1) {
				return fmt.Errorf("tier %s: only the last loss row, and it always, goes without below_permille (it takes the rest)", t.ID)
			}
			if !open && (*row.BelowPermille <= last || *row.BelowPermille > 1000) {
				return fmt.Errorf("tier %s: loss row %d: bounds rise within 1..1000 permille", t.ID, k)
			}
			if !open {
				last = *row.BelowPermille
			}
		}
		if t.SeriousMax != nil && (*t.SeriousMax < 0 || *t.SeriousMax > c.Segments) {
			return fmt.Errorf("tier %s: serious_max within 0..%d", t.ID, c.Segments)
		}
		if !biomonitorColour.MatchString(t.Colour) {
			return fmt.Errorf("tier %s: colour must be #RRGGBB", t.ID)
		}
		if math.IsNaN(t.Energy) || t.Energy < 0 || t.Energy > 16 || math.IsNaN(t.BlinkHz) || t.BlinkHz < 0 || t.BlinkHz > 8 {
			return fmt.Errorf("tier %s: energy 0..16 and blinking 0..8 Hz", t.ID)
		}
	}
	for _, s := range biomonitorStates {
		for _, wounded := range []bool{false, true} {
			if s != "ok" && !wounded {
				continue // a body weak, confused, down or dead is wounded by definition
			}
			shown := false
			for _, t := range c.Tiers {
				shown = shown || t.match(s, wounded)
			}
			if !shown {
				return fmt.Errorf("no tier shows a body %s (wounded %v)", s, wounded)
			}
		}
	}
	return nil
}

// biomonitorWearers: the people bound to a look with a biomonitor, sorted (the server publishes only them).
func biomonitorWearers(a Art) []string {
	out := []string{}
	for id, look := range a.PersonLooks {
		if l, ok := a.Looks[look]; ok && l.Biomonitor != nil {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// clientBiomonitor is the presentation of the table: per tier its colour, brightness and blinking (the client only
// draws the server's tier and segments with them).
func clientBiomonitor(c *BiomonitorCatalog) map[string]any {
	tiers := make([]map[string]any, 0, len(c.Tiers))
	for _, t := range c.Tiers {
		tiers = append(tiers, map[string]any{"id": t.ID, "colour": t.Colour, "energy": t.Energy, "blink_hz": t.BlinkHz})
	}
	return map[string]any{"segments": c.Segments, "off_colour": c.OffColour, "tiers": tiers}
}
