package gamepoc

import (
	"encoding/json"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCharacterCatalogRejectsMissingOrInvalidFrames(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "Assets/Resources/Characters")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(folder, "test.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 160, 256))); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	for name, rect := range map[string]image.Rectangle{"idle": image.Rect(0, 0, 32, 256), "short_idle": image.Rect(0, 0, 32, 32), "bad_idle": image.Rect(0, 0, 33, 256)} {
		f, err := os.Create(filepath.Join(folder, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, image.NewNRGBA(rect)); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	s := CharacterSheet{Image: "Characters/test", FrameW: 32, FrameH: 32, DrawSize: 64, FeetY: 23, Idle: 0, Walk: []int{1, 2, 3, 4}, Rows: []int{0, 1, 2, 3, 4, 5, 6, 7}}
	a := CharacterArt{FPS: 8, RateVariation: .08, Sheets: map[string]CharacterSheet{"human": s}, Actors: map[string]string{"npc": "human"}}
	if err := checkCharacterImages(root, a); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing", "traversal", "frame", "row", "geometry", "height_without_pixels", "pixels_without_height", "body_below_feet", "negative_height", "missing_idle", "unsafe_idle", "idle_frame", "idle_row", "idle_geometry"} {
		t.Run(kind, func(t *testing.T) {
			bad := s
			bad.Walk = append([]int(nil), s.Walk...)
			bad.Rows = append([]int(nil), s.Rows...)
			switch kind {
			case "missing":
				bad.Image = "Characters/missing"
			case "traversal":
				bad.Image = "Characters/../test"
			case "frame":
				bad.Walk[0] = 5
			case "row":
				bad.Rows[0] = 8
			case "geometry":
				bad.FrameW = 33
			case "height_without_pixels":
				bad.HeightMM = 1750
			case "pixels_without_height":
				bad.BodyHeightPX = 18
			case "body_below_feet":
				bad.HeightMM = 1750
				bad.BodyHeightPX = 24
			case "negative_height":
				bad.HeightMM = -1
			case "missing_idle":
				bad.IdleImage = "Characters/missing"
			case "unsafe_idle":
				bad.IdleImage = "Characters/../test"
			case "idle_frame":
				bad.IdleImage = "Characters/idle"
				bad.Idle = 1
			case "idle_row":
				bad.IdleImage = "Characters/short_idle"
			case "idle_geometry":
				bad.IdleImage = "Characters/bad_idle"
			}
			a.Sheets["human"] = bad
			if err := checkCharacterImages(root, a); err == nil {
				t.Fatal("invalid sheet accepted")
			}
		})
	}
	physical := s
	physical.HeightMM = 1750
	physical.BodyHeightPX = 18
	physical.IdleImage = "Characters/idle"
	a.Sheets["human"] = physical
	if err := checkCharacterImages(root, a); err != nil {
		t.Fatalf("valid physical and idle metadata rejected: %v", err)
	}
	a.Sheets["human"] = s
	a.Actors["npc"] = "absent"
	if err := checkCharacterImages(root, a); err == nil {
		t.Fatal("unknown binding accepted")
	}
}

func TestCharacterPhysicalScaleAndIdleProjection(t *testing.T) {
	a := CharacterArt{Sheets: map[string]CharacterSheet{"adult": {Image: "Characters/walk", IdleImage: "Characters/idle", HeightMM: 1750, BodyHeightPX: 44, FrameW: 64, FrameH: 64, FeetY: 53}, "legacy": {DrawSize: 64}}}
	data, err := json.Marshal(clientCharacters(a))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Sheets []struct {
			ID     string `json:"id"`
			Height int    `json:"height_mm"`
			Body   int    `json:"body_height_px"`
			Idle   string `json:"idle_image"`
			Feet   int    `json:"feet_y"`
		}
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sheets) != 2 || got.Sheets[0].Height != 1750 || got.Sheets[0].Body != 44 || got.Sheets[0].Feet != 53 || got.Sheets[0].Idle != "Characters/idle" || got.Sheets[1].Height != 0 || got.Sheets[1].Body != 0 {
		t.Fatalf("physical projection changed metadata: %s", data)
	}
}
func TestPresentationCameraOptionalOrderedBoundsAndProjection(t *testing.T) {
	for _, c := range []PresentationCamera{{}, {ZoomMin: .7, ZoomDefault: 2.2, ZoomMax: 4.5}} {
		if err := checkPresentationCamera(c); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []PresentationCamera{{ZoomDefault: 2}, {ZoomMin: 2, ZoomDefault: 1, ZoomMax: 3}, {ZoomMin: .7, ZoomDefault: 4, ZoomMax: 3}, {ZoomMin: .1, ZoomDefault: 1, ZoomMax: 3}, {ZoomMin: .7, ZoomDefault: 1, ZoomMax: 7}, {ZoomMin: .7, ZoomDefault: math.NaN(), ZoomMax: 3}, {ZoomMin: .7, ZoomDefault: 1, ZoomMax: math.Inf(1)}} {
		if checkPresentationCamera(c) == nil {
			t.Fatalf("invalid camera accepted: %+v", c)
		}
	}
	c := Compiled{}
	c.Art.Camera = PresentationCamera{ZoomMin: .7, ZoomDefault: 2.2, ZoomMax: 4.5}
	data, err := json.Marshal(clientArt(c))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Camera PresentationCamera `json:"camera"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Camera != c.Art.Camera {
		t.Fatalf("camera projection changed bounds: %s", data)
	}
}
