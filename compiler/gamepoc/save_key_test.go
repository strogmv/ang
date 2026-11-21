package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

// keySections: the top-level sections of world.json as generatePOC hashes them (the compiled world marshalled, plus
// the barks catalog the server embeds besides it).
func keySections(t *testing.T, c Compiled) map[string]json.RawMessage {
	t.Helper()
	world, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	sections := map[string]json.RawMessage{}
	if err := json.Unmarshal(world, &sections); err != nil {
		t.Fatal(err)
	}
	sections["barks_catalog"] = json.RawMessage(`{"packs":[{"lines":[{"id":"bark.a","measure":{"ru":{"runes":12}}}]}]}`)
	return sections
}

// testKeyWorld: a small world of every section of Compiled (json as written) with a life, terrain and localization.
func testKeyWorld() Compiled {
	var c Compiled
	c.Life = json.RawMessage(`{"actors":[{"id":"char.a","steps":[{"at_minute":0,"label":"работа"}]}],"locations":{"level.a":{"terrain":["#..#","#..#"]}}}`)
	c.Config = Config{Title: "Игра", LevelID: "level.a", TickHz: 10, SaveVersion: 1, Labels: map[string]string{"new_game": "Новая игра"}}
	c.Localization = LocalizationCatalog{DefaultLocale: "ru", Locales: []string{"ru", "en"}, Messages: []LocalizationMessage{{Key: "ui.a", Texts: map[string]string{"ru": "А", "en": "A"}}}}
	c.Menu = Menu{Background: "menu.png"}
	c.Loading = Loading{Caption: "Загрузка", Screens: map[string]LoadingScreen{"a": {Image: "a.png"}}}
	c.Art = Art{Palette: map[string]string{"bg": "#000"}}
	c.GodotScreens = &GodotScreens{}
	c.ContentHash = "content-1"
	c.Config.SaveCompatibleHashes = []string{strings.Repeat("b", 64)}
	return c
}

// testKeyPolicy classifies every section of the test world: checked, except the texts (shown) and what the server never
// decodes; the labels of the config out; the terrain's shape and the actors' steps strict.
func testKeyPolicy(t *testing.T, sections map[string]json.RawMessage) *SaveKeyPolicy {
	t.Helper()
	p := &SaveKeyPolicy{Sections: map[string]SaveKeyRule{}}
	for name := range sections {
		p.Sections[name] = SaveKeyRule{Key: saveKeyChecked, Reason: "the test world simulates with it"}
	}
	p.Sections["localization"] = SaveKeyRule{Key: saveKeyOut, Reads: saveReadsAside, Reason: "texts"}
	p.Sections["content_hash"] = SaveKeyRule{Key: saveKeyOut, Reads: saveReadsNone, Reason: "the whole package's identity"}
	p.Parts = []SaveKeyPart{
		{Path: "config/labels", Key: saveKeyOut, Reads: saveReadsNone, Reason: "menu words"},
		{Path: "config/title", Key: saveKeyOut, Reads: saveReadsNone, Reason: "window title"},
		{Path: "config/save_compatible_hashes", Key: saveKeyOut, Reads: saveReadsNone, Reason: "legacy content hashes"},
		{Path: "barks_catalog/**/measure", Key: saveKeyOut, Reads: saveReadsAside, Reason: "times a shown line"},
		{Path: "life/actors/*/steps", Key: saveKeyStrict, Reason: "a saved actor's step is an index"},
		{Path: "life/locations/*/terrain", Key: saveKeyStrict, Shape: true, Reason: "remembered cells are y*width+x"},
	}
	return p
}

func testKey(t *testing.T, c Compiled) SaveKeyCatalog {
	t.Helper()
	sections := keySections(t, c)
	key, err := computeSaveKey(testKeyPolicy(t, keySections(t, testKeyWorld())), sections)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// A menu, screen, text or picture change gives the same save key; a change of what the server simulates with does not.
func TestSaveKeyIgnoresPresentation(t *testing.T) {
	base := testKey(t, testKeyWorld())
	if base.Hash == "" || len(base.Strict) != 2 || base.Parts["life"] == "" || base.Parts["localization"] != "" {
		t.Fatalf("key %+v", base)
	}
	same := map[string]func(c *Compiled){
		"menu picture": func(c *Compiled) { c.Menu.Background = "menu-oil.png" },
		"loading screen": func(c *Compiled) {
			c.Loading.Screens = map[string]LoadingScreen{"a": {Image: "shaman.png"}, "b": {Image: "chum.png"}}
		},
		"screens": func(c *Compiled) { c.GodotScreens = &GodotScreens{Screens: []GodotScreen{{ID: "load_game"}}} },
		"art":     func(c *Compiled) { c.Art.Palette = map[string]string{"bg": "#111"} },
		"a text":  func(c *Compiled) { c.Localization.Messages[0].Texts = map[string]string{"ru": "Б", "en": "B"} },
		"a new text": func(c *Compiled) {
			c.Localization.Messages = append(c.Localization.Messages, LocalizationMessage{Key: "ui.b"})
		},
		"menu words":   func(c *Compiled) { c.Config.Labels = map[string]string{"new_game": "Новая история"} },
		"title":        func(c *Compiled) { c.Config.Title = "Futurocide" },
		"content hash": func(c *Compiled) { c.ContentHash = "content-2" },
		"legacy hashes": func(c *Compiled) {
			c.Config.SaveCompatibleHashes = []string{strings.Repeat("a", 64)}
		},
		"terrain glyphs, same size": func(c *Compiled) {
			c.Life = json.RawMessage(`{"actors":[{"id":"char.a","steps":[{"at_minute":0,"label":"работа"}]}],"locations":{"level.a":{"terrain":["####","#..#"]}}}`)
		},
	}
	policy := testKeyPolicy(t, keySections(t, testKeyWorld()))
	policy.Sections["content_hash"] = SaveKeyRule{Key: saveKeyOut, Reads: saveReadsNone, Reason: "identity"}
	for name, change := range same {
		c := testKeyWorld()
		change(&c)
		key := testKey(t, c)
		if name == "terrain glyphs, same size" {
			if key.Hash == base.Hash || key.Parts["life/locations/*/terrain"] != base.Parts["life/locations/*/terrain"] {
				t.Fatalf("%s: the terrain's shape is its strict part, its glyphs only checked", name)
			}
			continue
		}
		if key.Hash != base.Hash {
			t.Fatalf("%s changed the save key", name)
		}
	}
	differ := map[string]struct {
		change func(c *Compiled)
		strict string
	}{
		"a tuned number": {func(c *Compiled) { c.Config.TickHz = 20 }, ""},
		"one more step": {func(c *Compiled) {
			c.Life = json.RawMessage(`{"actors":[{"id":"char.a","steps":[{"at_minute":0,"label":"работа"},{"at_minute":600,"label":"сон"}]}],"locations":{"level.a":{"terrain":["#..#","#..#"]}}}`)
		}, "life/actors/*/steps"},
		"a wider level": {func(c *Compiled) {
			c.Life = json.RawMessage(`{"actors":[{"id":"char.a","steps":[{"at_minute":0,"label":"работа"}]}],"locations":{"level.a":{"terrain":["#...#","#...#"]}}}`)
		}, "life/locations/*/terrain"},
	}
	for name, d := range differ {
		c := testKeyWorld()
		d.change(&c)
		key := testKey(t, c)
		if key.Hash == base.Hash {
			t.Fatalf("%s kept the save key", name)
		}
		for _, p := range key.Strict {
			if (key.Parts[p] != base.Parts[p]) != (p == d.strict) {
				t.Fatalf("%s: strict part %s", name, p)
			}
		}
	}
	// a bark's measure (its text's length) is out; its id is not
	sections := keySections(t, testKeyWorld())
	sections["barks_catalog"] = json.RawMessage(`{"packs":[{"lines":[{"id":"bark.a","measure":{"ru":{"runes":40}}}]}]}`)
	if key, err := computeSaveKey(policy, sections); err != nil || key.Hash != base.Hash {
		t.Fatalf("a bark text's length changed the save key: %v", err)
	}
	sections["barks_catalog"] = json.RawMessage(`{"packs":[{"lines":[{"id":"bark.b","measure":{"ru":{"runes":12}}}]}]}`)
	if key, err := computeSaveKey(policy, sections); err != nil || key.Hash == base.Hash {
		t.Fatalf("a bark's id did not change the save key: %v", err)
	}
	if strings.Join(base.Unread, ",") != "config/labels,config/save_compatible_hashes,config/title,content_hash" {
		t.Fatalf("unread %v", base.Unread)
	}
}

// A new top-level section of the runtime package fails the generation until it is classified; so does a stale entry.
func TestSaveKeyRefusesAnUnclassifiedSection(t *testing.T) {
	sections := keySections(t, testKeyWorld())
	policy := testKeyPolicy(t, sections)
	sections["new_system"] = json.RawMessage(`{"rate":1}`)
	if _, err := computeSaveKey(policy, sections); err == nil || !strings.Contains(err.Error(), `"new_system"`) || !strings.Contains(err.Error(), "neither") {
		t.Fatalf("an unclassified section: %v", err)
	}
	delete(sections, "new_system")
	delete(sections, "life")
	if _, err := computeSaveKey(policy, sections); err == nil || !strings.Contains(err.Error(), `"life"`) {
		t.Fatalf("a stale section: %v", err)
	}
	sections = keySections(t, testKeyWorld())
	policy.Parts = append(policy.Parts, SaveKeyPart{Path: "life/nothing/*", Key: saveKeyStrict, Reason: "x"})
	if _, err := computeSaveKey(policy, sections); err == nil || !strings.Contains(err.Error(), "matches nothing") {
		t.Fatalf("a stale part: %v", err)
	}
	if _, err := computeSaveKey(nil, sections); err == nil {
		t.Fatal("no policy accepted")
	}
}

func TestSaveKeyPolicyRules(t *testing.T) {
	sections := keySections(t, testKeyWorld())
	for name, change := range map[string]func(p *SaveKeyPolicy){
		"no reason":         func(p *SaveKeyPolicy) { p.Sections["life"] = SaveKeyRule{Key: saveKeyChecked} },
		"an unknown key":    func(p *SaveKeyPolicy) { p.Sections["life"] = SaveKeyRule{Key: "maybe", Reason: "x"} },
		"out without reads": func(p *SaveKeyPolicy) { p.Sections["life"] = SaveKeyRule{Key: saveKeyOut, Reason: "x"} },
		"reads on a kept part": func(p *SaveKeyPolicy) {
			p.Sections["life"] = SaveKeyRule{Key: saveKeyChecked, Reads: saveReadsNone, Reason: "x"}
		},
		"the key's own section": func(p *SaveKeyPolicy) {
			p.Sections[saveKeySection] = SaveKeyRule{Key: saveKeyOut, Reads: saveReadsNone, Reason: "x"}
		},
		"a part of no section": func(p *SaveKeyPolicy) { p.Parts[0].Path = "nowhere/labels" },
		"a part twice":         func(p *SaveKeyPolicy) { p.Parts = append(p.Parts, p.Parts[0]) },
		"a whole section out":  func(p *SaveKeyPolicy) { p.Parts[0].Path = "config" },
		"a wildcard section":   func(p *SaveKeyPolicy) { p.Parts[0].Path = "*/labels" },
		"a trailing any depth": func(p *SaveKeyPolicy) { p.Parts[0].Path = "config/**" },
		"strict in strict":     func(p *SaveKeyPolicy) { p.Sections["life"] = SaveKeyRule{Key: saveKeyStrict, Reason: "x"} },
		"out in out": func(p *SaveKeyPolicy) {
			p.Sections["config"] = SaveKeyRule{Key: saveKeyOut, Reads: saveReadsNone, Reason: "x"}
		},
		"shape on out":          func(p *SaveKeyPolicy) { p.Parts[0].Shape = true },
		"a part without reason": func(p *SaveKeyPolicy) { p.Parts[4].Reason = " " },
		"a broken legacy record": func(p *SaveKeyPolicy) {
			p.Legacy = &SaveKeyRecord{Hash: "abc", Parts: map[string]string{"life": "abc"}}
		},
	} {
		p := testKeyPolicy(t, sections)
		change(p)
		if _, err := computeSaveKey(p, sections); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	p := testKeyPolicy(t, sections)
	key, err := computeSaveKey(p, sections)
	if err != nil {
		t.Fatal(err)
	}
	p.Legacy = &SaveKeyRecord{Hash: key.Hash, Parts: key.Parts}
	if legacy, err := computeSaveKey(p, sections); err != nil || legacy.Legacy == nil || legacy.Legacy.Hash != key.Hash || legacy.Hash != key.Hash {
		t.Fatalf("legacy record: %v", err)
	}
}

// The key goes into world.json as its last section; the rest of the file is byte for byte what it was.
func TestWithSaveKeyAppendsTheSection(t *testing.T) {
	world, err := json.MarshalIndent(testKeyWorld(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	key := testKey(t, testKeyWorld())
	out, err := withSaveKey(world, key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), string(world[:len(world)-2])) {
		t.Fatal("world.json changed before the key")
	}
	var back struct {
		SaveKey SaveKeyCatalog `json:"save_key"`
		Life    json.RawMessage
	}
	if err := json.Unmarshal(out, &back); err != nil || back.SaveKey.Hash != key.Hash || len(back.Life) == 0 {
		t.Fatalf("world.json with the key: %v", err)
	}
	sections := map[string]json.RawMessage{}
	if err := json.Unmarshal(out, &sections); err != nil {
		t.Fatal(err)
	}
	sections["barks_catalog"] = keySections(t, testKeyWorld())["barks_catalog"]
	if again, err := computeSaveKey(testKeyPolicy(t, keySections(t, testKeyWorld())), sections); err != nil || again.Hash != key.Hash {
		t.Fatalf("the key over a world.json that carries it: %v", err)
	}
	if _, err := withSaveKey([]byte(`[]`), key); err == nil {
		t.Fatal("not an object")
	}
}

// A strict part with each is hashed per item by its id (saves growth, owner 20.11): one more item, even before the
// others in the list, changes no item's hash; a changed item changes its own only. The part as one stays for records of
// the earlier form.
func TestSaveKeyStrictPartPerItem(t *testing.T) {
	sections := map[string]json.RawMessage{
		"life": json.RawMessage(`{"actors":[{"id":"char.a","steps":[{"activity":"work"},{"activity":"sleep"}]},{"id":"char.b","steps":[{"activity":"eat"}]}]}`),
	}
	policy := &SaveKeyPolicy{Sections: map[string]SaveKeyRule{"life": {Key: saveKeyChecked, Reason: "x"}},
		Parts: []SaveKeyPart{{Path: "life/actors/*/steps/*/activity", Key: saveKeyStrict, Each: "id", Reason: "positions per actor"}}}
	base, err := computeSaveKey(policy, sections)
	if err != nil {
		t.Fatal(err)
	}
	a, b, whole := "life/actors/*/steps/*/activity[char.a]", "life/actors/*/steps/*/activity[char.b]", "life/actors/*/steps/*/activity"
	if base.Parts[a] == "" || base.Parts[b] == "" || base.Parts[whole] == "" || len(base.Strict) != 0 || len(base.StrictEach) != 1 {
		t.Fatalf("per item %+v", base)
	}
	sections["life"] = json.RawMessage(`{"actors":[{"id":"char.new","steps":[{"activity":"post"}]},{"id":"char.a","steps":[{"activity":"work"},{"activity":"sleep"}]},{"id":"char.b","steps":[{"activity":"eat"}]}]}`)
	grown, err := computeSaveKey(policy, sections)
	if err != nil {
		t.Fatal(err)
	}
	if grown.Parts[a] != base.Parts[a] || grown.Parts[b] != base.Parts[b] || grown.Parts["life/actors/*/steps/*/activity[char.new]"] == "" || grown.Parts[whole] == base.Parts[whole] {
		t.Fatal("one more actor changed the others' hashes, or the part as one did not change")
	}
	sections["life"] = json.RawMessage(`{"actors":[{"id":"char.a","steps":[{"activity":"sleep"},{"activity":"work"}]},{"id":"char.b","steps":[{"activity":"eat"}]}]}`)
	moved, err := computeSaveKey(policy, sections)
	if err != nil || moved.Parts[a] == base.Parts[a] || moved.Parts[b] != base.Parts[b] {
		t.Fatalf("reordered steps of one actor: %v", err)
	}
	for name, life := range map[string]string{
		"an item without its id": `{"actors":[{"steps":[{"activity":"work"}]}]}`,
		"an id twice":            `{"actors":[{"id":"char.a","steps":[]},{"id":"char.a","steps":[]}]}`,
	} {
		sections["life"] = json.RawMessage(life)
		if _, err := computeSaveKey(policy, sections); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	sections["life"] = json.RawMessage(`{"actors":[]}`)
	for name, part := range map[string]SaveKeyPart{
		"each without a wildcard": {Path: "life/actors", Key: saveKeyStrict, Each: "id", Reason: "x"},
		"each on an out part":     {Path: "life/actors", Key: saveKeyOut, Reads: saveReadsNone, Each: "id", Reason: "x"},
	} {
		p := &SaveKeyPolicy{Sections: policy.Sections, Parts: []SaveKeyPart{part}}
		if err := p.validate(); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	p := &SaveKeyPolicy{Sections: policy.Sections, Legacy: &SaveKeyRecord{Hash: base.Hash, Parts: map[string]string{"life": base.Parts["life"]}, Aliases: []string{base.Hash}}}
	if err := p.validate(); err == nil {
		t.Fatal("a legacy record that is its own alias")
	}
	p.Legacy.Aliases = []string{"not-a-hash"}
	if err := p.validate(); err == nil {
		t.Fatal("a legacy alias that is no hash")
	}
}
