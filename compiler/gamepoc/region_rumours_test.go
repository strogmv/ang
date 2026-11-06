package gamepoc

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRegionRumourCatalogValidatesPrivateBindings(t *testing.T) {
	life := json.RawMessage(`{"locations":{"home.level":{},"stair.level":{}},"actors":[{"id":"speaker"}]}`)
	fixture := func() RegionCatalog {
		r := regionFixture()
		r.Nodes[1].LevelID = "stair.level"
		r.Rumours = []RegionRumour{{ID: "report.a", SubjectID: "subject", NodeID: "hidden", SourceActorID: "speaker", TextKey: "rumour.private.a", ContextKey: "rumour.private.context", ImageVariant: "narrow_stair"}}
		return r
	}
	valid := fixture()
	if err := valid.validate(life); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*RegionCatalog){func(r *RegionCatalog) { r.Rumours[0].NodeID = "absent" }, func(r *RegionCatalog) { r.Rumours[0].SourceActorID = "absent" }, func(r *RegionCatalog) { r.Rumours = append(r.Rumours, r.Rumours[0]) }, func(r *RegionCatalog) { r.Rumours[0].ImageVariant = "secret_geometry" }} {
		r := fixture()
		mutate(&r)
		if r.validate(life) == nil {
			t.Fatal("invalid report binding accepted")
		}
	}
	r := fixture()
	raw, err := clientResource(Compiled{Region: r})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"report.a", "rumour.private.a", "stair.level", "narrow_stair", "subject"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("private report catalog entered Unity resource: %s", secret)
		}
	}
}
