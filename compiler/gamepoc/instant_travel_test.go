package gamepoc

import (
	"context"
	"encoding/json"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/strogmv/ang/compiler/behavior"
	"github.com/strogmv/ang/compiler/emitter"
)

// Overlay the source templates onto the generated host in a scratch build. No
// generated project file is written; the real book population and inventory are
// used for the elapsed-night replay/performance regression.
func TestInstantTravelActualHostAndLife100ReplayNative(t *testing.T) {
	book := os.Getenv("ANG_C7_TRANSIT_PROJECT")
	if book == "" {
		t.Skip("set ANG_C7_TRANSIT_PROJECT to book checkout")
	}
	project := filepath.Join(book, "server")
	c, def, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	replace := map[string]string{}
	write := func(target string, body []byte) {
		t.Helper()
		if strings.HasSuffix(target, ".go") {
			body, err = format.Source(body)
			if err != nil {
				t.Fatalf("format %s: %v", target, err)
			}
		}
		name := filepath.Join(dir, strings.ReplaceAll(filepath.ToSlash(target), "/", "_"))
		if err := os.WriteFile(name, body, 0600); err != nil {
			t.Fatal(err)
		}
		replace[target] = name
	}
	pbName, pb, err := emitter.GeneratePBGo(def, "angelsdemons/server")
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/transport/grpc/pb/game/v1", pbName), pb)
	currentWorld, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/poc/world.json"), currentWorld)
	emitted, err := behavior.Render(c.AI, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range emitted {
		write(filepath.Join(project, "internal/behavior", name), body)
	}
	names := map[string]string{"runtime.go.tmpl": "runtime.gen.go", "life.go.tmpl": "life.gen.go", "transit_fare.go.tmpl": "transit_fare.gen.go", "instant_travel.go.tmpl": "instant_travel.gen.go", "instant_travel_test.go.tmpl": "instant_travel_test.go", "life_test.go.tmpl": "life_generated_test.go", "transit_fare_test.go.tmpl": "transit_fare_test.go"}
	names["population_test.go.tmpl"] = "population_generated_test.go"
	names["region.go.tmpl"] = "region.gen.go"
	names["wayfinding.go.tmpl"] = "wayfinding.gen.go"
	names["region_test.go.tmpl"] = "region_generated_test.go"
	for name, target := range names {
		data, err := os.ReadFile(filepath.Join("templates", name))
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(project, "internal/poc", target), data)
	}
	regionJSON, err := json.Marshal(c.Region)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "internal/poc/region_current_selection_test.go"), []byte(strings.Replace(regionCurrentSelectionHarness, "REGION_CATALOG_LITERAL", strconv.Quote(string(regionJSON)), 1)))
	data, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "./internal/poc", "-count=1", "-v")
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("actual generated host overlay: %v", err)
	}
}

const regionCurrentSelectionHarness = `package poc
import("encoding/json";"reflect";"testing";pb "angelsdemons/server/internal/transport/grpc/pb/game/v1")
func TestCurrentC7RegionalSelectionKeepsRequestedPhysicalDestination(t *testing.T){
 r:=lifeRuntimeFixture(t);r.world.Region=regionCatalog{};if err:=json.Unmarshal([]byte(REGION_CATALOG_LITERAL),&r.world.Region);err!=nil{t.Fatal(err)};r.state.Region=regionKnowledge{};r.syncRegionKnowledge();r.state.Life.SecondsPerTick=30
 cases:=[]struct{target,exit,arrival string}{
 {"level.poc_monorail","market_station","level.poc_monorail"},
 {"level.c7_market","station_market","level.c7_market"},
 {"level.c7_transport_hub","market_hub_walk","level.c7_transport_hub"},
 {"level.c7_market","hub_market_walk","level.c7_market"},
 {"level.poc_work","market_station","level.poc_monorail"},
 {"level.poc_work","station_work","level.poc_work"},
 {"level.poc_housing","work_station","level.poc_monorail"},
 {"level.poc_housing","station_home","level.poc_housing"},
 {"level.poc_monorail","home_station","level.poc_monorail"},
 }
 for _,c:=range cases{
  r.syncRegionKnowledge();before:=r.snapshotState();from:=r.currentLevel();if err:=r.lifeCommand(&pb.PocLifeCommand{Action:"route_preview",Destination:c.target});err!=nil{t.Fatalf("%s -> %s preview: %v",from,c.target,err)};view:=&pb.PocLifeView{};r.appendWayfinding(view,"ru");r.appendRegionMap(view,"ru")
  if !reflect.DeepEqual(before,r.snapshotState()){t.Fatal("selection/projection changed physical state or knowledge")};if view.Wayfinding.LocalExitId!=c.exit||!view.Wayfinding.Reachable{t.Fatalf("%s -> %s chose %s instead of %s",from,c.target,view.Wayfinding.LocalExitId,c.exit)}
  if view.RegionMap.SelectedDestinationId!=c.target||len(view.RegionMap.SelectedLocalLinkIds)!=1{t.Fatalf("first-leg map highlight missing: %+v",view.RegionMap)};linkID:=view.RegionMap.SelectedLocalLinkIds[0];bound:=false;for _,link:=range r.world.Region.Links{if link.ID==linkID&&r.state.Region.KnownLinks[link.ID]{for _,id:=range link.LifeEdges{if id==c.exit{bound=true}}}};if !bound{t.Fatal("highlight does not represent selected known physical edge")}
  edge:=r.world.Life.Edge(c.exit);if edge==nil{t.Fatal("missing real edge")};seconds:=r.state.Life.Seconds;if err:=r.lifeCommand(&pb.PocLifeCommand{Action:"route_go",Destination:c.target});err!=nil{t.Fatal(err)}
  for tick:=0;tick<900&&r.currentLevel()==from;tick++{r.step();if r.state.Life.Actor(playerActorID).Trip!=nil{t.Fatal("normal selected arrival exposed countdown")}}
  if r.currentLevel()!=c.arrival{t.Fatalf("%s selected %s physically arrived at %s instead of %s",from,c.target,r.currentLevel(),c.arrival)};if r.state.Life.Seconds-seconds<edge.TravelSeconds{t.Fatal("selection arrival lost real game travel time")}
 }
}
`
