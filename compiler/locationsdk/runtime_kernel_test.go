package locationsdk

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/behavior"
)

// Exercise the actual emitted Life kernel with the adapter's terrain, spawn and
// cadence. The runtime adapter must not grow a second toy movement/physics loop.
func TestRuntimeAdapterNavigationFeedsExistingGeneratedLifeKernel(t *testing.T) {
	bundle := requireRuntime(t, runtimeTestInput(), runtimeTestOptions())
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	files, err := behavior.Render([]byte(`{"actions":{},"tasks":{}}`), []byte(`{"cases":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, data := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := `package behavior
import("encoding/json";"testing")
func TestSDKRuntimeAdapterKernel(t *testing.T){
 var bundle struct{UnitMM int ` + "`json:\"unit_mm\"`" + `;MovementStepNS int64 ` + "`json:\"movement_step_ns\"`" + `;Levels []struct{
  ID string ` + "`json:\"id\"`" + `;UnitMM int ` + "`json:\"unit_mm\"`" + `;CellMeters float64 ` + "`json:\"cell_meters\"`" + `;Spawn Cell ` + "`json:\"spawn\"`" + `;Terrain []string ` + "`json:\"terrain\"`" + `;WalkableGlyphs []string ` + "`json:\"walkable_glyphs\"`" + `;MoveEveryTicks int64 ` + "`json:\"move_every_ticks\"`" + `;MovementStepNS int64 ` + "`json:\"movement_step_ns\"`" + `
 } ` + "`json:\"levels\"`" + `}
 if err:=json.Unmarshal([]byte(` + strconv.Quote(string(raw)) + `),&bundle);err!=nil{t.Fatal(err)}
 for _,level:=range bundle.Levels{
  walkable:=map[string]bool{};for _,glyph:=range level.WalkableGlyphs{walkable[glyph]=true}
  model:=LifeModel{SecondsPerTick:1,MoveTicks:level.MoveEveryTicks,Walkable:walkable,Locations:map[string]LifeLocation{level.ID:{Name:level.ID,Terrain:level.Terrain}},Actors:[]LifePerson{{ID:"char.hero",Name:"SDK hero",Steps:[]LifeStep{{Level:level.ID,At:level.Spawn,Activity:"rest"}}}}}
  if err:=model.Validate();err!=nil{t.Fatal(err)}
  state:=model.Start();state.ControlledActorID="char.hero";hero:=state.Actor("char.hero");hero.Manual=true;hero.Arrived=false
  goal:=hero.Position;maxDistance:=0
  for y,row:=range level.Terrain{for x:=range row{at:=Cell{X:x,Y:y};if model.Walks(level.ID,at)&&Distance(hero.Position,at)>maxDistance{goal,maxDistance=at,Distance(hero.Position,at)}}}
  route,allowed:=state.MoveRoute(model,hero.ID,level.ID,hero.Position,goal,0)
  if !allowed||len(route)<2{t.Fatalf("%s: missing real route through adapter terrain",level.ID)}
  steps:=len(route);hero.Path=route;hero.Goal=goal;hero.MoveStartTick=0
  moved,saved:=0,false
  for tick:=int64(1);tick<=int64(steps)*model.MoveTicks;tick++{
   before:=state.Actor("char.hero").Position;state.Advance(model,tick);hero=state.Actor("char.hero")
   distance:=Distance(before,hero.Position);if distance>1||!model.Walks(level.ID,hero.Position){t.Fatal("hero teleported or crossed blocked/void geometry")}
   if distance==1{moved++;if tick%model.MoveTicks!=0{t.Fatal("movement ignored the emitted authoritative cadence")}}
   if err:=state.Validate(model,tick);err!=nil{t.Fatal(err)}
   if !saved&&moved>0&&len(hero.Path)>0{data,err:=json.Marshal(state);if err!=nil{t.Fatal(err)};var restored LifeState;if err=json.Unmarshal(data,&restored);err!=nil{t.Fatal(err)};if err=restored.Validate(model,tick);err!=nil{t.Fatal(err)};state=restored;saved=true}
  }
  hero=state.Actor("char.hero")
  if hero.Position!=goal||moved!=steps||!saved||level.UnitMM!=500||level.CellMeters!=.5||moved*level.UnitMM!=steps*500{t.Fatalf("%s: half-meter route/cadence/save did not complete",level.ID)}
  blockedFound:=false
  for y,row:=range level.Terrain{for x:=range row{at:=Cell{X:x,Y:y};if !model.Walks(level.ID,at){before:=hero.Position;if _,allowed:=state.MoveRoute(model,hero.ID,level.ID,before,at,state.Tick);allowed{t.Fatal("blocked/unsupported adapter cell accepted")};if state.Actor("char.hero").Position!=before{t.Fatal("collision query moved the hero")};blockedFound=true;break}};if blockedFound{break}}
  if !blockedFound{t.Fatal("test layout lost its physical obstacle")}
  if level.MovementStepNS!=bundle.MovementStepNS{t.Fatal("client interval differs from emitted Go cadence")}
 }
}
`
	if err := os.WriteFile(filepath.Join(dir, "sdk_runtime_adapter_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sdk_runtime_adapter_kernel\n\ngo 1.25\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestSDKRuntimeAdapterKernel$", "-v", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("emitted Life kernel on real SDK navigation: %v\n%s", err, output)
	} else {
		t.Log(strings.TrimSpace(string(output)))
	}
}
