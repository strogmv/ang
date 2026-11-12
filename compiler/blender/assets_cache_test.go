package blender

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The per-instance verdict cache must reproduce an uncached validation byte for
// byte, and an external glTF buffer is one of its inputs.
func TestPythonArtValidationCacheIsByteIdenticalAndKeyedByBuffers(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	for name, data := range map[string]string{"runtime.py": runtime, "assets.py": assets} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `import json,pathlib,struct,subprocess,sys
root=pathlib.Path(sys.argv[1]);art=root/'art';art.mkdir()
def model(name,points):
    # One triangle in an external buffer beside the .gltf, as an exporter may write it.
    data=b''.join(struct.pack('<fff',*p) for p in points)+struct.pack('<HHH',0,1,2)+b'\0\0'
    (art/(name+'.bin')).write_bytes(data)
    (art/(name+'.gltf')).write_text(json.dumps({'asset':{'version':'2.0'},'scene':0,'scenes':[{'nodes':[0]}],
        'nodes':[{'name':name,'mesh':0}],'meshes':[{'primitives':[{'attributes':{'POSITION':0},'indices':1}]}],
        'buffers':[{'uri':name+'.bin','byteLength':len(data)}],
        'bufferViews':[{'buffer':0,'byteOffset':0,'byteLength':36},{'buffer':0,'byteOffset':36,'byteLength':6}],
        'accessors':[{'bufferView':0,'componentType':5126,'count':3,'type':'VEC3'},{'bufferView':1,'componentType':5123,'count':3,'type':'SCALAR'}]}))
    return str(art/(name+'.gltf'))
# Blender coordinates: x, up, -south. The wall triangle stands on walkable cell (0,0).
wall=model('wall',[(0.1,0,-0.1),(0.4,0,-0.1),(0.1,1,-0.1)])
far=model('far',[(5.1,0,-5.1),(5.4,0,-5.1),(5.1,1,-5.1)])
ident=[[1,0,0,0],[0,1,0,0],[0,0,1,0],[0,0,0,1]]
row=lambda asset,ref:{'required':True,'greybox':False,'asset':asset,'world':ident,'model_ref':ref}
contract={'geometry_contract_sha256':'test','budget':{'triangles':100,'lights':0,'shadowed_lights':0},'lights':[],
    'convention':{'model_reflection':ident},'unit_m':0.5,'tolerance_m':0.02,'legacy':True,
    'walkable':[{'x':0,'y':0,'elevation_m':0,'surface':'floor'}],'reserved_hidden':[],'scene_geometry':{'triangles':0},
    'locators':[{'name':'service.far','kind':'service','instance_id':'far','position_plan_m':[9,0,9]}],
    'identities':{'wall':row(wall,'model.wall'),'far':row(far,'model.far')}}
# The legacy wall conflict is measured once, then accepted as the baseline.
(root/'contract.json').write_text(json.dumps(contract))
r=subprocess.run([sys.executable,'-B',str(root/'assets.py'),'--contract',str(root/'contract.json'),'--out',str(root/'measure'),'--collect-conflicts'],capture_output=True,text=True)
conflicts=[c for c in json.loads((root/'measure/provenance.json').read_text())['validation']['conflicts'] if c['kind']=='walkable']
assert conflicts and r.returncode!=0, r.stdout+r.stderr
contract['known_conflicts']=[{'object_id':c['object_id'],'cell':c['cell'],'layer':c['layer'],'max_intrusion_m':c['max_intrusion_m'],'reason':'test'} for c in conflicts]
(root/'contract.json').write_text(json.dumps(contract))
def run(name,cache,*extra):
    command=[sys.executable,'-B',str(root/'assets.py'),'--contract',str(root/'contract.json'),'--out',str(root/name)]
    if cache:command+=['--cache-dir',str(root/'cache'),*extra]
    r=subprocess.run(command,capture_output=True,text=True);assert r.returncode==0,r.stdout+r.stderr
    return {p.name:p.read_bytes() for p in sorted((root/name).iterdir())},r.stdout
plain,_=run('plain',False);cold,out=run('cold',True);assert 'ANG_ART_CACHE 0 reused; 2 stored' in out,out
warm,out=run('warm',True);assert 'ANG_ART_CACHE 2 reused; 0 stored' in out,out
assert plain==cold==warm
# Refresh (the pipeline's no-cache run) reuses nothing and reports a stored verdict that differs.
fresh,out=run('refresh',True,'--cache-refresh');assert 'ANG_ART_CACHE 0 reused; 0 stored; 0 stored verdicts' in out,out
entry=next((root/'cache').rglob('*.json'));value=json.loads(entry.read_text());value['objects'][0]['triangles']+=1;entry.write_text(json.dumps(value))
fresh,out=run('refresh2',True,'--cache-refresh');assert 'ANG_ART_CACHE 0 reused; 1 stored; 1 stored verdicts' in out,out
assert fresh==plain
# A changed buffer of the far model is validated again; the wall verdict is reused.
model('far',[(5.1,0,-5.1),(5.6,0,-5.1),(5.1,1,-5.1)])
plain,_=run('plain2',False);cached,out=run('cached2',True);assert 'ANG_ART_CACHE 1 reused; 1 stored' in out,out
assert plain==cached and plain!=warm
# The plan prefilter skips only locators the exact test rejects: a triangle that
# enters the locator box by 1e-9 m is still reported, one 1e-9 m outside is not.
r=__import__('runpy').run_path(str(root/'runtime.py'))
class O:
    hide_render=False;name='t'
    def __init__(s,pts):s.pts=pts;s.matrix_world=None
    def get(s,k,d=None):return d
    def evaluated_get(s,_):return s
    def to_mesh(s):return s
    def calc_loop_triangles(s):
        class T:vertices=(0,1,2)
        class V:pass
        s.loop_triangles=[T()];s.vertices=[V() for _ in s.pts]
        for v,p in zip(s.vertices,s.pts):v.co=p
    def to_mesh_clear(s):pass
class M:
    def __matmul__(s,o):return o if isinstance(o,tuple) else s
c=dict(contract);c['walkable']=[];c['locators']=[{'name':'s','kind':'service','position_plan_m':[1.0,0,1.0]}]
radius=max(.05,c['unit_m']/2-c['tolerance_m'])
def edge(offset):
    x=1.0-radius+offset;return O([(x-0.2,0.5,-1.0),(x,0.5,-1.0),(x,1.5,-1.0)])
errors=[];r['validate_geometry'](c,[edge(1e-9)],M(),None,errors);assert errors and 'blocks service' in errors[0],errors
for offset in [-1e-9,-5e-7,-2e-6,-3.0]:
    errors=[];r['validate_geometry'](c,[edge(offset)],M(),None,errors);assert not errors,(offset,errors)
print('art cache PASS')`
	output, err := exec.Command(python, "-B", "-c", script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
