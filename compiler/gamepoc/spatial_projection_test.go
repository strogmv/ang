package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSpatialViewOmitsAuthoringAndOwnership(t *testing.T) {
	c := SpatialCatalog{Modules: []SpatialModule{{ID: "capsule", HeightMM: 3500}}, Levels: []SpatialLevel{{ID: "market", Terrain: []string{"secret-terrain"}, Zones: []SpatialZone{{ID: "zone", Purpose: "private-purpose"}}, Props: []SpatialProp{{ID: "crate", ModuleID: "capsule", Rotation: 180, Owner: "secret-owner", Purpose: "private-history"}}}}}
	b, err := json.Marshal(clientSpatial(c))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-terrain", "private-purpose", "secret-owner", "private-history", "\"owner\"", "\"purpose\""} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("view leaked %s", secret)
		}
	}
	if !strings.Contains(string(b), "crate") || !strings.Contains(string(b), `"rotation":180`) || !strings.Contains(string(b), `"module_id":"capsule"`) || !strings.Contains(string(b), `"height_mm":3500`) {
		t.Fatal("missing visual binding")
	}
}
