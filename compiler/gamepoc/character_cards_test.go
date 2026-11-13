package gamepoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cardFixture: a person with a card, the look the card names (a zone mask, a colourway in the card's colours, the card's
// height and caste, painted shading) and the portrait record.
func cardFixture(t *testing.T) (string, map[string]CharacterCard, Art, func(string) bool) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "art-source/characters/v1")
	if err := os.MkdirAll(filepath.Join(dir, "polina"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"looks": {"polina": {"file": "art-source/characters/v1/model/polina.glb", "sha256": "` + strings.Repeat("a", 64) + `",
		"surfaces": {"cloth": "M_cloth", "knit": "M_knit", "skin": "M_skin"},
		"zones": {"mask": "art-source/characters/v1/model/polina_zones.png", "mask_sha256": "` + strings.Repeat("b", 64) + `",
			"reference_srgb": {"cloth": "#5B666B", "knit": "#9A4632", "hair": "#3B2418"}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "look.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	portrait := `{"image": "art-source/characters/v1/polina/portrait.png", "sha256": "` + strings.Repeat("c", 64) + `", "source": "stand_in"}`
	if err := os.WriteFile(filepath.Join(dir, "polina/portrait.json"), []byte(portrait), 0o644); err != nil {
		t.Fatal(err)
	}
	h, painted := 1.66, "painted"
	a := Art{
		Looks: map[string]Look{"look.polina": {ID: "look.polina", Record: "art-source/characters/v1/look.json", Model: "polina", Caste: "bronze",
			Glow: []LookGlow{}, Colourway: "card", HeightM: &h, Shading: &painted,
			Colourways: map[string]LookColourway{"card": {Zones: map[string]string{"cloth": "#5B666B", "knit": "#9A4632", "hair": "#3B2418"}, Note: "the card"}}}},
		PersonLooks: map[string]string{"char.polina": "look.polina"},
	}
	cards := map[string]CharacterCard{"char.polina": {
		Person: "char.polina", Age: 22, Sex: "f", Build: "slim_average", HeightM: 1.66, Caste: "bronze", Job: "knits", Temperament: "quick",
		Silhouette: "a slim girl in a coverall", Skin: "#D6AC92", Hair: CardHair{Style: "a braid", Colour: "#3b2418"},
		ColourBlocks: []CardColourBlock{{Zone: "cloth", Colour: "#5b666b", What: "coverall"}, {Zone: "knit", Colour: "#9A4632", What: "knit"}},
		Face:         map[string]string{"nose": "straight", "brows": "straight", "cheekbones": "high", "eyes": "brown", "mouth": "wide", "marks": "freckles"},
		Clothing:     []CardPiece{{Piece: "coverall", What: "coverall", Block: "cloth", Reason: "the caste's uniform"}, {Piece: "knit_top", What: "a knit", Block: "knit", Reason: "cold"}},
		Accessories:  []CardPiece{{Piece: "reader", What: "a CRD reader", Reason: "payments"}},
		Look:         "look.polina", Portrait: "art-source/characters/v1/polina/portrait.json",
	}}
	return root, cards, a, func(id string) bool { return id == "char.polina" || id == "char.other" }
}

func TestCharacterCardsAccepted(t *testing.T) {
	root, cards, a, person := cardFixture(t)
	if err := checkLooks(root, a, person, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := checkCharacterCards(root, cards, a, person); err != nil {
		t.Fatal(err)
	}
	// without a portrait and without accessories: still a card
	c := cards["char.polina"]
	c.Portrait, c.Accessories = "", nil
	if err := checkCharacterCards(root, map[string]CharacterCard{"char.polina": c}, a, person); err != nil {
		t.Fatal(err)
	}
	// the look's row carries the shading
	looks, _ := clientLooks(a, nil)
	if len(looks) != 1 || looks[0]["shading"] != "painted" {
		t.Fatalf("shading not projected: %v", looks)
	}
}

func TestCharacterCardsRejected(t *testing.T) {
	cases := map[string]func(root string, c *CharacterCard, a *Art){
		"key is not the person": func(_ string, c *CharacterCard, _ *Art) { c.Person = "char.other" },
		"unknown person":        func(_ string, c *CharacterCard, a *Art) { c.Person = "char.nobody" },
		"too young":             func(_ string, c *CharacterCard, _ *Art) { c.Age = 12 },
		"sex":                   func(_ string, c *CharacterCard, _ *Art) { c.Sex = "x" },
		"build":                 func(_ string, c *CharacterCard, _ *Art) { c.Build = "superhero" },
		"height":                func(_ string, c *CharacterCard, _ *Art) { c.HeightM = 2.4 },
		"caste":                 func(_ string, c *CharacterCard, _ *Art) { c.Caste = "steel" },
		"no job":                func(_ string, c *CharacterCard, _ *Art) { c.Job = " " },
		"no silhouette":         func(_ string, c *CharacterCard, _ *Art) { c.Silhouette = "" },
		"no hair style":         func(_ string, c *CharacterCard, _ *Art) { c.Hair.Style = "" },
		"face part missing":     func(_ string, c *CharacterCard, _ *Art) { delete(c.Face, "mouth") },
		"bad hair colour":       func(_ string, c *CharacterCard, _ *Art) { c.Hair.Colour = "brown" },
		"bad skin colour":       func(_ string, c *CharacterCard, _ *Art) { c.Skin = "#12345" },
		"one block":             func(_ string, c *CharacterCard, _ *Art) { c.ColourBlocks = c.ColourBlocks[:1] },
		"four blocks": func(_ string, c *CharacterCard, _ *Art) {
			c.ColourBlocks = append(c.ColourBlocks, CardColourBlock{Zone: "apron", Colour: "#333333", What: "a"}, CardColourBlock{Zone: "boot", Colour: "#222222", What: "b"})
		},
		"block twice": func(_ string, c *CharacterCard, _ *Art) {
			c.ColourBlocks = []CardColourBlock{c.ColourBlocks[0], c.ColourBlocks[0]}
		},
		"block colour":       func(_ string, c *CharacterCard, _ *Art) { c.ColourBlocks[1].Colour = "red" },
		"block without what": func(_ string, c *CharacterCard, _ *Art) { c.ColourBlocks[1].What = "" },
		"no clothing":        func(_ string, c *CharacterCard, _ *Art) { c.Clothing = nil },
		"piece twice": func(_ string, c *CharacterCard, _ *Art) {
			c.Accessories = []CardPiece{{Piece: "coverall", What: "x", Reason: "y"}}
		},
		"piece without reason":   func(_ string, c *CharacterCard, _ *Art) { c.Clothing[1].Reason = "" },
		"piece bad name":         func(_ string, c *CharacterCard, _ *Art) { c.Clothing[1].Piece = "Knit Top" },
		"piece of unknown block": func(_ string, c *CharacterCard, _ *Art) { c.Clothing[1].Block = "apron" },
		"unknown look":           func(_ string, c *CharacterCard, _ *Art) { c.Look = "look.nobody" },
		"look not worn":          func(_ string, _ *CharacterCard, a *Art) { a.PersonLooks = map[string]string{} },
		"look of another caste":  func(_ string, c *CharacterCard, _ *Art) { c.Caste = "copper" },
		"look at another height": func(_ string, c *CharacterCard, _ *Art) { c.HeightM = 1.70 },
		"look without height": func(_ string, _ *CharacterCard, a *Art) {
			l := a.Looks["look.polina"]
			l.HeightM = nil
			a.Looks["look.polina"] = l
		},
		"block colour differs": func(_ string, c *CharacterCard, _ *Art) { c.ColourBlocks[1].Colour = "#9A4633" },
		"block zone not in look": func(_ string, c *CharacterCard, _ *Art) {
			c.ColourBlocks[1].Zone = "apron"
			c.Clothing[1].Block = "apron"
		},
		"hair colour differs":      func(_ string, c *CharacterCard, _ *Art) { c.Hair.Colour = "#000000" },
		"portrait outside":         func(_ string, c *CharacterCard, _ *Art) { c.Portrait = "../portrait.json" },
		"portrait missing":         func(_ string, c *CharacterCard, _ *Art) { c.Portrait = "art-source/characters/v1/polina/none.json" },
		"portrait not json record": func(_ string, c *CharacterCard, _ *Art) { c.Portrait = "art-source/characters/v1/polina/portrait.png" },
		"portrait bad source": func(root string, c *CharacterCard, _ *Art) {
			_ = os.WriteFile(filepath.Join(root, "art-source/characters/v1/polina/portrait.json"), []byte(`{"image": "art-source/characters/v1/polina/portrait.png", "sha256": "`+strings.Repeat("c", 64)+`", "source": "ai"}`), 0o644)
		},
		"portrait without sha": func(root string, c *CharacterCard, _ *Art) {
			_ = os.WriteFile(filepath.Join(root, "art-source/characters/v1/polina/portrait.json"), []byte(`{"image": "art-source/characters/v1/polina/portrait.png", "source": "painted"}`), 0o644)
		},
		"portrait image outside": func(root string, c *CharacterCard, _ *Art) {
			_ = os.WriteFile(filepath.Join(root, "art-source/characters/v1/polina/portrait.json"), []byte(`{"image": "/tmp/p.png", "sha256": "`+strings.Repeat("c", 64)+`", "source": "painted"}`), 0o644)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root, cards, a, person := cardFixture(t)
			c := cards["char.polina"]
			c.ColourBlocks = append([]CardColourBlock{}, c.ColourBlocks...)
			c.Clothing = append([]CardPiece{}, c.Clothing...)
			face := map[string]string{}
			for k, v := range c.Face {
				face[k] = v
			}
			c.Face = face
			mutate(root, &c, &a)
			if err := checkCharacterCards(root, map[string]CharacterCard{"char.polina": c}, a, person); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
}

func TestLooksShadingRejected(t *testing.T) {
	root, _, a, person := cardFixture(t)
	bad := "sprite"
	l := a.Looks["look.polina"]
	l.Shading = &bad
	a.Looks["look.polina"] = l
	if err := checkLooks(root, a, person, nil, nil); err == nil || !strings.Contains(err.Error(), "shading") {
		t.Fatalf("unknown shading accepted: %v", err)
	}
	// painted shading without a zone mask in the record
	if err := os.WriteFile(filepath.Join(root, "art-source/characters/v1/look.json"), []byte(`{"looks": {"polina": {"file": "art-source/characters/v1/model/polina.glb", "sha256": "`+
		strings.Repeat("a", 64)+`", "surfaces": {"cloth": "M_cloth"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	painted := "painted"
	l.Shading = &painted
	l.Colourway, l.Colourways = "", nil
	a.Looks["look.polina"] = l
	if err := checkLooks(root, a, person, nil, nil); err == nil || !strings.Contains(err.Error(), "zone mask") {
		t.Fatalf("painted shading without a zone mask accepted: %v", err)
	}
}
