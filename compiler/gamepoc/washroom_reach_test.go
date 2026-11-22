package gamepoc

import (
	"strings"
	"testing"
)

// Plan 3 of the baseline fixes (21.11.2026): ANG lists the duty places from which the washroom on duty (there and back,
// the doors, the basin) cannot fit the brain's budget even with no queue.
func TestWashroomReachListsTheDutyPlacesTooFarForTenMinutes(t *testing.T) {
	row := "." + strings.Repeat(".", 98) + "."
	life := `{"move_ticks":3,"brain":{"hygiene_seconds":600},"locations":{"hall":{"terrain":["` + row + `"]}},"actors":[` +
		`{"id":"near","name":"Near","steps":[{"level":"hall","at":{"x":10,"y":0},"activity":"service"}]},` +
		`{"id":"far","name":"Far","steps":[{"level":"hall","at":{"x":90,"y":0},"activity":"service"},{"level":"hall","at":{"x":91,"y":0},"activity":"rest"}]}]}`
	c := Compiled{Life: []byte(life), Walkable: map[string]bool{".": true}, Config: Config{TickHz: 10},
		SanitaryServices: &SanitaryCatalog{NPCHygiene: true, ServiceAvailable: true, DoorSeconds: 0.6, EntrySeconds: 2, ExitSeconds: 2,
			Units: []SanitaryUnit{{ID: "wc", Kind: "wc", LevelID: "hall", EntryCell: Point{X: 0}, ServiceSeconds: 240}, {ID: "basin", Kind: "basin", LevelID: "hall", EntryCell: Point{X: 0}, ServiceSeconds: 300}}}}
	report, err := washroomReach(c)
	if err != nil {
		t.Fatal(err)
	}
	text := string(report)
	// near: 2 × 10 cells × 3 s + 32 + 300 + 32 = 424 s; far: 2 × 90 × 3 + 364 = 904 s; the rest place is no duty place
	if !strings.Contains(text, "Мест смены у санузлов: 2, из них не укладываются: 1.") || !strings.Contains(text, "| Far (`far`) | hall | (90,0) | 90 | 904 |") || strings.Contains(text, "`near`") || strings.Contains(text, "(91,0)") {
		t.Fatalf("wrong washroom reach list:\n%s", text)
	}
	// with the duty margin (data: the people on the way, a frame of ×30) near needs 424 + 180 = 604 s: listed too
	c.SanitaryServices.DutyMarginSeconds = 180
	if report, _ := washroomReach(c); !strings.Contains(string(report), "из них не укладываются: 2.") || !strings.Contains(string(report), "| Near (`near`) | hall | (10,0) | 10 | 424 |") {
		t.Fatalf("the margin does not count:\n%s", report)
	}
	c.SanitaryServices.NPCHygiene = false
	if report, _ := washroomReach(c); report != nil {
		t.Fatal("a world without the Life's washroom got a list")
	}
}
