package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	clothingRuntime "github.com/strogmv/ang/compiler/clothing/runtime"
)

// The clothing system v1 (the game's art-source/clothing/v1/README.md; CUE content/world/wardrobe.cue). Clothes are items apart
// from bodies and people: a base body (with its heads and hair styles) and items authored once and fitted on every base body by
// the wardrobe build; a person is a body + a head + a hair style + a list of items, chosen by an outfit rule (by caste, job, sex
// and age; the same person always the same by the hash of his id) or named for one person. The build's record
// (art-source/clothing/v1/wardrobe.json) is the proof of what each item is on each body — its GLB and SHA-256, the regions it
// has faces in, the share of each body region it hides from the game's camera over the body and over every other item, its
// shapes, fit shapes, swing shapes and cover plane, the tint slots and materials of the body's wardrobe. Here the data is
// checked against it and projected for the client (actor_outfits.gd): it merges an outfit into one mesh and one material per
// distance step, dropping what a higher layer covers.

// Wardrobe is world.art.wardrobe.
type Wardrobe struct {
	Record string                  `json:"record"`
	Layers []string                `json:"layers"`
	Steps  LookSteps               `json:"steps"`
	Bodies map[string]WardrobeBody `json:"bodies"`
	Items  map[string]WardrobeItem `json:"items"`
}

// WardrobeBody: a base body, its heads (an old head names when its age marks start and are full) and hair styles (on which
// heads), the materials of its own surfaces.
type WardrobeBody struct {
	ID        string                  `json:"id"`
	Sex       string                  `json:"sex"`
	Heads     map[string]WardrobeHead `json:"heads"`
	Hair      map[string]WardrobeHair `json:"hair"`
	Materials map[string]LookMaterial `json:"materials"`
	Note      string                  `json:"note"`
}

// WardrobeHead: a face of a body.
type WardrobeHead struct {
	Ageing *WardrobeAgeing `json:"ageing,omitempty"`
	Note   string          `json:"note"`
}

// WardrobeAgeing: the ages the head's age marks start and are full (the old skin painting in its second UV).
type WardrobeAgeing struct {
	FromAge int64 `json:"from_age"`
	FullAge int64 `json:"full_age"`
}

// WardrobeHair: the heads a hair style is fitted on.
type WardrobeHair struct {
	Heads []string `json:"heads"`
}

// WardrobeItem: the worn view of garments (see wardrobe.cue): how the physical items it names (world.items with a garment
// component) look on a body. Their name, price and makers are the items'; their surfaces' state (dirt, blood, wetness, wear)
// is the clothing SDK's (world.clothing garments, one state per layer); here only the view: slots, draw layer, covered regions,
// the bodies it is fitted on, render materials, motion, and which garment layer each of its surfaces shows.
type WardrobeItem struct {
	ID        string                  `json:"id"`
	Items     []string                `json:"items"`
	Surfaces  map[string]string       `json:"surfaces"`
	Slots     []string                `json:"slots"`
	Layer     string                  `json:"layer"`
	Covers    []string                `json:"covers"`
	Bodies    []string                `json:"bodies"`
	Materials map[string]LookMaterial `json:"materials"`
	Motion    *WardrobeMotion         `json:"motion,omitempty"`
	HidesHair bool                    `json:"hides_hair,omitempty"`
	Note      string                  `json:"note"`
}

// wardrobeWorld: what the wardrobe hangs off in the world — the clothing SDK's garments (their layer ids) and the physical
// items (world.items; those with a garment component: item -> garment).
type wardrobeWorld struct {
	Garments    map[string][]string
	Items       map[string]bool
	ItemGarment map[string]string
}

// readWardrobeWorld reads world.items (id, garment) and the clothing catalog's garments for the wardrobe's checks.
func readWardrobeWorld(world cue.Value, garments map[string]clothingRuntime.GarmentDefinition) (wardrobeWorld, error) {
	ww := wardrobeWorld{Garments: map[string][]string{}, Items: map[string]bool{}, ItemGarment: map[string]string{}}
	for id, g := range garments {
		for _, l := range g.Layers {
			ww.Garments[id] = append(ww.Garments[id], l.ID)
		}
	}
	it, err := world.LookupPath(cue.ParsePath("items")).Fields()
	if err != nil {
		return ww, err
	}
	for it.Next() {
		id := it.Selector().Unquoted()
		ww.Items[id] = true
		if g := it.Value().LookupPath(cue.ParsePath("garment")); g.Exists() {
			s, err := g.String()
			if err != nil {
				return ww, fmt.Errorf("world.items %s: garment: %w", id, err)
			}
			ww.ItemGarment[id] = s
		}
	}
	return ww, nil
}

// WardrobeMotion: an item's motion (the swing of its panels, as a look's swing).
type WardrobeMotion struct {
	Swing *LookSwing `json:"swing,omitempty"`
}

// OutfitRule is world.art.outfit_rules[name] (see wardrobe.cue).
type OutfitRule struct {
	Body    string                             `json:"body"`
	Caste   string                             `json:"caste"`
	Heads   map[string]int64                   `json:"heads"`
	Hair    map[string]int64                   `json:"hair"`
	Wear    []string                           `json:"wear"`
	Choose  map[string]map[string]OutfitOption `json:"choose,omitempty"`
	Colours map[string]map[string]int64        `json:"colours"`
	Ages    []OutfitAge                        `json:"ages,omitempty"`
	HeightM *PopulationHeight                  `json:"height_m,omitempty"`
	Shapes  map[string]PopulationShape         `json:"shapes,omitempty"`
	Posture *LookPosture                       `json:"posture,omitempty"`
	Idle    *string                            `json:"idle,omitempty"`
	Shading *string                            `json:"shading,omitempty"`
}

// OutfitOption: one option of a choose group: its items and share.
type OutfitOption struct {
	Items []string `json:"items"`
	Share int64    `json:"share"`
}

// OutfitAge: from an age up, other hair styles and colours.
type OutfitAge struct {
	FromAge int64                       `json:"from_age"`
	Hair    map[string]int64            `json:"hair,omitempty"`
	Colours map[string]map[string]int64 `json:"colours,omitempty"`
}

// PersonOutfit is world.art.person_outfits[person] (see wardrobe.cue).
type PersonOutfit struct {
	Body    string             `json:"body"`
	Caste   string             `json:"caste"`
	Head    string             `json:"head"`
	Hair    string             `json:"hair,omitempty"`
	Items   []string           `json:"items"`
	Colours map[string]string  `json:"colours"`
	Shapes  map[string]float64 `json:"shapes,omitempty"`
	HeightM *float64           `json:"height_m,omitempty"`
	Age     *int64             `json:"age,omitempty"`
	Use     string             `json:"use"`
	Posture *LookPosture       `json:"posture,omitempty"`
	Idle    *string            `json:"idle,omitempty"`
	Shading *string            `json:"shading,omitempty"`
}

// wardrobeRecord: art-source/clothing/v1/wardrobe.json (put_wardrobe.py).
type wardrobeRecord struct {
	Bodies map[string]wardrobeRecordBody `json:"bodies"`
}

type wardrobeRecordBody struct {
	Atlas     struct{ File, SHA256 string }     `json:"atlas"`
	BodyGLB   struct{ File, SHA256 string }     `json:"body_glb"`
	Shapes    []string                          `json:"shapes"`
	Slots     []struct{ Owner, Zone string }    `json:"slots"`
	Materials []struct{ Owner, Surface string } `json:"materials"`
	Regions   []string                          `json:"regions"`
	Heads     map[string]json.RawMessage        `json:"heads"`
	Hair      map[string]struct {
		Heads []string `json:"heads"`
	} `json:"hair"`
	Parts map[string]struct {
		Triangles map[string]int64 `json:"triangles"`
	} `json:"parts"`
	Items map[string]wardrobeRecordItem `json:"items"`
}

type wardrobeRecordItem struct {
	File        string                        `json:"file"`
	SHA256      string                        `json:"sha256"`
	Triangles   map[string]int64              `json:"triangles"`
	Regions     []string                      `json:"regions"`
	Covered     map[string]float64            `json:"covered"`
	CoveredOver map[string]map[string]float64 `json:"covered_over"`
	Shapes      []string                      `json:"shapes"`
	FitShapes   []string                      `json:"fit_shapes"`
	SwingShapes map[string]lookRecordSwing    `json:"swing_shapes"`
	Cover       *struct {
		Point  []float64 `json:"point"`
		Normal []float64 `json:"normal"`
	} `json:"cover"`
}

var (
	wardrobeLayers = []string{"skin", "under", "feet", "garment", "one_piece", "over", "outer", "head"}
	wardrobeSlots  = map[string]bool{"torso": true, "legs": true, "feet": true, "over_torso": true, "over_waist": true, "knees": true, "belt": true, "bag": true, "head": true, "hands": true}
	wardrobeName   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	wardrobeColour = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	// What every outfit must cover (nobody bare): the trunk front and back and the thighs.
	wardrobeDecent = []string{"chest_front", "chest_back", "waist_front", "waist_back", "thigh_L", "thigh_R"}
	// A region of the body is covered when at least this share of its faces is hidden (a few faces at an opening's edge: a region
	// of 40 faces loses 2.5 % per face). What an item hides of another item under it is culled per triangle by the client's pack
	// (tools/godot-link/export_wardrobe.gd hidden_by), not by region.
	wardrobeCoveredShare = 0.95
	// The triangles of one part of an outfit at each step (the near step of the richest item stays within a look's budget).
	wardrobeStepMax = map[string]int64{"near": 14000, "crowd": 8000}
)

func wardrobeLayerIndex(l string) int {
	for i, x := range wardrobeLayers {
		if x == l {
			return i
		}
	}
	return -1
}

// checkWardrobe validates world.art.wardrobe, outfit_rules and person_outfits against the build's record (root: the game tree).
func checkWardrobe(root string, a Art, person func(string) bool, ww wardrobeWorld) error {
	w := a.Wardrobe
	if w == nil {
		if len(a.OutfitRules) > 0 || len(a.PersonOutfits) > 0 {
			return fmt.Errorf("art.outfit_rules / person_outfits need art.wardrobe")
		}
		return nil
	}
	if !lookPath(w.Record, ".json") {
		return fmt.Errorf("art.wardrobe: the record %q must be a .json under art-source/", w.Record)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(w.Record)))
	if err != nil {
		return fmt.Errorf("art.wardrobe: %w", err)
	}
	var rec wardrobeRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return fmt.Errorf("art.wardrobe: record %s: %w", w.Record, err)
	}
	if strings.Join(w.Layers, ",") != strings.Join(wardrobeLayers, ",") {
		return fmt.Errorf("art.wardrobe: layers must be %v (the client and the build know this order)", wardrobeLayers)
	}
	if w.Steps.NearUpToFrameM < 1 || w.Steps.NearUpToFrameM > 16 || w.Steps.HysteresisM < 0 || w.Steps.HysteresisM >= 1 {
		return fmt.Errorf("art.wardrobe: steps: near_up_to_frame_m 1..16, hysteresis_m 0..1")
	}
	for id, b := range w.Bodies {
		rb, ok := rec.Bodies[id]
		if !ok || b.ID != id || (b.Sex != "f" && b.Sex != "m") {
			return fmt.Errorf("art.wardrobe.bodies %s: no such body in the record %s (or a bad id or sex)", id, w.Record)
		}
		if !lookPath(rb.BodyGLB.File, ".glb") || !lookSHA.MatchString(rb.BodyGLB.SHA256) || !lookPath(rb.Atlas.File, ".png") || !lookSHA.MatchString(rb.Atlas.SHA256) {
			return fmt.Errorf("art.wardrobe.bodies %s: the record gives no safe body.glb and atlas under art-source/ with SHA-256", id)
		}
		for h, head := range b.Heads {
			if _, ok := rb.Heads[h]; !ok || rb.Parts["head@"+h].Triangles == nil {
				return fmt.Errorf("art.wardrobe.bodies %s: head %q is not in the record", id, h)
			}
			if g := head.Ageing; g != nil && (g.FromAge < 16 || g.FullAge <= g.FromAge || g.FullAge > 90) {
				return fmt.Errorf("art.wardrobe.bodies %s: head %s: ageing 16 <= from_age < full_age <= 90", id, h)
			}
		}
		for st, hair := range b.Hair {
			if len(hair.Heads) == 0 {
				return fmt.Errorf("art.wardrobe.bodies %s: hair %s is worn on no head", id, st)
			}
			for _, h := range hair.Heads {
				if _, ok := b.Heads[h]; !ok || rb.Parts["hair@"+st+"@"+h].Triangles == nil {
					return fmt.Errorf("art.wardrobe.bodies %s: hair %s on head %s is not in the record", id, st, h)
				}
			}
		}
		for _, m := range rb.Materials {
			if m.Owner == "skin" || strings.HasPrefix(m.Owner, "head@") || strings.HasPrefix(m.Owner, "hair@") {
				if _, ok := b.Materials[m.Surface]; !ok {
					return fmt.Errorf("art.wardrobe.bodies %s: the body's surface %s (%s) has no material", id, m.Surface, m.Owner)
				}
			}
		}
		if err := checkMaterialNumbers("art.wardrobe.bodies "+id, b.Materials); err != nil {
			return err
		}
	}
	drawnBy := map[string]string{}
	for id, it := range w.Items {
		where := "art.wardrobe.items " + id
		if !wardrobeName.MatchString(id) || it.ID != id {
			return fmt.Errorf("%s: a plain name equal to its key", where)
		}
		if strings.TrimSpace(it.Note) == "" {
			return fmt.Errorf("%s: a note: why it looks so", where)
		}
		if len(it.Slots) == 0 {
			return fmt.Errorf("%s: name its slots", where)
		}
		for _, s := range it.Slots {
			if !wardrobeSlots[s] {
				return fmt.Errorf("%s: unknown slot %q", where, s)
			}
		}
		layer := wardrobeLayerIndex(it.Layer)
		if layer <= 0 {
			return fmt.Errorf("%s: layer %q is not a layer above the skin", where, it.Layer)
		}
		// the physical items it draws: each a world item with a garment (the clothing SDK's surfaces), drawn by this view only;
		// each surface of the view shows a layer every one of those garments has
		if len(it.Items) == 0 {
			return fmt.Errorf("%s: name the physical items it draws (world.items with a garment component)", where)
		}
		for _, iid := range it.Items {
			if other, ok := drawnBy[iid]; ok {
				return fmt.Errorf("%s: item %s is drawn by art.wardrobe.items %s too (one view per item)", where, iid, other)
			}
			drawnBy[iid] = id
			g, ok := ww.ItemGarment[iid]
			if !ok {
				if ww.Items[iid] {
					return fmt.Errorf("%s: world.items %s has no garment (the clothing SDK's definition of its surfaces: world.clothing.garments)", where, iid)
				}
				return fmt.Errorf("%s: no world item %s", where, iid)
			}
			layers, ok := ww.Garments[g]
			if !ok {
				return fmt.Errorf("%s: garment %q of %s is not in world.clothing", where, g, iid)
			}
			for surface, l := range it.Surfaces {
				if !contains(layers, l) {
					return fmt.Errorf("%s: surface %s shows layer %q, which garment %s (of %s) does not have (%v)", where, surface, l, g, iid, layers)
				}
			}
		}
		if err := checkMaterialNumbers(where, it.Materials); err != nil {
			return err
		}
		if len(it.Bodies) == 0 {
			return fmt.Errorf("%s: name the bodies it is fitted on", where)
		}
		for _, bid := range it.Bodies {
			b, ok := w.Bodies[bid]
			if !ok {
				return fmt.Errorf("%s: unknown body %q", where, bid)
			}
			_ = b
			rb := rec.Bodies[bid]
			ri, ok := rb.Items[id]
			if !ok || !lookPath(ri.File, ".glb") || !lookSHA.MatchString(ri.SHA256) {
				return fmt.Errorf("%s: the record %s has no GLB with SHA-256 of it on body %s (run the wardrobe build)", where, w.Record, bid)
			}
			for step, max := range wardrobeStepMax {
				if ri.Triangles[step] <= 0 || ri.Triangles[step] > max {
					return fmt.Errorf("%s: on body %s the %s step has %d triangles (1..%d)", where, bid, step, ri.Triangles[step], max)
				}
			}
			if ri.Triangles["crowd"] >= ri.Triangles["near"] {
				return fmt.Errorf("%s: on body %s the crowd step is not lighter than the near one", where, bid)
			}
			surfaces := map[string]bool{}
			for _, m := range rb.Materials {
				if m.Owner == id {
					surfaces[m.Surface] = true
					if _, ok := it.Materials[m.Surface]; !ok {
						return fmt.Errorf("%s: surface %s on body %s has no material", where, m.Surface, bid)
					}
				}
			}
			for s := range it.Materials {
				if !surfaces[s] {
					return fmt.Errorf("%s: material %q is not a surface of it in the record on body %s", where, s, bid)
				}
			}
			for s := range surfaces {
				if _, ok := it.Surfaces[s]; !ok {
					return fmt.Errorf("%s: surface %s on body %s shows no garment layer (surfaces: {%s: <layer>})", where, s, bid, s)
				}
			}
			for s := range it.Surfaces {
				if !surfaces[s] {
					return fmt.Errorf("%s: surfaces names %q, not a surface of it in the record on body %s", where, s, bid)
				}
			}
			for _, r := range it.Covers {
				if !contains(rb.Regions, r) {
					return fmt.Errorf("%s: covers %q, not a region of body %s", where, r, bid)
				}
				if ri.Covered[r] < wardrobeCoveredShare {
					return fmt.Errorf("%s: covers %s, but on body %s it hides %.1f %% of the body's faces there (the record; %.0f %% needed)", where, r, bid, ri.Covered[r]*100, wardrobeCoveredShare*100)
				}
			}
			if it.Motion != nil && it.Motion.Swing != nil || len(ri.SwingShapes) > 0 {
				l := Look{}
				if it.Motion != nil {
					l.Swing = it.Motion.Swing
				}
				if err := checkLookSwing(where+" on "+bid, l, ri.SwingShapes, w.Record); err != nil {
					return err
				}
			}
			if it.HidesHair != (ri.Cover != nil) {
				return fmt.Errorf("%s: hides_hair must say what the record says: a cover plane on body %s is %v", where, bid, ri.Cover != nil)
			}
		}
	}
	// and the other way: every world item with a garment is drawn by one view (a garment one can wear has a look on a body)
	worn := make([]string, 0, len(ww.ItemGarment))
	for iid := range ww.ItemGarment {
		worn = append(worn, iid)
	}
	sort.Strings(worn)
	for _, iid := range worn {
		if _, ok := drawnBy[iid]; !ok {
			return fmt.Errorf("world.items %s: garment %s, but no art.wardrobe item draws it (its view: art.wardrobe.items[].items)", iid, ww.ItemGarment[iid])
		}
	}
	for name, r := range a.OutfitRules {
		if _, ok := a.PopulationLooks[name]; ok {
			return fmt.Errorf("art.outfit_rules %s: also a population look (one name, one of the two)", name)
		}
		if err := checkOutfitRule(name, r, w, rec); err != nil {
			return err
		}
	}
	for id, p := range a.PersonOutfits {
		if !person(id) {
			return fmt.Errorf("art.person_outfits %s: no such person in world.persons", id)
		}
		if _, own := a.PersonLooks[id]; p.Use == "look" && !own {
			return fmt.Errorf("art.person_outfits %s: use look, but the person has no look of his own (art.person_looks)", id)
		}
		if p.Use != "look" && p.Use != "outfit" {
			return fmt.Errorf("art.person_outfits %s: use outfit or look", id)
		}
		if _, ok := a.PersonPopulations[id]; ok && p.Use == "outfit" {
			return fmt.Errorf("art.person_outfits %s: also drawn by a general rule (art.person_populations)", id)
		}
		b, ok := w.Bodies[p.Body]
		if !ok {
			return fmt.Errorf("art.person_outfits %s: unknown body %q", id, p.Body)
		}
		if _, ok := b.Heads[p.Head]; !ok {
			return fmt.Errorf("art.person_outfits %s: head %q is not a head of body %s", id, p.Head, p.Body)
		}
		if p.Hair != "" && !contains(b.Hair[p.Hair].Heads, p.Head) {
			return fmt.Errorf("art.person_outfits %s: hair %q is not fitted on head %s", id, p.Hair, p.Head)
		}
		if err := checkOutfitItems("art.person_outfits "+id, p.Body, p.Items, w); err != nil {
			return err
		}
		if err := checkOutfitColours("art.person_outfits "+id, p.Body, p.Items, w, rec, onlyKeys(p.Colours)); err != nil {
			return err
		}
		for k, c := range p.Colours {
			if !wardrobeColour.MatchString(c) {
				return fmt.Errorf("art.person_outfits %s: colour %s is not #RRGGBB", id, k)
			}
		}
		for s, v := range p.Shapes {
			if !contains(rec.Bodies[p.Body].Shapes, s) || math.IsNaN(v) || v < 0 || v > 1 {
				return fmt.Errorf("art.person_outfits %s: shape %s is not a shape of body %s (0..1)", id, s, p.Body)
			}
		}
		if p.Posture != nil {
			if err := checkPostureGroups("art.person_outfits "+id, *p.Posture); err != nil {
				return err
			}
		}
		if p.Idle != nil && !lookIdles[*p.Idle] {
			return fmt.Errorf("art.person_outfits %s: idle %q is not a stance", id, *p.Idle)
		}
	}
	return nil
}

func onlyKeys(m map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func checkMaterialNumbers(where string, mats map[string]LookMaterial) error {
	for s, m := range mats {
		for _, v := range []*float64{&m.Roughness, &m.Metallic, m.Clearcoat, m.ClearcoatRoughness, m.Specular} {
			if v != nil && (math.IsNaN(*v) || *v < 0 || *v > 1) {
				return fmt.Errorf("%s: material %s: every number within 0..1", where, s)
			}
		}
	}
	return nil
}

// checkOutfitItems: every item exists and is fitted on the body; no two of them take one slot.
func checkOutfitItems(where, body string, items []string, w *Wardrobe) error {
	slots := map[string]string{}
	for _, id := range items {
		it, ok := w.Items[id]
		if !ok {
			return fmt.Errorf("%s: unknown item %q", where, id)
		}
		if !contains(it.Bodies, body) {
			return fmt.Errorf("%s: item %s is not fitted on body %s", where, id, body)
		}
		for _, s := range it.Slots {
			if other, ok := slots[s]; ok {
				return fmt.Errorf("%s: items %s and %s both take slot %s", where, other, id, s)
			}
			slots[s] = id
		}
	}
	covered := map[string]bool{}
	for _, id := range items {
		for _, r := range w.Items[id].Covers {
			covered[r] = true
		}
	}
	for _, r := range wardrobeDecent {
		if !covered[r] {
			return fmt.Errorf("%s: nothing worn covers %s (the trunk and the thighs must be covered: %v)", where, r, items)
		}
	}
	return nil
}

// checkOutfitColours: every colour key is "skin", "hair", a zone of a worn item ("<zone>") or one item's zone ("<item>.<zone>").
func checkOutfitColours(where, body string, items []string, w *Wardrobe, rec wardrobeRecord, keys map[string]bool) error {
	zones := map[string]bool{"skin": true, "hair": true}
	for _, s := range rec.Bodies[body].Slots {
		if contains(items, s.Owner) {
			zones[s.Zone] = true
			zones[s.Owner+"."+s.Zone] = true
		}
	}
	for k := range keys {
		if !zones[k] {
			return fmt.Errorf("%s: colour key %q is not skin, hair, a zone of a worn item or <item>.<zone>", where, k)
		}
	}
	return nil
}

// checkOutfitRule: the body, heads and hair are the wardrobe's and the record's; every choice of the rule (wear + one option of
// each group) is a valid outfit; shares 0..100 with a positive sum; colours name zones some choice wears; ages ascend.
func checkOutfitRule(name string, r OutfitRule, w *Wardrobe, rec wardrobeRecord) error {
	where := "art.outfit_rules " + name
	b, ok := w.Bodies[r.Body]
	if !ok {
		return fmt.Errorf("%s: unknown body %q", where, r.Body)
	}
	if !lookCastes[r.Caste] {
		return fmt.Errorf("%s: unknown caste %q", where, r.Caste)
	}
	positive := func(what string, m map[string]int64) error {
		total := int64(0)
		for k, v := range m {
			if v < 0 || v > 100 {
				return fmt.Errorf("%s: %s %s: share 0..100", where, what, k)
			}
			total += v
		}
		if total <= 0 {
			return fmt.Errorf("%s: %s: the shares add up to nothing", where, what)
		}
		return nil
	}
	if err := positive("heads", r.Heads); err != nil {
		return err
	}
	for h := range r.Heads {
		if _, ok := b.Heads[h]; !ok {
			return fmt.Errorf("%s: head %q is not a head of body %s", where, h, r.Body)
		}
	}
	hairOK := func(hair map[string]int64) error {
		if err := positive("hair", hair); err != nil {
			return err
		}
		for h, share := range r.Heads {
			if share == 0 {
				continue
			}
			worn := false
			for st, s := range hair {
				hs, ok := b.Hair[st]
				if !ok {
					return fmt.Errorf("%s: hair %q is not a style of body %s", where, st, r.Body)
				}
				worn = worn || (s > 0 && contains(hs.Heads, h))
			}
			if !worn {
				return fmt.Errorf("%s: no hair style of the rule is fitted on head %s", where, h)
			}
		}
		return nil
	}
	if err := hairOK(r.Hair); err != nil {
		return err
	}
	// every choice: wear + one option of each group
	groups := make([]string, 0, len(r.Choose))
	for g := range r.Choose {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	choices := [][]string{append([]string{}, r.Wear...)}
	for _, g := range groups {
		opts := r.Choose[g]
		total := int64(0)
		var next [][]string
		for _, o := range sortedOptions(opts) {
			total += opts[o].Share
			if opts[o].Share < 0 || opts[o].Share > 100 {
				return fmt.Errorf("%s: choose %s.%s: share 0..100", where, g, o)
			}
			if opts[o].Share == 0 {
				continue
			}
			for _, c := range choices {
				next = append(next, append(append([]string{}, c...), opts[o].Items...))
			}
		}
		if total <= 0 {
			return fmt.Errorf("%s: choose %s: the shares add up to nothing", where, g)
		}
		choices = next
	}
	all := map[string]bool{}
	for _, c := range choices {
		if err := checkOutfitItems(where, r.Body, c, w); err != nil {
			return err
		}
		for _, id := range c {
			all[id] = true
		}
	}
	allItems := make([]string, 0, len(all))
	for id := range all {
		allItems = append(allItems, id)
	}
	colourKeys := func(m map[string]map[string]int64) error {
		keys := map[string]bool{}
		for k, opts := range m {
			keys[k] = true
			total := int64(0)
			for c, v := range opts {
				if !wardrobeColour.MatchString(c) || v < 0 || v > 100 {
					return fmt.Errorf("%s: colours %s: #RRGGBB with a share 0..100", where, k)
				}
				total += v
			}
			if total <= 0 {
				return fmt.Errorf("%s: colours %s: the shares add up to nothing", where, k)
			}
		}
		return checkOutfitColours(where, r.Body, allItems, w, rec, keys)
	}
	if err := colourKeys(r.Colours); err != nil {
		return err
	}
	last := int64(0)
	for _, band := range r.Ages {
		if band.FromAge < 16 || band.FromAge > 90 || band.FromAge <= last {
			return fmt.Errorf("%s: ages: from_age 16..90, ascending", where)
		}
		last = band.FromAge
		if band.Hair != nil {
			if err := hairOK(band.Hair); err != nil {
				return err
			}
		}
		if err := colourKeys(band.Colours); err != nil {
			return err
		}
	}
	if h := r.HeightM; h != nil && (math.IsNaN(h.Min) || h.Min < lookMinHeightM || h.Max > lookMaxHeightM || h.Min > h.Max) {
		return fmt.Errorf("%s: height_m min..max within %g..%g", where, lookMinHeightM, lookMaxHeightM)
	}
	if len(r.Shapes) > 0 {
		total := int64(0)
		for s, o := range r.Shapes {
			if s != lookShapeNone && !contains(rec.Bodies[r.Body].Shapes, s) {
				return fmt.Errorf("%s: shape %q is not a shape of body %s", where, s, r.Body)
			}
			if o.Share < 0 || o.Share > 100 || o.Min < 0 || o.Max > 1 || o.Min > o.Max || (s == lookShapeNone && (o.Min != 0 || o.Max != 0)) {
				return fmt.Errorf("%s: shape %s: share 0..100, 0 <= min <= max <= 1 (none: no amount)", where, s)
			}
			total += o.Share
		}
		if total <= 0 {
			return fmt.Errorf("%s: shapes: the shares add up to nothing", where)
		}
	}
	if r.Posture != nil {
		if err := checkPostureGroups(where, *r.Posture); err != nil {
			return err
		}
	}
	if r.Idle != nil && !lookIdles[*r.Idle] {
		return fmt.Errorf("%s: idle %q is not a stance", where, *r.Idle)
	}
	if r.Shading != nil && !lookShadings[*r.Shading] {
		return fmt.Errorf("%s: shading %q", where, *r.Shading)
	}
	return nil
}

func sortedOptions(m map[string]OutfitOption) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedShares(m map[string]int64, field string) []map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{field: k, "share": m[k]})
	}
	return out
}

func clientColourOptions(m map[string]map[string]int64) []map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{"key": k, "options": sortedShares(m[k], "colour")})
	}
	return out
}

// clientWardrobe is the Godot projection: art.wardrobe {record, layers, steps, idles, bodies, items}, art.outfit_rules,
// art.person_outfits (every list sorted, so the client's picks by hash are the same on every machine).
func clientWardrobe(a Art) (map[string]any, []map[string]any, []map[string]any) {
	w := a.Wardrobe
	ids := make([]string, 0, len(w.Bodies))
	for id := range w.Bodies {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	bodies := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		b := w.Bodies[id]
		heads := make([]map[string]any, 0, len(b.Heads))
		for _, h := range sortedKeys(b.Heads) {
			row := map[string]any{"id": h}
			if g := b.Heads[h].Ageing; g != nil {
				row["ageing"] = map[string]int64{"from_age": g.FromAge, "full_age": g.FullAge}
			}
			heads = append(heads, row)
		}
		hair := make([]map[string]any, 0, len(b.Hair))
		for _, st := range sortedKeys(b.Hair) {
			hair = append(hair, map[string]any{"id": st, "heads": b.Hair[st].Heads})
		}
		bodies = append(bodies, map[string]any{"id": id, "sex": b.Sex, "pack": "res://actors-assets/wardrobe/" + id + ".scn",
			"heads": heads, "hair": hair, "materials": b.Materials})
	}
	items := make([]map[string]any, 0, len(w.Items))
	for _, id := range sortedKeys(w.Items) {
		it := w.Items[id]
		row := map[string]any{"id": id, "slots": it.Slots, "layer": it.Layer, "covers": it.Covers, "materials": it.Materials,
			"bodies": it.Bodies, "items": it.Items, "surfaces": it.Surfaces}
		if it.Motion != nil && it.Motion.Swing != nil {
			row["motion"] = map[string]any{"swing": it.Motion.Swing}
		}
		if it.HidesHair {
			row["hides_hair"] = true
		}
		items = append(items, row)
	}
	idles := map[string]bool{}
	rules := make([]map[string]any, 0, len(a.OutfitRules))
	for _, name := range sortedKeys(a.OutfitRules) {
		r := a.OutfitRules[name]
		row := map[string]any{"id": name, "body": r.Body, "caste": r.Caste, "heads": sortedShares(r.Heads, "id"),
			"hair": sortedShares(r.Hair, "id"), "wear": r.Wear, "colours": clientColourOptions(r.Colours)}
		if len(r.Choose) > 0 {
			groups := make([]map[string]any, 0, len(r.Choose))
			for _, g := range sortedKeys(r.Choose) {
				opts := make([]map[string]any, 0, len(r.Choose[g]))
				for _, o := range sortedOptions(r.Choose[g]) {
					items := r.Choose[g][o].Items
					if items == nil {
						items = []string{}
					}
					opts = append(opts, map[string]any{"id": o, "items": items, "share": r.Choose[g][o].Share})
				}
				groups = append(groups, map[string]any{"id": g, "options": opts})
			}
			row["choose"] = groups
		}
		if len(r.Ages) > 0 {
			bands := make([]map[string]any, 0, len(r.Ages))
			for _, band := range r.Ages {
				b := map[string]any{"from_age": band.FromAge}
				if band.Hair != nil {
					b["hair"] = sortedShares(band.Hair, "id")
				}
				if band.Colours != nil {
					b["colours"] = clientColourOptions(band.Colours)
				}
				bands = append(bands, b)
			}
			row["ages"] = bands
		}
		if r.HeightM != nil {
			row["height_m"] = map[string]float64{"min": r.HeightM.Min, "max": r.HeightM.Max}
		}
		if len(r.Shapes) > 0 {
			row["shapes"] = clientPopulationShapes(r.Shapes)
		}
		if r.Posture != nil {
			row["posture"] = r.Posture
		}
		if r.Idle != nil {
			row["idle"] = *r.Idle
			idles[*r.Idle] = true
		}
		if r.Shading != nil {
			row["shading"] = *r.Shading
		}
		rules = append(rules, row)
	}
	people := make([]map[string]any, 0, len(a.PersonOutfits))
	for _, id := range sortedKeys(a.PersonOutfits) {
		p := a.PersonOutfits[id]
		row := map[string]any{"id": id, "body": p.Body, "caste": p.Caste, "head": p.Head, "hair": p.Hair, "items": p.Items, "colours": p.Colours, "use": p.Use}
		if len(p.Shapes) > 0 {
			row["shapes"] = p.Shapes
		}
		if p.HeightM != nil {
			row["height_m"] = *p.HeightM
		}
		if p.Age != nil {
			row["age"] = *p.Age
		}
		if p.Posture != nil {
			row["posture"] = p.Posture
		}
		if p.Idle != nil {
			row["idle"] = *p.Idle
			idles[*p.Idle] = true
		}
		if p.Shading != nil {
			row["shading"] = *p.Shading
		}
		people = append(people, row)
	}
	wardrobe := map[string]any{"record": w.Record, "layers": w.Layers, "steps": w.Steps, "bodies": bodies, "items": items,
		"idles": sortedKeys(idles)}
	return wardrobe, rules, people
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
