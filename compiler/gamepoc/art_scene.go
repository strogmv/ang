package gamepoc

import (
	"fmt"
	"image"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ArtScene is a visual assembly, never a simulation map or an observation.
// All destination and source rectangles use a top-left origin in canvas pixels.
type ArtScene struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Subtitle   string            `json:"subtitle"`
	Background string            `json:"background"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Layers     []ArtSceneLayer   `json:"layers"`
	Hotspots   []ArtSceneHotspot `json:"hotspots"`
}
type ArtSceneSourceRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}
type ArtSceneLayer struct {
	ID          string              `json:"id"`
	Image       string              `json:"image"`
	X           float64             `json:"x"`
	Y           float64             `json:"y"`
	Width       float64             `json:"width"`
	Height      float64             `json:"height"`
	Z           int                 `json:"z"`
	SourceRect  *ArtSceneSourceRect `json:"source_rect,omitempty"`
	Label       string              `json:"label,omitempty"`
	Description string              `json:"description,omitempty"`
}
type ArtSceneHotspot struct {
	ID          string  `json:"id"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
}

func artSceneImagePath(s string) bool {
	return strings.HasPrefix(s, "MarketArt/") && s != "MarketArt/" && path.Clean(s) == s && !strings.ContainsAny(s, "\\:\x00") && !strings.Contains(s, "..") && path.Ext(s) == ""
}
func artSceneRect(x, y, w, h float64, width, height int) bool {
	for _, v := range []float64{x, y, w, h} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return x >= 0 && y >= 0 && w > 0 && h > 0 && x+w <= float64(width) && y+h <= float64(height)
}
func checkArtScenes(scenes []ArtScene) error {
	if len(scenes) > 32 {
		return fmt.Errorf("art.scenes: at most 32 scenes")
	}
	ids := map[string]bool{}
	for _, s := range scenes {
		if strings.TrimSpace(s.ID) == "" || ids[s.ID] || strings.TrimSpace(s.Title) == "" {
			return fmt.Errorf("art.scenes: empty/duplicate identity or title %q", s.ID)
		}
		ids[s.ID] = true
		if s.Width < 1 || s.Width > 4096 || s.Height < 1 || s.Height > 4096 || !artSceneImagePath(s.Background) {
			return fmt.Errorf("art scene %s: invalid canvas/background", s.ID)
		}
		if len(s.Layers) > 1024 || len(s.Hotspots) > 512 {
			return fmt.Errorf("art scene %s: too many layers/hotspots", s.ID)
		}
		members := map[string]bool{}
		for _, l := range s.Layers {
			if strings.TrimSpace(l.ID) == "" || members[l.ID] {
				return fmt.Errorf("art scene %s: duplicate/empty layer %q", s.ID, l.ID)
			}
			members[l.ID] = true
			if !artSceneImagePath(l.Image) || !artSceneRect(l.X, l.Y, l.Width, l.Height, s.Width, s.Height) || l.Z < -100000 || l.Z > 100000 {
				return fmt.Errorf("art scene %s layer %s: invalid image/placement", s.ID, l.ID)
			}
			if (l.Label == "") != (l.Description == "") {
				return fmt.Errorf("art scene %s layer %s: label and description must be paired", s.ID, l.ID)
			}
			if r := l.SourceRect; r != nil && (r.X < 0 || r.Y < 0 || r.W < 1 || r.H < 1 || r.X > 4096-r.W || r.Y > 4096-r.H) {
				return fmt.Errorf("art scene %s layer %s: invalid source rectangle", s.ID, l.ID)
			}
		}
		for _, h := range s.Hotspots {
			if strings.TrimSpace(h.ID) == "" || members[h.ID] || strings.TrimSpace(h.Label) == "" || strings.TrimSpace(h.Description) == "" || !artSceneRect(h.X, h.Y, h.Width, h.Height, s.Width, s.Height) {
				return fmt.Errorf("art scene %s: invalid hotspot %q", s.ID, h.ID)
			}
			members[h.ID] = true
		}
	}
	return nil
}
func clientArtScenes(scenes []ArtScene) []ArtScene {
	result := append([]ArtScene{}, scenes...)
	// Keep the author's scene selection order; layer depth has an explicit stable tie.
	for i := range result {
		result[i].Layers = append([]ArtSceneLayer{}, result[i].Layers...)
		sort.Slice(result[i].Layers, func(a, b int) bool {
			x, y := result[i].Layers[a], result[i].Layers[b]
			if x.Z != y.Z {
				return x.Z < y.Z
			}
			return x.ID < y.ID
		})
		result[i].Hotspots = append([]ArtSceneHotspot{}, result[i].Hotspots...)
	}
	return result
}
func checkArtSceneImages(unity string, scenes []ArtScene) error {
	if err := checkArtScenes(scenes); err != nil {
		return err
	}
	configs := map[string]image.Config{}
	read := func(name string) (image.Config, error) {
		if c, ok := configs[name]; ok {
			return c, nil
		}
		f, err := os.Open(filepath.Join(unity, "Assets/Resources", name+".png"))
		if err != nil {
			return image.Config{}, err
		}
		c, format, err := image.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			return c, err
		}
		if format != "png" || c.Width < 1 || c.Height < 1 || c.Width > 4096 || c.Height > 4096 {
			return c, fmt.Errorf("%s: PNG dimensions must be 1..4096", name)
		}
		configs[name] = c
		return c, nil
	}
	for _, s := range scenes {
		if _, err := read(s.Background); err != nil {
			return fmt.Errorf("art scene %s background: %w", s.ID, err)
		}
		for _, l := range s.Layers {
			c, err := read(l.Image)
			if err != nil {
				return fmt.Errorf("art scene %s layer %s: %w", s.ID, l.ID, err)
			}
			if r := l.SourceRect; r != nil && (r.X > c.Width-r.W || r.Y > c.Height-r.H) {
				return fmt.Errorf("art scene %s layer %s: source rectangle exceeds PNG", s.ID, l.ID)
			}
		}
	}
	return nil
}
