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
	} `json:"looks"`
}

var (
	lookID      = regexp.MustCompile(`^look\.[a-z0-9_]+$`)
	lookModel   = regexp.MustCompile(`^[a-z0-9_]+$`)
	lookSHA     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	lookCastes  = map[string]bool{"silver": true, "bronze": true, "copper": true, "gold": true}
	lookWhen    = map[string]bool{"always": true, "on_duty": true}
	lookMaxGlow = 16.0
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
// surfaces; every bound person exists; an on_duty glow belongs to someone who guards a turret post. The GLB
// is not required (it stays outside Git): without it the client keeps the mannequin.
func checkLooks(root string, a Art, person func(string) bool, turrets *TurretWatchCatalog) error {
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
		looks = append(looks, map[string]any{"id": id, "record": l.Record, "model": l.Model, "caste": l.Caste,
			"pack": "res://actors-assets/looks/" + l.Model + ".scn", "glow": glow})
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
