package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Character cards (the game's design/characters-pipeline/v1/PIPELINE.md; content/world/character_cards.cue): one general card
// for any person of the world — who he is (age, sex, build, height, caste, job, temperament, from the world's data and the
// canon) and how he looks (a one-line silhouette, two or three colour blocks, clothing pieces each with a reason in the world,
// hair, skin, face features, accessories). The person's 3D look is built from the card (art-source/characters/v1), so the
// card and the look must agree: the look exists and the person wears it, its caste and body height are the card's, and the
// colour of each block and of the hair is the colour of that zone in the colourway the look is worn in (the large colour of the
// look comes from the card). The portrait (a record under art-source/: the image outside Git, its SHA-256 and whether it is
// the stand-in or the painting) is the dialogue portrait of the person; the client does not show it yet.

// CharacterCard is one world.character_cards row.
type CharacterCard struct {
	Person       string            `json:"person"`
	Age          int64             `json:"age"`
	Sex          string            `json:"sex"`
	Build        string            `json:"build"`
	HeightM      float64           `json:"height_m"`
	Caste        string            `json:"caste"`
	Job          string            `json:"job"`
	Temperament  string            `json:"temperament"`
	Silhouette   string            `json:"silhouette"`
	ColourBlocks []CardColourBlock `json:"colour_blocks"`
	Hair         CardHair          `json:"hair"`
	Skin         string            `json:"skin"`
	Face         map[string]string `json:"face"`
	Clothing     []CardPiece       `json:"clothing"`
	Accessories  []CardPiece       `json:"accessories"`
	Look         string            `json:"look"`
	Portrait     string            `json:"portrait,omitempty"`
	// MinorReason: why a person younger than an adult (ageMin) is a person of the world at all — at work or in the market (a
	// youth of 13..15: the school shift and an errand, an apprentice); required below ageMin, not allowed from it.
	MinorReason string `json:"minor_reason,omitempty"`
}

// CardColourBlock: the zone of the look's mask a block colours, its colour and what it is.
type CardColourBlock struct {
	Zone   string `json:"zone"`
	Colour string `json:"colour"`
	What   string `json:"what"`
}

// CardHair: the hairstyle and the hair's colour.
type CardHair struct {
	Style  string `json:"style"`
	Colour string `json:"colour"`
}

// CardPiece: a clothing piece or an accessory, the block it is coloured by (if any) and why the person wears it.
type CardPiece struct {
	Piece  string `json:"piece"`
	What   string `json:"what"`
	Block  string `json:"block,omitempty"`
	Reason string `json:"reason"`
}

// cardPortrait is the portrait record (game-relative .json under art-source/).
type cardPortrait struct {
	Image  string `json:"image"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
}

var (
	cardBuilds      = map[string]bool{"slight": true, "slim": true, "slim_average": true, "average": true, "sturdy": true, "heavy": true}
	cardSexes       = map[string]bool{"f": true, "m": true}
	cardFaceParts   = []string{"nose", "brows", "cheekbones", "eyes", "mouth", "marks"}
	cardSources     = map[string]bool{"stand_in": true, "painted": true}
	cardMinBlocks   = 2
	cardMaxBlocks   = 3
	cardHeightSlack = 0.005
	// cardMinorMin: the youngest person a card may describe (a youth at work or on an errand, with its reason in the world).
	cardMinorMin  = int64(13)
	cardPieceName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// checkCharacterCards validates world.character_cards against the persons, the looks and the game tree (root: the folder that
// holds art-source/).
func checkCharacterCards(root string, cards map[string]CharacterCard, a Art, person func(string) bool) error {
	for id, c := range cards {
		if !personID.MatchString(id) || c.Person != id {
			return fmt.Errorf("character_cards %q: the key must be a person id (char.…) and equal its person", id)
		}
		if !person(id) {
			return fmt.Errorf("character_cards %s: no such person in world.persons", id)
		}
		if c.Age < cardMinorMin || c.Age > ageMax {
			return fmt.Errorf("character_cards %s: age %d is not within %d..%d", id, c.Age, cardMinorMin, ageMax)
		}
		if c.Age < ageMin && strings.TrimSpace(c.MinorReason) == "" {
			return fmt.Errorf("character_cards %s: age %d is under %d: say in minor_reason why a youth is at work or in the market", id, c.Age, ageMin)
		}
		if c.Age >= ageMin && c.MinorReason != "" {
			return fmt.Errorf("character_cards %s: minor_reason is only for a person under %d", id, ageMin)
		}
		if !cardSexes[c.Sex] {
			return fmt.Errorf("character_cards %s: sex must be f or m, not %q", id, c.Sex)
		}
		if !cardBuilds[c.Build] {
			return fmt.Errorf("character_cards %s: unknown build %q", id, c.Build)
		}
		if math.IsNaN(c.HeightM) || c.HeightM < lookMinHeightM || c.HeightM > lookMaxHeightM {
			return fmt.Errorf("character_cards %s: height_m %g is not an adult's height (%g..%g m)", id, c.HeightM, lookMinHeightM, lookMaxHeightM)
		}
		if !lookCastes[c.Caste] {
			return fmt.Errorf("character_cards %s: unknown caste %q", id, c.Caste)
		}
		for field, v := range map[string]string{"job": c.Job, "temperament": c.Temperament, "silhouette": c.Silhouette, "hair.style": c.Hair.Style} {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("character_cards %s: %s is empty", id, field)
			}
		}
		for _, f := range cardFaceParts {
			if strings.TrimSpace(c.Face[f]) == "" {
				return fmt.Errorf("character_cards %s: face.%s is empty (say what it is, or \"нет\")", id, f)
			}
		}
		if !lookColour.MatchString(c.Hair.Colour) || !lookColour.MatchString(c.Skin) {
			return fmt.Errorf("character_cards %s: hair.colour and skin must be #RRGGBB", id)
		}
		if n := len(c.ColourBlocks); n < cardMinBlocks || n > cardMaxBlocks {
			return fmt.Errorf("character_cards %s: %d colour blocks; a card has %d..%d (Disco Elysium: a few big blocks)", id, n, cardMinBlocks, cardMaxBlocks)
		}
		blocks := map[string]string{}
		for _, b := range c.ColourBlocks {
			if !lookPlainName.MatchString(b.Zone) || !lookColour.MatchString(b.Colour) || strings.TrimSpace(b.What) == "" {
				return fmt.Errorf("character_cards %s: colour block %q needs a plain zone, a #RRGGBB colour and what it is", id, b.Zone)
			}
			if _, dup := blocks[b.Zone]; dup {
				return fmt.Errorf("character_cards %s: colour block %s twice", id, b.Zone)
			}
			blocks[b.Zone] = b.Colour
		}
		if len(c.Clothing) == 0 {
			return fmt.Errorf("character_cards %s: name the clothing pieces", id)
		}
		pieces := map[string]bool{}
		for _, p := range append(append([]CardPiece{}, c.Clothing...), c.Accessories...) {
			if !cardPieceName.MatchString(p.Piece) || strings.TrimSpace(p.What) == "" {
				return fmt.Errorf("character_cards %s: piece %q needs a plain name and what it is", id, p.Piece)
			}
			if pieces[p.Piece] {
				return fmt.Errorf("character_cards %s: piece %s twice", id, p.Piece)
			}
			pieces[p.Piece] = true
			if strings.TrimSpace(p.Reason) == "" {
				return fmt.Errorf("character_cards %s: piece %s has no reason in the world (doc 57)", id, p.Piece)
			}
			if p.Block != "" {
				if _, ok := blocks[p.Block]; !ok {
					return fmt.Errorf("character_cards %s: piece %s is coloured by %q, which is not a colour block of the card", id, p.Piece, p.Block)
				}
			}
		}
		// the look: exists, the person wears it, its caste, height and colours are the card's
		l, ok := a.Looks[c.Look]
		if !ok {
			return fmt.Errorf("character_cards %s: unknown look %q", id, c.Look)
		}
		if a.PersonLooks[id] != c.Look {
			return fmt.Errorf("character_cards %s: the person does not wear %s (art.person_looks)", id, c.Look)
		}
		if l.Caste != c.Caste {
			return fmt.Errorf("character_cards %s: look %s is of caste %s, the card of %s", id, c.Look, l.Caste, c.Caste)
		}
		if l.HeightM == nil || math.Abs(*l.HeightM-c.HeightM) > cardHeightSlack {
			return fmt.Errorf("character_cards %s: look %s must be drawn at the card's height %g m (height_m)", id, c.Look, c.HeightM)
		}
		cw, ok := l.Colourways[l.Colourway]
		if !ok {
			return fmt.Errorf("character_cards %s: look %s has no colourway it is worn in (a card's look needs a zone mask)", id, c.Look)
		}
		for zone, colour := range blocks {
			if got, ok := cw.Zones[zone]; !ok || !strings.EqualFold(got, colour) {
				return fmt.Errorf("character_cards %s: colour block %s is %s, but look %s colours zone %s %q", id, zone, colour, c.Look, zone, got)
			}
		}
		if got, ok := cw.Zones["hair"]; ok && !strings.EqualFold(got, c.Hair.Colour) {
			return fmt.Errorf("character_cards %s: the hair is %s, but look %s colours it %s", id, c.Hair.Colour, c.Look, got)
		}
		if c.Portrait != "" {
			if err := checkCardPortrait(root, id, c.Portrait); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkCardPortrait(root, id, record string) error {
	if !lookPath(record, ".json") {
		return fmt.Errorf("character_cards %s: portrait %q must be a .json record under art-source/", id, record)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(record)))
	if err != nil {
		return fmt.Errorf("character_cards %s: portrait: %w", id, err)
	}
	var p cardPortrait
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("character_cards %s: portrait %s: %w", id, record, err)
	}
	if !lookPath(p.Image, ".png") || !lookSHA.MatchString(p.SHA256) || !cardSources[p.Source] {
		return fmt.Errorf("character_cards %s: portrait %s needs a .png under art-source/, its SHA-256 and a source (stand_in or painted)", id, record)
	}
	return nil
}
