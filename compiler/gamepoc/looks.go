package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Look is one 3D look of people in the Godot client (world.art.looks, silver caste v1 part B): a model on
// the shared UAL skeleton and its glowing surfaces. The model itself (a GLB) stays outside Git; its record
// (look.json in the game tree: file and SHA-256 per model, written by the art step) names the exact build.
// The client packs the model on the shared UAL clips (tools/godot-link/prepare_looks.py) and keeps the
// mannequin when the model is absent or is another build, so the game always starts.
type Look struct {
	ID string `json:"id"`
	// Record: the look.json of the art step (game-relative path under art-source/); Model: its key there.
	Record string     `json:"record"`
	Model  string     `json:"model"`
	Caste  string     `json:"caste"`
	Glow   []LookGlow `json:"glow"`
	// Colourways tint the model's zone mask (the record's zones): the atlas is drawn in the zones' reference colours, the
	// game divides by them and multiplies by the colourway's (client-godot/shaders/look_zones.gdshader); nothing is
	// baked. Colourway names the one worn; a model without a mask has none.
	Colourway  string                   `json:"colourway,omitempty"`
	Colourways map[string]LookColourway `json:"colourways,omitempty"`
	// Materials of the surfaces that do not glow, by the record's surface names (roughness, metal, clearcoat …).
	Materials map[string]LookMaterial `json:"materials,omitempty"`
	// Biomonitor: the glow surface whose second UV holds the segment index of the world.biomonitor row (biomonitor.go).
	Biomonitor *LookBiomonitor `json:"biomonitor,omitempty"`
	// HeightM: the body's crown-to-sole height in the rest pose, metres (an adult: lookMinHeightM..lookMaxHeightM). The
	// client scales the look uniformly so the rest height it measured at pack time equals this, and its steps grow with
	// it (the gait follows the body). Nil: the model's own height as built.
	HeightM *float64 `json:"height_m,omitempty"`
	// Idle: the stance the wearer stands in when he does not walk, work or sit (one of lookIdles: "post", a guard on duty
	// at his post). The clip is a stance of the shared UAL rig the client packs with the look (art-source/anim-stances);
	// nil: the shared Idle. Explicitly empty is rejected (name a stance or leave the field out).
	Idle *string `json:"idle,omitempty"`
	// Parts (market people v1, part B): surfaces of the model (part_* in its record) that only some wearers wear — the
	// fitter's kit, the trader's apron, a woman's bun or tail (looks_parts.go). Nil: the model is drawn whole, as built.
	Parts *LookParts `json:"parts,omitempty"`
	// Ageing (part B): the surface whose second UV set holds the face painted older; the client blends toward it by the
	// wearer's age from data (looks_parts.go). Nil: no age marks.
	Ageing *LookAgeing `json:"ageing,omitempty"`
	// Shading (characters pipeline v1): how the client draws the look. Nil — the build's materials with the zone tint
	// (look_zones); "painted" — the painted 3D of the paint test (shaders/people_painted.gdshader) as the look's own material:
	// the atlas is the painting projected onto the UV, the game's light falls in painted bands, no specular. Needs a zone mask.
	Shading *string `json:"shading,omitempty"`
}

// LookColourway: a colour per tinted zone of the record's mask.
type LookColourway struct {
	Zones map[string]string `json:"zones"`
	Note  string            `json:"note"`
}

// LookMaterial: a surface's material from data; Clearcoat, ClearcoatRoughness and Specular are optional (nil: as built).
type LookMaterial struct {
	Roughness          float64  `json:"roughness"`
	Metallic           float64  `json:"metallic"`
	Clearcoat          *float64 `json:"clearcoat,omitempty"`
	ClearcoatRoughness *float64 `json:"clearcoat_roughness,omitempty"`
	Specular           *float64 `json:"specular,omitempty"`
}

// LookBiomonitor names the glow surface of the biomonitor row.
type LookBiomonitor struct {
	Surface string `json:"surface"`
}

// LookGlow is one glowing surface of a look. Energy is the brightness of the surface's emission
// (0 = off). When: "always", or "on_duty" — only while the person guards a post of world.turret_watch on
// the level being shown (the same schedule-derived guard list the turret uses to ignore him, so «the lights
// burn» and «the turret does not watch him» always coincide). TrackingEnergy (on_duty only): the brightness
// at the full watch of the post's turrets; between, it follows the server's turret eye share.
type LookGlow struct {
	Surface        string   `json:"surface"`
	Energy         float64  `json:"energy"`
	When           string   `json:"when"`
	TrackingEnergy *float64 `json:"tracking_energy,omitempty"`
}

// lookRecord is the art step's look.json: per model the GLB (game-relative, outside Git), its SHA-256 and
// the names of its surfaces (surface -> material name in the GLB).
type lookRecord struct {
	Looks map[string]struct {
		File     string            `json:"file"`
		SHA256   string            `json:"sha256"`
		Surfaces map[string]string `json:"surfaces"`
		// Surfaces whose primitives carry a second UV set (a biomonitor's segment index lives there).
		UV2Surfaces []string `json:"uv2_surfaces"`
		// The zone mask beside the GLB (outside Git) and the colour each tinted zone has in the atlas.
		Zones *lookRecordZones `json:"zones"`
	} `json:"looks"`
}

// lookRecordZones: the model's zone mask (game-relative .png outside Git, its SHA-256) and the colour each tinted zone
// has in the atlas (the reference the game divides by).
type lookRecordZones struct {
	Mask          string            `json:"mask"`
	MaskSHA256    string            `json:"mask_sha256"`
	ReferenceSRGB map[string]string `json:"reference_srgb"`
}

var (
	lookID      = regexp.MustCompile(`^look\.[a-z0-9_]+$`)
	lookModel   = regexp.MustCompile(`^[a-z0-9_]+$`)
	lookSHA     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	lookCastes  = map[string]bool{"silver": true, "bronze": true, "copper": true, "gold": true}
	lookWhen    = map[string]bool{"always": true, "on_duty": true}
	lookMaxGlow = 16.0
	lookColour  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	// The height of an adult body a look may be drawn at (crown to sole, metres).
	lookMinHeightM, lookMaxHeightM = 1.45, 2.05
	// The idle stances of the shared UAL rig a look may name (the clips in art-source/anim-stances/v1/stances.json).
	lookIdles = map[string]bool{"post": true}
	// The ways a look may be drawn besides the build's own materials (art.looks[].shading).
	lookShadings = map[string]bool{"painted": true}
)

// lookPath is a game-relative path that stays under art-source/ (no absolute path, no '..').
func lookPath(p, suffix string) bool {
	clean := path.Clean(p)
	return p != "" && clean == p && !path.IsAbs(p) && !strings.Contains(p, "\\") && strings.HasPrefix(p, "art-source/") &&
		!strings.HasPrefix(clean, "../") && !strings.Contains(clean, "/../") && strings.HasSuffix(p, suffix)
}

// lookDuty is the post a person guards on a level: the turret posts of world.turret_watch that list him.
type lookDuty struct {
	Post    string   `json:"post"`
	Level   string   `json:"level"`
	Turrets []string `json:"turrets"`
}

func lookDuties(turrets *TurretWatchCatalog, person string) []lookDuty {
	if turrets == nil {
		return nil
	}
	byKey := map[string]*lookDuty{}
	keys := []string{}
	for _, p := range turrets.Posts {
		guard := false
		for _, g := range p.Guards {
			guard = guard || g == person
		}
		if !guard {
			continue
		}
		key := p.LevelID + "\x00" + p.GuardPost
		if byKey[key] == nil {
			byKey[key] = &lookDuty{Post: p.GuardPost, Level: p.LevelID}
			keys = append(keys, key)
		}
		byKey[key].Turrets = append(byKey[key].Turrets, p.ID)
	}
	sort.Strings(keys)
	out := make([]lookDuty, 0, len(keys))
	for _, k := range keys {
		d := *byKey[k]
		sort.Strings(d.Turrets)
		out = append(out, d)
	}
	return out
}

// checkLooks validates world.art.looks and world.art.person_looks against the game tree (root: the folder
// that holds server/ and art-source/): every record exists and names its model with a SHA-256 and the glow
// surfaces; every bound person exists; an on_duty glow belongs to someone who guards a turret post; a height is an
// adult's; an idle names a stance of the shared rig. A model with a zone mask is worn in a colourway that colours
// exactly the mask's tinted zones; materials belong to surfaces of the model that do not glow; a biomonitor row is a
// glow surface with a second UV set, driven by world.biomonitor. The GLB and the mask are not required (they stay
// outside Git): without them the client keeps the mannequin.
func checkLooks(root string, a Art, person func(string) bool, turrets *TurretWatchCatalog, monitor *BiomonitorCatalog) error {
	records := map[string]lookRecord{}
	for key, l := range a.Looks {
		if !lookID.MatchString(key) || l.ID != key {
			return fmt.Errorf("art.looks %q: the id must be look.<name> and equal its key", key)
		}
		if !lookCastes[l.Caste] {
			return fmt.Errorf("art.looks %s: unknown caste %q", key, l.Caste)
		}
		if !lookModel.MatchString(l.Model) {
			return fmt.Errorf("art.looks %s: unsafe model name %q", key, l.Model)
		}
		if !lookPath(l.Record, ".json") {
			return fmt.Errorf("art.looks %s: the record %q must be a .json under art-source/", key, l.Record)
		}
		rec, ok := records[l.Record]
		if !ok {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(l.Record)))
			if err != nil {
				return fmt.Errorf("art.looks %s: %w", key, err)
			}
			if err := json.Unmarshal(raw, &rec); err != nil {
				return fmt.Errorf("art.looks %s: record %s: %w", key, l.Record, err)
			}
			records[l.Record] = rec
		}
		model, ok := rec.Looks[l.Model]
		if !ok {
			return fmt.Errorf("art.looks %s: record %s has no model %q", key, l.Record, l.Model)
		}
		if !lookPath(model.File, ".glb") || !lookSHA.MatchString(model.SHA256) {
			return fmt.Errorf("art.looks %s: record %s gives model %q no safe .glb file under art-source/ and SHA-256", key, l.Record, l.Model)
		}
		seen := map[string]bool{}
		for _, g := range l.Glow {
			if !strings.HasPrefix(g.Surface, "glow_") || model.Surfaces[g.Surface] == "" {
				return fmt.Errorf("art.looks %s: glow surface %q is not a glow_ surface of model %s in %s", key, g.Surface, l.Model, l.Record)
			}
			if seen[g.Surface] {
				return fmt.Errorf("art.looks %s: glow surface %s twice", key, g.Surface)
			}
			seen[g.Surface] = true
			if !lookWhen[g.When] {
				return fmt.Errorf("art.looks %s: glow %s: when must be always or on_duty, not %q", key, g.Surface, g.When)
			}
			if math.IsNaN(g.Energy) || g.Energy < 0 || g.Energy > lookMaxGlow {
				return fmt.Errorf("art.looks %s: glow %s: energy must be within 0..%g", key, g.Surface, lookMaxGlow)
			}
			if t := g.TrackingEnergy; t != nil {
				if g.When != "on_duty" {
					return fmt.Errorf("art.looks %s: glow %s: tracking_energy needs when: on_duty (the post's turrets)", key, g.Surface)
				}
				if math.IsNaN(*t) || *t < g.Energy || *t > lookMaxGlow {
					return fmt.Errorf("art.looks %s: glow %s: tracking_energy must be within energy..%g", key, g.Surface, lookMaxGlow)
				}
			}
		}
		if h := l.HeightM; h != nil && (math.IsNaN(*h) || *h < lookMinHeightM || *h > lookMaxHeightM) {
			return fmt.Errorf("art.looks %s: height_m %g is not an adult's height (crown to sole, %g..%g m)", key, *h, lookMinHeightM, lookMaxHeightM)
		}
		if i := l.Idle; i != nil && !lookIdles[*i] {
			return fmt.Errorf("art.looks %s: idle %q is not a stance of the shared rig (post); leave it out for the shared Idle", key, *i)
		}
		if err := checkLookZones(key, l, model.Zones, l.Record); err != nil {
			return err
		}
		if sh := l.Shading; sh != nil {
			if !lookShadings[*sh] {
				return fmt.Errorf("art.looks %s: shading %q is not a way of drawing a look (painted); leave it out for the build's materials", key, *sh)
			}
			if model.Zones == nil {
				return fmt.Errorf("art.looks %s: shading %s needs a zone mask in the model's record %s (the painter paints its zones)", key, *sh, l.Record)
			}
		}
		if err := checkLookParts(key, l, model.Surfaces, model.UV2Surfaces, l.Record); err != nil {
			return err
		}
		for surface, m := range l.Materials {
			if model.Surfaces[surface] == "" || strings.HasPrefix(surface, "glow_") {
				return fmt.Errorf("art.looks %s: material %q is not a surface of model %s in %s that does not glow", key, surface, l.Model, l.Record)
			}
			for _, v := range []*float64{&m.Roughness, &m.Metallic, m.Clearcoat, m.ClearcoatRoughness, m.Specular} {
				if v != nil && (math.IsNaN(*v) || *v < 0 || *v > 1) {
					return fmt.Errorf("art.looks %s: material %s: every number within 0..1", key, surface)
				}
			}
		}
		if b := l.Biomonitor; b != nil {
			uv2 := false
			for _, s := range model.UV2Surfaces {
				uv2 = uv2 || s == b.Surface
			}
			switch {
			case !strings.HasPrefix(b.Surface, "glow_") || model.Surfaces[b.Surface] == "":
				return fmt.Errorf("art.looks %s: biomonitor surface %q is not a glow_ surface of model %s in %s", key, b.Surface, l.Model, l.Record)
			case seen[b.Surface]:
				return fmt.Errorf("art.looks %s: biomonitor surface %s is also a glow of the look: one surface, one driver", key, b.Surface)
			case !uv2:
				return fmt.Errorf("art.looks %s: biomonitor surface %s has no second UV set in %s (the segment index)", key, b.Surface, l.Record)
			case monitor == nil:
				return fmt.Errorf("art.looks %s: a biomonitor row needs the shared table world.biomonitor", key)
			}
		}
	}
	for id, look := range a.PersonLooks {
		l, ok := a.Looks[look]
		if !ok {
			return fmt.Errorf("art.person_looks %s: unknown look %q", id, look)
		}
		if !person(id) {
			return fmt.Errorf("art.person_looks %s: no such person in world.persons", id)
		}
		for _, g := range l.Glow {
			if g.When == "on_duty" && len(lookDuties(turrets, id)) == 0 {
				return fmt.Errorf("art.person_looks %s: look %s lights %s on duty, but the person guards no post of world.turret_watch", id, look, g.Surface)
			}
		}
	}
	return nil
}

// checkLookZones: a model with a zone mask (a safe .png under art-source/ with its SHA-256 and the reference colour of
// each tinted zone) is worn in a colourway of the look, and every colourway colours exactly those zones; a model without
// a mask has no colourway.
func checkLookZones(key string, l Look, zones *lookRecordZones, record string) error {
	if zones == nil {
		if l.Colourway != "" || len(l.Colourways) > 0 {
			return fmt.Errorf("art.looks %s: colourways need a zone mask in the model's record %s", key, record)
		}
		return nil
	}
	if !lookPath(zones.Mask, ".png") || !lookSHA.MatchString(zones.MaskSHA256) || len(zones.ReferenceSRGB) == 0 || len(zones.ReferenceSRGB) > 4 {
		return fmt.Errorf("art.looks %s: record %s gives the zone mask no safe .png under art-source/, SHA-256 and 1..4 reference zones", key, record)
	}
	for zone, c := range zones.ReferenceSRGB {
		if !lookColour.MatchString(c) {
			return fmt.Errorf("art.looks %s: record %s: reference colour of zone %s is not #RRGGBB", key, record, zone)
		}
	}
	if _, ok := l.Colourways[l.Colourway]; !ok {
		return fmt.Errorf("art.looks %s: the model has a zone mask; name the colourway it is worn in (one of the look's colourways), not %q", key, l.Colourway)
	}
	for name, cw := range l.Colourways {
		if !lookModel.MatchString(strings.ToLower(name)) || strings.TrimSpace(cw.Note) == "" {
			return fmt.Errorf("art.looks %s: colourway %q needs a plain name and a note", key, name)
		}
		for zone := range zones.ReferenceSRGB {
			if !lookColour.MatchString(cw.Zones[zone]) {
				return fmt.Errorf("art.looks %s: colourway %s gives zone %s of the mask no #RRGGBB colour", key, name, zone)
			}
		}
		for zone := range cw.Zones {
			if _, ok := zones.ReferenceSRGB[zone]; !ok {
				return fmt.Errorf("art.looks %s: colourway %s colours %q, which is not a tinted zone of the mask in %s", key, name, zone, record)
			}
		}
	}
	return nil
}

// clientLooks is the Godot projection: the looks (sorted by id) and who wears which, with the posts each
// person guards (resolved here from world.turret_watch, so the client never guesses a guard by name).
func clientLooks(a Art, turrets *TurretWatchCatalog) ([]map[string]any, []map[string]any) {
	ids := make([]string, 0, len(a.Looks))
	for id := range a.Looks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	looks := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		l := a.Looks[id]
		glow := append([]LookGlow{}, l.Glow...)
		sort.Slice(glow, func(i, j int) bool { return glow[i].Surface < glow[j].Surface })
		row := map[string]any{"id": id, "record": l.Record, "model": l.Model, "caste": l.Caste,
			"pack": "res://actors-assets/looks/" + l.Model + ".scn", "glow": glow}
		if l.Colourway != "" {
			row["colourway"], row["colourways"] = l.Colourway, l.Colourways
		}
		if len(l.Materials) > 0 {
			row["materials"] = l.Materials
		}
		if l.Biomonitor != nil {
			row["biomonitor"] = l.Biomonitor
		}
		if l.HeightM != nil {
			row["height_m"] = *l.HeightM
		}
		if l.Idle != nil {
			row["idle"] = *l.Idle
		}
		if l.Parts != nil {
			row["parts"] = clientLookParts(*l.Parts)
		}
		if l.Ageing != nil {
			row["ageing"] = l.Ageing
		}
		if l.Shading != nil {
			row["shading"] = *l.Shading
		}
		looks = append(looks, row)
	}
	people := make([]string, 0, len(a.PersonLooks))
	for id := range a.PersonLooks {
		people = append(people, id)
	}
	sort.Strings(people)
	wearers := make([]map[string]any, 0, len(people))
	for _, id := range people {
		wearers = append(wearers, map[string]any{"id": id, "look": a.PersonLooks[id], "duty": lookDuties(turrets, id)})
	}
	return looks, wearers
}

// PopulationLook binds the people of a crowd who are not persons of the world (the hall visitors of world.visitors: no
// person id, only the `look` of their kind) to a 3D look (market people v1). Colourways: the look's colourways the
// wearers are drawn in, each with its share (1..100). The client picks a wearer's colourway from these shares by his id,
// which is stable for the whole visit, so the same walker always wears the same colours and a crowd is not a line of
// clones. A population look without a binding (or a look whose model is not packed) keeps the tinted mannequin.
type PopulationLook struct {
	Look       string           `json:"look"`
	Colourways map[string]int64 `json:"colourways"`
	// HeightM: the wearers' body heights (the look's height_m as a spread): each person's height between Min and Max,
	// picked by his id; nil — the look's own height_m (or the model as built).
	HeightM *PopulationHeight `json:"height_m,omitempty"`
	// Ages (part B): colourways by the wearer's age — a wearer of at least FromAge is drawn in that band's colourways (the
	// highest band that fits: grey hair of the older). Wearers without an age (the crowd) use Colourways.
	Ages []PopulationAge `json:"ages,omitempty"`
	// Parts (characters v1, job 4: bright accents): the shares of the options of a parts.choose group of the look for this
	// population — each wearer's option is picked by another hash of his id with these shares (0: never), so who wears the
	// signal vest is a rule in data (a share per population), not a hand-placed list. Every option of the group is named.
	// A group not named here keeps every option equally likely, as before. An age band's choose narrows the options and
	// keeps their shares.
	Parts map[string]map[string]int64 `json:"parts,omitempty"`
}

// PopulationAge is one band of colourways by age (looks_parts.go). Choose (optional) narrows a parts.choose group of the
// look for the band's wearers to the named options (the older women: a bun or tied back, never the short cut).
type PopulationAge struct {
	FromAge    int64               `json:"from_age"`
	Colourways map[string]int64    `json:"colourways"`
	Choose     map[string][]string `json:"choose,omitempty"`
}

// PopulationHeight is a range of adult body heights in metres (the same 1.45-2.05 as a look's height_m).
type PopulationHeight struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

var populationName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// checkPopulationLooks: each binding names a population look that some visitor kind of world.visitors wears (when the
// world has visitors; a binding nobody wears is a typo), a known 3D look with a zone mask, and one or more of that
// look's colourways with shares 1..100.
func checkPopulationLooks(a Art, visitors *VisitorCatalog) error {
	worn := map[string]bool{}
	if visitors != nil {
		for _, k := range visitors.Kinds {
			worn[k.Look] = true
		}
	}
	// part B: a population look a person of the world is drawn as (art.person_populations) is worn too
	for _, pp := range a.PersonPopulations {
		worn[pp.Population] = true
	}
	for name, p := range a.PopulationLooks {
		if !populationName.MatchString(name) {
			return fmt.Errorf("art.population_looks %q: a population look is a plain name (the look of a visitor kind)", name)
		}
		if !worn[name] {
			return fmt.Errorf("art.population_looks %s: no kind of world.visitors wears this look and no person of art.person_populations", name)
		}
		l, ok := a.Looks[p.Look]
		if !ok {
			return fmt.Errorf("art.population_looks %s: unknown look %q", name, p.Look)
		}
		if len(l.Colourways) == 0 {
			return fmt.Errorf("art.population_looks %s: look %s has no colourways (a crowd look needs a zone mask and its colourways)", name, p.Look)
		}
		if len(p.Colourways) == 0 {
			return fmt.Errorf("art.population_looks %s: name the colourways its wearers are drawn in, with their shares", name)
		}
		if h := p.HeightM; h != nil {
			if math.IsNaN(h.Min) || math.IsNaN(h.Max) || h.Min < lookMinHeightM || h.Max > lookMaxHeightM || h.Min > h.Max {
				return fmt.Errorf("art.population_looks %s: height_m min..max must lie within %g..%g m with min <= max", name, lookMinHeightM, lookMaxHeightM)
			}
		}
		for cw, share := range p.Colourways {
			if _, ok := l.Colourways[cw]; !ok {
				return fmt.Errorf("art.population_looks %s: %q is not a colourway of look %s", name, cw, p.Look)
			}
			if share < 1 || share > 100 {
				return fmt.Errorf("art.population_looks %s: the share of colourway %s must be within 1..100", name, cw)
			}
		}
		if err := checkPopulationAges(name, p, l); err != nil {
			return err
		}
		if err := checkPopulationParts(name, p, l); err != nil {
			return err
		}
	}
	return nil
}

// clientPopulationLooks is the Godot projection: one row per population look (sorted), its look and its colourways with
// their shares (sorted by name, so the client's pick by id is the same on every machine).
func clientPopulationLooks(a Art) []map[string]any {
	names := make([]string, 0, len(a.PopulationLooks))
	for n := range a.PopulationLooks {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		p := a.PopulationLooks[n]
		cws := make([]string, 0, len(p.Colourways))
		for cw := range p.Colourways {
			cws = append(cws, cw)
		}
		sort.Strings(cws)
		rows := make([]map[string]any, 0, len(cws))
		for _, cw := range cws {
			rows = append(rows, map[string]any{"id": cw, "share": p.Colourways[cw]})
		}
		row := map[string]any{"id": n, "look": p.Look, "colourways": rows}
		if p.HeightM != nil {
			row["height_m"] = map[string]float64{"min": p.HeightM.Min, "max": p.HeightM.Max}
		}
		if len(p.Ages) > 0 {
			row["ages"] = clientPopulationAges(p.Ages)
		}
		if len(p.Parts) > 0 {
			row["parts"] = clientPopulationParts(p.Parts)
		}
		out = append(out, row)
	}
	return out
}
