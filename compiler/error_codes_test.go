package compiler

import "testing"

func TestStableErrorCodesAreUniqueAndNonEmpty(t *testing.T) {
	seen := map[string]struct{}{}
	for _, code := range StableErrorCodes {
		if code == "" {
			t.Fatalf("found empty error code in registry")
		}
		if _, ok := seen[code]; ok {
			t.Fatalf("duplicate error code in registry: %s", code)
		}
		seen[code] = struct{}{}
	}
}
