package gamepoc

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrainMotionExternalImagesAndLegacyPNG(t *testing.T) {
	unity := t.TempDir()
	writePNG := func(name string) {
		t.Helper()
		path := filepath.Join(unity, "Assets/Resources", name+".png")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 8, 8)))
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	writePNG("MarketArt/hub")
	scene := PaintedScene{LevelID: "level.c7_transport_hub", Width: 1672, Height: 941, Background: "MarketArt/hub", AmbientPassages: []PaintedAmbientPassage{{ID: "hub.lower_freight_rail", MotionSource: "train_motion"}}}
	if err := checkPaintedImages(unity, []PaintedScene{scene}); err != nil {
		t.Fatalf("geometry-only external passage demanded PNG: %v", err)
	}
	scene.AmbientPassages = append(scene.AmbientPassages, PaintedAmbientPassage{ID: "passenger", Image: "MarketArt/passenger", SourceRect: &ArtSceneSourceRect{W: 8, H: 8}})
	if err := checkPaintedImages(unity, []PaintedScene{scene}); err == nil {
		t.Fatal("legacy PNG was no longer required")
	}
	writePNG("MarketArt/passenger")
	for _, mode := range []string{"", "calendar"} {
		scene.AmbientPassages[1].MotionSource = mode
		if err := checkPaintedImages(unity, []PaintedScene{scene}); err != nil {
			t.Fatal(err)
		}
	}
	scene.AmbientPassages[1].SourceRect.W = 9
	if err := checkPaintedImages(unity, []PaintedScene{scene}); err == nil || !strings.Contains(err.Error(), "source rectangle") {
		t.Fatalf("legacy crop validation bypassed: %v", err)
	}
	scene.AmbientPassages[1].SourceRect.W = 8
	scene.AmbientPassages[1].Track = &PaintedAmbientTrack{Image: "MarketArt/track", SourceRect: &ArtSceneSourceRect{W: 8, H: 8}}
	if err := checkPaintedImages(unity, []PaintedScene{scene}); err == nil {
		t.Fatal("legacy track PNG was no longer required")
	}
	writePNG("MarketArt/track")
	if err := checkPaintedImages(unity, []PaintedScene{scene}); err != nil {
		t.Fatal(err)
	}
}

// Explicit integration gate: Generate performs asset validation and writes ALL
// normal outputs. Opt in only with a disposable *-stage project, never ROOT.
func TestTrainMotionFullGeneration(t *testing.T) {
	project := os.Getenv("ANG_TRAIN_MOTION_GENERATE_PROJECT")
	if project == "" {
		t.Skip("set ANG_TRAIN_MOTION_GENERATE_PROJECT to a writable disposable stage/server")
	}
	absolute, err := filepath.Abs(project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(absolute), "-stage/") {
		t.Fatal("full generation test requires an explicitly isolated -stage project")
	}
	if err = Generate(absolute); err != nil {
		t.Fatalf("complete Generate, including authored image consumers: %v", err)
	}
	worldRaw, err := os.ReadFile(filepath.Join(absolute, "internal/poc/world.json"))
	if err != nil {
		t.Fatal(err)
	}
	var world Compiled
	if err = json.Unmarshal(worldRaw, &world); err != nil {
		t.Fatal(err)
	}
	if world.TrainMotion == nil {
		t.Fatal("generated server lost train catalog")
	}
	bindings := world.TrainMotion.Bindings
	if len(bindings) != 3 || bindings[1].LevelID != "level.c7_transport_hub" || bindings[1].PassageID != "hub.lower_freight_rail" {
		t.Fatal("generated server retained old station route")
	}
	clientRaw, err := os.ReadFile(filepath.Join(absolute, world.Config.UnityDir, "Assets/Resources/PocConfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	var client struct {
		ContentHash string `json:"content_hash"`
		Art         struct {
			Painted []PaintedScene `json:"painted"`
		} `json:"art"`
	}
	if err = json.Unmarshal(clientRaw, &client); err != nil {
		t.Fatal(err)
	}
	if len(client.ContentHash) != 64 {
		t.Fatal("generated client has no content hash")
	}
	hub, station := false, false
	for _, scene := range client.Art.Painted {
		for _, p := range scene.AmbientPassages {
			if p.ID == "hub.lower_freight_rail" {
				hub = p.MotionSource == "train_motion" && p.Image == "" && p.MetricRail != nil
			}
			if p.ID == "station.back_rail" {
				station = p.MotionSource != "train_motion" && p.Image != "" && p.DwellSeconds == 20 && len(p.BerthDoors) == 2
			}
		}
	}
	if !hub || !station {
		t.Fatal("full generated resource lost external geometry or legacy passenger PNG/berth")
	}
}
