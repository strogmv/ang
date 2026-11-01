package gamepoc

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Read actual authoring and PNG dimensions without generating server/client
// output. Unit fixtures above remain independent of this optional checkout.
func TestStorefrontActualMarketBindings(t *testing.T) {
	project := os.Getenv("ANG_STOREFRONT_PROJECT")
	if project == "" {
		t.Skip("set ANG_STOREFRONT_PROJECT to the book server directory")
	}
	c, _, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	var market *PaintedScene
	for i := range c.Art.Painted {
		if c.Art.Painted[i].LevelID == "level.c7_market" {
			market = &c.Art.Painted[i]
		}
	}
	if market == nil || len(market.Storefronts) != 8 {
		t.Fatal("expected eight actual market storefronts")
	}
	book := filepath.Dir(project)
	var manifest struct {
		SourcePlateSize struct{ Width, Height float64 }   `json:"source_plate_size"`
		Entries         map[string]PaintedStorefrontLayer `json:"entries"`
		Fronts          []struct {
			ID         string                `json:"id"`
			SourceQuad []PaintedPoint        `json:"source_quad"`
			Goods      string                `json:"goods"`
			GoodsRect  PaintedStorefrontRect `json:"goods_rect"`
		} `json:"fronts"`
	}
	raw, err := os.ReadFile(filepath.Join(book, "design/market-stalls/front-kit-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fronts) != len(market.Storefronts) || manifest.SourcePlateSize.Width <= 0 || manifest.SourcePlateSize.Height <= 0 {
		t.Fatal("invalid calibrated art manifest")
	}
	for _, f := range market.Storefronts {
		found := false
		for _, author := range manifest.Fronts {
			if author.ID == f.ID {
				found = true
				if len(author.SourceQuad) != 4 {
					t.Fatal("invalid manifest facade")
				}
				for i, p := range author.SourceQuad {
					if math.Abs(f.Quad[i].X-p.X*float64(market.Width)/manifest.SourcePlateSize.Width) > .000001 || math.Abs(f.Quad[i].Y-p.Y*float64(market.Height)/manifest.SourcePlateSize.Height) > .000001 {
						t.Fatalf("%s drifted from reviewed facade calibration", f.ID)
					}
				}
				expected := manifest.Entries[author.Goods]
				if f.Goods.Image != expected.Image || f.Goods.SourceRect == nil || expected.SourceRect == nil || *f.Goods.SourceRect != *expected.SourceRect || f.Goods.Rect != author.GoodsRect {
					t.Fatalf("%s bound wrong goods atlas slice", f.ID)
				}
			}
		}
		if !found {
			t.Fatalf("unreviewed facade %s", f.ID)
		}
	}
	if err = checkPaintedImages(filepath.Join(book, "client-unity"), []PaintedScene{*market}); err != nil {
		t.Fatal(err)
	}
	t.Log("Actual CUE compiler and all eight calibrated facade/PNG bounds PASS; no Player result implied")
}

func storefrontFixture() (PaintedScene, PaintedStorefront) {
	layer := PaintedStorefrontLayer{Image: "MarketArt/Stalls/panel", SourceRect: &ArtSceneSourceRect{X: 10, Y: 10, W: 100, H: 100}, Rect: PaintedStorefrontRect{W: 1, H: 1}}
	return PaintedScene{LevelID: "test", Width: 256, Height: 256, Navigation: []string{"..", ".#"}}, PaintedStorefront{ID: "stall.a", MerchantID: "merchant", ObservationCell: Point{0, 0}, Quad: []PaintedPoint{{10, 20}, {110, 10}, {112, 70}, {12, 80}}, OpenSecond: 7 * 3600, CloseSecond: 22 * 3600, OpeningMS: 1600, ClosingMS: 1800, ClosureMode: "pack_away", Recess: layer, Shutter: layer, Goods: layer}
}

func TestStorefrontRejectsInvalidGeometryScheduleAndCoverage(t *testing.T) {
	check := func(s PaintedScene, fronts ...PaintedStorefront) error {
		return checkPaintedStorefronts(s, fronts, map[string]bool{".": true}, map[string]bool{"merchant": true})
	}
	s, f := storefrontFixture()
	if err := check(s, f); err != nil {
		t.Fatal(err)
	}
	f.OpenSecond, f.CloseSecond = 22*3600, 7*3600
	if err := check(s, f); err != nil {
		t.Fatalf("overnight opening should work: %v", err)
	}
	tests := []struct {
		name, match string
		mutate      func(*PaintedStorefront)
	}{
		{"private or missing merchant", "Life actor", func(f *PaintedStorefront) { f.MerchantID = "unbound" }},
		{"inside blocked body", "walkable approach", func(f *PaintedStorefront) { f.ObservationCell = Point{1, 1} }},
		{"bow tie", "convex", func(f *PaintedStorefront) { f.Quad[1], f.Quad[2] = f.Quad[2], f.Quad[1] }},
		{"NaN canvas", "outside canvas", func(f *PaintedStorefront) { f.Quad[1].X = math.NaN() }},
		{"outside canvas", "outside canvas", func(f *PaintedStorefront) { f.Quad[1].X = 300 }},
		{"undefined all day", "distinct", func(f *PaintedStorefront) { f.CloseSecond = f.OpenSecond }},
		{"slow unbounded transition", "durations", func(f *PaintedStorefront) { f.OpeningMS = 10001 }},
		{"unsupported policy", "closure_mode", func(f *PaintedStorefront) { f.ClosureMode = "delete_stock" }},
		{"uncovered closed edge", "complete opaque", func(f *PaintedStorefront) { f.Shutter.Rect.W = .99 }},
		{"uncovered original goods", "complete opaque", func(f *PaintedStorefront) { f.Recess.Rect.Y = .01 }},
		{"overflowing goods", "goods layer", func(f *PaintedStorefront) { f.Goods.Rect.X = .1 }},
		{"unsafe image", "goods layer", func(f *PaintedStorefront) { f.Goods.Image = "../escape" }},
		{"holiday without art", "holiday needs", func(f *PaintedStorefront) { f.HolidayWindows = []PaintedStorefrontWindow{{10, 20}} }},
		{"overlapping events", "ordered and disjoint", func(f *PaintedStorefront) {
			f.Festival = &f.Goods
			f.HolidayWindows = []PaintedStorefrontWindow{{10, 20}, {19, 22}}
		}},
		{"invalid event", "holiday needs", func(f *PaintedStorefront) {
			f.Festival = &f.Goods
			f.HolidayWindows = []PaintedStorefrontWindow{{20, 20}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, f := storefrontFixture()
			tc.mutate(&f)
			err := check(s, f)
			if err == nil || !strings.Contains(err.Error(), tc.match) {
				t.Fatalf("want %q, got %v", tc.match, err)
			}
		})
	}
	s, f = storefrontFixture()
	if err := check(s, f, f); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal("duplicate stable identity accepted", err)
	}
	other := f
	other.ID = "stall.b"
	if err := check(s, f, other); err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatal("two opaque facades on one surface accepted", err)
	}
	f.Festival = &f.Goods
	f.HolidayWindows = []PaintedStorefrontWindow{{10, 20}, {20, 30}}
	if err := check(s, f); err != nil {
		t.Fatal(err)
	}
}
