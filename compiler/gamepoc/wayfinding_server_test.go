package gamepoc

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWayfindingCatalogGroundingIdentityAndPrivacy(t *testing.T) {
	life := json.RawMessage(`{"locations":{"hall":{"terrain":["#####","#...#","#####"]}},"edges":[{"id":"door","from":"hall","exit":{"x":3,"y":1}}]}`)
	base := WayfindingDestination{ID: "way.hall.med", NameKey: "wayfinding.hall.med", Level: "hall", Color: "#83A7BD", Kind: "medical", At: WayfindingCell{3, 1}, GuideFrom: &WayfindingCell{1, 1}, MarkerNumber: 7}
	texts := LocalizationCatalog{Messages: []LocalizationMessage{{Key: base.NameKey}}}
	check := func(ds ...WayfindingDestination) error {
		return (WayfindingCatalog{Destinations: ds}).validate(life, map[string]bool{".": true}, texts)
	}
	if err := check(base); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*WayfindingDestination){"wall": func(d *WayfindingDestination) { d.At = WayfindingCell{0, 0} }, "bad_guide": func(d *WayfindingDestination) { d.GuideFrom = &WayfindingCell{9, 1} }, "missing_text": func(d *WayfindingDestination) { d.NameKey = "untranslated" }, "bad_color": func(d *WayfindingDestination) { d.Color = "red" }, "false_exit": func(d *WayfindingDestination) { d.Kind = "exit"; d.EdgeID = "missing" }, "wrong_door": func(d *WayfindingDestination) { d.Kind = "exit"; d.EdgeID = "door"; d.At = WayfindingCell{2, 1} }, "fake_service": func(d *WayfindingDestination) { d.Kind = "healing" }, "invalid_number": func(d *WayfindingDestination) { d.MarkerNumber = 100 }} {
		t.Run(name, func(t *testing.T) {
			d := base
			change(&d)
			if check(d) == nil {
				t.Fatal("invalid approach accepted")
			}
		})
	}
	if check(base, base) == nil {
		t.Fatal("duplicate ID accepted")
	}
	other := base
	other.ID = "way.other"
	if check(base, other) == nil {
		t.Fatal("duplicate physical marker number accepted")
	}
	disconnected := json.RawMessage(`{"locations":{"hall":{"terrain":["#####","#.#.#","#####"]}},"edges":[]}`)
	if (WayfindingCatalog{Destinations: []WayfindingDestination{base}}).validate(disconnected, map[string]bool{".": true}, texts) == nil {
		t.Fatal("disconnected permanent guide accepted")
	}
	var many []WayfindingDestination
	for i := 0; i < 25; i++ {
		d := base
		d.ID = string(rune('a' + i))
		d.MarkerNumber = 0
		many = append(many, d)
	}
	if check(many...) == nil {
		t.Fatal("25 destinations accepted")
	}
	c := paintedFixture()
	c.Wayfinding = WayfindingCatalog{Destinations: []WayfindingDestination{{ID: "PRIVATE_DESTINATION", NameKey: "wayfinding.private.name", Level: "PRIVATE_LEVEL"}}}
	data, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"PRIVATE_DESTINATION", "wayfinding.private.name", "PRIVATE_LEVEL", "\"wayfinding\""} {
		if strings.Contains(string(data), hidden) {
			t.Fatalf("private catalog exported to Unity resource: %s", hidden)
		}
	}
}
func TestAuthoredWayfindingGroundedConnectedAndLocalized(t *testing.T) {
	book := os.Getenv("ANG_C7_TRANSIT_PROJECT")
	if book == "" {
		t.Skip("set ANG_C7_TRANSIT_PROJECT to book checkout")
	}
	c, _, err := readCompiled(filepath.Join(book, "server"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Wayfinding.Destinations) < 4 {
		t.Fatal("missing authored routes")
	}
	kinds := map[string]bool{}
	for _, d := range c.Wayfinding.Destinations {
		kinds[d.Kind] = true
	}
	for _, k := range []string{"medical", "sanitary", "shop", "exit"} {
		if !kinds[k] {
			t.Errorf("missing %s", k)
		}
	}
}

// The harness executes the actual projection/command template, actual regional
// knowledge code and actual Space.Path. Stubs supply unrelated game/wire shapes.
func TestWayfindingServerPreviewAuthorityPrivacyAndInvalidationNative(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "behavior", "templates", "reactions.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	a, b := strings.Index(source, "type Space struct {"), strings.Index(source, "func(s Space) routeCost(")
	if a < 0 || b < a {
		t.Fatal("Space.Path missing")
	}
	move := hudTemplate(t, "life.go.tmpl")
	am, bm := strings.Index(move, "func(r *Runtime)lifeWalkTo("), strings.Index(move, "func(r *Runtime)validateLifeSave(")
	if am < 0 || bm < am {
		t.Fatal("shared movement helper missing")
	}
	files := map[string]string{
		"go.mod":                     "module angelsdemons/server\n\ngo 1.23\n",
		"internal/behavior/space.go": "package behavior\ntype Cell struct{X,Y int}\nfunc Distance(a,b Cell)int{x,y:=a.X-b.X,a.Y-b.Y;if x<0{x=-x};if y<0{y=-y};return x+y}\n" + source[a:b],
		"internal/behavior/life.go": `package behavior
 type LifeTrip struct{Edge string}
 type LifeActor struct{ID,Level,OverrideLevel,OverrideEdge,Activity,Label,AdmissionDeniedEdge string;Position,Goal,OverrideAt Cell;Path,PendingPath []Cell;Manual,Arrived bool;MoveStartTick,WaitingDeparture int64;Trip *LifeTrip;Visits map[string]int}
 type LifeEdge struct{ID,From,To,Mode,NameKey string;Exit,Entry Cell}
 type LifeLocation struct{Name string;Terrain []string}
 type LifeModel struct{Locations map[string]LifeLocation;Edges []LifeEdge}
 func(m LifeModel)Edge(id string)*LifeEdge{for i:=range m.Edges{if m.Edges[i].ID==id{return &m.Edges[i]}};return nil}
 func(m LifeModel)NextEdge(from,to string)*LifeEdge{for i:=range m.Edges{if m.Edges[i].From==from&&m.Edges[i].To==to{return &m.Edges[i]}};return nil}
 type LifeState struct{Actors []LifeActor;Seconds int64;Enabled bool}
 func(s *LifeState)Actor(id string)*LifeActor{for i:=range s.Actors{if s.Actors[i].ID==id{return &s.Actors[i]}};return nil}
 func(s *LifeState)MoveRoute(m LifeModel,id,level string,from,to Cell,tick int64)([]Cell,bool){rows:=m.Locations[level].Terrain;space:=Space{Width:len(rows[0]),Height:len(rows),MaxSearchCells:len(rows)*len(rows[0]),Walkable:func(c Cell)bool{return rows[c.Y][c.X]=='.'}};path:=space.Path(from,to,Cell{},0);return path,from==to||len(path)>0}
 `,
		"internal/transport/grpc/pb/game/v1/wire.go": `package pb
 type PocPoint struct{X,Y int32};type PocLifeCommand struct{Action,Destination string}
 type PocLifeView struct{Wayfinding *PocWayfindingView;RegionMap *PocRegionMap}
 type PocWayfindingView struct{LevelId,SelectedDestinationId,SelectedName,SelectedColor,Status,LocalExitId,LocalExitMode,LocalExitName,RejectedDestinationId string;Destinations []*PocWayfindingDestination;Guides []*PocWayfindingGuide;Path []*PocPoint;Reachable bool}
 type PocWayfindingDestination struct{Id,Name,Color,Kind,TravelMode string;X,Y,Number int32;ServiceAvailable bool}
 type PocWayfindingGuide struct{Id,Color string;Path []*PocPoint}
 type PocRegionMap struct{Id,Name,CurrentNode,CurrentDeck string;Nodes []*PocRegionNode;Links []*PocRegionLink;Decks []*PocRegionDeck}
 type PocRegionNode struct{Id,Name,DeckId,Knowledge,LevelId,Description string;X,Y,Width,Height float64;Current,CanTravel bool}
 type PocRegionLink struct{Id,FromNode,ToNode,Mode string};type PocRegionDeck struct{Id,Name string;ElevationM float64}
 `,
		"internal/poc/wayfinding.go": hudTemplate(t, "wayfinding.go.tmpl"), "internal/poc/region.go": hudTemplate(t, "region.go.tmpl"), "internal/poc/movement.go": "package poc\nimport \"angelsdemons/server/internal/behavior\"\n" + move[am:bm],
		"internal/poc/host_test.go": wayfindingServerHarness,
	}
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-count=1", "-v")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual server wayfinding harness: %v\n%s", err, out)
	}
}

const wayfindingServerHarness = `package poc
import("encoding/json";"reflect";"strings";"testing";"angelsdemons/server/internal/behavior";pb "angelsdemons/server/internal/transport/grpc/pb/game/v1")
const playerActorID="hero"
type textError struct{Key string};func(e textError)Error()string{return e.Key}
type seenCell struct{Glyph string};type state=hostState
type hostState struct{Life behavior.LifeState;LifeExploration map[string]map[int]seenCell;Region regionKnowledge;Epoch string;Paused bool;Tick int64}
type worldModel struct{Life behavior.LifeModel;Wayfinding wayfindingCatalog;Region regionCatalog;Walkable map[string]bool}
type Runtime struct{world worldModel;state hostState;wayfinding wayfindingSession;entered string;syncs int}
func(r *Runtime)localText(locale,key,fallback string)string{if strings.HasPrefix(key,"wayfinding."){return locale+":"+key};return fallback}
func(r *Runtime)lifeExitKnown(e behavior.LifeEdge)bool{return r.wayfindingWalks(e.From,e.Exit)}
func(r *Runtime)lifeEdgeName(locale string,e behavior.LifeEdge)string{return r.regionLevelName(locale,e.To)}
func(r *Runtime)resetInstantTravel(){}
func(r *Runtime)syncLifeHero(){r.syncs++}
func(r *Runtime)lifeCommand(q *pb.PocLifeCommand)error{if q.Action=="enter"{r.entered=q.Destination;return nil};return r.wayfindingCommand(q.Action,q.Destination)}
func fixture()*Runtime{rows:=[]string{"#########","#.......#","#.###.#.#","#.....#.#","#######.#","#.......#","#########"};r:=&Runtime{};r.world.Walkable=map[string]bool{".":true};r.world.Life.Locations=map[string]behavior.LifeLocation{"hall":{Name:"Hall",Terrain:rows},"out":{Name:"Other room",Terrain:rows},"secret":{Name:"SECRET_ROOM",Terrain:rows}};r.world.Life.Edges=[]behavior.LifeEdge{{ID:"door",From:"hall",To:"out",Mode:"walk",Exit:behavior.Cell{7,1}},{ID:"SECRET_EDGE",From:"hall",To:"secret",Mode:"rail",Exit:behavior.Cell{1,1}}};r.state=hostState{Epoch:"epoch1",Tick:44,Life:behavior.LifeState{Enabled:true,Seconds:100,Actors:[]behavior.LifeActor{{ID:playerActorID,Level:"hall",Position:behavior.Cell{1,1},Goal:behavior.Cell{3,1},Path:[]behavior.Cell{{2,1},{3,1}},MoveStartTick:42,Visits:map[string]int{"hall":1}}}},LifeExploration:map[string]map[int]seenCell{"hall":{}}};for y,row:=range rows{for x:=range row{r.state.LifeExploration["hall"][y*9+x]=seenCell{string(row[x])}}};r.world.Wayfinding=wayfindingCatalog{Destinations:[]wayfindingDestination{{ID:"way.med",NameKey:"wayfinding.med",Color:"#79A99A",Kind:"medical",Level:"hall",At:behavior.Cell{7,5},GuideFrom:&behavior.Cell{1,1},MarkerNumber:7},{ID:"way.hidden",NameKey:"wayfinding.hidden",Color:"#83A7BD",Kind:"sanitary",Level:"hall",At:behavior.Cell{1,5},MarkerNumber:2}}};delete(r.state.LifeExploration["hall"],5*9+1);r.world.Region=regionCatalog{ID:"region",Nodes:[]regionNode{{ID:"node.hall",LevelID:"hall",InitialKnowledge:"visited"},{ID:"node.out",LevelID:"out",InitialKnowledge:"known"},{ID:"node.secret",Name:"SECRET_ROOM",LevelID:"secret",InitialKnowledge:"unknown"}},Links:[]regionLink{{ID:"link.out",FromNode:"node.hall",ToNode:"node.out",InitialKnown:true,LifeEdges:[]string{"door"}},{ID:"SECRET_LINK",FromNode:"node.hall",ToNode:"node.secret",LifeEdges:[]string{"SECRET_EDGE"}}}};r.state.Region=regionKnowledge{Initialized:true,KnownNodes:map[string]bool{"node.hall":true,"node.out":true},KnownLinks:map[string]bool{"link.out":true},Visited:map[string]int64{"node.hall":0}};return r}
func project(r *Runtime)*pb.PocWayfindingView{v:=&pb.PocLifeView{};r.appendWayfinding(v,"en");return v.Wayfinding}
func snapshot(r *Runtime)string{b,_:=json.Marshal(r.state);return string(b)}
func TestReadOnlySelectionPathsAndPrivacy(t *testing.T){r:=fixture();before:=snapshot(r);if err:=r.wayfindingCommand("route_preview","way.med");err!=nil{t.Fatal(err)};v:=project(r);if snapshot(r)!=before||r.syncs!=0||!v.Reachable||v.SelectedName!="en:wayfinding.med"{t.Fatal("selection mutated intent/calendar/knowledge or failed localization")};for i,p:=range v.Path{if r.world.Life.Locations["hall"].Terrain[p.Y][p.X]!='.'{t.Fatal("wall crossing")};if i>0{q:=v.Path[i-1];if behavior.Distance(behavior.Cell{int(p.X),int(p.Y)},behavior.Cell{int(q.X),int(q.Y)})!=1{t.Fatal("noncardinal path")}}};for _,d:=range v.Destinations{if d.Id=="way.hidden"{t.Fatal("unknown destination leaked")};if d.Id=="way.med"&&(d.ServiceAvailable||d.Number!=7){t.Fatal("fake service or unstable sign number")}};raw,_:=json.Marshal(v);if strings.Contains(string(raw),"SECRET"){t.Fatal("private region data leaked")};r.wayfindingCommand("route_clear","");if snapshot(r)!=before||project(r).SelectedDestinationId!=""{t.Fatal("clear cancelled gameplay")}}
func TestHiddenBodiesDoNotChangeGuidance(t *testing.T){r:=fixture();r.wayfindingCommand("route_preview","way.med");before:=project(r);r.state.Life.Actors=append(r.state.Life.Actors,behavior.LifeActor{ID:"hiddenNPC",Level:"hall",Position:behavior.Cell{7,3},Path:[]behavior.Cell{{7,4}}});if !reflect.DeepEqual(before,project(r)){t.Fatal("hidden occupancy changed preview")}}
func TestExplicitGoPauseStaleAndAcceptedSegment(t *testing.T){r:=fixture();r.wayfindingCommand("route_preview","way.med");r.state.Paused=true;before:=snapshot(r);if r.wayfindingCommand("route_go","way.med")==nil||snapshot(r)!=before{t.Fatal("paused Go moved")};if r.wayfindingCommand("route_preview","way.med")!=nil{t.Fatal("paused selection blocked")};r.state.Paused=false;if r.wayfindingCommand("route_go","old")==nil{t.Fatal("stale Go accepted")};h:=r.state.Life.Actor(playerActorID);at:=h.Position;if err:=r.wayfindingCommand("route_go","way.med");err!=nil{t.Fatal(err)};if h.Position!=at||h.Path[0]!=(behavior.Cell{2,1})||h.Goal!=(behavior.Cell{7,5})||!h.Manual||r.syncs!=1{t.Fatal("Go bypassed real intent helper or accepted segment")};if project(r).SelectedDestinationId!="way.med"{t.Fatal("Go erased preview")}}
func TestExactMemoryCacheInvalidationAndDiscoveryNumbers(t *testing.T){r:=fixture();r.wayfindingCommand("route_preview","way.med");if !project(r).Reachable{t.Fatal("fixture")};r.state.LifeExploration["hall"][3*9+7]=seenCell{"#"};if v:=project(r);v.Reachable||len(v.Guides)>0{t.Fatal("stale cache crosses changed memory")};r.state.LifeExploration["hall"][3*9+7]=seenCell{"."};if !project(r).Reachable{t.Fatal("same-size memory correction not refreshed")};r.state.LifeExploration["hall"][5*9+1]=seenCell{"."};for _,d:=range project(r).Destinations{if d.Id=="way.med"&&d.Number!=7{t.Fatal("discovery renumbered an existing sign")}};delete(r.state.LifeExploration["hall"],5*9+7);if project(r).SelectedDestinationId!=""{t.Fatal("unknown selected target remained")};r.state.Epoch="loaded";if project(r).SelectedDestinationId!=""{t.Fatal("UI preview survived a real epoch/load")}}
func TestExitAtDoorRemainsReadyAndRejectionAcknowledgesExactRequest(t *testing.T){r:=fixture();h:=r.state.Life.Actor(playerActorID);h.Position=behavior.Cell{7,1};h.Path=nil;h.AdmissionDeniedEdge="door";if r.wayfindingCommand("route_preview","exit:door")!=nil{t.Fatal("known exit selection failed")};v:=project(r);if v.Status!="ready"||!v.Reachable||len(v.Path)!=1{t.Fatal("exit/denied boarding point cannot be explicitly retried")};if r.wayfindingCommand("route_go","exit:door")!=nil||r.entered!="door"{t.Fatal("standing at exit failed explicit enter")};before:=snapshot(r);if r.wayfindingCommand("route_preview","client-request-A")==nil{t.Fatal("unknown request accepted")};v=project(r);if v.Status!="unknown"||v.RejectedDestinationId!="client-request-A"||v.SelectedDestinationId!=""||snapshot(r)!=before{t.Fatal("rejected request lacks exact read-only ack")};if r.wayfindingCommand("route_preview","way.med")!=nil{t.Fatal("subsequent valid selection failed")};if project(r).RejectedDestinationId!=""{t.Fatal("old rejection contaminated next selection")};r.wayfindingCommand("route_clear","");if project(r).RejectedDestinationId!=""{t.Fatal("clear retained rejection")}}
func TestRegionalFirstLegOnlyKnownPhysicalExit(t *testing.T){r:=fixture();before:=snapshot(r);if err:=r.wayfindingCommand("route_preview","out");err!=nil{t.Fatal(err)};v:=project(r);if v.LocalExitId!="door"||v.LocalExitMode!="walk"||!v.Reachable||snapshot(r)!=before{t.Fatal("regional preview not exact known local leg")};if r.wayfindingCommand("route_go","out")!=nil||r.entered!="door"{t.Fatal("Go did not use previewed physical edge")};if r.wayfindingCommand("route_preview","secret")==nil{t.Fatal("unknown regional target accepted")};raw,_:=json.Marshal(project(r));if strings.Contains(string(raw),"SECRET"){t.Fatal("unknown regional target leaked")}}
`
