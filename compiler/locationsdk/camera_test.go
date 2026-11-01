package locationsdk

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestLayoutCameraIsOptionalPresentationWithIndependentIdentity(t *testing.T) {
	input := runtimeTestInput()
	baseline := requireRuntime(t, input, runtimeTestOptions())
	oldLock, err := LayoutFingerprint(input, input.Layouts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	plain := requireBuilt(t, input.Catalog, input.Layouts[0])
	for _, value := range []any{input.Layouts[0], plain, baseline.Levels[0]} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"camera"`) || strings.Contains(string(data), `"camera_authored"`) {
			t.Fatal("absent camera changed legacy wire data and fingerprints")
		}
	}
	input.Layouts[0].Camera = &LayoutCamera{Projection: "orthographic", ElevationDegrees: 30, AzimuthDegrees: 20}
	compiled := requireBuilt(t, input.Catalog, input.Layouts[0])
	if compiled.Camera == input.Layouts[0].Camera || !reflect.DeepEqual(compiled.Camera, input.Layouts[0].Camera) {
		t.Fatal("compiled camera must own a copy of the authored contract")
	}
	changed := requireRuntime(t, input, runtimeTestOptions())
	index := -1
	for i, level := range changed.Levels {
		if level.ID == input.Layouts[0].ID {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("runtime omitted authored layout")
	}
	if !changed.Levels[index].CameraAuthored || !reflect.DeepEqual(changed.Levels[index].Camera, input.Layouts[0].Camera) {
		t.Fatal("runtime lost the camera")
	}
	if changed.ContentHash == baseline.ContentHash {
		t.Fatal("runtime content identity ignored authored camera")
	}
	newLock, err := LayoutFingerprint(input, input.Layouts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if newLock == oldLock {
		t.Fatal("authoring lock ignored camera")
	}
	input.Locks = []LayoutLock{{LayoutID: input.Layouts[0].ID, ExpectedHash: oldLock}}
	if err := ValidateLocks(input); err == nil {
		t.Fatal("old scene lock accepted a changed camera")
	}
	compiled.Camera = nil
	if !reflect.DeepEqual(compiled, plain) {
		t.Fatal("camera changed compiled physical terrain, navigation, identity or presentation states")
	}
	changed.Levels[index].Camera = nil
	changed.Levels[index].CameraAuthored = false
	changed.ContentHash = baseline.ContentHash
	if !reflect.DeepEqual(changed, baseline) {
		t.Fatal("camera changed runtime physical geometry, movement or navigation")
	}
}

func TestLayoutCameraRejectsUnsafeOrIncompleteProjection(t *testing.T) {
	for _, axis := range []string{"elevation", "azimuth"} {
		bounds := []float64{14.999, 75.001, 0, -1, math.NaN(), math.Inf(1), math.Inf(-1)}
		valid := []float64{15, 30, 75}
		code := "camera_elevation"
		if axis == "azimuth" {
			bounds = []float64{4.999, 85.001, 0, -1, math.NaN(), math.Inf(1), math.Inf(-1)}
			valid = []float64{5, 20, 45, 85}
			code = "camera_azimuth"
		}
		for _, value := range append(bounds, valid...) {
			layout := testLayouts()[0]
			layout.Camera = &LayoutCamera{Projection: "orthographic", ElevationDegrees: 30, AzimuthDegrees: 20}
			if axis == "elevation" {
				layout.Camera.ElevationDegrees = value
			} else {
				layout.Camera.AzimuthDegrees = value
			}
			got, err := Build(testCatalog(), layout)
			accepted := false
			for _, allowed := range valid {
				accepted = accepted || value == allowed
			}
			if accepted && err != nil {
				t.Fatalf("valid %s %v: %v", axis, value, err)
			}
			if !accepted && (err == nil || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != code || got.Diagnostics[0].Path != "layout.camera."+axis+"_degrees") {
				t.Fatalf("invalid %s %v: %#v / %v", axis, value, got.Diagnostics, err)
			}
		}
	}
	for _, projection := range []string{"", "perspective", "Orthographic"} {
		layout := testLayouts()[0]
		layout.Camera = &LayoutCamera{Projection: projection, ElevationDegrees: 30, AzimuthDegrees: 20}
		got, err := Build(testCatalog(), layout)
		if err == nil || got.Diagnostics[0].Code != "unsupported_camera_projection" {
			t.Fatalf("invalid projection %q: %v", projection, err)
		}
	}
}

func TestHeroViewHeightPresentationOnlyAndValidation(t *testing.T) {
	input := runtimeTestInput()
	input.Layouts[0].Camera = &LayoutCamera{Projection: "orthographic", ElevationDegrees: 35, AzimuthDegrees: 37}
	baseline := requireBuilt(t, input.Catalog, input.Layouts[0])
	encoded, err := json.Marshal(baseline.Camera)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "hero_view_height_m") {
		t.Fatal("omitted framing changed legacy camera wire identity")
	}
	originalLock, err := LayoutFingerprint(input, input.Layouts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, height := range []float64{0, 6, 15, 60} {
		input.Layouts[0].Camera.HeroViewHeightM = height
		level := requireBuilt(t, input.Catalog, input.Layouts[0])
		if level.Camera.HeroViewHeightM != height {
			t.Fatal("compiled framing lost")
		}
		level.Camera.HeroViewHeightM = 0
		if !reflect.DeepEqual(level, baseline) {
			t.Fatal("framing changed physical layout/navigation/presentation states")
		}
	}
	input.Layouts[0].Camera.HeroViewHeightM = 15
	changedLock, err := LayoutFingerprint(input, input.Layouts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if changedLock == originalLock {
		t.Fatal("authoring lock ignored hero framing")
	}
	for _, bad := range []float64{-1, 5.999, 60.001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		input.Layouts[0].Camera.HeroViewHeightM = bad
		level, err := Build(input.Catalog, input.Layouts[0])
		if err == nil || level.Diagnostics[0].Code != "camera_hero_view_height" || level.Diagnostics[0].Path != "layout.camera.hero_view_height_m" {
			t.Fatalf("invalid height %v accepted/misdiagnosed: %v", bad, err)
		}
	}
}
