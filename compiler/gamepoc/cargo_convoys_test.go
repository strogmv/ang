package gamepoc

import (
	"math"
	"testing"
)

func TestCargoFilletRoundsAUTurnAndRejectsShortLegs(t *testing.T) {
	// A U-turn between two lanes 6 m apart with a 3 m radius is one semicircle.
	track, err := cargoFillet([][2]float64{{13.5, 20}, {13.5, 5.5}, {19.5, 5.5}, {19.5, 13}}, 3)
	if err != nil {
		t.Fatal(err)
	}
	length := 0.0
	for i := 1; i < len(track); i++ {
		step := math.Hypot(track[i][0]-track[i-1][0], track[i][1]-track[i-1][1])
		if step > 0.4+11.5 {
			t.Fatalf("gap %v", step)
		}
		length += step
		// Every point of the arc keeps the radius from the turn centre (16.5, 8.5).
		if track[i][1] < 8.5-1e-6 && math.Abs(math.Hypot(track[i][0]-16.5, track[i][1]-8.5)-3) > 1e-4 {
			t.Fatalf("point %v is off the 3 m arc", track[i])
		}
	}
	want := 11.5 + math.Pi*3 + 4.5
	if math.Abs(length-want) > 0.02 {
		t.Fatalf("length %.3f, want %.3f", length, want)
	}
	if track[0] != [2]float64{13.5, 20} || track[len(track)-1] != [2]float64{19.5, 13} {
		t.Fatal("ends moved")
	}
	if _, err = cargoFillet([][2]float64{{0, 0}, {0, 2}, {6, 2}}, 3); err == nil {
		t.Fatal("a leg shorter than the radius was accepted")
	}
	if _, err = cargoFillet([][2]float64{{0, 0}, {0, 5}, {0, 1}}, 3); err == nil {
		t.Fatal("a reversing track was accepted")
	}
	straight, err := cargoFillet([][2]float64{{0, 0}, {0, 5}}, 3)
	if err != nil || len(straight) != 2 {
		t.Fatal("a straight track must stay two points")
	}
}

func TestLiftingDeckSideNamesTheOpenLongSide(t *testing.T) {
	type rect = struct{ X, Y, W, H int }
	deck := rect{86, 4, 24, 16}
	if !liftingDeckSide("south", deck, rect{86, 20, 24, 1}, 23) || liftingDeckSide("north", deck, rect{86, 20, 24, 1}, 23) {
		t.Fatal("south barrier misread")
	}
	if !liftingDeckSide("north", rect{21, 35, 24, 16}, rect{21, 34, 24, 1}, 31) || liftingDeckSide("north", rect{21, 35, 24, 16}, rect{21, 34, 24, 1}, 36) {
		t.Fatal("north barrier or approach misread")
	}
	if liftingDeckSide("south", deck, rect{87, 20, 22, 1}, 23) {
		t.Fatal("a barrier narrower than the deck was accepted")
	}
}

// A body occupies the cells whose middles lie inside its rectangle: a tug 2.89 m long and 1.26 m wide standing along a
// row covers six columns by three rows of 0.5 m cells (12.12, cargo to the market v1).
func TestCargoRectCellsCoverTheBodyByCellMiddles(t *testing.T) {
	cells := map[[2]int]bool{}
	cargoRectCells(CargoPose{X: 8.612, Y: 11.3, YawDegrees: 0}, 0.952, 1.938, 0.63, 0.5, func(x, y int) { cells[[2]int{x, y}] = true })
	for x := 16; x <= 20; x++ {
		for y := 21; y <= 23; y++ {
			if !cells[[2]int{x, y}] {
				t.Fatalf("cell (%d,%d) under the tug is not part of its body", x, y)
			}
		}
	}
	if cells[[2]int{21, 22}] || cells[[2]int{17, 20}] || cells[[2]int{17, 24}] {
		t.Fatal("a cell whose middle lies outside the tug was counted")
	}
	turned := map[[2]int]bool{}
	cargoRectCells(CargoPose{X: 11.3, Y: 8.612, YawDegrees: 90}, 0.952, 1.938, 0.63, 0.5, func(x, y int) { turned[[2]int{x, y}] = true })
	if len(turned) != len(cells) {
		t.Fatalf("the same body turned a quarter covers %d cells, not %d", len(turned), len(cells))
	}
}

// The reverse leg's pose looks against the motion; a hitched trailer follows one drawbar behind on the same track.
func TestCargoPosesFollowTheTrackAndBackUp(t *testing.T) {
	var v CargoConvoy
	v.Tug.HitchBackM, v.Trailer.EyeM = 0.902, 2.68
	tug, trailer := v.cargoPoses([][2]float64{{0, 0}, {10, 0}}, 5, false)
	if math.Abs(tug.X-5) > 1e-9 || cargoTurn(tug.YawDegrees, 0) > 1e-6 || math.Abs(trailer.X-(5-0.902-2.68)) > 1e-9 || cargoTurn(trailer.YawDegrees, 0) > 1e-6 {
		t.Fatalf("forward poses %+v %+v", tug, trailer)
	}
	back, _ := v.cargoPoses([][2]float64{{0, 0}, {10, 0}}, 5, true)
	if cargoTurn(back.YawDegrees, 180) > 1e-6 {
		t.Fatalf("a tug backing east must look west, looks %.1f", back.YawDegrees)
	}
}

func TestCargoCraneCarriesClearOfAParkedTrailer(t *testing.T) {
	b1, dp, b2 := [2]float64{5.67, 11.3}, [2]float64{5.9, 13.5}, [2]float64{6.33, 15.8}
	// B1 -> B2 passes over the trailer on the departure place: 1.5 m leaves the load against its 2.05 m roof
	if cargoCraneClears(b1, b2, dp, 2.55, 1.5, 2.05) {
		t.Fatal("a load 1.5 m up passed over a 2.05 m trailer")
	}
	if !cargoCraneClears(b1, b2, dp, 2.55, 2.3, 2.05) {
		t.Fatal("a load above the roof with clearance was refused")
	}
	// a way that keeps a body length off the parked trailer needs no height
	if !cargoCraneClears(b1, [2]float64{5.67, 8.0}, b2, 2.55, 0.5, 2.05) {
		t.Fatal("a way away from the parked trailer was refused")
	}
}
