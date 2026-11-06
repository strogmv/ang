package gamepoc

import (
	"bytes"
	"cuelang.org/go/cue/cuecontext"
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

func imageCatalogFixture() RegionCatalog {
	r := regionFixture()
	r.Nodes[1].LevelID = "stair.level"
	r.RumourImages = []RegionRumourImage{{ID: "first", ResourcePath: "RegionRumours/first"}}
	r.Rumours = []RegionRumour{{ID: "report.first", SubjectID: "testimony", NodeID: "hidden", SourceActorID: "speaker", TextKey: "rumour.text", ContextKey: "rumour.context", ImageVariant: "first"}}
	return r
}
func TestRegionRumourImageCatalogueRejectsUnsafeDuplicateAndUnboundData(t *testing.T) {
	life := json.RawMessage(`{"locations":{"home.level":{},"stair.level":{}},"actors":[{"id":"speaker"}]}`)
	for name, mutate := range map[string]func(*RegionCatalog){
		"unsafe_id":      func(r *RegionCatalog) { r.RumourImages[0].ID = "../first" },
		"path_traversal": func(r *RegionCatalog) { r.RumourImages[0].ResourcePath = "RegionRumours/../first" },
		"outside_root":   func(r *RegionCatalog) { r.RumourImages[0].ResourcePath = "Other/first" },
		"absolute_path":  func(r *RegionCatalog) { r.RumourImages[0].ResourcePath = "/RegionRumours/first" },
		"backslash":      func(r *RegionCatalog) { r.RumourImages[0].ResourcePath = `RegionRumours\first` },
		"empty_segment":  func(r *RegionCatalog) { r.RumourImages[0].ResourcePath = "RegionRumours//first" },
		"extension":      func(r *RegionCatalog) { r.RumourImages[0].ResourcePath = "RegionRumours/first.png" },
		"duplicate_id": func(r *RegionCatalog) {
			r.RumourImages = append(r.RumourImages, RegionRumourImage{ID: "first", ResourcePath: "RegionRumours/other"})
		},
		"duplicate_path": func(r *RegionCatalog) {
			r.RumourImages = append(r.RumourImages, RegionRumourImage{ID: "other", ResourcePath: "RegionRumours/first"})
		},
		"unbound_variant": func(r *RegionCatalog) { r.Rumours[0].ImageVariant = "missing" },
	} {
		t.Run(name, func(t *testing.T) {
			r := imageCatalogFixture()
			mutate(&r)
			if r.validate(life) == nil {
				t.Fatal("unsafe/ambiguous image data accepted")
			}
		})
	}
}
func TestRegionRumourThirdImageCompilesFromCUEDataWithoutCodeChange(t *testing.T) {
	// This complete CUE fixture is test-only, not an extra canonical place or art asset.
	fixture := `{
 id:"fixture.region",name:"Fixture"
 decks:[{id:"deck",name:"Deck",elevation_m:0}]
 nodes:[{id:"node.fixture",name:"Fixture",deck_id:"deck",x:0,y:0,width:1,height:1,level_id:"fixture.level",initial_knowledge:"unknown",description:"Fixture"}]
 links:[]
 rumour_images:[{id:"first",resource_path:"RegionRumours/first"},{id:"second",resource_path:"RegionRumours/second"},{id:"fixture_third",resource_path:"RegionRumours/fixture/third"}]
 rumours:[{id:"report.fixture",subject_id:"fixture",node_id:"node.fixture",source_actor_id:"speaker",text_key:"rumour.fixture.text",context_key:"rumour.fixture.context",image_variant:"fixture_third"}]
 }`
	var r RegionCatalog
	if err := cuecontext.New().CompileString(fixture).Decode(&r); err != nil {
		t.Fatal(err)
	}
	life := json.RawMessage(`{"locations":{"fixture.level":{}},"actors":[{"id":"speaker"}]}`)
	if err := r.validate(life); err != nil {
		t.Fatal(err)
	}
	source := hudTemplate(t, "region.cs.tmpl")
	if strings.Contains(source, "fixture_third") {
		t.Fatal("third image hardcoded into production template")
	}
	parsed, err := template.New("region").Delims("<<ANG", "ANG>>").Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, Compiled{Region: r}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fixture_third") || !strings.Contains(out.String(), "RegionRumours/fixture/third") {
		t.Fatal("third authored image absent from generated client allowlist")
	}
}
