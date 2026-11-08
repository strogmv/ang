package blender

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTriangleBudgetCountsSharedArtPerPlacementAndEmittedFloor(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	path := filepath.Join(t.TempDir(), "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys
count=runpy.run_path(sys.argv[1])['installed_triangle_budget']
contract={'identities':{
 'library':{'required':False,'model_ref':'lamp'},
 'lamp.a':{'required':True,'model_ref':'lamp'},
 'lamp.b':{'required':True,'model_ref':'lamp'},
 'floor':{'required':True,'model_ref':'','procedural':True,'blockout':[{'triangles':12}]},
 'replaced.wall':{'required':True,'model_ref':'','rendered_by':'lamp.a'},
 },'scene_geometry':{'triangles':48}}
warnings=[];result=count(contract,{'library':216},warnings)
assert result['triangles']==480 and result['bound_models_lod0']==432 and result['emitter_primitives']==48 and not warnings, result
assert set(result['consumers'])=={'lamp.a','lamp.b'}
# A missing bound source must be reported, never silently charged zero.
warnings=[];contract['identities']['lost']={'required':True,'model_ref':'missing'}
result=count(contract,{'library':216},warnings)
assert result['status']=='partial_static_level' and warnings and 'lost' in warnings[0]
# Distinct library sources for a shared model cannot disagree undetected.
warnings=[];contract['identities']['duplicate']={'required':False,'model_ref':'lamp'}
result=count(contract,{'library':216,'duplicate':222},warnings)
assert result['bound_models_lod0']==0 and any('conflicting' in w for w in warnings)
# An isolated model export is not labelled as a complete level count.
warnings=[];result=count(contract,{'library':216},warnings,True)
assert result['status']=='partial_export' and result['emitter_primitives']==0
print('Shared placement, exact emitted floor, missing source and partial export budget PASS')`
	output, err := exec.Command(python, "-B", "-c", script, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

// Exercise the generated Python module boundary without requiring bpy imports.
// This detects a runtime helper wired into an exporter but omitted from imports.
func TestPythonExportRuntimeHelperBindings(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	for name, source := range map[string]string{"runtime.py": runtime, "export.py": exporter, "assets.py": assets} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `import ast, pathlib, sys
root=pathlib.Path(sys.argv[1])
helpers={n.name for n in ast.parse((root/'runtime.py').read_text()).body if isinstance(n,(ast.FunctionDef,ast.ClassDef))}
for name in ['export.py','assets.py']:
 tree=ast.parse((root/name).read_text())
 imported={a.name for n in tree.body if isinstance(n,ast.ImportFrom) and n.module=='runtime' for a in n.names}
 local={n.name for n in tree.body if isinstance(n,(ast.FunctionDef,ast.ClassDef))}
 called={n.func.id for n in ast.walk(tree) if isinstance(n,ast.Call) and isinstance(n.func,ast.Name)}
 assert not (called & helpers)-imported-local, (name, (called & helpers)-imported-local)
`
	output, err := exec.Command(python, "-B", "-c", script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
