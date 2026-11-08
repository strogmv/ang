package blender

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func testSource() (locationsdk.CompiledLevel, locationsdk.Layout, placement.Manifest, *Pipeline) {
	level := locationsdk.CompiledLevel{ID: "test.room", UnitMM: 500, Width: 4, Depth: 4, Elevation: 6, Terrain: []string{"####", "#..#", "#..#", "####"}, StructuralHash: "source-hash"}
	layout := locationsdk.Layout{ID: level.ID, Floor: []locationsdk.FloorArea{{ID: "floor.test", Rect: locationsdk.Rect{X: 1, Y: 1, W: 2, H: 2}}}}
	manifest := placement.Manifest{LayoutID: level.ID, WorldFrame: placement.WorldFrame, Instances: map[string]placement.Instance{}}
	for _, angle := range []int{0, 90, 180, 270} {
		id := []string{"a", "b", "c", "d"}[angle/90]
		manifest.Instances[id] = placement.Instance{WorldTransform: placement.Pose(locationsdk.Cell{X: 4, Y: 7}, 3, 2, 3, angle), SourceKind: "instance", ModuleID: "kit.test", ModelRef: "model.test"}
	}
	pipeline := &Pipeline{ToleranceM: .02, Instances: map[string]ArtSpec{"floor.test": {Greybox: true}}, Locators: []Locator{{Name: "exit.test", Kind: "exit", PositionPlanM: placement.Vec3{.75, 3, .75}}}}
	return level, layout, manifest, pipeline
}
func TestContractWorldRoundtripAndStrictCUE(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	c, err := Build(level, layout, manifest, pipeline, nil, nil, nil, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	for id, instance := range manifest.Instances {
		got := c.Identities[id].World
		b := instance.WorldTransform.BasisColumns()
		for axis := 0; axis < 3; axis++ {
			for row := 0; row < 3; row++ {
				if math.Abs(got[row][axis]-b[axis][row]) > 1e-12 {
					t.Fatalf("%s changed source basis", id)
				}
			}
			if got[axis][3] != instance.WorldTransform.Position[axis] {
				t.Fatalf("%s moved", id)
			}
		}
	}
	if !c.Identities["floor.test"].Greybox || c.Identities["kit.test"].Required || len(c.Walkable) != 4 || c.FrameM != 12 {
		t.Fatalf("wrong source contract: %+v", c)
	}
	if c.Convention.BlenderToWorld[1][2] != 1 || c.Convention.BlenderToWorld[2][1] != -1 {
		t.Fatal("world axes moved")
	}
	pipeline.Instances["typo"] = ArtSpec{Greybox: true}
	if _, err = Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("unknown ID did not teach author: %v", err)
	}
	delete(pipeline.Instances, "typo")
	pipeline.ToleranceM = 0
	if _, err = Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "tolerance_m") {
		t.Fatalf("missing explicit tolerance accepted: %v", err)
	}
}
func TestReportGuardsGeometryAndAllowsResourceInstallation(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	a, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://source.glb"}, nil, "", "/tmp/client")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://installed.tscn"}, nil, "", "/tmp/client")
	if err != nil {
		t.Fatal(err)
	}
	if a.GeometryContractSHA != b.GeometryContractSHA {
		t.Fatal("installing an adapter invalidates semantic contract")
	}
	report := ArtReport{Schema: "ang.blender-export.v1", GeometryContractSHA: a.GeometryContractSHA}
	report.Validation.Warnings = []string{"review light budget"}
	data, _ := json.Marshal(report)
	if warnings, err := ValidateReport(b, data); err != nil || len(warnings) != 1 {
		t.Fatalf("verified export rejected: %v", err)
	}
	level.StructuralHash = "other"
	changed, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateReport(changed, data); err == nil {
		t.Fatal("stale art report accepted")
	}
	report.Validation.Errors = []string{"object Wall blocks exit"}
	data, _ = json.Marshal(report)
	if _, err = ValidateReport(b, data); err == nil || !strings.Contains(err.Error(), "Wall") {
		t.Fatal("art validation failure hidden")
	}
}
func TestPythonSATAndIdentityDiagnostics(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
r=runpy.run_path(sys.argv[1]);sat=r['triangle_rect']
assert sat([(0,0),(2,0),(0,2)],(.2,.2,.4,.4))
assert not sat([(0,0),(2,0),(0,2)],(1.5,1.5,2,2))
assert sat([(1,0),(1,2),(1,1)],(.5,.5,1.5,1.5))
box=r['triangle_box']
assert box([(0,0,0),(2,0,2),(0,2,2)],(.05,.05,.1),(.1,.1,.2))
assert not box([(0,0,0),(2,0,2),(0,2,2)],(.05,.05,1.5),(.1,.1,1.7))
assert box([(1,0,0),(1,2,0),(1,1,2)],(.9,.9,.5),(1.1,1.1,1))
class Obj(dict):
 def __init__(self,name,**tags): super().__init__(tags);self.name=name;self.children_recursive=[]
class Col(dict):
 def __init__(self,name,objects=(),children=(),**tags): super().__init__(tags);self.name=name;self.all_objects=objects;self.children=children
class Scene: pass
obj=Obj('author mesh');col=Col('Room',objects=[obj],ang_id='room');scene=Scene();scene.collection=Col('root',children=[col]);scene.objects=[obj]
roots,members=r['identity_tags'](scene,{'room':{}});assert members=={'author mesh':'room'}
obj['ang_id']='room'
try: r['identity_tags'](scene,{'room':{}});assert False
except ValueError as e: assert 'author mesh' in str(e) and 'Room' in str(e) and 'duplicate' in str(e)
obj.clear();col['ang_id']='typo'
try: r['identity_tags'](scene,{'room':{}});assert False
except ValueError as e: assert 'Room' in str(e) and 'typo' in str(e) and 'CUE' in str(e)
print('SAT, inherited ID, duplicate root and unknown ID PASS')`
	command := exec.Command(python, "-B", "-c", script, path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestLegacyConflictCUEIsExplicitAndBounded(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	entry := KnownConflict{ObjectID: "original.wall", Cell: locationsdk.Cell{X: 1, Y: 1}, Layer: "floor.test", MaxIntrusionM: .024, Reason: "Approved original source overhang; do not enlarge."}
	pipeline.KnownConflicts = []KnownConflict{entry}
	if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil {
		t.Fatal("new strict level accepted baseline")
	}
	pipeline.Legacy = true
	if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err != nil {
		t.Fatal(err)
	}
	pipeline.KnownConflicts = append(pipeline.KnownConflicts, entry)
	if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil {
		t.Fatal("duplicate exception accepted")
	}
	pipeline.KnownConflicts = []KnownConflict{entry}
	pipeline.KnownConflicts[0].MaxIntrusionM = 2
	if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil {
		t.Fatal("unbounded intrusion baseline accepted")
	}
}

func TestClippedIntrusionAndShrinkingBaseline(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
r=runpy.run_path(sys.argv[1]);clip=r['clip_polygon_box'];depth=r['projected_intrusion'];check=r['check_known_conflicts']
poly=clip([(0,.024,0),(1,.024,0),(1,.024,2)],(0,0,.02),(.5,.5,1.85));assert abs(depth(poly,(0,0,.5,.5))-.024)<1e-12
poly=clip([(0,0,0),(2,0,2),(0,2,2)],(.05,.05,1.5),(.1,.1,1.7));assert poly==[]
assert abs(depth([(0,0,0),(.5,0,0),(.5,.5,0),(0,.5,0)],(0,0,.5,.5))-.25)<1e-12
actual={'object_id':'old.wall','cell':{'x':1,'y':2},'layer':'floor','max_intrusion_m':.024,'kind':'walkable'}
base={k:v for k,v in actual.items() if k!='kind'};base['reason']='Approved source overhang'
contract={'legacy':True,'known_conflicts':[base]};errors=[];warnings=[]
summary=check(contract,[actual],errors,warnings);assert not errors and warnings and summary['accepted_count']==1
actual=dict(actual,max_intrusion_m=.034);errors=[];check(contract,[actual],errors,[]);assert not errors
actual=dict(actual,max_intrusion_m=.03401);errors=[];summary=check(contract,[actual],errors,[]);assert summary['grown_count']==1 and 'GREW' in errors[0]
errors=[];summary=check(contract,[],errors,[]);assert summary['stale_count']==1 and 'STALE' in errors[0]
errors=[];summary=check({'legacy':False,'known_conflicts':[]},[actual],errors,[]);assert summary['new_count']==1 and 'NEW' in errors[0]
consumers=r['art_consumers'];ids={'unused.kit':{'required':False,'model_ref':'kit.model'},'proc':{'required':True,'greybox':False,'model_ref':''}}
assert consumers(ids,'unused.kit')==[]
ids['unused.kit']['model_ref']='';assert consumers(ids,'unused.kit')==[]
ids['real']={'required':True,'greybox':False,'model_ref':'kit.model','module_id':'unused.kit'};assert len(consumers(ids,'unused.kit'))==1
print('Actual clipped depth, strict/new, unchanged, growth/stale baseline and partial consumer scope PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestGeneratedWrapperResolvesActiveArtWithStableSemanticHash(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "raw.glb"), []byte("asset bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://new.tscn"}, nil, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.Identities["a"].AssetBasis != "godot" {
		t.Fatal("missing wrapper basis must already be proper Godot")
	}
	wrapper := "[gd_scene format=3]\n[ext_resource type=\"PackedScene\" path=\"res://raw.glb\" id=\"asset\"]\n[node name=\"Art\" type=\"Node3D\"]\n[node name=\"Source\" parent=\".\" instance=ExtResource(\"asset\")]\nscale = Vector3(1, 1, -1)\n"
	if err = os.WriteFile(filepath.Join(dir, "new.tscn"), []byte(wrapper), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://new.tscn"}, nil, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Identities["a"].Asset != filepath.Join(dir, "raw.glb") || a.GeometryContractSHA != b.GeometryContractSHA {
		t.Fatal("publishing a wrapper failed to resolve or moved semantic contract")
	}
}

func TestPythonConcreteWallZonesAndImportGeometryMath(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	for name, data := range map[string]string{"runtime.py": runtime, "import.py": importer} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `import sys,runpy
sys.path.insert(0,sys.argv[1]);r=runpy.run_path(sys.argv[1]+'/runtime.py');imp=runpy.run_path(sys.argv[1]+'/import.py');role=r['resolve_wall_role']
c={'architecture':{'setting':'dome_interior','wall_caste':'bronze','concrete_role':'concrete.bronze','wall_zones':{'silver_zone':{'caste':'silver','reason':'Explicit mixed station zone'}}},'materials':{'concrete.bronze':{'surface_kind':'concrete','surface_caste':'bronze'},'concrete.silver':{'surface_kind':'concrete','surface_caste':'silver'},'steel':{'surface_kind':'steel'}},'identities':{'wall':{}}}
e=[];o=[];assert role(c,'wall.mesh','wall',{'ang_wall':True},e,o)=='concrete.bronze' and not e
assert role(c,'wall.mesh','wall',{'ang_wall':True,'ang_zone':'silver_zone'},e,o)=='concrete.silver' and not e and o
role(c,'wall.mesh','wall',{'ang_wall':True,'ang_material_role':'steel'},e,[]);assert e
c['architecture']['wall_overrides']={'wall.mesh':{'material_role':'steel','reason':'Functional removable steel access leaf'}};e=[];o=[];role(c,'wall.mesh','wall',{'ang_wall':True,'ang_material_role':'steel'},e,o);assert not e and o[0]['reason']
rows=imp['rectangles']({(1,2),(2,2),(1,3),(2,3)},'floor');assert rows[0]['rect']=={'x':1,'y':2,'w':2,'h':2} and len(rows)==1
assert imp['aligned'](1.5,.5,.02,'root')==3
try:imp['aligned'](1.51,.5,.02,'root');assert False
except ValueError:pass
# Two outward-wound horizontal faces form the exact solid interval; a cut
# doorway has only its upper header interval, never a full-height rectangle.
tri=[[(0,0,0),(1,1,0),(1,0,0)],[(0,0,0),(0,1,0),(1,1,0)],[(0,0,1),(1,0,1),(1,1,1)],[(0,0,1),(1,1,1),(0,1,1)]]
assert imp['vertical_solid_intervals'](tri,.4,.4)==[(0.,1.)]
header=[[(p[0],p[1],p[2]+2.1) for p in t] for t in tri];assert imp['vertical_solid_intervals'](header,.4,.4)==[(2.1,3.1)]
print('Concrete caste defaults, zone override, exception reasons, floor raster spans and closed intervals PASS')`
	output, err := exec.Command(python, "-B", "-c", script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestNavigationRetainsAuthoredFloorIDsWithoutDuplicateDraws(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	level.Navigation = &locationsdk.Navigation{BaseSurface: "actual.base", PlanWidth: 4, PlanDepth: 4, Surfaces: []locationsdk.NavSurface{{ID: "actual.base", Walkable: true, ElevationMM: 3000, SlabMM: 100, Slab: []locationsdk.Rect{{X: 1, Y: 1, W: 2, H: 2}}}}}
	c, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	alias := c.Identities["floor.test"]
	base := c.Identities["actual.base"]
	if !alias.Floor || alias.Required || alias.Layer != "actual.base" || len(alias.Blockout) != 0 {
		t.Fatalf("floor alias lost or duplicates physical draw: %+v", alias)
	}
	if !base.Required || len(base.Blockout) != 1 {
		t.Fatalf("actual floor lost geometry: %+v", base)
	}
}

func TestContractUsesSDKMovementGlyphs(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	level.Terrain = []string{"####", "#.+#", "# .#", "####"}
	c, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Walkable) != 2 {
		t.Fatalf("sight-clear blocked '+' became traversable: %+v", c.Walkable)
	}
	level.Navigation = &locationsdk.Navigation{BaseSurface: "floor.test", PlanWidth: 4, PlanDepth: 4, Surfaces: []locationsdk.NavSurface{{ID: "floor.test", Walkable: true, ElevationMM: 3000, Slab: []locationsdk.Rect{{X: 1, Y: 1, W: 2, H: 2}}}}}
	c, err = Build(level, layout, manifest, pipeline, nil, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Walkable) != 2 {
		t.Fatalf("multideck contract disagrees with movement: %+v", c.Walkable)
	}
	if !locationsdk.IsWalkableTerrainGlyph('.') || locationsdk.IsWalkableTerrainGlyph('+') || locationsdk.IsWalkableTerrainGlyph('#') || locationsdk.IsWalkableTerrainGlyph(' ') {
		t.Fatal("SDK movement policy changed")
	}
}

func TestSharedModuleVariantsHaveDeterministicKitIdentity(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	q := manifest.Instances["a"]
	q.ModelRef = "variant.west"
	manifest.Instances["a"] = q
	q = manifest.Instances["b"]
	q.ModelRef = "variant.east"
	manifest.Instances["b"] = q
	context := CatalogContext{Catalog: locationsdk.Catalog{Modules: []locationsdk.Module{{ID: "kit.test", GeometryID: "geo.test"}}, Geometries: []locationsdk.Geometry{{ID: "geo.test", Width: 3, Depth: 2, Height: 4}}}, ModuleModels: map[string]string{"kit.test": "variant.east"}}
	expected := ""
	for i := 0; i < 60; i++ {
		c, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", "", context)
		if err != nil {
			t.Fatal(err)
		}
		module := c.Identities["kit.test"]
		if module.ModelRef != "variant.east" || module.ModuleID != "kit.test" {
			t.Fatalf("module variant guessed from map order: %+v", module)
		}
		if expected == "" {
			expected = c.GeometryContractSHA
		}
		if c.GeometryContractSHA != expected {
			t.Fatal("equivalent source produced another semantic signature")
		}
	}
}

func TestPortLocatorsConsumeCompiledRotatedApproach(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	level.Ports = []locationsdk.CompiledPort{{InstanceID: "b", ID: "public", Approach: []locationsdk.Cell{{X: 8, Y: 13}, {X: 9, Y: 13}}}}
	pipeline.PortLocators = []PortLocator{{Name: "service.queue", InstanceID: "b", PortID: "public", Kind: "service"}}
	c, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, locator := range c.Locators {
		if locator.Name == "service.queue" {
			found = true
			if locator.PositionPlanM != (placement.Vec3{4.75, 3, 6.75}) {
				t.Fatalf("port was re-rotated or moved: %+v", locator)
			}
		}
	}
	if !found {
		t.Fatal("public port locator missing")
	}
	pipeline.PortLocators[0].PortID = "typo"
	if _, err = Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("unknown port not actionable: %v", err)
	}
}

func TestCoveredBodyObstacleKeepsBoundOwnerObligation(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	level.Navigation = &locationsdk.Navigation{BaseSurface: "base", Surfaces: []locationsdk.NavSurface{{ID: "base", Slab: []locationsdk.Rect{{W: 4, H: 4}}}}, Obstacles: []locationsdk.NavObstacle{{ID: "prop.collision", Rect: locationsdk.Rect{X: 1, Y: 1, W: 1, H: 1}, BottomMM: 3000, TopMM: 4000, BlocksMovement: true, BlocksSight: true}}}
	manifest.Instances["prop.collision"] = placement.Instance{SourceKind: "obstacle", WorldTransform: placement.PlanTransform(placement.Vec3{.5, 3, .5}, 0)}
	f := &godot.Finishing{Replacements: []godot.Replacement{{SurfaceID: "prop.collision", InstanceID: "a", Reason: "Same bound source mesh"}}}
	c, err := Build(level, layout, manifest, pipeline, f, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Identities["prop.collision"].RenderedBy != "a" || c.Identities["a"].RenderedBy != "" {
		t.Fatal("proxy or real art owner obligation confused")
	}
	f.Replacements[0].InstanceID = "missing"
	if _, err = Build(level, layout, manifest, pipeline, f, nil, nil, "", ""); err == nil {
		t.Fatal("missing render owner hid a collision volume")
	}
}
