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

func TestMechanismPreserveNodesIsAuthoredAndPortable(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	pipeline.Instances["model.test"] = ArtSpec{PreserveNodes: true}
	c, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://mechanism.glb"}, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !c.Identities["model.test"].PreserveNodes {
		t.Fatal("mechanism export policy lost")
	}
	pipeline.Instances["model.test"] = ArtSpec{}
	static, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://mechanism.glb"}, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if static.Identities["model.test"].PreserveNodes || c.GeometryContractSHA == static.GeometryContractSHA {
		t.Fatal("node policy must invalidate the cached exporter contract")
	}
}

func TestReservedHiddenVolumesAreStrictAndInvalidateArt(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	pipeline.ReservedHidden = []HiddenReserve{{ID: "cavity", Min: [3]float64{1, 2, -.5}, Max: [3]float64{2, 3, -.4}, Reason: "empty reserved cable trough"}}
	c, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ReservedHidden) != 1 {
		t.Fatal("reserved cavity lost")
	}
	pipeline.ReservedHidden[0].Max[2] = -.6
	if _, err = Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil {
		t.Fatal("inverted hidden cavity accepted")
	}
}

func TestReservedHiddenChecksActualTriangles(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
from types import SimpleNamespace as N
r=runpy.run_path(sys.argv[1])
class I:
 def __matmul__(self,v):return v
class O:
 name='fixture';hide_render=False;matrix_world=I()
 def __init__(self,tri):self.tri=tri
 def get(self,k,d=None):return d
 def evaluated_get(self,d):return self
 def to_mesh(self):return N(vertices=[N(co=p) for p in self.tri],loop_triangles=[N(vertices=[0,1,2])],calc_loop_triangles=lambda:None)
 def to_mesh_clear(self):pass
contract={'unit_m':.5,'tolerance_m':.02,'walkable':[],'locators':[],'reserved_hidden':[{'id':'empty','min_m':[0,0,0],'max_m':[1,1,1]}]}
errors=[];rows=r['validate_geometry'](contract,[O([(.2,.2,-.2),(.8,.2,-.2),(.2,.8,-.2)])],I(),None,errors)
assert errors and rows[0]['kind']=='reserved_hidden' and 'empty' in errors[0]
contract['reserved_hidden'][0].update(min_m=[.05,.05,1.5],max_m=[.1,.1,1.7])
errors=[];rows=r['validate_geometry'](contract,[O([(0,0,0),(2,2,0),(0,2,-2)])],I(),None,errors)
assert not errors and not rows, 'AABB overlap alone is not occupation'
print('Reserved cavity triangle occupancy and sloped AABB false-positive PASS')`
	if out, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestReservedHiddenRejectsEnclosedSolidAndProceduralDraw(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	pipeline.ReservedHidden = []HiddenReserve{{ID: "empty", Min: [3]float64{-.1, -.1, -.1}, Max: [3]float64{.1, .1, .1}, Reason: "reserved cavity"}}
	finish := &godot.Finishing{Materials: map[string]godot.Material{"fixture.steel": {Resource: "res://fixture-steel.tres", Provenance: "Valid material for the reserved-volume negative fixture."}}, Architecture: &godot.Architecture{Setting: "exterior", Draws: []godot.ArchitectureDraw{{ID: "solid", Kind: "trim", MaterialRole: "fixture.steel", Reason: "Actual procedural solid for reserved-volume rejection.", Position: [3]float64{0, 0, 0}, Size: [3]float64{2, 2, 2}}}}}
	if _, err := Build(level, layout, manifest, pipeline, finish, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "reserved_hidden") {
		t.Fatalf("solid procedural draw swallowed cavity: %v", err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
from types import SimpleNamespace as N
r=runpy.run_path(sys.argv[1]);vertices=[(-1,-1,-1),(1,-1,-1),(1,1,-1),(-1,1,-1),(-1,-1,1),(1,-1,1),(1,1,1),(-1,1,1)]
indices=[0,2,1,0,3,2,4,5,6,4,6,7,0,1,5,0,5,4,1,2,6,1,6,5,2,3,7,2,7,6,3,0,4,3,4,7];triangles=[[vertices[i] for i in indices[n:n+3]] for n in range(0,len(indices),3)]
assert all(not r['triangle_box'](tri,(-.1,-.1,-.1),(.1,.1,.1)) for tri in triangles), 'fixture must miss every surface'
assert r['closed_mesh_contains'](triangles,(0,0,0))
assert not r['closed_mesh_contains'](triangles,(3,0,0))
class I:
 def __matmul__(self,v):return v
class O:
 name='solid';hide_render=False;matrix_world=I()
 def get(self,k,d=None):return d
 def evaluated_get(self,d):return self
 def to_mesh(self):return N(vertices=[N(co=p) for p in vertices],loop_triangles=[N(vertices=indices[n:n+3]) for n in range(0,len(indices),3)],calc_loop_triangles=lambda:None)
 def to_mesh_clear(self):pass
contract={'unit_m':.5,'tolerance_m':.02,'walkable':[],'locators':[],'reserved_hidden':[{'id':'empty','min_m':[-.1,-.1,-.1],'max_m':[.1,.1,.1]}]}
errors=[];conflicts=r['validate_geometry'](contract,[O()],I(),None,errors)
assert errors and 'encloses' in errors[0] and conflicts[0].get('containment_witness_plan_m')==[0,0,0]
print('Closed solid containment without triangle crossing rejected; procedural solid rejected')`
	if raw, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
}

func TestReservedHiddenRejectsFinalFloorAndCornerPrisms(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	level.Navigation = &locationsdk.Navigation{BaseSurface: "floor.test", Surfaces: []locationsdk.NavSurface{{ID: "floor.test", ElevationMM: 3000, SlabMM: 250, Slab: []locationsdk.Rect{{W: 4, H: 4}}}}}
	// The original dock mistake: the reserve overlaps only50mm at the bottom
	// of an ordinary slab, so no model mesh or architecture.draws can catch it.
	pipeline.ReservedHidden = []HiddenReserve{{ID: "floor-negative", Min: [3]float64{.6, .6, 2.76}, Max: [3]float64{.7, .7, 2.8}, Reason: "Must not contain the bottom of a250mm floor slab."}}
	if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "reserved_hidden") {
		t.Fatalf("floor slab swallowed hidden reserve: %v", err)
	}
	// Inspect the final roof rather than the original slab: real clearance works.
	roof := &godot.Finishing{FloorRecesses: []godot.FloorRecess{{ID: "roof", SurfaceID: "floor.test", OwnerInstance: "a", Rect: [4]float64{0, 0, 2, 2}, CoverThicknessM: .03, Reason: "Actual thin roof clears the hidden volume."}}}
	if _, err := Build(level, layout, manifest, pipeline, roof, nil, nil, "", ""); err != nil {
		t.Fatalf("final roof was ignored: %v", err)
	}
	corner := &godot.Finishing{FloorOpenings: []godot.FloorOpening{{ID: "octagonal-frame", SurfaceID: "floor.test", OwnerInstance: "a", Rect: [4]float64{.5, .5, 1.5, 1.5}, CornerCutM: .2, Reason: "Actual chamfered frame."}}}
	pipeline.ReservedHidden[0] = HiddenReserve{ID: "corner-negative", Min: [3]float64{.51, .51, 2.8}, Max: [3]float64{.54, .54, 2.9}, Reason: "Must not contain the retained triangular floor corner."}
	if _, err := Build(level, layout, manifest, pipeline, corner, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "reserved_hidden") {
		t.Fatalf("floor corner swallowed hidden reserve: %v", err)
	}
	// The same corner AABB also covers part of the aperture; that empty region
	// must remain clear instead of receiving a false-positive box intersection.
	pipeline.ReservedHidden[0].Min = [3]float64{.65, .65, 2.8}
	pipeline.ReservedHidden[0].Max = [3]float64{.68, .68, 2.9}
	if _, err := Build(level, layout, manifest, pipeline, corner, nil, nil, "", ""); err != nil {
		t.Fatalf("corner AABB treated as occupied triangle: %v", err)
	}
	// Ordinary SDK obstacles are emitted solids too, not just architecture draws.
	level.Navigation.Obstacles = []locationsdk.NavObstacle{{ID: "fixture.obstacle", Rect: locationsdk.Rect{W: 1, H: 1}, BottomMM: 3000, TopMM: 4000, BlocksMovement: true, BlocksSight: true, Style: "panel", Reason: "Actual procedural obstacle."}}
	pipeline.ReservedHidden[0] = HiddenReserve{ID: "obstacle-negative", Min: [3]float64{.1, .1, 3.1}, Max: [3]float64{.2, .2, 3.2}, Reason: "Must not contain the generated obstacle."}
	if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil || !strings.Contains(err.Error(), "reserved_hidden") {
		t.Fatalf("SDK obstacle swallowed hidden reserve: %v", err)
	}
}

func TestSelectiveMotionAndSpatialBatchingContract(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	spec := ArtSpec{PreserveNodeNames: []string{"leaf.*", "control"}, BatchPartitions: []MeshPartition{{Name: "body.a", Axis: "x", Min: -2, Max: 2}, {Name: "body.b", Axis: "x", Min: 3, Max: 7}}}
	pipeline.Instances["model.test"] = spec
	first, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://mechanism.glb"}, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	row := first.Identities["model.test"]
	if len(row.PreserveNodeNames) != 2 || len(row.BatchPartitions) != 2 || row.BatchPartitions[1].Name != "body.b" {
		t.Fatal("selective export policy lost", row)
	}
	spec.PreserveNodeNames = []string{"leaf.*"}
	pipeline.Instances["model.test"] = spec
	other, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://mechanism.glb"}, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if other.GeometryContractSHA == first.GeometryContractSHA {
		t.Fatal("changing preserved nodes must invalidate cached export")
	}
	for _, bad := range []ArtSpec{
		{PreserveNodeNames: []string{""}}, {PreserveNodeNames: []string{"leaf", "leaf"}},
		{BatchPartitions: []MeshPartition{{Name: "body", Axis: "x", Min: 2, Max: 2}}},
		{BatchPartitions: []MeshPartition{{Name: "body", Axis: "xy", Min: 0, Max: 2}}},
		{BatchPartitions: []MeshPartition{{Name: "body", Axis: "x", Min: 0, Max: 2}, {Name: "other", Axis: "x", Min: 1, Max: 3}}},
		{BatchPartitions: []MeshPartition{{Name: "body", Axis: "x", Min: 0, Max: 2}, {Name: "body", Axis: "x", Min: 3, Max: 4}}},
	} {
		pipeline.Instances["model.test"] = bad
		if _, err := Build(level, layout, manifest, pipeline, nil, nil, nil, "", ""); err == nil {
			t.Fatalf("invalid batch policy accepted: %+v", bad)
		}
	}
}

func TestPythonSelectiveBatchingRejectsMissingNodesAndAmbiguousFrames(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
r=runpy.run_path(sys.argv[1]);keep=r['selected_node_names'];partition=r['batch_partition']
assert keep(['leaf.L','leaf.R','frame','control'],['leaf.*','control'])=={'leaf.L','leaf.R','control'}
try: keep(['frame'],['leaf.*']);assert False
except ValueError as e: assert 'matches no source node' in str(e)
parts=[{'name':'car.a','axis':'x','min':-2,'max':2},{'name':'car.b','axis':'x','min':3,'max':7}]
assert partition((0,0,0),parts)=='car.a'
assert partition((5,0,0),parts)=='car.b'
for x in [2.5,8]:
 try: partition((x,0,0),parts);assert False
 except ValueError as e: assert 'belongs to 0 groups' in str(e)
try: partition((0,0,0),[parts[0],parts[0]]);assert False
except ValueError as e: assert 'belongs to 2 groups' in str(e)
print('selective identities and disjoint moving frames PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestRegisteredArtifactResourceDoesNotChangeGeometryContract(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	a, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://kit/shared.tscn"}, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(level, layout, manifest, pipeline, nil, map[string]string{"model.test": "res://models/duplicate.tscn"}, nil, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if a.Identities["model.test"].AssetResource != "res://kit/shared.tscn" {
		t.Fatal("registered PackedScene resource lost")
	}
	if a.GeometryContractSHA != b.GeometryContractSHA {
		t.Fatal("resource relocation changed geometry contract")
	}
}

func TestPythonArtifactReuseRequiresUnchangedNamedMotionFrames(t *testing.T) {
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
r=runpy.run_path(sys.argv[1]);signature=r['hierarchy_signature'];unchanged=r['source_pose_unchanged']
I=[[1,0,0,0],[0,1,0,0],[0,0,1,0],[0,0,0,1]]
class Identity:
 def __matmul__(self,other):return other
class Node:
 def __init__(self,name,kind,parent=None):self.name=name;self.type=kind;self.parent=parent;self.matrix_local=[row[:]for row in I];self.matrix_world=[row[:]for row in I]
hinge=Node('hinge','EMPTY');leaf=Node('leaf','MESH',hinge);slot=Node('weight.slot','EMPTY',hinge)
objects=[hinge,leaf,slot];first=signature(objects,Identity());row={'preserve_node_names':['hinge','weight.slot']};source={'source_pose_signature':first,'source_artifact_pose_signature':'canonical.graph'}
assert unchanged(source,row,first,'canonical.graph')
closed_vertices_world=[line[:]for line in leaf.matrix_world]
# Shift the hinge and compensate the child's local pose: same closed mesh,
# different pivot and motion frame. Geometry alone must not authorize reuse.
hinge.matrix_world[0][3]=.125;leaf.matrix_local[0][3]=-.125
assert leaf.matrix_world==closed_vertices_world
assert not unchanged(source,row,signature(objects,Identity()),'canonical.graph')
hinge.matrix_world[0][3]=0;leaf.matrix_local[0][3]=0;leaf.parent=None
assert not unchanged(source,row,signature(objects,Identity()),'canonical.graph')
leaf.parent=hinge;slot.matrix_local[1][3]=.25
assert not unchanged(source,row,signature(objects,Identity()),'canonical.graph')
assert not unchanged({},row,first,'canonical.graph')
assert not unchanged(source,row,first,'different.asset.graph')
print('same closed geometry, changed pivot/parent/text-slot frames reject artifact reuse PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestPythonStaticBatchingNeverDropsAuthoredColourOrSecondUV(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
from types import SimpleNamespace as Mesh
check=runpy.run_path(sys.argv[1])['check_batch_attributes']
check(Mesh(uv_layers=[],color_attributes=[]),'flat material')
check(Mesh(uv_layers=['UVMap'],color_attributes=[]),'textured material')
check(Mesh(uv_layers=['UV0','UV1'],color_attributes=[]),'two UV sets are copied exactly')
try: check(Mesh(uv_layers=['UV0'],color_attributes=['paint']),'painted authored panel');assert False
except ValueError as e: assert 'painted authored panel' in str(e) and 'preserve this named node' in str(e)
# Every UV set is written back per corner, in order, with its defaults.
ns=runpy.run_path(sys.argv[1])
class Layer:
 def __init__(s,name,n): s.name=name;s.data=[Mesh(uv=None) for _ in range(n)];s.active_render=False;s.active_clone=False
class Layers(list):
 active_index=-1
 def new(s,name): layer=Layer(name,3);s.append(layer);return layer
target=Mesh(uv_layers=Layers(),loops=[Mesh(index=i,vertex_index=i) for i in range(3)])
first=[(0.0,0.0),(1.0,0.0),(0.0,1.0)];second=[(0.25,0.5),(0.75,0.5),(0.25,0.125)]
ns['write_batch_uv_layers'](target,(('UV0','UV1'),1,0,1),[first,second])
assert [l.name for l in target.uv_layers]==['UV0','UV1']
assert [d.uv for d in target.uv_layers[0].data]==first and [d.uv for d in target.uv_layers[1].data]==second
assert target.uv_layers.active_index==1 and target.uv_layers[0].active_render and target.uv_layers[1].active_clone and not target.uv_layers[1].active_render
source=Mesh(uv_layers=[Mesh(name='UV0',active_render=True,active_clone=False),Mesh(name='UV1',active_render=False,active_clone=True)])
source.uv_layers=type('L',(list,),{'active_index':1})(source.uv_layers)
assert ns['mesh_uv_layout'](source)==(('UV0','UV1'),1,0,1)
print('every UV set kept exactly; authored vertex colour requires explicit preservation PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestPythonEqualDummyPBRNeverMergesDifferentCUEMaterialRoles(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
from types import SimpleNamespace as Mat
key=runpy.run_path(sys.argv[1])['material_batch_key']
def material(name):return Mat(name=name,diffuse_color=[.5,.5,.5,1],roughness=1,metallic=0,use_backface_culling=False,use_nodes=False)
a=material('ANG_ROLE__concrete.bronze');b=material('ANG_ROLE__concrete.copper')
assert key(a)!=key(b)
assert key(a)==key(material(a.name))
a=material('source.grey.a');b=material('source.grey.b');assert key(a)==key(b)
b.roughness=.75;assert key(a)!=key(b)
b.roughness=1
assert key(a,{'source.grey.a':'bronze','source.grey.b':'copper'})!=key(b,{'source.grey.a':'bronze','source.grey.b':'copper'})
print('identical dummy PBR preserves distinct CUE roles; equal ordinary materials can batch PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestMaterialBindingsArePartOfTheAuthoringContract(t *testing.T) {
	level, layout, manifest, pipeline := testSource()
	finish := &godot.Finishing{Materials: map[string]godot.Material{
		"bronze": {Color: [4]float64{.5, .5, .5, 1}, Roughness: 1, Provenance: "Fixture bronze role"},
		"copper": {Color: [4]float64{.5, .5, .5, 1}, Roughness: 1, Provenance: "Fixture copper role"},
	}, MaterialBindings: map[string]string{"ordinary.a": "bronze", "ordinary.b": "copper"}}
	a, err := Build(level, layout, manifest, pipeline, finish, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.MaterialBindings["ordinary.a"] != "bronze" || a.MaterialBindings["ordinary.b"] != "copper" {
		t.Fatal("CUE bindings lost before material batching")
	}
	finish.MaterialBindings["ordinary.a"] = "copper"
	b, err := Build(level, layout, manifest, pipeline, finish, nil, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.GeometryContractSHA == b.GeometryContractSHA {
		t.Fatal("changing the semantic role of a source alias must invalidate an art export")
	}
	if a.MaterialBindings["ordinary.a"] != "bronze" {
		t.Fatal("authoring contract retained a mutable alias map")
	}
}

func TestPythonBatchPolicyUsesOnlyUsedAliasesAndRejectsLostBoundNames(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "export.py"), []byte("# stable fixture program\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
r=runpy.run_path(sys.argv[1]);policy=r['batch_policy_signature'];check=r['check_retained_material_aliases']
row={'preserve_node_names':['leaf'],'batch_partitions':[]};names=['ordinary.a','ordinary.b']
a=policy(row,names,{'ordinary.a':'bronze','irrelevant':'copper'})
b=policy(row,names,{'ordinary.a':'bronze','irrelevant':'gold'})
assert a==b
assert a!=policy(row,names,{'ordinary.a':'copper','irrelevant':'copper'})
assert a!=policy(row,names,{'ordinary.a':'bronze','ordinary.b':'copper'})
check(names,['ordinary.a'],{'irrelevant':'gold'})
check(names,['ordinary.a'],{'ordinary.a':'bronze'})
try: check(names,['ordinary.a'],{'ordinary.b':'copper'});assert False
except ValueError as e: assert 'ordinary.b' in str(e) and 're-export the original authoring source' in str(e)
print('only used aliases affect reuse; a newly bound lost alias cannot silently reuse merged art PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
