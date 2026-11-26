package gamepoc

import "testing"

func TestCombatGridCoverRejectsInvalidCatalog(t *testing.T) {
	for _, raw := range []string{`{"rules":{"grid":{"cell_mm":500,"diagonal":"orthogonal"}}}`, `{"rules":{"grid":{"cell_mm":1000,"diagonal":"free"}}}`, `{"rules":{"grid":{"cell_mm":1000,"diagonal":"orthogonal"},"cover":{"half_penalty":0.8,"full_penalty":0.2}}}`} {
		if validateCombatGridCover([]byte(raw)) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
