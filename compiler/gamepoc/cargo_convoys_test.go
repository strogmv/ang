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
