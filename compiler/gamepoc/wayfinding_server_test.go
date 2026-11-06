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
	medical := hudTemplate(t, "medical_capsules.go.tmpl")
	medicalStart, medicalEnd := strings.Index(medical, "func (r *Runtime) medicalReserved("), strings.Index(medical, "func (r *Runtime) medicalOwnsActor(")
	if medicalStart < 0 || medicalEnd <= medicalStart {
		t.Fatal("real medical reservation helper missing")
	}
	sanitary := hudTemplate(t, "sanitary_services.go.tmpl")
	fragments := "package poc\n"
	for _, pair := range [][2]string{{"func (r *Runtime) sanitaryReserved(", "func (r *Runtime) sanitaryOwnsActor("}, {"func (r *Runtime) serviceReserved(", "func (r *Runtime) serviceOwnsActor("}} {
		start, end := strings.Index(sanitary, pair[0]), strings.Index(sanitary, pair[1])
		if start < 0 || end <= start {
			t.Fatal("real service reservation helper missing")
		}
		fragments += sanitary[start:end]
	}
	move := hudTemplate(t, "life.go.tmpl")
	am, bm := strings.Index(move, "func (r *Runtime) lifeWalkTo("), strings.Index(move, "func (r *Runtime) validateLifeSave(")
	if am < 0 || bm < am {
		t.Fatal("shared movement helper missing")
	}
	lifeKernel, err := os.ReadFile(filepath.Join("..", "behavior", "templates", "life.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	kernel := string(lifeKernel)
	navStart, navEnd := strings.Index(kernel, "func(m LifeModel)NextEdge("), strings.Index(kernel, "func(m LifeModel)Start()")
	if navStart < 0 || navEnd < navStart {
		t.Fatal("actual Life graph routing missing")
	}
	lifeNavigation := kernel[navStart:navEnd]
	files := map[string]string{
		"go.mod":                     "module angelsdemons/server\n\ngo 1.23\n",
		"internal/behavior/space.go": "package behavior\ntype Cell struct{X,Y int}\nfunc Distance(a,b Cell)int{x,y:=a.X-b.X,a.Y-b.Y;if x<0{x=-x};if y<0{y=-y};return x+y}\n" + source[a:b],
		"internal/behavior/life.go": `package behavior
 import "sort"
 type LifeTrip struct{Edge string}
 type LifeActor struct{ID,Level,OverrideLevel,OverrideEdge,Activity,Label,AdmissionDeniedEdge,ServiceAttachment string;Position,Goal,OverrideAt Cell;Path,PendingPath []Cell;Manual,Arrived,CombatHold bool;MoveStartTick,WaitingDeparture int64;Trip *LifeTrip;Visits map[string]int}
 type LifeEdge struct{ID,From,To,Mode,NameKey string;Exit,Entry Cell;TravelSeconds,PeriodSeconds int64}
 type LifeLocation struct{Name string;Terrain []string}
 type LifeModel struct{Locations map[string]LifeLocation;Edges []LifeEdge}
 func(m LifeModel)Edge(id string)*LifeEdge{for i:=range m.Edges{if m.Edges[i].ID==id{return &m.Edges[i]}};return nil}
 ` + lifeNavigation + `
 type LifePerson struct{Name string}
 // Route-only fixture has no speech/perception module. Rumour source availability fails closed;
 // full source speech/LOS integration is exercised by region_rumours_generated_test.go.
 func(m LifeModel)Person(id string)*LifePerson{return nil}
 func(a *LifeActor)OnFloor()bool{return a!=nil&&a.Level!=""&&a.Trip==nil}
 type LifeState struct{Actors []LifeActor;Seconds int64;Enabled bool}
 // Causal recording is tested with the full Life kernel; this fixture isolates route privacy.
 func(s *LifeState)ObserveDecisions(){}
 func(s *LifeState)Actor(id string)*LifeActor{for i:=range s.Actors{if s.Actors[i].ID==id{return &s.Actors[i]}};return nil}
 // Movement v2 body (document 87): never capable in this planar fixture, so the cell intent stays under test.
 func(s LifeState)GateModel(m LifeModel)LifeModel{return m};func(m LifeModel)BodyCapable(level string)bool{return false}
 func(s *LifeState)EnsureHeroBody(m LifeModel){};func(s *LifeState)RouteHeroBody(m LifeModel,target Cell,x,y float64,point,run bool,seq uint64)bool{return false}
 func(s *LifeState)MoveRoute(m LifeModel,id,level string,from,to Cell,tick int64)([]Cell,bool){rows:=m.Locations[level].Terrain;space:=Space{Width:len(rows[0]),Height:len(rows),MaxSearchCells:len(rows)*len(rows[0]),Walkable:func(c Cell)bool{return rows[c.Y][c.X]=='.'}};path:=space.Path(from,to,Cell{},0);return path,from==to||len(path)>0}
 `,
		"internal/transport/grpc/pb/game/v1/wire.go": `package pb
 type PocPoint struct{X,Y int32};type PocLifeCommand struct{Action,Destination string}
 type PocLifeView struct{Wayfinding *PocWayfindingView;RegionMap *PocRegionMap}
 type PocWayfindingView struct{LevelId,SelectedDestinationId,SelectedGuideId,SelectedName,SelectedColor,Status,LocalExitId,LocalExitMode,LocalExitName,RejectedDestinationId string;Destinations []*PocWayfindingDestination;Guides []*PocWayfindingGuide;Path []*PocPoint;Reachable bool}
 type PocWayfindingDestination struct{Id,Name,Color,Kind,TravelMode string;X,Y,Number int32;ServiceAvailable bool}
 type PocWayfindingShapePoint struct{X,Y float64;Junction bool};type PocWayfindingGuide struct{HalfWidthU,HalfWidthV float64;Id,Color string;Path []*PocPoint;Shape []*PocWayfindingShapePoint;AxisU,AxisV *PocWayfindingShapePoint;Underlays []*PocPoint;Authored bool}
 type PocRegionMap struct{Id,Name,CurrentNode,CurrentDeck,SelectedDestinationId string;SelectedLocalLinkIds []string;Nodes []*PocRegionNode;Links []*PocRegionLink;Decks []*PocRegionDeck;RumourSources []*PocRegionRumourSource;HeardRumours []*PocRegionRumour}
 type PocRegionRumourSource struct{ActorId,Name string}
 type PocRegionRumour struct{Id,SubjectId,SourceActorId,SourceName,Text,Context,ImageVariant,ObservedNodeId,ObservationKey string;Active,Visited bool}
 type PocRegionNode struct{Id,Name,DeckId,Knowledge,LevelId,Description,Kind,Label string;X,Y,Width,Height float64;Current,CanTravel bool}
 type PocRegionLink struct{Id,FromNode,ToNode,Mode string};type PocRegionDeck struct{Id,Name string;ElevationM float64}
 `,
		"internal/poc/service_reservations.go": fragments,
		"internal/poc/medical_reservations.go": "package poc\n" + medical[medicalStart:medicalEnd],
		"internal/poc/wayfinding.go":           hudTemplate(t, "wayfinding.go.tmpl"), "internal/poc/region.go": hudTemplate(t, "region.go.tmpl"), "internal/poc/movement.go": "package poc\nimport \"angelsdemons/server/internal/behavior\"\n" + move[am:bm],
		"internal/poc/region_rumours.go": hudTemplate(t, "region_rumours.go.tmpl"),
		"internal/poc/host_test.go":      wayfindingServerHarness,
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
type medicalPatient struct{UnitID string}
type medicalState struct{Patients map[string]medicalPatient}
type sanitaryState struct{Users map[string]medicalPatient}
type hostState struct{Sanitary sanitaryState;Medical medicalState;Life behavior.LifeState;LifeExploration map[string]map[int]seenCell;Region regionKnowledge;Epoch string;Paused bool;Tick int64}
type worldModel struct{Life behavior.LifeModel;Wayfinding wayfindingCatalog;Region regionCatalog;Walkable map[string]bool}
type Runtime struct{world worldModel;state hostState;wayfinding wayfindingSession;entered string;syncs int}
func(r *Runtime)localText(locale,key,fallback string)string{if strings.HasPrefix(key,"wayfinding."){return locale+":"+key};return fallback}
func(r *Runtime)lifeExitKnown(e behavior.LifeEdge)bool{return r.wayfindingWalks(e.From,e.Exit)}
func(r *Runtime)lifeEdgeName(locale string,e behavior.LifeEdge)string{return r.regionLevelName(locale,e.To)}
func(r *Runtime)resetInstantTravel(){}
// Game-time motion clock of the full host; route privacy does not depend on it.
func(r *Runtime)lifeMotionTick()int64{return r.state.Tick}
func(r *Runtime)syncLifeHero(){r.syncs++}
func(r *Runtime)heroBodyNow(){}
// No conversation is available in this route-only fixture: preserve fail-closed source privacy.
func(r *Runtime)barkCanSpeak(id string)bool{return false}
func(r *Runtime)lifeViewRuntime()*Runtime{return nil}
func(r *Runtime)sightWithin(from,to behavior.Cell,radius int)bool{return false}
func location(p behavior.Cell)behavior.Cell{return p}

func(r *Runtime)lifeCommand(q *pb.PocLifeCommand)error{if q.Action=="enter"{r.entered=q.Destination;return nil};return r.wayfindingCommand(q.Action,q.Destination)}
func fixture()*Runtime{rows:=[]string{"#########","#.......#","#.###.#.#","#.....#.#","#######.#","#.......#","#########"};r:=&Runtime{};r.world.Walkable=map[string]bool{".":true};r.world.Life.Locations=map[string]behavior.LifeLocation{"hall":{Name:"Hall",Terrain:rows},"out":{Name:"Other room",Terrain:rows},"secret":{Name:"SECRET_ROOM",Terrain:rows}};r.world.Life.Edges=[]behavior.LifeEdge{{ID:"door",From:"hall",To:"out",Mode:"walk",Exit:behavior.Cell{7,1}},{ID:"SECRET_EDGE",From:"hall",To:"secret",Mode:"rail",Exit:behavior.Cell{1,1}}};r.state=hostState{Epoch:"epoch1",Tick:44,Life:behavior.LifeState{Enabled:true,Seconds:100,Actors:[]behavior.LifeActor{{ID:playerActorID,Level:"hall",Position:behavior.Cell{1,1},Goal:behavior.Cell{3,1},Path:[]behavior.Cell{{2,1},{3,1}},MoveStartTick:42,Visits:map[string]int{"hall":1}}}},LifeExploration:map[string]map[int]seenCell{"hall":{}}};for y,row:=range rows{for x:=range row{r.state.LifeExploration["hall"][y*9+x]=seenCell{string(row[x])}}};r.world.Wayfinding=wayfindingCatalog{Destinations:[]wayfindingDestination{{ID:"way.med",NameKey:"wayfinding.med",Color:"#79A99A",Kind:"medical",Level:"hall",At:behavior.Cell{7,5},GuideFrom:&behavior.Cell{1,1},MarkerNumber:7},{ID:"way.hidden",NameKey:"wayfinding.hidden",Color:"#83A7BD",Kind:"sanitary",Level:"hall",At:behavior.Cell{1,5},MarkerNumber:2}}};delete(r.state.LifeExploration["hall"],5*9+1);r.world.Region=regionCatalog{ID:"region",Nodes:[]regionNode{{ID:"node.hall",LevelID:"hall",InitialKnowledge:"visited"},{ID:"node.out",LevelID:"out",InitialKnowledge:"known"},{ID:"node.secret",Name:"SECRET_ROOM",LevelID:"secret",InitialKnowledge:"unknown"}},Links:[]regionLink{{ID:"link.out",FromNode:"node.hall",ToNode:"node.out",InitialKnown:true,LifeEdges:[]string{"door"}},{ID:"SECRET_LINK",FromNode:"node.hall",ToNode:"node.secret",LifeEdges:[]string{"SECRET_EDGE"}}}};r.state.Region=regionKnowledge{Initialized:true,KnownNodes:map[string]bool{"node.hall":true,"node.out":true},KnownLinks:map[string]bool{"link.out":true},Visited:map[string]int64{"node.hall":0}};return r}
func project(r *Runtime)*pb.PocWayfindingView{v:=&pb.PocLifeView{};r.appendWayfinding(v,"en");return v.Wayfinding}
func snapshot(r *Runtime)string{b,_:=json.Marshal(r.state);return string(b)}
func TestReadOnlySelectionPathsAndPrivacy(t *testing.T){r:=fixture();before:=snapshot(r);if err:=r.wayfindingCommand("route_preview","way.med");err!=nil{t.Fatal(err)};v:=project(r);if snapshot(r)!=before||r.syncs!=0||!v.Reachable||v.SelectedName!="en:wayfinding.med"{t.Fatal("selection mutated intent/calendar/knowledge or failed localization")};for i,p:=range v.Path{if r.world.Life.Locations["hall"].Terrain[p.Y][p.X]!='.'{t.Fatal("wall crossing")};if i>0{q:=v.Path[i-1];if behavior.Distance(behavior.Cell{int(p.X),int(p.Y)},behavior.Cell{int(q.X),int(q.Y)})!=1{t.Fatal("noncardinal path")}}};for _,d:=range v.Destinations{if d.Id=="way.hidden"{t.Fatal("unknown destination leaked")};if d.Id=="way.med"&&(d.ServiceAvailable||d.Number!=7){t.Fatal("fake service or unstable sign number")}};raw,_:=json.Marshal(v);if strings.Contains(string(raw),"SECRET"){t.Fatal("private region data leaked")};r.wayfindingCommand("route_clear","");if snapshot(r)!=before||project(r).SelectedDestinationId!=""{t.Fatal("clear cancelled gameplay")}}
func TestServiceAttachmentAndReservationHoldRealMovement(t *testing.T){for _,mode:=range []string{"attachment","reservation","sanitary"}{t.Run(mode,func(t *testing.T){r:=fixture();h:=r.state.Life.Actor(playerActorID);if mode=="attachment"{h.ServiceAttachment="capsule.private"}else if mode=="reservation"{r.state.Medical.Patients=map[string]medicalPatient{playerActorID:{UnitID:"capsule.private"}}}else{r.state.Sanitary.Users=map[string]medicalPatient{playerActorID:{UnitID:"sanitary.private"}}};before:=snapshot(r);if err:=r.wayfindingCommand("route_preview","way.med");err!=nil{t.Fatal(err)};v:=project(r);if !v.Reachable||snapshot(r)!=before{t.Fatal("service changed read-only geographic guidance")};raw,_:=json.Marshal(v);if strings.Contains(string(raw),"capsule.private"){t.Fatal("private service identity leaked into guidance")};if r.wayfindingCommand("route_go","way.med")==nil||snapshot(r)!=before||r.syncs!=0{t.Fatal("Go stole actor from attachment/reservation")}})}}
func TestHiddenBodiesDoNotChangeGuidance(t *testing.T){r:=fixture();r.wayfindingCommand("route_preview","way.med");before:=project(r);r.state.Life.Actors=append(r.state.Life.Actors,behavior.LifeActor{ID:"hiddenNPC",Level:"hall",Position:behavior.Cell{7,3},Path:[]behavior.Cell{{7,4}}});if !reflect.DeepEqual(before,project(r)){t.Fatal("hidden occupancy changed preview")}}
func TestExplicitGoPauseStaleAndAcceptedSegment(t *testing.T){r:=fixture();r.wayfindingCommand("route_preview","way.med");r.state.Paused=true;before:=snapshot(r);if r.wayfindingCommand("route_go","way.med")==nil||snapshot(r)!=before{t.Fatal("paused Go moved")};if r.wayfindingCommand("route_preview","way.med")!=nil{t.Fatal("paused selection blocked")};r.state.Paused=false;if r.wayfindingCommand("route_go","old")==nil{t.Fatal("stale Go accepted")};h:=r.state.Life.Actor(playerActorID);at:=h.Position;if err:=r.wayfindingCommand("route_go","way.med");err!=nil{t.Fatal(err)};if h.Position!=at||h.Path[0]!=(behavior.Cell{2,1})||h.Goal!=(behavior.Cell{7,5})||!h.Manual||r.syncs!=1{t.Fatal("Go bypassed real intent helper or accepted segment")};if project(r).SelectedDestinationId!="way.med"{t.Fatal("Go erased preview")}}
func TestExactMemoryCacheInvalidationAndDiscoveryNumbers(t *testing.T){r:=fixture();r.wayfindingCommand("route_preview","way.med");if !project(r).Reachable{t.Fatal("fixture")};r.state.LifeExploration["hall"][3*9+7]=seenCell{"#"};if v:=project(r);v.Reachable||len(v.Guides)>0{t.Fatal("stale cache crosses changed memory")};r.state.LifeExploration["hall"][3*9+7]=seenCell{"."};if !project(r).Reachable{t.Fatal("same-size memory correction not refreshed")};r.state.LifeExploration["hall"][5*9+1]=seenCell{"."};for _,d:=range project(r).Destinations{if d.Id=="way.med"&&d.Number!=7{t.Fatal("discovery renumbered an existing sign")}};delete(r.state.LifeExploration["hall"],5*9+7);if project(r).SelectedDestinationId!=""{t.Fatal("unknown selected target remained")};r.state.Epoch="loaded";if project(r).SelectedDestinationId!=""{t.Fatal("UI preview survived a real epoch/load")}}
func TestExitAtDoorRemainsReadyAndRejectionAcknowledgesExactRequest(t *testing.T){r:=fixture();h:=r.state.Life.Actor(playerActorID);h.Position=behavior.Cell{7,1};h.Path=nil;h.AdmissionDeniedEdge="door";if r.wayfindingCommand("route_preview","exit:door")!=nil{t.Fatal("known exit selection failed")};v:=project(r);if v.Status!="ready"||!v.Reachable||len(v.Path)!=1{t.Fatal("exit/denied boarding point cannot be explicitly retried")};if r.wayfindingCommand("route_go","exit:door")!=nil||r.entered!="door"{t.Fatal("standing at exit failed explicit enter")};before:=snapshot(r);if r.wayfindingCommand("route_preview","client-request-A")==nil{t.Fatal("unknown request accepted")};v=project(r);if v.Status!="unknown"||v.RejectedDestinationId!="client-request-A"||v.SelectedDestinationId!=""||snapshot(r)!=before{t.Fatal("rejected request lacks exact read-only ack")};if r.wayfindingCommand("route_preview","way.med")!=nil{t.Fatal("subsequent valid selection failed")};if project(r).RejectedDestinationId!=""{t.Fatal("old rejection contaminated next selection")};r.wayfindingCommand("route_clear","");if project(r).RejectedDestinationId!=""{t.Fatal("clear retained rejection")}}
func TestRegionalFirstLegOnlyKnownPhysicalExit(t *testing.T){r:=fixture();before:=snapshot(r);if err:=r.wayfindingCommand("route_preview","out");err!=nil{t.Fatal(err)};v:=project(r);if v.LocalExitId!="door"||v.LocalExitMode!="walk"||!v.Reachable||snapshot(r)!=before{t.Fatal("regional preview not exact known local leg")};if r.wayfindingCommand("route_go","out")!=nil||r.entered!="door"{t.Fatal("Go did not use previewed physical edge")};if r.wayfindingCommand("route_preview","secret")==nil{t.Fatal("unknown regional target accepted")};raw,_:=json.Marshal(project(r));if strings.Contains(string(raw),"SECRET"){t.Fatal("unknown regional target leaked")}}
func TestRegionalExactDestinationCannotOvershootIntoHub(t *testing.T){r:=fixture();r.world.Region.Nodes[2].InitialKnowledge="known";r.state.Region.KnownNodes["node.secret"]=true;r.world.Region.Links=[]regionLink{{ID:"known.station",FromNode:"node.hall",ToNode:"node.out",LifeEdges:[]string{"door"}},{ID:"station.to.hub",FromNode:"node.out",ToNode:"node.secret"},{ID:"known.hub",FromNode:"node.hall",ToNode:"node.secret",LifeEdges:[]string{"SECRET_EDGE"}}};r.state.Region.KnownLinks=map[string]bool{"known.station":true,"station.to.hub":true,"known.hub":true};before:=snapshot(r);if err:=r.wayfindingCommand("route_preview","out");err!=nil{t.Fatal(err)};v:=project(r);if v.LocalExitId!="door"||snapshot(r)!=before{t.Fatal("station request overshot into hub or mutated player")};m:=&pb.PocLifeView{};r.appendRegionMap(m);if m.RegionMap.SelectedDestinationId!="out"||len(m.RegionMap.SelectedLocalLinkIds)!=1||m.RegionMap.SelectedLocalLinkIds[0]!="known.station"{t.Fatalf("map first link disagrees with resolved exit: %+v",m.RegionMap)};if r.wayfindingCommand("route_go","out")!=nil||r.entered!="door"{t.Fatal("Go boarded a different target edge")}}
func TestRegionalLandmarkPrefixStopsAtFirstPlayableRoom(t *testing.T){r:=fixture();rows:=r.world.Life.Locations["out"];r.world.Life.Locations["work"]=rows;r.world.Life.Edges=append(r.world.Life.Edges,behavior.LifeEdge{ID:"station.work",From:"out",To:"work"});r.world.Region.Nodes=append(r.world.Region.Nodes,regionNode{ID:"junction",InitialKnowledge:"known"},regionNode{ID:"node.work",LevelID:"work",InitialKnowledge:"known"});r.state.Region.KnownNodes["node.secret"]=true;r.state.Region.KnownNodes["junction"]=true;r.state.Region.KnownNodes["node.work"]=true;r.world.Region.Links=[]regionLink{{ID:"a.junction",FromNode:"node.hall",ToNode:"junction"},{ID:"b.station",FromNode:"junction",ToNode:"node.out"},{ID:"c.hub",FromNode:"junction",ToNode:"node.secret"},{ID:"d.work",FromNode:"node.out",ToNode:"node.work"}};r.state.Region.KnownLinks=map[string]bool{};for _,link:=range r.world.Region.Links{r.state.Region.KnownLinks[link.ID]=true};if r.wayfindingCommand("route_preview","work")!=nil||project(r).LocalExitId!="door"{t.Fatal("landmark branch wandered through first playable station to hub")}}
func TestRegionalHighlightDoesNotRevealParallelHiddenLink(t *testing.T){r:=fixture();r.world.Region.Links=append(r.world.Region.Links,regionLink{ID:"HIDDEN_PARALLEL_LIFT",FromNode:"node.hall",ToNode:"node.out",LifeEdges:[]string{"unbuilt.lift"}});if r.wayfindingCommand("route_preview","out")!=nil{t.Fatal("known target failed")};v:=&pb.PocLifeView{};r.appendRegionMap(v);raw,_:=json.Marshal(v);if strings.Contains(string(raw),"HIDDEN_PARALLEL_LIFT")||len(v.RegionMap.SelectedLocalLinkIds)!=1||v.RegionMap.SelectedLocalLinkIds[0]!="link.out"{t.Fatal("first-leg highlight leaked/guessed a hidden parallel link")};r.state.Epoch="reloaded";v=&pb.PocLifeView{};r.appendRegionMap(v);if v.RegionMap.SelectedDestinationId!=""||len(v.RegionMap.SelectedLocalLinkIds)>0{t.Fatal("stale UI epoch retained map route")}}
func TestHeardRumourDoesNotUnlockRegionalGuidance(t *testing.T){
 r:=fixture();q:=regionRumour{ID:"report.fixture",SubjectID:"rumoured.place",NodeID:"node.secret",SourceActorID:"absent.source",TextKey:"rumour.text",ContextKey:"rumour.context",ImageVariant:"narrow_stair"};r.world.Region.Rumours=[]regionRumour{q};r.state.Region.Reports=[]regionReportMemory{{ID:q.ID,HeardSecond:100}};r.state.Region.ActiveReports=map[string]string{q.SubjectID:q.ID};before:=snapshot(r)
 v:=&pb.PocLifeView{};r.appendRegionMap(v);raw,_:=json.Marshal(v.RegionMap);if len(v.RegionMap.HeardRumours)!=1||!v.RegionMap.HeardRumours[0].Active||len(v.RegionMap.RumourSources)!=0||strings.Contains(string(raw),"SECRET_ROOM")||strings.Contains(string(raw),"node.secret"){t.Fatal("testimony leaked true place or absent source")};if r.validateRegionRumours(r.state)!=nil{t.Fatal("actual report memory validator rejected fixture")};if r.wayfindingCommand("route_preview","secret")==nil||r.regionCanGoto("secret")||snapshot(r)!=before{t.Fatal("heard report granted real route or changed physical knowledge")}
}
func TestExplicitParallelWalkAndRailRemainDistinct(t *testing.T){r:=fixture();r.world.Life.Edges=append(r.world.Life.Edges,behavior.LifeEdge{ID:"rail.door",From:"hall",To:"out",Mode:"rail",Exit:behavior.Cell{7,1}});r.world.Region.Links=append(r.world.Region.Links,regionLink{ID:"rail.link",FromNode:"node.hall",ToNode:"node.out",LifeEdges:[]string{"rail.door"}});r.state.Region.KnownLinks["rail.link"]=true;for _,id:=range []string{"door","rail.door"}{if r.wayfindingCommand("route_preview","exit:"+id)!=nil||project(r).LocalExitId!=id{t.Fatal("parallel explicit exit selection collapsed")};if r.wayfindingCommand("route_go","exit:"+id)!=nil||r.entered!=id{t.Fatal("parallel explicit exit boarded another mode")}}}
func shapeFixture()*Runtime{r:=fixture();rows:=[]string{"#########","#.......#","#.......#","#.......#","#.......#","#.......#","#########"};r.world.Life.Locations["hall"]=behavior.LifeLocation{Terrain:rows};for y,row:=range rows{for x:=range row{r.state.LifeExploration["hall"][y*9+x]=seenCell{string(row[x])}}};d:=&r.world.Wayfinding.Destinations[0];d.GuideRoute=[]wayfindingGuidePoint{{X:1,Y:1},{X:1.45,Y:1},{X:1.45,Y:5},{X:7,Y:5}};for y:=1;y<=5;y++{for x:=1;x<=2;x++{d.GuideSupport=append(d.GuideSupport,behavior.Cell{X:x,Y:y})}};for x:=3;x<=7;x++{d.GuideSupport=append(d.GuideSupport,behavior.Cell{X:x,Y:5})};return r}
func TestAuthoredShapeIsReadOnlyPresentationAndCachedAcrossTicks(t *testing.T){r:=shapeFixture();before:=snapshot(r);v:=project(r);if snapshot(r)!=before||len(v.Guides)!=1||len(v.Guides[0].Shape)!=4||v.Guides[0].Shape[1].X!=1.45{t.Fatal("authored shape lost precision, leaked private guides or mutated simulation")};first:=v.Guides[0].Shape[0];for tick:=0;tick<30;tick++{r.state.Tick++;r.state.Life.Actors=append(r.state.Life.Actors,behavior.LifeActor{ID:"irrelevant",Level:"out"});next:=project(r);if next.Guides[0].Shape[0]!=first{t.Fatal("same remembered geometry rebuilt paint for tick/occupancy changes")}};if err:=r.wayfindingCommand("route_preview","way.med");err!=nil{t.Fatal(err)};expected:=r.wayfindingPath("hall",behavior.Cell{1,1},behavior.Cell{7,5});v=project(r);if !reflect.DeepEqual(v.Path,wayfindingPoints(expected)){t.Fatal("paint centreline replaced authoritative movement path")};if r.wayfindingCommand("route_go","way.med")!=nil||r.state.Life.Actor(playerActorID).Goal!=(behavior.Cell{7,5}){t.Fatal("authored paint changed normal explicit Go")}}
func TestAuthoredShapeWithholdsWholeLineForUnknownBufferedCell(t *testing.T){for _,cell:=range []behavior.Cell{{X:1,Y:3},{X:2,Y:3}}{r:=shapeFixture();if len(project(r).Guides[0].Shape)!=4{t.Fatal("fixture missing shape")};delete(r.state.LifeExploration["hall"],cell.Y*9+cell.X);v:=project(r);if len(v.Guides)!=1||len(v.Guides[0].Path)<2||len(v.Guides[0].Shape)!=0{t.Fatal("unknown interior/buffer cell leaked partial shape, joined unknown gap or removed safe fallback")};r.state.LifeExploration["hall"][cell.Y*9+cell.X]=seenCell{"#"};if len(project(r).Guides[0].Shape)>0{t.Fatal("remembered wall exposed private true-floor shape")};r.state.LifeExploration["hall"][cell.Y*9+cell.X]=seenCell{"."};if len(project(r).Guides[0].Shape)!=4{t.Fatal("remembered geometry correction did not invalidate suppressed shape")}}}
func TestInfrastructurePaintUnderPropKeepsCollisionAndKnowledge(t *testing.T){
 r:=shapeFixture();at:=behavior.Cell{X:1,Y:3};rows:=r.world.Life.Locations["hall"].Terrain;rows[3]="##......#";r.state.LifeExploration["hall"][3*9+1]=seenCell{"#"};d:=&r.world.Wayfinding.Destinations[0];d.GuideUnderlays=[]behavior.Cell{at};d.GuideFrame=&wayfindingFrame{U:wayfindingGuidePoint{X:1},V:wayfindingGuidePoint{Y:1}};
 v:=project(r);if len(v.Guides[0].Shape)!=4||len(v.Guides[0].Underlays)!=1||v.Guides[0].AxisU==nil||!v.Guides[0].Authored{t.Fatal("known prop substrate was confused with movement floor")};if r.wayfindingWalks("hall",at){t.Fatal("floor paint made prop walkable")};delete(r.state.LifeExploration["hall"],3*9+1);v=project(r);if len(v.Guides[0].Shape)!=0||len(v.Guides[0].Underlays)!=0||v.Guides[0].AxisU!=nil||!v.Guides[0].Authored{t.Fatal("unknown covered-floor support leaked geometry or lost authored intent")}
}
func TestSelectedGuideBindsOnlyPublishedExactPhysicalEdge(t *testing.T){
 r:=shapeFixture();r.wayfindingCommand("route_preview","way.med");if project(r).SelectedGuideId!="way.med"{t.Fatal("local authored selection has no known guide binding")}
 d:=r.world.Wayfinding.Destinations[0];d.ID="way.station";d.EdgeID="door";d.At=behavior.Cell{7,1};d.Kind="exit";d.GuideRoute=[]wayfindingGuidePoint{{X:1,Y:1},{X:7,Y:1}};d.GuideSupport=nil;for x:=1;x<=7;x++{d.GuideSupport=append(d.GuideSupport,behavior.Cell{X:x,Y:1})};r.world.Wayfinding.Destinations=append(r.world.Wayfinding.Destinations,d)
 if err:=r.wayfindingCommand("route_preview","out");err!=nil{t.Fatal(err)};v:=project(r);if v.SelectedGuideId!="way.station"||v.LocalExitId!="door"{t.Fatal("regional target did not select its precise published local edge guide")}
 delete(r.state.LifeExploration["hall"],1*9+4);v=project(r);if v.SelectedGuideId!=""{t.Fatal("unknown guide interior leaked a selected guide ID")}
 r.state.LifeExploration["hall"][1*9+4]=seenCell{"."};if project(r).SelectedGuideId!="way.station"{t.Fatal("discovered complete guide not bound")};r.state.Epoch="reload";if project(r).SelectedGuideId!=""{t.Fatal("selection survived reload")}
}
func TestFixedJoinMetadataPublishesOnlyWithKnownShape(t *testing.T){r:=shapeFixture();d:=&r.world.Wayfinding.Destinations[0];d.GuideJunctions=[]int{1};d.GuideWidth=&wayfindingWidth{U:.03,V:.04};v:=project(r);if !v.Guides[0].Shape[1].Junction||v.Guides[0].HalfWidthU!=.03||v.Guides[0].HalfWidthV!=.04{t.Fatal("fixed physical join/scale absent")};before:=v.Guides[0].Shape[1];r.state.Tick++;if project(r).Guides[0].Shape[1]!=before{t.Fatal("known fixed vertex changed with time")};delete(r.state.LifeExploration["hall"],3*9+1);v=project(r);if len(v.Guides[0].Shape)!=0||v.Guides[0].HalfWidthU!=0||v.Guides[0].HalfWidthV!=0{t.Fatal("hidden graph/scale metadata published without known shape")}}
func TestAuthoredShapeFailsClosedWithoutDerivedSupport(t *testing.T){r:=shapeFixture();r.world.Wayfinding.Destinations[0].GuideSupport=nil;v:=project(r);if len(v.Guides)!=1||len(v.Guides[0].Shape)>0{t.Fatal("unsupported legacy private shape was trusted")};r.state.Epoch="loaded";if len(project(r).Guides[0].Shape)>0{t.Fatal("epoch reset trusted absent support")}}

`
