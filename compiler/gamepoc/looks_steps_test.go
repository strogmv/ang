package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stepsFixture: the look fixture's model with two distance steps in its record and the look naming when near is drawn.
func stepsFixture(t *testing.T) (string, Art, *TurretWatchCatalog) {
	t.Helper()
	root, a, turrets := lookFixture(t)
	writeSteps(t, root, `{"near": {"mesh": "Silver_mark", "triangles": 32000, "budget": 33000},
		"crowd": {"mesh": "Silver_mark_crowd", "triangles": 17800, "budget": 18000}}`)
	l := a.Looks["look.guard"]
	l.Steps = &LookSteps{NearUpToFrameM: 5, HysteresisM: 0.5}
	a.Looks["look.guard"] = l
	return root, a, turrets
}

func writeSteps(t *testing.T, root, steps string) {
	t.Helper()
	path := filepath.Join(root, "art-source/caste/v1/look.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), `"surfaces"`, `"steps": `+steps+`, "surfaces"`, 1))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLookStepsAcceptedAndProjected(t *testing.T) {
	root, a, turrets := stepsFixture(t)
	person := func(id string) bool { return id == "char.guard" }
	if err := checkLooks(root, a, person, turrets, nil); err != nil {
		t.Fatal(err)
	}
	looks, _ := clientLooks(a, turrets)
	raw, _ := json.Marshal(looks)
	if !strings.Contains(string(raw), `"steps":{"near_up_to_frame_m":5,"hysteresis_m":0.5}`) {
		t.Fatalf("the steps are not projected: %s", raw)
	}
}

func TestLookStepsRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	set := func(f, h float64) func(string, *Art) {
		return func(_ string, a *Art) {
			l := a.Looks["look.guard"]
			l.Steps = &LookSteps{NearUpToFrameM: f, HysteresisM: h}
			a.Looks["look.guard"] = l
		}
	}
	cases := map[string]func(root string, a *Art){
		"frame below 1 m":       set(0.5, 0.2),
		"frame above 16 m":      set(20, 0.2),
		"hysteresis negative":   set(5, -0.1),
		"hysteresis of a metre": set(5, 1),
		"model with steps, look without": func(_ string, a *Art) {
			l := a.Looks["look.guard"]
			l.Steps = nil
			a.Looks["look.guard"] = l
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := stepsFixture(t)
			mutate(root, &a)
			if err := checkLooks(root, a, person, turrets, nil); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
	records := map[string]string{
		"one step only":        `{"near": {"mesh": "Silver_mark", "triangles": 32000, "budget": 33000}}`,
		"an unknown step":      `{"near": {"mesh": "Silver_mark", "triangles": 32000, "budget": 33000}, "far": {"mesh": "Silver_mark_far", "triangles": 9000, "budget": 9000}}`,
		"over the budget":      `{"near": {"mesh": "Silver_mark", "triangles": 34000, "budget": 33000}, "crowd": {"mesh": "Silver_mark_crowd", "triangles": 17800, "budget": 18000}}`,
		"one mesh for both":    `{"near": {"mesh": "Silver_mark", "triangles": 32000, "budget": 33000}, "crowd": {"mesh": "Silver_mark", "triangles": 17800, "budget": 18000}}`,
		"crowd richer":         `{"near": {"mesh": "Silver_mark", "triangles": 17000, "budget": 33000}, "crowd": {"mesh": "Silver_mark_crowd", "triangles": 17800, "budget": 18000}}`,
		"a mesh name unsafe":   `{"near": {"mesh": "../x", "triangles": 32000, "budget": 33000}, "crowd": {"mesh": "Silver_mark_crowd", "triangles": 17800, "budget": 18000}}`,
		"no triangles counted": `{"near": {"mesh": "Silver_mark", "triangles": 0, "budget": 33000}, "crowd": {"mesh": "Silver_mark_crowd", "triangles": 0, "budget": 18000}}`,
	}
	for name, steps := range records {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := lookFixture(t)
			writeSteps(t, root, steps)
			l := a.Looks["look.guard"]
			l.Steps = &LookSteps{NearUpToFrameM: 5, HysteresisM: 0.5}
			a.Looks["look.guard"] = l
			if err := checkLooks(root, a, person, turrets, nil); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
	t.Run("look with steps, model without", func(t *testing.T) {
		root, a, turrets := lookFixture(t)
		l := a.Looks["look.guard"]
		l.Steps = &LookSteps{NearUpToFrameM: 5, HysteresisM: 0.5}
		a.Looks["look.guard"] = l
		if err := checkLooks(root, a, person, turrets, nil); err == nil {
			t.Fatal("accepted")
		}
	})
}
