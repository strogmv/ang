package locationsdk

import (
	"strings"
	"testing"
)

func TestInteriorRejectsUndeclaredDrop(t *testing.T) {
	l := testLayouts()[0]
	l.Interior = &InteriorEnvelope{Envelope: Rect{0, 0, 14, 10}}
	l.Floor = []FloorArea{{ID: "west", Rect: Rect{0, 0, 13, 10}, Reason: "floor stops before the east wall"}}
	_, err := Build(testCatalog(), l)
	if err == nil || !strings.Contains(err.Error(), "undeclared_interior_drop") {
		t.Fatalf("negative interior fixture accepted: %v", err)
	}
	l.Interior.Voids = []InteriorVoid{{ID: "rail", Kind: "track", Rect: Rect{13, 0, 1, 10}, Reason: "declared inaccessible railway"}}
	requireBuilt(t, testCatalog(), l)
	l.Floor = append(l.Floor, FloorArea{ID: "mistake", Rect: Rect{13, 0, 1, 1}, Reason: "track must not become walking floor"})
	_, err = Build(testCatalog(), l)
	if err == nil || !strings.Contains(err.Error(), "interior_void_supported") {
		t.Fatalf("walkable track accepted: %v", err)
	}
}

func TestSecondInteriorLayoutNeedsOnlyData(t *testing.T) {
	l := testLayouts()[1]
	l.Floor = []FloorArea{{ID: "hall", Rect: Rect{0, 0, 16, 12}, Reason: "continuous interior floor"}}
	l.Interior = &InteriorEnvelope{Envelope: Rect{0, 0, 16, 12}}
	requireBuilt(t, testCatalog(), l)
}

func TestDomeInteriorRequiresWallEnvelope(t *testing.T) {
	l := testLayouts()[0]
	l.Setting = "dome_interior"
	_, err := Build(testCatalog(), l)
	if err == nil || !strings.Contains(err.Error(), "interior_envelope_required") {
		t.Fatalf("missing envelope accepted: %v", err)
	}
}
