package gamepoc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInspectionLocalizationRunsAfterAuthorization(t *testing.T) {
	s := hudTemplate(t, "inspection.go.tmpl")
	filtered := strings.Index(s, "description,ok:=obj.Describe")
	projected := strings.Index(s, "r.localizedInspection(obj,description,locale)")
	if filtered < 0 || projected < filtered {
		t.Fatal("inspection must authorize perception before translating/projecting prose")
	}
	for _, want := range []string{`locales ...string`, `obj.ID+".name"`, `description.TextKey`, `description.Fragments`, `line.TextKey`, `error.inspection.debug_only`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing generic localized inspection contract %q", want)
		}
	}
	projection := s[strings.Index(s, "func(r *Runtime)localizedInspection("):strings.Index(s, "func(r *Runtime)setInspectionAttribute(")]
	for _, bad := range []string{"obj.Fragments", "obj.Context", "obj.Source", "obj.Facts", "level.poc_", "prop.a1."} {
		if strings.Contains(projection, bad) {
			t.Errorf("localized projection must not enumerate private authoring data or hardcode content: %q", bad)
		}
	}
}

// Execute the actual host projection, actual inspection predicates and actual
// localizer in a tiny module. Stubs supply only unrelated host/protobuf shapes;
// no copy of the authorization or localization algorithm is used in this test.
func TestInspectionLocalizedProjectionNative(t *testing.T) {
	behaviorTemplate, err := os.ReadFile(filepath.Join("..", "behavior", "templates", "inspection.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":                          "module angelsdemons/server\n\ngo 1.23\n",
		"internal/behavior/inspection.go": string(behaviorTemplate),
		"internal/behavior/host_shapes.go": `package behavior
type Cell struct{X,Y int}
type LifeTrip struct{Edge string}
type LifeActor struct{Level string;Position Cell;Trip *LifeTrip}
func Distance(a,b Cell)int{x,y:=a.X-b.X,a.Y-b.Y;if x<0{x=-x};if y<0{y=-y};return x+y}
`,
		"internal/transport/grpc/pb/game/v1/wire.go": `package pb
type PocObserverView struct{Visible bool}
type PocGameState struct{Life *PocLifeView;PlayerView *PocObserverView}
type PocLifeView struct{Inspections []*PocInspectionObject;InspectionAttributes []*PocInspectionAttribute}
type PocInspectionAttribute struct{Id string;Value int32}
type PocInspectionObject struct{Id,Name,LevelId,TextKey,BaseText string;X,Y int32;Fragments []*PocInspectionLine}
type PocInspectionLine struct{Id,TextKey,Text,Sense string}
`,
		"internal/poc/inspection.go":   hudTemplate(t, "inspection.go.tmpl"),
		"internal/poc/localization.go": hudTemplate(t, "localization.go.tmpl"),
		"internal/poc/projection_test.go": `package poc
import("encoding/json";"reflect";"strings";"testing";"angelsdemons/server/internal/behavior";pb "angelsdemons/server/internal/transport/grpc/pb/game/v1")
type hostPoint struct{X,Y int}
type maintenanceUnit struct{ID,Phase string}
type worldModel struct{Localization localizationCatalog;Config struct{DebugPerception bool};Life struct{Inspection behavior.InspectionModel}}
type hostState struct{Life struct{Observer behavior.InspectionObserver;Maintenance struct{Units []maintenanceUnit}}}
type Runtime struct{world worldModel;state hostState;smellReachable bool}
func(r *Runtime)sightLine(a,b hostPoint)bool{return r.smellReachable}
func location(c behavior.Cell)hostPoint{return hostPoint{c.X,c.Y}}
func viewSees(v *pb.PocObserverView,x,y int32)bool{return v!=nil&&v.Visible}
func fixture()*Runtime{
 r:=&Runtime{smellReachable:true};r.world.Localization=localizationCatalog{DefaultLocale:"ru",FallbackLocale:"en",Locales:[]string{"ru","en"}}
 for _,row:=range []struct{key,ru,en string}{
  {"fixture.object.name","RU_NAME","EN_NAME"},{"inspect.fixture.base","RU_BASE","EN_BASE"},
  {"inspect.fixture.etched","RU_ETCHED","EN_ETCHED"},{"inspect.fixture.scent","RU_SCENT","EN_SCENT"},
  {"inspect.fixture.route_secret","RU_SECRET_ROUTE","EN_SECRET_ROUTE"},{"inspect.fixture.active","RU_ACTIVE","EN_ACTIVE"},
  {"error.inspection.debug_only","RU_DEBUG_ONLY","EN_DEBUG_ONLY"},
 }{r.world.Localization.Messages=append(r.world.Localization.Messages,localizationMessage{Key:row.key,Context:"TRANSLATOR_CONTEXT_SECRET",Texts:map[string]string{"ru":row.ru,"en":row.en}})}
 obj:=behavior.InspectionObject{ID:"fixture.object",Level:"fixture.work",At:behavior.Cell{X:2,Y:1},Name:"RAW_NAME",TextKey:"inspect.fixture.base",BaseText:"RAW_BASE",Source:"SOURCE_SECRET",Context:behavior.InspectionContextMetadata{OriginalUse:"ORIGINAL_USE_SECRET",CurrentUse:"CURRENT_USE_SECRET",Condition:"CONDITION_SECRET",AuthorNote:"AUTHOR_NOTE_SECRET"},Facts:map[string]string{"equipment_phase":"active"},Fragments:[]behavior.InspectionFragment{
  {ID:"etched",TextKey:"inspect.fixture.etched",Text:"RAW_ETCHED",Sense:"visual",MaxDistance:2,Requires:map[string]int{"observation":50}},
  {ID:"scent",TextKey:"inspect.fixture.scent",Text:"RAW_SCENT",Sense:"smell",MaxDistance:2,Requires:map[string]int{"smell_sensitivity":30}},
  {ID:"route",TextKey:"inspect.fixture.route_secret",Text:"RAW_SECRET_ROUTE",Sense:"visual",MaxDistance:2,Knowledge:[]string{"private.route"}},
  {ID:"active",TextKey:"inspect.fixture.active",Text:"RAW_ACTIVE",Sense:"visual",MaxDistance:2,When:map[string]string{"equipment_phase":"active"}},
 }}
 r.world.Life.Inspection=behavior.InspectionModel{Objects:[]behavior.InspectionObject{obj}}
 r.state.Life.Observer=behavior.InspectionObserver{Attributes:map[string]int{"observation":0,"smell_sensitivity":0}}
 r.state.Life.Maintenance.Units=[]maintenanceUnit{{ID:"fixture.object",Phase:"stopped"}};return r
}
func project(r *Runtime,h *behavior.LifeActor,visible bool,locale string)*pb.PocGameState{m:=&pb.PocGameState{Life:&pb.PocLifeView{},PlayerView:&pb.PocObserverView{Visible:visible}};r.appendLifeInspection(m,r,h,locale);return m}
func wireJSON(t *testing.T,m *pb.PocGameState)string{t.Helper();b,e:=json.Marshal(m.Life.Inspections);if e!=nil{t.Fatal(e)};return string(b)}
func line(m *pb.PocGameState,id string)*pb.PocInspectionLine{if len(m.Life.Inspections)==0{return nil};for _,f:=range m.Life.Inspections[0].Fragments{if f.Id==id{return f}};return nil}
func TestActualLocalizedFilteredProjection(t *testing.T){
 r:=fixture();h:=&behavior.LifeActor{Level:"fixture.work",Position:behavior.Cell{X:1,Y:1}}
 beforeWorld,_:=json.Marshal(r.world);beforeObserver:=r.state.Life.Observer.Clone()
 en:=project(r,h,true,"en");ru:=project(r,h,true,"ru")
 if len(en.Life.Inspections)!=1||en.Life.Inspections[0].Name!="EN_NAME"||en.Life.Inspections[0].BaseText!="EN_BASE"{t.Fatalf("English projection: %+v",en.Life.Inspections)}
 if ru.Life.Inspections[0].Name!="RU_NAME"||ru.Life.Inspections[0].BaseText!="RU_BASE"{t.Fatal("connection locale was ignored")}
 if len(en.Life.Inspections[0].Fragments)!=0{t.Fatal("weak observer received gated details or a stale authored phase")}
 raw:=wireJSON(t,en)
 for _,secret:=range []string{"route_secret","etched","scent","SECRET","RAW_","Context","Source","Facts","Knowledge","Requires","Condition"}{if strings.Contains(raw,secret){t.Fatalf("private authored key/text/context leaked: %s in %s",secret,raw)}}
 afterWorld,_:=json.Marshal(r.world);if string(beforeWorld)!=string(afterWorld)||!reflect.DeepEqual(beforeObserver,r.state.Life.Observer){t.Fatal("per-connection language changed world/observer state")}
 // The old no-locale host call remains a default-locale projection for fixtures.
 def:=&pb.PocGameState{Life:&pb.PocLifeView{},PlayerView:&pb.PocObserverView{Visible:true}};r.appendLifeInspection(def,r,h);if def.Life.Inspections[0].Name!="RU_NAME"{t.Fatal("default locale compatibility")}
 r.state.Life.Observer.Attributes["observation"]=80;r.state.Life.Observer.Attributes["smell_sensitivity"]=80
 strong:=project(r,h,true,"en");if line(strong,"etched")==nil||line(strong,"etched").Text!="EN_ETCHED"||line(strong,"scent")==nil||line(strong,"scent").Text!="EN_SCENT"||line(strong,"route")!=nil||line(strong,"active")!=nil{t.Fatal("localized fragments bypassed capability/knowledge/live-fact filtering")}
 r.smellReachable=false;blocked:=project(r,h,true,"en");if line(blocked,"scent")!=nil||line(blocked,"etched")==nil{t.Fatal("smell reachability lost during translation")};r.smellReachable=true
 r.state.Life.Observer.Knowledge=[]string{"private.route"};learned:=project(r,h,true,"en");if line(learned,"route")==nil||line(learned,"route").Text!="EN_SECRET_ROUTE"{t.Fatal("authorized learned detail not localized")}
 r.state.Life.Maintenance.Units[0].Phase="active";active:=project(r,h,true,"en");if line(active,"active")==nil||line(active,"active").Text!="EN_ACTIVE"{t.Fatal("current phase not localized")}
}
func TestCurrentVisionLevelTransitAndRangeRemainPrivate(t *testing.T){
 r:=fixture();h:=&behavior.LifeActor{Level:"fixture.work",Position:behavior.Cell{X:1,Y:1}};r.state.Life.Observer.Attributes["observation"]=80
 for _,visible:=range []bool{false}{if len(project(r,h,visible,"en").Life.Inspections)!=0{t.Fatal("unseen object leaked through translation")}}
 h.Level="fixture.other";if len(project(r,h,true,"en").Life.Inspections)!=0{t.Fatal("off-level object leaked")};h.Level="fixture.work"
 h.Trip=&behavior.LifeTrip{Edge:"fixture.walk"};if len(project(r,h,true,"en").Life.Inspections)!=0{t.Fatal("in-transit object leaked")};h.Trip=nil
 h.Position=behavior.Cell{X:20,Y:20};if len(project(r,h,true,"en").Life.Inspections[0].Fragments)!=0{t.Fatal("out-of-range detail localized")}
 err:=r.setInspectionAttribute("observation",90);if err==nil||r.localError("en",err)!="EN_DEBUG_ONLY"{t.Fatal("debug-only rejection must be keyed/localized")}
}
`,
	}
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-count=1", "-v")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual inspection/localization templates: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
