package gamepoc

import (
	"encoding/json"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func artSceneFixture() ArtScene {
	return ArtScene{ID: "market", Title: "Market", Subtitle: "Visual assembly", Background: "MarketArt/background", Width: 2048, Height: 1143,
		Layers:   []ArtSceneLayer{{ID: "person", Image: "MarketArt/people", X: 300, Y: 450, Width: 70, Height: 190, Z: 10, SourceRect: &ArtSceneSourceRect{X: 0, Y: 0, W: 64, H: 128}, Label: "Worker", Description: "Inspect costume and scale."}},
		Hotspots: []ArtSceneHotspot{{ID: "shop", X: 700, Y: 300, Width: 100, Height: 100, Label: "Shop", Description: "A display capsule."}}}
}
func TestArtSceneValidation(t *testing.T) {
	if err := checkArtScenes(nil); err != nil {
		t.Fatal(err)
	}
	if err := checkArtScenes([]ArtScene{artSceneFixture()}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"canvas", "background", "traversal", "extension", "backslash", "layer_id", "hotspot_id", "bounds", "nan", "infinity", "zero_size", "depth", "crop", "label", "duplicate_scene"} {
		t.Run(name, func(t *testing.T) {
			s := artSceneFixture()
			scenes := []ArtScene{s}
			switch name {
			case "canvas":
				s.Width = 4097
			case "background":
				s.Background = "Other/bg"
			case "traversal":
				s.Layers[0].Image = "MarketArt/../secret"
			case "extension":
				s.Layers[0].Image = "MarketArt/people.png"
			case "backslash":
				s.Layers[0].Image = "MarketArt/a\\b"
			case "layer_id":
				s.Layers = append(s.Layers, s.Layers[0])
			case "hotspot_id":
				s.Hotspots[0].ID = s.Layers[0].ID
			case "bounds":
				s.Layers[0].X = 2040
			case "nan":
				s.Layers[0].X = math.NaN()
			case "infinity":
				s.Hotspots[0].Height = math.Inf(1)
			case "zero_size":
				s.Layers[0].Width = 0
			case "depth":
				s.Layers[0].Z = 100001
			case "crop":
				s.Layers[0].SourceRect.W = 0
			case "label":
				s.Layers[0].Description = ""
			case "duplicate_scene":
				scenes = append(scenes, s)
			}
			scenes[0] = s
			if err := checkArtScenes(scenes); err == nil {
				t.Fatal("accepted invalid art scene")
			}
		})
	}
}
func TestArtSceneImagesAndAtlasBounds(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Assets/Resources/MarketArt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, w, h int) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, w, h)))
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	s := artSceneFixture()
	if err := checkArtSceneImages(root, []ArtScene{s}); err == nil {
		t.Fatal("missing images accepted")
	}
	write("background", 256, 128)
	write("people", 128, 128)
	if err := checkArtSceneImages(root, []ArtScene{s}); err != nil {
		t.Fatal(err)
	}
	s.Layers[0].SourceRect.X = 100
	if err := checkArtSceneImages(root, []ArtScene{s}); err == nil {
		t.Fatal("atlas overflow accepted")
	}
	s.Layers[0].SourceRect = nil
	if err := checkArtSceneImages(root, []ArtScene{s}); err != nil {
		t.Fatal(err)
	}
	write("people", 4097, 1)
	if err := checkArtSceneImages(root, []ArtScene{s}); err == nil {
		t.Fatal("oversized texture accepted")
	}
}
func TestArtSceneProjectionStableLayerOrder(t *testing.T) {
	s := artSceneFixture()
	s.Layers[0].Z = 2
	b := s.Layers[0]
	b.ID = "b"
	b.Z = 1
	a := b
	a.ID = "a"
	s.Layers = append(s.Layers, b, a)
	got := clientArtScenes([]ArtScene{s})
	if got[0].Layers[0].ID != "a" || got[0].Layers[1].ID != "b" || got[0].Layers[2].ID != "person" {
		t.Fatal("layer order unstable")
	}
	if s.Layers[0].ID != "person" {
		t.Fatal("projection mutated source")
	}
	data, err := json.Marshal(clientArt(Compiled{Art: Art{Scenes: []ArtScene{s}}}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"source_rect":{"x":0,"y":0,"w":64,"h":128}`) || !strings.Contains(string(data), `"scenes":[`) {
		t.Fatal("missing client scene/crop")
	}
}
func TestArtSceneTemplateIsPresentationOnly(t *testing.T) {
	data, err := sources.ReadFile("templates/art_scene.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"DrawArtPreviewHud(artScene,label,description)", "--poc-art-preview", "--poc-art-capture", "mode=visual_assembly", "CaptureScreenshotAsTexture()", "1f-(float)(crop.y+crop.h)/texture.height", "rect.Contains(Event.current.mousePosition)", "FilterMode.Bilinear", "while(!UnityEngine.Rendering.SplashScreen.isFinished)", "GUI.color=Color.white;GUI.contentColor=Color.white;GUI.enabled=true;"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing contract %q", want)
		}
	}
	for _, bad := range []string{"ClickMove(", "session.Send", "state.Player", "Time.timeScale"} {
		if strings.Contains(s, bad) {
			t.Errorf("visual preview invokes gameplay %q", bad)
		}
	}
	start := sceneTemplateSection(t, "unity.cs.tmpl", "        async void Start()", "        string Pref(")
	if strings.Index(start, "if (TryStartArtPreview()) return;") > strings.Index(start, "host = Process.Start(start);") {
		t.Fatal("art CLI starts server")
	}
	importer, err := sources.ReadFile("templates/editor.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Assets/Resources/MarketArt/", "t.maxTextureSize=4096", "t.mipmapEnabled=true", "FilterMode.Bilinear", "TextureImporterCompression.Uncompressed"} {
		if !strings.Contains(string(importer), want) {
			t.Errorf("missing art importer contract %q", want)
		}
	}
}
