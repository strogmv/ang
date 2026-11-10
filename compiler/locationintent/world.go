package locationintent

import (
	"fmt"
	"math"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func (s *state) pointCell(zone string, at [2]float64) (cell, bool) {
	z, known := s.zones[zone]
	if !known {
		return cell{}, false
	}
	return cell{X: z.X + int(math.Floor(at[0]/unitM)), Y: z.Y + int(math.Floor(at[1]/unitM))}, true
}

// world fills what the server-side CUE needs: Life edges in both directions,
// wayfinding markers, inspections, the region node and localisation.
func (s *state) world(d *Derived, fixtures map[string]DerivedFixture) {
	in := s.in
	w := World{LevelID: in.ID, LayoutID: d.Layout.ID, NameKey: in.Title.Key, Mechanisms: in.Mechanisms}
	text := func(t *Text) {
		if t != nil && t.Key != "" && t.RU != "" {
			w.Localization = append(w.Localization, *t)
		}
	}
	text(&in.Title)
	for _, z := range in.Zones {
		text(z.Title)
	}
	node := "hd." + s.short
	if in.Region.Node != "" {
		node = in.Region.Node
	}
	link := func(mode, id, to, linkID, toNode string, edges []string) {
		regionMode := map[string]string{"walk": "corridor", "rail": "monorail", "lift": "lift"}[mode]
		if linkID == "" {
			linkID = mode + "." + s.short + "_" + id
		}
		if toNode == "" {
			toNode = "hd." + stableShort(to)
		}
		w.RegionLinks = append(w.RegionLinks, RegionLink{ID: linkID, FromNode: node, ToNode: toNode, Mode: regionMode, InitialKnown: in.Region.Knowledge == "known", LifeEdges: edges})
	}
	for i, e := range d.Exits {
		src := in.Exits[indexExit(in.Exits, e.ID)]
		text(src.Name)
		text(src.ReverseName)
		if e.Status == "planned" {
			continue
		}
		seats := src.Seats
		if seats == 0 {
			seats = 8
		}
		nameKey, reverseKey := "", ""
		if src.Name != nil {
			nameKey = src.Name.Key
		}
		if src.ReverseName != nil {
			reverseKey = src.ReverseName.Key
		}
		at, landing := d.Exits[i].At, d.Exits[i].Landing
		peerExit, peerEntry := "", ""
		if src.Peer != nil {
			peerExit, peerEntry = src.Peer.Exit, src.Peer.Entry
		}
		// ExitZone is the hook for the hub's exit zones (a floor zone and a server
		// nameplate per leaving edge): the threshold cells are that zone.
		w.LifeEdges = append(w.LifeEdges,
			LifeEdge{ID: e.Edge, From: in.ID, To: e.To, Exit: &at, PeerEntry: e.To + "#" + e.Reverse, Mode: src.Mode, TravelSeconds: src.TravelSeconds, Seats: seats, NameKey: nameKey, Fare: src.Fare, ClosedReasonKey: src.ClosedReasonKey, ExitZone: e.Threshold, PeerCUE: peerEntry},
			LifeEdge{ID: e.Reverse, From: e.To, To: in.ID, PeerExit: e.To + "#" + e.Reverse, Entry: &landing, Mode: src.Mode, TravelSeconds: src.TravelSeconds, Seats: seats, NameKey: reverseKey, Fare: src.Fare, ClosedReasonKey: src.ClosedReasonKey, PeerCUE: peerExit})
		if src.ClosedReason != nil {
			text(src.ClosedReason)
		}
		link(src.Mode, e.ID, e.To, src.RegionLink, src.RegionNode, []string{e.Edge, e.Reverse})
		if src.Marker != nil && e.Status == "live" {
			w.Markers = append(w.Markers, DerivedMarker{ID: "way." + s.short + "." + e.ID, Number: src.Marker.Number, Key: src.Marker.Key, Kind: "exit", At: at, EdgeID: e.Edge})
			text(&Text{Key: src.Marker.Key, RU: src.Marker.RU, EN: src.Marker.EN, Context: src.Marker.Context})
		}
	}
	for _, m := range in.Mechanisms {
		if m.Edges == nil {
			continue
		}
		var rect locationsdk.Rect
		for _, dm := range d.Mechanisms {
			if dm.ID == m.ID {
				rect = dm.Rect
			}
		}
		// People step on and off at the floor cell in front of the south edge,
		// or the first floor cell that touches the mechanism.
		port, found := cell{X: rect.X + rect.W/2, Y: rect.Y + rect.H}, false
		if s.floor[port] != "" && s.blocked[port] == "" {
			found = true
		}
		for _, c := range cellsOf(locationsdk.Rect{X: rect.X - 1, Y: rect.Y - 1, W: rect.W + 2, H: rect.H + 2}) {
			inside := c.X >= rect.X && c.Y >= rect.Y && c.X < rect.X+rect.W && c.Y < rect.Y+rect.H
			if !found && !inside && s.floor[c] != "" && s.blocked[c] == "" {
				port, found = c, true
			}
		}
		if !found || m.Edges.To == "" || m.Edges.Out == "" || m.Edges.In == "" || m.Edges.TravelSeconds <= 0 {
			s.fail("exit_unbound", m.ID, &port, "mechanism edges need out, in, to, travel_seconds and a free floor cell beside the mechanism", "complete edges or remove them")
			continue
		}
		seats := m.Edges.Seats
		if seats == 0 {
			seats = 8
		}
		nameKey, inKey := "", ""
		if m.Edges.Name != nil {
			nameKey = m.Edges.Name.Key
			text(m.Edges.Name)
		}
		if m.Edges.InName != nil {
			inKey = m.Edges.InName.Key
			text(m.Edges.InName)
		}
		p := port
		peerExit, peerEntry := "", ""
		if m.Edges.Peer != nil {
			peerExit, peerEntry = m.Edges.Peer.Exit, m.Edges.Peer.Entry
		}
		w.LifeEdges = append(w.LifeEdges,
			LifeEdge{ID: m.Edges.Out, From: in.ID, To: m.Edges.To, Exit: &p, PeerEntry: m.Edges.To + "#" + m.Edges.In, Mode: "walk", TravelSeconds: m.Edges.TravelSeconds, Seats: seats, NameKey: nameKey, ExitZone: []cell{p}, PeerCUE: peerEntry},
			LifeEdge{ID: m.Edges.In, From: m.Edges.To, To: in.ID, PeerExit: m.Edges.To + "#" + m.Edges.In, Entry: &p, Mode: "walk", TravelSeconds: m.Edges.TravelSeconds, Seats: seats, NameKey: inKey, PeerCUE: peerExit})
		link("lift", m.ID, m.Edges.To, m.Edges.RegionLink, m.Edges.RegionNode, []string{m.Edges.Out, m.Edges.In})
		d.Layout.Entries = append(d.Layout.Entries, port)
	}
	for _, f := range in.Fixtures {
		df, placed := fixtures[f.ID]
		if !placed {
			continue
		}
		if f.Marker != nil {
			w.Markers = append(w.Markers, DerivedMarker{ID: "way." + s.short + "." + f.ID, Number: f.Marker.Number, Key: f.Marker.Key, Kind: "landmark", At: df.Approach, Object: f.ID})
			text(&Text{Key: f.Marker.Key, RU: f.Marker.RU, EN: f.Marker.EN, Context: f.Marker.Context})
		}
		if f.Inspection != nil {
			w.Inspections = append(w.Inspections, DerivedInspection{ID: "prop." + s.short + "." + f.ID, Level: in.ID, Object: f.ID, At: df.Approach, TextKey: f.Inspection.Key, Text: *f.Inspection, Purpose: f.Purpose})
			text(f.Inspection)
		}
	}
	for _, m := range in.Markers {
		at, ok := s.pointCell(m.Zone, m.AtM)
		if !ok {
			s.fail("unknown_zone", m.ID, nil, "marker names zone "+m.Zone, "use a zone ID of this file")
			continue
		}
		w.Markers = append(w.Markers, DerivedMarker{ID: "way." + s.short + "." + m.ID, Number: m.Number, Key: m.Key, Kind: "landmark", At: at})
		text(&Text{Key: m.Key, RU: m.RU, EN: m.EN, Context: m.Context})
	}
	for _, i := range in.Inspections {
		at, ok := s.pointCell(i.Zone, i.AtM)
		if !ok {
			s.fail("unknown_zone", i.ID, nil, "inspection names zone "+i.Zone, "use a zone ID of this file")
			continue
		}
		w.Inspections = append(w.Inspections, DerivedInspection{ID: "prop." + s.short + "." + i.ID, Level: in.ID, At: at, TextKey: i.Text.Key, Text: i.Text, Purpose: i.Text.Context})
		t := i.Text
		text(&t)
	}
	w.RegionNode = RegionNode{ID: node, Name: in.Region.Name.RU, NameEN: in.Region.Name.EN, DeckID: in.Region.Deck, LevelID: in.ID, InitialKnowledge: in.Region.Knowledge, Description: in.Region.Description.RU, DescriptionEN: in.Region.Description.EN, Kind: in.Region.Kind, Box: in.Region.Box}
	w.PlayerRangeCells = 24
	for _, p := range in.People {
		f, placed := fixtures[p.Post]
		if !placed {
			s.fail("person_without_post", p.Char, nil, "post "+p.Post+" is not a placed fixture", "name a fixture ID of this file")
			continue
		}
		post := PersonPost{Char: p.Char, Level: in.ID, At: f.Approach, Post: p.Post, Label: p.Label}
		if p.Hours != nil {
			post.From, post.To = p.Hours.From, p.Hours.To
		}
		w.People = append(w.People, post)
	}
	for _, h := range in.Hidden {
		z, known := s.zones[h.Zone]
		if !known {
			s.fail("unknown_zone", h.ID, nil, "hidden volume names zone "+h.Zone, "use a zone ID of this file")
			continue
		}
		x, y := float64(z.X)*unitM+h.Volume.X, float64(z.Y)*unitM+h.Volume.Y
		w.Hidden = append(w.Hidden, HiddenVolume{ID: h.ID, Ref: h.Ref, MinM: [3]float64{x, y, in.ElevationM + h.Volume.Z}, MaxM: [3]float64{x + h.Volume.W, y + h.Volume.H, in.ElevationM + h.Volume.Z + h.Volume.D}, Reason: h.TitleRU})
	}
	for i := range w.Markers {
		w.Markers[i].Color = s.tier.MarkerColor
		if len(d.Layout.Entries) > 0 {
			from := d.Layout.Entries[0]
			w.Markers[i].GuideFrom = &from
		}
	}
	d.World = w
}

// coherence is the whole-location check: a derived level that fails here is
// drawn on the plan but never generated.
func (s *state) coherence(d *Derived, entries []cell) {
	walkable := func(c cell) bool {
		if s.blocked[c] != "" {
			return false
		}
		if s.floor[c] != "" {
			return true
		}
		exit := s.thresh[c]
		for _, e := range d.Exits {
			if e.ID == exit && e.Status != "planned" {
				return true
			}
		}
		return false
	}
	nearObject := func(c cell) bool {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				n := cell{X: c.X + dx, Y: c.Y + dy}
				if s.objects[n] != "" || s.blocked[n] != "" {
					return true
				}
			}
		}
		return false
	}
	for _, m := range d.World.Markers {
		at := m.At
		if m.Kind == "exit" {
			continue // placed on its own threshold by construction
		}
		if !walkable(at) {
			s.fail("marker_off_floor", m.ID, &at, "the marker does not stand on walkable floor", "move it onto the floor in front of its object")
		} else if !nearObject(at) {
			s.fail("marker_on_bare_floor", m.ID, &at, "no fixture, mechanism or doorway within one cell", "attach the marker to a fixture (marker: on the fixture) or move it to the object it names")
		}
	}
	for _, i := range d.World.Inspections {
		at := i.At
		if !walkable(at) {
			s.fail("inspection_off_floor", i.ID, &at, "the inspection point is not walkable floor", "move it onto the floor in front of its object")
		} else if !nearObject(at) {
			s.fail("inspection_on_bare_floor", i.ID, &at, "there is nothing to inspect within one cell", "attach the inspection to a fixture or move it to a real object")
		}
	}
	for _, l := range d.Lamps {
		at := l.Cell
		if side, isWall := strings.CutPrefix(l.Mount, "wall:"); isWall {
			out := cell{X: at.X + sideStep[side].X, Y: at.Y + sideStep[side].Y}
			if !s.wall[out] && s.thresh[out] == "" {
				s.fail("lamp_unmounted", l.ID, &at, "no wall behind the lamp", "put the rule on an outer wall")
			}
		} else if s.floor[at] == "" {
			s.fail("lamp_unmounted", l.ID, &at, "a ceiling lamp needs floor under it", "move the lamp over floor")
		}
	}
	// The same thing the art gate measures later, caught in milliseconds: no
	// derived wall, mass or door leaf may stand on walkable floor.
	if d.Finishing != nil && d.Finishing.Architecture != nil {
		for _, w := range d.Finishing.Architecture.Draws {
			if w.Kind == "trim" {
				continue
			}
			const eps = 0.002
			x0, x1 := w.Position[0]-w.Size[0]/2+eps, w.Position[0]+w.Size[0]/2-eps
			y0, y1 := w.Position[2]-w.Size[2]/2+eps, w.Position[2]+w.Size[2]/2-eps
			bottom := w.Position[1] - w.Size[1]/2 - s.in.ElevationM
			if bottom > 1.9 {
				continue // a lintel above head height
			}
			for cy := int(math.Floor(y0 / unitM)); cy <= int(math.Floor(y1/unitM)); cy++ {
				for cx := int(math.Floor(x0 / unitM)); cx <= int(math.Floor(x1/unitM)); cx++ {
					c := cell{X: cx, Y: cy}
					if s.floor[c] != "" || s.thresh[c] != "" {
						s.fail("wall_on_floor", w.ID, &c, "a derived wall or leaf stands on walkable floor", "this is a derivation fault: report the intent that produced it")
						cy, cx = 1<<30-1, 1<<30-1
					}
				}
			}
		}
	}
	if len(d.Layout.Entries) == 0 {
		s.fail("no_live_exit", s.in.ID, nil, "nobody can enter: no live exit and no mechanism edge", "add an exit with status live or mechanism edges")
		return
	}
	reach := map[cell]bool{}
	queue := []cell{}
	for _, e := range d.Layout.Entries {
		if walkable(e) && !reach[e] {
			reach[e] = true
			queue = append(queue, e)
		}
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, side := range sideOrder {
			n := cell{X: p.X + sideStep[side].X, Y: p.Y + sideStep[side].Y}
			if walkable(n) && !reach[n] {
				reach[n] = true
				queue = append(queue, n)
			}
		}
	}
	for _, z := range s.in.Zones {
		free, reached := 0, 0
		var first *cell
		for _, c := range cellsOf(s.zones[z.ID]) {
			if s.floor[c] == z.ID && s.blocked[c] == "" {
				free++
				if reach[c] {
					reached++
				} else if first == nil {
					at := c
					first = &at
				}
			}
		}
		if free > 0 && reached == 0 {
			s.fail("zone_unreachable", z.ID, first, fmt.Sprintf("none of its %d free cells can be reached from an entrance", free), "open a passage: zones must touch along free floor, or add an exit into this zone")
		}
	}
	for _, f := range d.Fixtures {
		at := f.Approach
		if !walkable(at) {
			s.fail("approach_blocked", f.ID, &at, "the cell in front of the fixture is not free floor", "leave the front of the fixture clear or turn it to another wall")
		} else if !reach[at] {
			s.fail("approach_unreachable", f.ID, &at, "nobody can walk to the front of the fixture", "clear a path from an entrance")
		}
	}
}
