package gamepoc

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func atlasFixture(t *testing.T) (string, CharacterArt) {
	t.Helper()
	root := t.TempDir()
	folder := filepath.Join(root, "Assets/Resources/Characters/Reference")
	if e := os.MkdirAll(folder, 0755); e != nil {
		t.Fatal(e)
	}
	f, e := os.Create(filepath.Join(folder, "atlas.png"))
	if e != nil {
		t.Fatal(e)
	}
	if e = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 1024, 1536))); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	s := CharacterSheet{Image: "Characters/Reference/atlas", FrameW: 256, FrameH: 384, DrawSize: 64, FeetY: 360, HeightMM: 1750, BodyHeightPX: 320, Idle: 0, Walk: []int{1, 2, 3}, Rows: []int{0, 1, 1, 2, 3, 3, 2, 0}, SmoothFilter: true}
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			s.Frames = append(s.Frames, CharacterFrame{Row: row, Column: col, X: col*256 + 5, Y: row*384 + 4, W: 245, H: 375, FeetX: 120 + col, FeetY: 360 - row, BodyHeightPX: 320 - row})
		}
	}
	return root, CharacterArt{FPS: 8, DefaultSheets: []string{"adult"}, Sheets: map[string]CharacterSheet{"adult": s}}
}
func TestCharacterExplicitAtlasAndDefaultProjection(t *testing.T) {
	root, a := atlasFixture(t)
	if e := checkCharacterImages(root, a); e != nil {
		t.Fatal(e)
	}
	data, e := json.Marshal(clientCharacters(a))
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		DefaultSheets []string `json:"default_sheets"`
		Sheets        []struct {
			Frames []CharacterFrame `json:"frames"`
			Smooth bool             `json:"smooth_filter"`
		}
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	if len(got.DefaultSheets) != 1 || len(got.Sheets[0].Frames) != 16 || !got.Sheets[0].Smooth || got.Sheets[0].Frames[1].FeetX != 121 {
		t.Fatal(string(data))
	}
}
func TestCharacterExplicitAtlasRejectsInvalidFrames(t *testing.T) {
	root, a := atlasFixture(t)
	original := a.Sheets["adult"]
	for _, name := range []string{"duplicate", "missing", "outside", "feet", "body", "negative", "idle_image", "unknown_default", "duplicate_default"} {
		t.Run(name, func(t *testing.T) {
			s := original
			s.Frames = append([]CharacterFrame(nil), original.Frames...)
			a.DefaultSheets = []string{"adult"}
			switch name {
			case "duplicate":
				s.Frames[1] = s.Frames[0]
			case "missing":
				s.Frames = s.Frames[1:]
			case "outside":
				s.Frames[0].X = 1024
			case "feet":
				s.Frames[0].FeetX = 246
			case "body":
				s.Frames[0].BodyHeightPX = 361
			case "negative":
				s.Frames[0].Row = -1
			case "idle_image":
				s.IdleImage = s.Image
			case "unknown_default":
				a.DefaultSheets = []string{"missing"}
			case "duplicate_default":
				a.DefaultSheets = []string{"adult", "adult"}
			}
			a.Sheets["adult"] = s
			if e := checkCharacterImages(root, a); e == nil {
				t.Fatal("invalid atlas accepted")
			}
		})
	}
}
func TestHudPortraitPathPNGAndProjection(t *testing.T) {
	root, a := atlasFixture(t)
	_ = a
	h := &HudArt{HeroPortrait: "Characters/Reference/atlas"}
	if e := checkHudImage(root, h); e != nil {
		t.Fatal(e)
	}
	if clientArt(Compiled{Art: Art{HUD: h}})["hud"] != h {
		t.Fatal("missing projection")
	}
	for _, path := range []string{"../secret", "/tmp/file", "UI/missing", "UI/x.png", "UI//x"} {
		if e := checkHudImage(root, &HudArt{HeroPortrait: path}); e == nil {
			t.Fatal(path)
		}
	}
	p := filepath.Join(root, "Assets/Resources/notpng.png")
	if e := os.WriteFile(p, []byte("invalid"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := checkHudImage(root, &HudArt{HeroPortrait: "notpng"}); e == nil {
		t.Fatal("invalid PNG accepted")
	}
	if e := checkHudImage(root, nil); e != nil {
		t.Fatal(e)
	}
}
