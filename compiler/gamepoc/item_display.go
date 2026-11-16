package gamepoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// Items and their display v1 (the game's design/items-display/v1): world.art.item_display gives every item id one look —
// a body (one 3D thing: its model and far step, how it rests, its stack looks, the grip in a hand, where it is carried,
// its state parts, its shop rows, its icon turn) or a system glyph — and ANG checks it against
//   - the items (world.items): a body only for a physical item, a glyph only for a digital one;
//   - the build record (art-source/items/v1/record.json, written by the game's scripts/item-icons.py from the files):
//     every model with its SHA-256 (the client file must still be that file), triangles within the owner's budgets, size
//     and state nodes; every icon with its cache key (recomputed here: a changed model, state, turn, renderer or
//     item_look.gd makes the icon stale) and the PNG's SHA-256;
//   - the places that draw items: the torgomat's goods and buy-back (world.vending), the stall heaps (world.market_goods),
//     the hero's start (placements owned by the hero): each must have a look of its own, and the torgomat's and the
//     stalls' scenes must be the look's own (one source).
//
// The worn context is derived from the wardrobe (the view that lists the item), never restated. Every other item is a
// stand-in: a glyph by its tags, nothing drawn in the world (the coverage). Presentation only: the server never reads it.
type ItemDisplayCatalog struct {
	Record         string              `json:"record"`
	NearUpToFrameM float64             `json:"near_up_to_frame_m"`
	Bodies         map[string]ItemBody `json:"bodies"`
	Items          map[string]ItemLook `json:"items"`
	StandIn        ItemStandIn         `json:"stand_in"`
	// the contact shadow under a thing resting in the world: strength, tint, the soft edge past its footprint (m)
	Contact struct {
		Strength float64    `json:"strength"`
		Tint     [3]float64 `json:"tint"`
		SoftM    float64    `json:"soft_m"`
	} `json:"contact"`
	view      map[string]any      // the client's resource (clientItemDisplay)
	record    itemRecord          // the build record as read
	worn      map[string]string   // item -> wardrobe view
	wornSlots map[string][]string // item -> the slots of its wardrobe view
	standIns  map[string]string   // item -> glyph
	required  map[string]string   // item -> why it must have a look of its own
}
type ItemBody struct {
	Model   string      `json:"model"`
	Far     string      `json:"far,omitempty"`
	Budget  string      `json:"budget"`
	Rest    string      `json:"rest"`
	Stack   *ItemStack  `json:"stack,omitempty"`
	Held    *ItemHeld   `json:"held,omitempty"`
	Carried ItemCarried `json:"carried"`
	Icon    ItemIcon    `json:"icon"`
	States  []string    `json:"states"`
	Shop    *ItemShop   `json:"shop,omitempty"`
	Source  string      `json:"source"`
	Note    string      `json:"note"`
}
type ItemStack struct {
	Few    string `json:"few"`
	Half   string `json:"half"`
	Full   string `json:"full"`
	FullAt int    `json:"full_at"`
}
type ItemHeld struct {
	Hand    string     `json:"hand"`
	GripM   [3]float64 `json:"grip_m"`
	TurnDeg [3]float64 `json:"turn_deg"`
}
type ItemCarried struct {
	At      string      `json:"at"`
	OffsetM *[3]float64 `json:"offset_m,omitempty"`
	TurnDeg *[3]float64 `json:"turn_deg,omitempty"`
}
type ItemIcon struct {
	YawDeg   float64 `json:"yaw_deg"`
	PitchDeg float64 `json:"pitch_deg"`
	Zoom     float64 `json:"zoom"`
}
type ItemRows struct {
	Row  string `json:"row,omitempty"`
	Full string `json:"full"`
	Half string `json:"half"`
	Few  string `json:"few"`
}
type ItemShop struct {
	Torgomat *ItemRows           `json:"torgomat,omitempty"`
	Stall    map[string]ItemRows `json:"stall,omitempty"`
}
type ItemState struct {
	Fill   string   `json:"fill,omitempty"`
	Charge *float64 `json:"charge,omitempty"`
	Used   *bool    `json:"used,omitempty"`
}
type ItemLook struct {
	Body  string     `json:"body,omitempty"`
	Glyph string     `json:"glyph,omitempty"`
	State *ItemState `json:"state,omitempty"`
}
type ItemStandIn struct {
	// a glyph named for one item (worn things before their own look)
	Items map[string]string `json:"items"`
	Tags  []struct {
		Tag   string `json:"tag"`
		Glyph string `json:"glyph"`
	} `json:"tags"`
	Default string `json:"default"`
}

// itemRecord is the build record (scripts/item-icons.py).
type itemRecord struct {
	Schema   string `json:"schema"`
	About    string `json:"about"`
	Renderer struct {
		Tool string `json:"tool_sha256"`
		Look string `json:"look_sha256"`
		PX   int    `json:"px"`
	} `json:"renderer"`
	Models map[string]itemRecordModel `json:"models"`
	Icons  map[string]itemRecordIcon  `json:"icons"`
}
type itemRecordModel struct {
	SHA256       string            `json:"sha256"`
	Content      string            `json:"content_sha256"`
	Files        map[string]string `json:"files"`
	Triangles    int               `json:"triangles"`
	FarTriangles int               `json:"far_triangles,omitempty"`
	SizeM        [3]float64        `json:"size_m"`
	MinM         [3]float64        `json:"min_m"`
	Nodes        []string          `json:"nodes"`
}
type itemRecordIcon struct {
	PNG    string `json:"png"`
	Key    string `json:"key"`
	PX     int    `json:"px"`
	SHA256 string `json:"sha256"`
}

var itemBudgets = map[string]int{"small": 1500, "medium": 5000, "large": 15000}

const itemShopRowBudget = 5000 // a row of goods or a pile: the owner's medium

var itemGlyphs = map[string]bool{"chip": true, "battery": true, "bread": true, "lantern": true, "thread": true, "tools": true, "head": true, "torso": true}

// itemSlotGlyphs: the glyph of a worn thing's wardrobe slot (its first slot found here)
var itemSlotGlyphs = map[string]string{"head": "head", "torso": "torso", "over_torso": "torso", "legs": "torso"}

// itemWorldItem: what the display needs of world.items.
type itemWorldItem struct {
	Carrier string   `json:"carrier"`
	Tags    []string `json:"tags"`
}

// readItemDisplay reads and checks world.art.item_display (nil when the world has none). root is the game (the folder
// that holds server/, client-godot/, art-source/); client the Godot project; hero the player's actor id.
func readItemDisplay(world cue.Value, root, client, hero string, vending *VendingCatalog, craft *CraftCatalog) (*ItemDisplayCatalog, error) {
	v := world.LookupPath(cue.ParsePath("art.item_display"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("art.item_display: %w", err)
	}
	var c ItemDisplayCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("art.item_display: %w", err)
	}
	var items map[string]itemWorldItem
	if err := decodeWorldPart(world, "items", &items); err != nil {
		return nil, err
	}
	required, err := itemDisplayRequired(world, hero, vending, craft)
	if err != nil {
		return nil, err
	}
	worn, slots, err := itemDisplayWorn(world)
	if err != nil {
		return nil, err
	}
	c.required, c.worn, c.wornSlots = required, worn, slots
	if c.Record == "" || !strings.HasPrefix(c.Record, "art-source/") || strings.Contains(c.Record, "..") {
		return nil, fmt.Errorf("art.item_display: the record %q must be a file under art-source/", c.Record)
	}
	recRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.Record)))
	if err != nil {
		return nil, fmt.Errorf("art.item_display: the record %s: %w (run python3 scripts/item-icons.py)", c.Record, err)
	}
	if err := json.Unmarshal(recRaw, &c.record); err != nil || c.record.Schema != "items-display.record.v1" {
		return nil, fmt.Errorf("art.item_display: the record %s is not an items-display.record.v1 file", c.Record)
	}
	if err := checkItemDisplay(&c, items, client); err != nil {
		return nil, fmt.Errorf("art.item_display: %w", err)
	}
	if err := checkItemDisplayPlaces(&c, vending, craft); err != nil {
		return nil, fmt.Errorf("art.item_display: %w", err)
	}
	c.view = clientItemDisplay(&c)
	return &c, nil
}

func decodeWorldPart(world cue.Value, path string, out any) error {
	v := world.LookupPath(cue.ParsePath(path))
	if !v.Exists() {
		return nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return fmt.Errorf("world.%s: %w", path, err)
	}
	return json.Unmarshal(raw, out)
}

// itemDisplayRequired: the items some place of the game draws, which must have a look of their own.
func itemDisplayRequired(world cue.Value, hero string, vending *VendingCatalog, craft *CraftCatalog) (map[string]string, error) {
	need := map[string]string{}
	if vending != nil {
		for _, u := range vending.Units {
			for _, sh := range u.Shelves {
				need[sh.Item] = "a shelf of torgomat " + u.ID
			}
			for _, b := range u.Buys {
				need[b.Item] = "the buy-back tray of torgomat " + u.ID
			}
		}
	}
	if craft != nil {
		for _, s := range craft.Stalls {
			for _, h := range s.Heaps {
				need[h.Item] = "heap " + h.ID + " of stall " + s.Place
			}
		}
	}
	var levels map[string]struct {
		Placements []struct {
			ID      string `json:"id"`
			Owner   string `json:"owner"`
			Content struct {
				Kind string `json:"kind"`
				Item string `json:"item"`
			} `json:"content"`
		} `json:"placements"`
	}
	if err := decodeWorldPart(world, "levels", &levels); err != nil {
		return nil, err
	}
	for lid, l := range levels {
		for _, p := range l.Placements {
			if p.Owner == hero && p.Content.Kind == "fixed" && p.Content.Item != "" {
				need[p.Content.Item] = "the hero's start (" + lid + " " + p.ID + ")"
			}
		}
	}
	return need, nil
}

// itemDisplayWorn: item -> the wardrobe view that lists it (art.wardrobe.items[view].items) and that view's slots, read
// from CUE.
func itemDisplayWorn(world cue.Value) (map[string]string, map[string][]string, error) {
	var wardrobe struct {
		Items map[string]struct {
			Items []string `json:"items"`
			Slots []string `json:"slots"`
		} `json:"items"`
	}
	if err := decodeWorldPart(world, "art.wardrobe", &wardrobe); err != nil {
		return nil, nil, err
	}
	worn := map[string]string{}
	slots := map[string][]string{}
	views := make([]string, 0, len(wardrobe.Items))
	for view := range wardrobe.Items {
		views = append(views, view)
	}
	sort.Strings(views)
	for _, view := range views {
		for _, item := range wardrobe.Items[view].Items {
			if other, ok := worn[item]; ok {
				return nil, nil, fmt.Errorf("art.wardrobe: %s is worn as %s and as %s (one view per item)", item, other, view)
			}
			worn[item] = view
			slots[item] = wardrobe.Items[view].Slots
		}
	}
	return worn, slots, nil
}

// itemIconKey: the cache key of an icon — the same line the game's scripts/item-icons.py hashes.
func itemIconKey(content string, s *ItemState, icon ItemIcon, renderer, look string, px int) string {
	num := func(p *float64) string {
		if p == nil {
			return "-"
		}
		return fmt.Sprintf("%.3f", *p)
	}
	fill, used, charge := "-", "-", "-"
	if s != nil {
		if s.Fill != "" {
			fill = s.Fill
		}
		charge = num(s.Charge)
		if s.Used != nil {
			used = fmt.Sprint(*s.Used)
		}
	}
	yaw, pitch, zoom := icon.YawDeg, icon.PitchDeg, icon.Zoom
	line := strings.Join([]string{"items-icon.v1", "model=" + content, "fill=" + fill, "charge=" + charge, "used=" + used,
		"yaw=" + num(&yaw), "pitch=" + num(&pitch), "zoom=" + num(&zoom), "renderer=" + renderer, "look=" + look,
		fmt.Sprintf("px=%d", px)}, "|")
	sum := sha256.Sum256([]byte(line))
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// clientFile: a res:// path of the Godot project as a file.
func clientFile(client, res string) (string, bool) {
	if !strings.HasPrefix(res, "res://") || strings.Contains(res, "..") {
		return "", false
	}
	return filepath.Join(client, filepath.FromSlash(strings.TrimPrefix(res, "res://"))), true
}

func checkItemDisplay(c *ItemDisplayCatalog, items map[string]itemWorldItem, client string) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	if !finite(c.NearUpToFrameM) || c.NearUpToFrameM < 3 || c.NearUpToFrameM > 16 {
		return fmt.Errorf("near_up_to_frame_m %v: 3..16 m (the camera's frames)", c.NearUpToFrameM)
	}
	if !finite(c.Contact.Strength) || c.Contact.Strength <= 0 || c.Contact.Strength > 1 || !finite(c.Contact.SoftM) || c.Contact.SoftM < 0.01 || c.Contact.SoftM > 0.2 {
		return fmt.Errorf("contact: strength 0..1 and soft_m 0.01..0.2 m (the shadow's edge past the footprint), got %v / %v", c.Contact.Strength, c.Contact.SoftM)
	}
	for k, v := range c.Contact.Tint {
		if !finite(v) || v < 0 || v > 1 {
			return fmt.Errorf("contact.tint[%d] %v: 0..1", k, v)
		}
	}
	// the record: every model file is still the recorded one
	checked := map[string]itemRecordModel{}
	model := func(where, res string, limit int) (itemRecordModel, error) {
		if m, ok := checked[res]; ok {
			return m, nil
		}
		m, ok := c.record.Models[res]
		if !ok {
			return m, fmt.Errorf("%s: model %s is not in the record %s (run python3 scripts/item-icons.py)", where, res, c.Record)
		}
		if len(m.Files) == 0 || m.Files[res] != m.SHA256 {
			return m, fmt.Errorf("%s: the record gives %s no SHA-256 of its own file", where, res)
		}
		for file, want := range m.Files {
			path, ok := clientFile(client, file)
			if !ok {
				return m, fmt.Errorf("%s: %s loads %q, not a client resource", where, res, file)
			}
			got, err := fileSHA256(path)
			if err != nil {
				return m, fmt.Errorf("%s: %s: %w", where, file, err)
			}
			if got != want {
				return m, fmt.Errorf("%s: %s changed since the record (run python3 scripts/item-icons.py)", where, file)
			}
		}
		if m.Triangles < 1 || m.Triangles > limit {
			return m, fmt.Errorf("%s: %s has %d triangles, the budget is %d (owner 09.11)", where, res, m.Triangles, limit)
		}
		checked[res] = m
		return m, nil
	}
	nodes := func(m itemRecordModel, prefix string) []string {
		var out []string
		for _, n := range m.Nodes {
			if strings.HasPrefix(n, prefix) {
				out = append(out, n)
			}
		}
		return out
	}
	ids := make([]string, 0, len(c.Bodies))
	for id := range c.Bodies {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		b := c.Bodies[id]
		where := "body " + id
		limit, ok := itemBudgets[b.Budget]
		if !ok {
			return fmt.Errorf("%s: budget %q", where, b.Budget)
		}
		m, err := model(where, b.Model, limit)
		if err != nil {
			return err
		}
		if b.Budget != "small" && b.Far == "" {
			return fmt.Errorf("%s: a %s body needs a far step (owner 09.11: LOD on the models)", where, b.Budget)
		}
		if b.Far != "" {
			f, err := model(where+" far", b.Far, limit)
			if err != nil {
				return err
			}
			if f.Triangles >= m.Triangles {
				return fmt.Errorf("%s: the far step %s (%d) is not lighter than the model (%d)", where, b.Far, f.Triangles, m.Triangles)
			}
		}
		if b.Stack != nil {
			for _, r := range []string{b.Stack.Few, b.Stack.Half, b.Stack.Full} {
				if _, err := model(where+" stack", r, itemShopRowBudget); err != nil {
					return err
				}
			}
		}
		if b.Shop != nil {
			if t := b.Shop.Torgomat; t != nil {
				for _, r := range []string{t.Full, t.Half, t.Few} {
					if _, err := model(where+" torgomat", r, itemShopRowBudget); err != nil {
						return err
					}
				}
			}
			for look, rows := range b.Shop.Stall {
				for _, r := range []string{rows.Full, rows.Half, rows.Few} {
					if _, err := model(where+" stall "+look, r, 6000); err != nil {
						return err
					}
				}
			}
		}
		// the grip lies on the thing (its bounds and 2 cm)
		if h := b.Held; h != nil {
			for k := 0; k < 3; k++ {
				if !finite(h.GripM[k]) || h.GripM[k] < m.MinM[k]-0.02 || h.GripM[k] > m.MinM[k]+m.SizeM[k]+0.02 {
					return fmt.Errorf("%s: the grip %v is off the model (bounds from %v, size %v)", where, h.GripM, m.MinM, m.SizeM)
				}
			}
		}
		if b.Carried.At != "none" && (b.Carried.OffsetM == nil || b.Carried.TurnDeg == nil) {
			return fmt.Errorf("%s: carried at %s needs its offset and turn", where, b.Carried.At)
		}
		// the state parts the model has for each channel
		for _, ch := range b.States {
			switch ch {
			case "fill":
				if len(nodes(m, "state_fill_")) == 0 {
					return fmt.Errorf("%s: channel fill, but %s has no state_fill_* part", where, b.Model)
				}
			case "used":
				if len(nodes(m, "state_used_")) == 0 {
					return fmt.Errorf("%s: channel used, but %s has no state_used_* part", where, b.Model)
				}
			case "charge":
				gauge := nodes(m, "state_charge_")
				if len(gauge) < 2 {
					return fmt.Errorf("%s: channel charge needs at least two gauge segments state_charge_1..n in %s", where, b.Model)
				}
				for k := 1; k <= len(gauge); k++ {
					if !containsString(gauge, fmt.Sprintf("state_charge_%d", k)) {
						return fmt.Errorf("%s: the gauge of %s is not state_charge_1..%d", where, b.Model, len(gauge))
					}
				}
			default:
				return fmt.Errorf("%s: channel %q is not published by the server", where, ch)
			}
		}
		if strings.TrimSpace(b.Source) == "" || strings.TrimSpace(b.Note) == "" {
			return fmt.Errorf("%s: a body needs its source and its world reason", where)
		}
	}
	// the record of the renderer is the renderer of this project
	tool, err := fileSHA256(filepath.Join(client, "tools/item_icons.gd"))
	if err != nil {
		return fmt.Errorf("the icon renderer: %w", err)
	}
	look, err := fileSHA256(filepath.Join(client, "item_look.gd"))
	if err != nil {
		return fmt.Errorf("the item look: %w", err)
	}
	if c.record.Renderer.Tool != tool || c.record.Renderer.Look != look {
		return fmt.Errorf("the icons were drawn by another renderer or item_look.gd (run python3 scripts/item-icons.py)")
	}
	// the looks
	ids = ids[:0]
	for id := range c.Items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		l := c.Items[id]
		it, ok := items[id]
		if !ok {
			return fmt.Errorf("item %s has a look but is not an item of world.items", id)
		}
		if l.Glyph != "" {
			if !itemGlyphs[l.Glyph] {
				return fmt.Errorf("item %s: glyph %q is not a glyph of the UI kit", id, l.Glyph)
			}
			if it.Carrier != "digital" || l.State != nil {
				return fmt.Errorf("item %s: a glyph is the look of a digital thing only (a physical one needs a body)", id)
			}
			if err := checkGlyphFile(client, l.Glyph); err != nil {
				return err
			}
			continue
		}
		b, ok := c.Bodies[l.Body]
		if !ok {
			return fmt.Errorf("item %s: no body %s", id, l.Body)
		}
		if it.Carrier != "physical" {
			return fmt.Errorf("item %s: a body is the look of a physical thing", id)
		}
		// the state the item fixes is exactly the body's channels
		var s ItemState
		if l.State != nil {
			s = *l.State
		}
		give := map[string]bool{"fill": s.Fill != "", "charge": s.Charge != nil, "used": s.Used != nil}
		for ch, given := range give {
			if given != containsString(b.States, ch) {
				return fmt.Errorf("item %s: its state must give exactly the channels of %s %v", id, l.Body, b.States)
			}
		}
		if s.Fill != "" {
			if len(nodesWith(c.record.Models[b.Model].Nodes, "state_fill_"+s.Fill)) == 0 && s.Fill == "full" {
				return fmt.Errorf("item %s: fill full, but %s has no state_fill_full part", id, b.Model)
			}
		}
		icon, ok := c.record.Icons[id]
		if !ok {
			return fmt.Errorf("item %s: no icon in the record (run python3 scripts/item-icons.py)", id)
		}
		key := itemIconKey(c.record.Models[b.Model].Content, l.State, b.Icon, tool, look, c.record.Renderer.PX)
		if icon.Key != key {
			return fmt.Errorf("item %s: its icon is stale (run python3 scripts/item-icons.py)", id)
		}
		path, ok := clientFile(client, icon.PNG)
		if !ok || !strings.HasSuffix(icon.PNG, ".png") {
			return fmt.Errorf("item %s: the icon %q is not a client PNG", id, icon.PNG)
		}
		if got, err := fileSHA256(path); err != nil || got != icon.SHA256 {
			return fmt.Errorf("item %s: the icon file %s is missing or not the recorded one", id, icon.PNG)
		}
	}
	// the places that draw an item need its own look; a worn item needs its look in the world too
	for item, why := range c.required {
		if l, ok := c.Items[item]; !ok || l.Body == "" && items[item].Carrier == "physical" {
			return fmt.Errorf("item %s is drawn by %s and has no look of its own (a stand-in is not allowed there)", item, why)
		}
	}
	// a worn item is drawn on a body by its wardrobe view; its look in the world (folded on a shelf) may come later: until
	// then it is a stand-in in the screens
	for item, view := range c.worn {
		if _, ok := items[item]; !ok {
			return fmt.Errorf("wardrobe view %s wears %s, which is not an item", view, item)
		}
	}
	// stand-ins: every other item, by its tags
	if !itemGlyphs[c.StandIn.Default] {
		return fmt.Errorf("stand_in.default %q is not a glyph of the UI kit", c.StandIn.Default)
	}
	for _, row := range c.StandIn.Tags {
		if !itemGlyphs[row.Glyph] {
			return fmt.Errorf("stand_in.tags %s: %q is not a glyph of the UI kit", row.Tag, row.Glyph)
		}
	}
	for id, g := range c.StandIn.Items {
		if _, ok := items[id]; !ok {
			return fmt.Errorf("stand_in.items: %s is not an item", id)
		}
		if _, own := c.Items[id]; own {
			return fmt.Errorf("stand_in.items: %s has a look of its own (a stand-in glyph would never show)", id)
		}
		if !itemGlyphs[g] {
			return fmt.Errorf("stand_in.items %s: %q is not a glyph of the UI kit", id, g)
		}
	}
	c.standIns = map[string]string{}
	for id, it := range items {
		if _, ok := c.Items[id]; ok {
			continue
		}
		// the glyph named for the item; else a worn thing's slot; else the first entry of the catalog's list whose tag
		// the item has (specific kinds first)
		glyph := c.StandIn.Default
		if g, ok := c.StandIn.Items[id]; ok {
			glyph = g
		} else if g := itemSlotGlyph(c.wornSlots[id]); g != "" {
			glyph = g
		} else {
			for _, row := range c.StandIn.Tags {
				if containsString(it.Tags, row.Tag) {
					glyph = row.Glyph
					break
				}
			}
		}
		c.standIns[id] = glyph
		if err := checkGlyphFile(client, glyph); err != nil {
			return err
		}
	}
	return nil
}

// itemSlotGlyph: the glyph of a worn thing by the first of its wardrobe slots that has one ("" when none).
func itemSlotGlyph(slots []string) string {
	for _, slot := range slots {
		if g, ok := itemSlotGlyphs[slot]; ok {
			return g
		}
	}
	return ""
}

func nodesWith(nodes []string, prefix string) []string {
	var out []string
	for _, n := range nodes {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func checkGlyphFile(client, glyph string) error {
	if _, err := os.Stat(filepath.Join(client, "assets/icons", glyph+".svg")); err != nil {
		return fmt.Errorf("glyph %s: %w", glyph, err)
	}
	return nil
}

// checkItemDisplayPlaces: the torgomat's and the stalls' scenes of an item are its look's own (one source).
func checkItemDisplayPlaces(c *ItemDisplayCatalog, vending *VendingCatalog, craft *CraftCatalog) error {
	if vending != nil {
		ids := make([]string, 0, len(vending.Models))
		for id := range vending.Models {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			m := vending.Models[id]
			b, ok := c.Bodies[c.Items[id].Body]
			if !ok {
				return fmt.Errorf("vending: %s has scenes but no body in the item display", id)
			}
			if m.Item != b.Model {
				return fmt.Errorf("vending: the unit of %s is %s, its look's body is %s", id, m.Item, b.Model)
			}
			if m.Full != "" && (b.Shop == nil || b.Shop.Torgomat == nil || m.Full != b.Shop.Torgomat.Full || m.Half != b.Shop.Torgomat.Half || m.Few != b.Shop.Torgomat.Few) {
				return fmt.Errorf("vending: the shelf rows of %s are not its look's torgomat rows", id)
			}
		}
	}
	if craft != nil {
		for _, s := range craft.Stalls {
			for _, h := range s.Heaps {
				b := c.Bodies[c.Items[h.Item].Body]
				found := false
				if b.Shop != nil {
					for _, rows := range b.Shop.Stall {
						if craft.Models[h.Heaps.Full].Scene == rows.Full && craft.Models[h.Heaps.Half].Scene == rows.Half && craft.Models[h.Heaps.Few].Scene == rows.Few {
							found = true
						}
					}
				}
				if !found {
					return fmt.Errorf("market_goods %s heap %s: its heaps are not stall rows of the look of %s", s.Place, h.ID, h.Item)
				}
			}
		}
	}
	return nil
}

// clientItemDisplay: what item_view.gd reads (presentation.json item_display).
func clientItemDisplay(c *ItemDisplayCatalog) map[string]any {
	bodies := map[string]any{}
	for id, b := range c.Bodies {
		m := c.record.Models[b.Model]
		row := map[string]any{"model": b.Model, "rest": b.Rest, "carried": b.Carried, "states": b.States,
			"triangles": m.Triangles, "size_m": m.SizeM, "min_m": m.MinM}
		if b.Far != "" {
			row["far"] = b.Far
		}
		if b.Stack != nil {
			row["stack"] = b.Stack
		}
		if b.Held != nil {
			row["held"] = b.Held
		}
		if b.Shop != nil {
			row["shop"] = b.Shop
		}
		bodies[id] = row
	}
	looks := map[string]any{}
	for id, l := range c.Items {
		row := map[string]any{}
		if l.Glyph != "" {
			row["glyph"] = l.Glyph
			row["icon"] = "res://assets/icons/" + l.Glyph + ".svg"
		} else {
			row["body"] = l.Body
			row["icon"] = c.record.Icons[id].PNG
			if l.State != nil {
				row["state"] = l.State
			}
		}
		if view, ok := c.worn[id]; ok {
			row["worn"] = map[string]any{"view": view}
		}
		looks[id] = row
	}
	own, glyph := 0, 0
	for _, l := range c.Items {
		if l.Body != "" {
			own++
		} else {
			glyph++
		}
	}
	return map[string]any{"near_up_to_frame_m": c.NearUpToFrameM, "bodies": bodies, "items": looks, "stand_ins": c.standIns,
		"contact":  map[string]any{"strength": c.Contact.Strength, "tint": c.Contact.Tint, "soft_m": c.Contact.SoftM},
		"coverage": map[string]int{"own": own, "glyph": glyph, "stand_in": len(c.standIns)}}
}
