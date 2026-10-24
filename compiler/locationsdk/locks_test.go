package locationsdk

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func requireFingerprint(t *testing.T, input Input, id string) string {
	t.Helper()
	hash, err := LayoutFingerprint(input, id)
	if err != nil || len(hash) != 64 {
		t.Fatal("layout fingerprint failed", err)
	}
	return hash
}

func lockedTestInput(t *testing.T) Input {
	t.Helper()
	in := Input{Catalog: surfaceTestCatalog(), Layouts: testLayouts()}
	in.Locks = []LayoutLock{{LayoutID: in.Layouts[0].ID, ExpectedHash: requireFingerprint(t, in, in.Layouts[0].ID)}}
	return in
}

func requireLockCode(t *testing.T, input Input, code string) {
	t.Helper()
	err := ValidateLocks(input)
	var invalid *ValidationError
	if !errors.As(err, &invalid) || invalid.Diagnostics[0].Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
	out, err := CompileRuntime(input, runtimeTestOptions())
	if !errors.As(err, &invalid) || invalid.Diagnostics[0].Code != code || !reflect.DeepEqual(out, RuntimeBundle{}) {
		t.Fatal("runtime compilation bypassed locks or exposed partial output", err)
	}
}

func TestLayoutLocksPermitIndependentEditsButRejectCompleteUsedDependencyChanges(t *testing.T) {
	base := lockedTestInput(t)
	unused := base.Catalog.Surfaces[0]
	unused.ID, unused.TopImage = "surface.unused", "LocationSDK/unused"
	base.Catalog.Surfaces = append(base.Catalog.Surfaces, unused)
	independent := copyValue(t, base)
	independent.Layouts[1].Instances[0].Reason = "other room may evolve independently"
	independent.Catalog.Surfaces[2].TopRepeatMM.X++
	if err := ValidateLocks(independent); err != nil {
		t.Fatal("unrelated layout or unused material invalidated a lock", err)
	}
	for _, tt := range []struct {
		name string
		edit func(*Input)
	}{
		{"layout prose", func(in *Input) { in.Layouts[0].Instances[0].Reason += " changed" }},
		{"seed", func(in *Input) { in.Layouts[0].Seed++ }},
		{"spawn order", func(in *Input) { in.Layouts[0].Entries = append([]Cell{{X: 1, Y: 5}}, in.Layouts[0].Entries...) }},
		{"shared geometry", func(in *Input) { in.Catalog.Geometries[0].Height++ }},
		{"shared brightness", func(in *Input) { in.Catalog.Times[1].Brightness -= .1 }},
		{"shared event", func(in *Input) {
			in.Catalog.Events[1].ActivityTags = append(in.Catalog.Events[1].ActivityTags, "new_activity")
		}},
		{"initial time", func(in *Input) { slices.Reverse(in.Catalog.Times) }},
		{"used floor surface", func(in *Input) { in.Catalog.Surfaces[1].SideImage = "LocationSDK/changed" }},
		{"decoration dimensions", func(in *Input) { in.Catalog.Decorations[0].Width = 2 }},
		{"guide centerline", func(in *Input) {
			in.Layouts[0].ReservedPaths[0].Points = []Cell{{X: 1, Y: 5}, {X: 5, Y: 5}, {X: 11, Y: 5}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := copyValue(t, base)
			tt.edit(&in)
			requireLockCode(t, in, "lock_mismatch")
		})
	}
}

func TestLayoutLockWorkflowRequiresExplicitHashReplacement(t *testing.T) {
	in := lockedTestInput(t)
	if err := ValidateLocks(in); err != nil {
		t.Fatal(err)
	}
	previous := in.Locks[0].ExpectedHash
	in.Layouts[0].Instances[0].Reason += " deliberately revised"
	requireLockCode(t, in, "lock_mismatch")
	// Read-only fingerprinting works even while the old expectation rejects
	// compilation; neither computing it nor failing validation auto-accepts it.
	current := requireFingerprint(t, in, in.Locks[0].LayoutID)
	if current == previous || in.Locks[0].ExpectedHash != previous {
		t.Fatal("fingerprint changed or automatically accepted the expectation")
	}
	in.Locks[0].ExpectedHash = current
	if err := ValidateLocks(in); err != nil {
		t.Fatal("explicitly accepted candidate still rejected", err)
	}
	requireRuntime(t, in, runtimeTestOptions())
}

func TestLayoutLocksRejectMalformedDeletedAndDuplicateIdentities(t *testing.T) {
	for _, tt := range []struct {
		name, code string
		edit       func(*Input)
	}{
		{"malformed hash", "lock_hash", func(in *Input) { in.Locks[0].ExpectedHash = "hash" }},
		{"uppercase hash", "lock_hash", func(in *Input) { in.Locks[0].ExpectedHash = strings.Repeat("A", 64) }},
		{"empty id", "lock_identity", func(in *Input) { in.Locks[0].LayoutID = "" }},
		{"duplicate lock", "lock_identity", func(in *Input) { in.Locks = append(in.Locks, in.Locks[0]) }},
		{"unknown id", "lock_unknown_layout", func(in *Input) { in.Locks[0].LayoutID = "absent" }},
		{"deleted layout", "lock_unknown_layout", func(in *Input) { in.Layouts = in.Layouts[1:] }},
		{"wrong hash", "lock_mismatch", func(in *Input) { in.Locks[0].ExpectedHash = strings.Repeat("0", 64) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := lockedTestInput(t)
			tt.edit(&in)
			requireLockCode(t, in, tt.code)
		})
	}
}

func TestLayoutFingerprintCanonicalOrderMutableSafetyAndLegacyNoLockPayload(t *testing.T) {
	in := lockedTestInput(t)
	before := copyValue(t, in)
	want := requireFingerprint(t, in, in.Locks[0].LayoutID)
	slices.Reverse(in.Layouts)
	slices.Reverse(in.Catalog.Geometries)
	slices.Reverse(in.Catalog.Modules)
	slices.Reverse(in.Catalog.Surfaces)
	for i := range in.Catalog.Geometries {
		slices.Reverse(in.Catalog.Geometries[i].Footprint)
		slices.Reverse(in.Catalog.Geometries[i].Blocked)
	}
	for i := range in.Layouts {
		slices.Reverse(in.Layouts[i].Instances)
	}
	if actual := requireFingerprint(t, in, in.Locks[0].LayoutID); actual != want {
		t.Fatal("semantically unordered records changed layout fingerprint")
	}
	if err := ValidateLocks(in); err != nil {
		t.Fatal(err)
	}
	copy := copyValue(t, before)
	requireFingerprint(t, copy, copy.Locks[0].LayoutID)
	if !reflect.DeepEqual(copy, before) {
		t.Fatal("fingerprinting mutates input")
	}
	withLocks := requireRuntime(t, before, runtimeTestOptions())
	before.Locks = nil
	withoutLocks := requireRuntime(t, before, runtimeTestOptions())
	if !reflect.DeepEqual(withLocks, withoutLocks) {
		t.Fatal("lock metadata changed runtime geometry or content hash")
	}
	data, err := json.Marshal(before)
	if err != nil || strings.Contains(string(data), `"locks"`) {
		t.Fatal("absent locks changed legacy serialized input", err)
	}
}
