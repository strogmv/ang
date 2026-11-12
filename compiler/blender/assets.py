"""ANG art validation for CUE-first levels; reads existing glTF, never runs Blender."""
import argparse, base64, hashlib, json, math, os, struct, sys
from pathlib import Path
from types import SimpleNamespace as NS
sys.path.insert(0,str(Path(__file__).resolve().parent))
from runtime import digest, write_json, validate_geometry, check_known_conflicts, installed_triangle_budget

class Point(tuple):
    x=property(lambda self:self[0]);y=property(lambda self:self[1]);z=property(lambda self:self[2])

class Matrix:
    def __init__(self,rows=None):self.rows=rows or [[float(i==j) for j in range(4)] for i in range(4)]
    def __matmul__(self,other):
        a=self.rows
        if isinstance(other,Matrix):return Matrix([[sum(a[i][k]*other.rows[k][j] for k in range(4)) for j in range(4)] for i in range(4)])
        return Point(sum(a[i][j]*other[j] for j in range(3))+a[i][3] for i in range(3))
    def inverse(self):
        a=[list(r)+[float(i==j) for j in range(4)] for i,r in enumerate(self.rows)]
        for j in range(4):
            pivot=max(range(j,4),key=lambda i:abs(a[i][j]))
            a[j],a[pivot]=a[pivot],a[j]
            if abs(a[j][j])<1e-12:raise ValueError('singular model transform')
            scale=a[j][j];a[j]=[v/scale for v in a[j]]
            for i in range(4):
                if i!=j:
                    scale=a[i][j];a[i]=[v-scale*w for v,w in zip(a[i],a[j])]
        return Matrix([r[4:] for r in a])

class MeshObject:
    hide_render=False
    def __init__(self,name,vertices,indices,matrix,materials):
        self.name=name;self.matrix_world=matrix;self.materials=materials
        self.vertices=[NS(co=p) for p in vertices]
        self.loop_triangles=[NS(vertices=tuple(indices[i:i+3])) for i in range(0,len(indices),3)]
    def get(self,key,default=None):return default
    def evaluated_get(self,_):return self
    def to_mesh(self):return self
    def calc_loop_triangles(self):pass
    def to_mesh_clear(self):pass

def node_matrix(node):
    if 'matrix' in node:return Matrix([[node['matrix'][j*4+i] for j in range(4)] for i in range(4)])
    x,y,z,w=node.get('rotation',[0,0,0,1]);scale=node.get('scale',[1,1,1]);p=node.get('translation',[0,0,0])
    n=math.sqrt(x*x+y*y+z*z+w*w)
    if n==0:raise ValueError('zero glTF quaternion')
    x,y,z,w=[v/n for v in [x,y,z,w]]
    a=[[1-2*(y*y+z*z),2*(x*y-z*w),2*(x*z+y*w),p[0]], [2*(x*y+z*w),1-2*(x*x+z*z),2*(y*z-x*w),p[1]], [2*(x*z-y*w),2*(y*z+x*w),1-2*(x*x+y*y),p[2]], [0,0,0,1]]
    for i in range(3):
        for j in range(3):a[i][j]*=scale[j]
    return Matrix(a)

def read_asset(path,root,instance,matrix,source_hashes):
    path=path.resolve();source_hashes[str(path)]=digest(path)
    raw=path.read_bytes();binary=None
    if path.suffix=='.glb':
        magic,version,length=struct.unpack_from('<III',raw)
        if magic!=0x46546c67 or version!=2 or length!=len(raw):raise ValueError(f'{path}: invalid glTF2 container')
        pos=12;g=None
        while pos<len(raw):
            size,kind=struct.unpack_from('<II',raw,pos);chunk=raw[pos+8:pos+8+size];pos+=8+size
            if kind==0x4e4f534a:g=json.loads(chunk)
            elif kind==0x004e4942:binary=chunk
    else:g=json.loads(raw)
    buffers=[]
    for spec in g.get('buffers',[]):
        uri=spec.get('uri')
        if uri is None:
            if binary is None:raise ValueError(f'{path}: missing binary buffer')
            buffers.append(binary)
        elif uri.startswith('data:'):buffers.append(base64.b64decode(uri.split(',',1)[1]))
        else:
            if '://' in uri or Path(uri).is_absolute() or '..' in Path(uri).parts:raise ValueError(f'{path}: external buffer must stay beside the model')
            p=path.parent/uri;source_hashes[str(p)]=digest(p);buffers.append(p.read_bytes())
    widths={'SCALAR':1,'VEC2':2,'VEC3':3,'VEC4':4,'MAT4':16};types={5120:'b',5121:'B',5122:'h',5123:'H',5125:'I',5126:'f'}
    def accessor(index):
        a=g['accessors'][index]
        if 'sparse' in a:raise ValueError(f'{path}: sparse mesh accessor needs an evaluated art export')
        view=g['bufferViews'][a['bufferView']];fmt='<'+types[a['componentType']]*widths[a['type']];size=struct.calcsize(fmt)
        data=buffers[view['buffer']];offset=view.get('byteOffset',0)+a.get('byteOffset',0);stride=view.get('byteStride',size)
        return [struct.unpack_from(fmt,data,offset+i*stride) for i in range(a['count'])]
    objects=[]
    def visit(index,parent):
        node=g['nodes'][index];transform=parent@node_matrix(node)
        if 'mesh' in node:
            mesh=g['meshes'][node['mesh']]
            for part,primitive in enumerate(mesh['primitives']):
                name=instance+'/'+node.get('name',mesh.get('name','mesh'+str(node['mesh'])))+'/'+str(part)
                if primitive.get('mode',4)!=4:raise ValueError(f'object {name!r}: export this primitive as triangles')
                vertices=accessor(primitive['attributes']['POSITION'])
                indices=[v[0] for v in accessor(primitive['indices'])] if 'indices' in primitive else list(range(len(vertices)))
                if len(indices)%3:raise ValueError(f'object {name!r}: incomplete triangle')
                mat=g.get('materials',[])[primitive['material']].get('name','') if 'material' in primitive else ''
                objects.append(MeshObject(name,vertices,indices,transform,[mat]))
        for child in node.get('children',[]):visit(child,transform)
    for node in g['scenes'][g.get('scene',0)]['nodes']:visit(node,matrix)
    return objects

class InstanceCache:
    """Verdicts of single CUE instances, keyed by everything one verdict reads.

    A verdict depends on the bound art bytes and its absolute path (it is recorded
    in sources), the instance ID, pose and basis, the level's walkable cells,
    locators and reserved volumes, this validator's code and the Python build.
    The level-wide checks (known conflicts, budgets, attachments) always run.
    An instance with errors is never stored, so it is validated on every run.
    With refresh, nothing is reused: every verdict is computed and stored, and a
    stored verdict that differs from the fresh one is counted as a mismatch."""
    SCHEMA='ang.art-instance-cache.v1'
    def __init__(self,folder,contract,refresh=False):
        here=Path(__file__).resolve().parent
        geometry={name:contract.get(name) for name in ['unit_m','tolerance_m','walkable','locators','reserved_hidden']}
        geometry['model_reflection']=contract['convention']['model_reflection']
        self.folder=Path(folder);self.refresh=refresh;self.hits=0;self.stored=0;self.mismatches=[]
        self.base=self.sha({'schema':self.SCHEMA,'python':sys.version,'code':{name:digest(here/name) for name in ['assets.py','runtime.py']},'geometry':geometry})
    @staticmethod
    def sha(value):
        return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    def key(self,instance,row,asset,asset_sha):
        return self.sha({'base':self.base,'instance':instance,'world':row['world'],'asset_basis':row.get('asset_basis'),'asset':str(asset.resolve()),'asset_sha256':asset_sha})
    def path(self,key):
        return self.folder/'ang-art'/key[:2]/(key+'.json')
    def load(self,key,asset):
        if self.refresh:return None
        path=self.path(key)
        try:
            entry=json.loads(path.read_text())
            if entry['schema']!=self.SCHEMA or entry['key']!=key:return None
            # External glTF buffers are read beside the model: their bytes are inputs too.
            if any(name!=str(asset.resolve()) and (not Path(name).is_file() or digest(name)!=value) for name,value in entry['sources']):return None
            os.utime(path)
        except (OSError,ValueError,KeyError,TypeError):return None
        self.hits+=1
        return entry
    def store(self,key,sources,objects):
        path=self.path(key)
        if self.refresh and path.is_file():
            try:old=json.loads(path.read_text())
            except (OSError,ValueError):old={}
            if old.get('sources')==json.loads(json.dumps(sources)) and old.get('objects')==objects:return
            self.mismatches.append(key)
        try:
            path.parent.mkdir(parents=True,exist_ok=True)
            temporary=path.with_name('.'+path.name+'.'+str(os.getpid()))
            temporary.write_text(json.dumps({'schema':self.SCHEMA,'key':key,'sources':sources,'objects':objects}))
            os.replace(temporary,path);self.stored+=1
        except OSError as error:
            print('ANG_ART_CACHE not stored:',error,file=sys.stderr)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--contract',required=True);parser.add_argument('--out',required=True)
    parser.add_argument('--attachments-existing');parser.add_argument('--collect-conflicts',action='store_true')
    parser.add_argument('--cache-dir',help='reuse the verdicts of unchanged instances; omit to validate every instance')
    parser.add_argument('--cache-refresh',action='store_true',help='with --cache-dir: reuse nothing, store every fresh verdict and count stored ones that differ')
    args=parser.parse_args();contract=json.loads(Path(args.contract).read_text());out=Path(args.out);out.mkdir(parents=True,exist_ok=True)
    cache=InstanceCache(args.cache_dir,contract,args.cache_refresh) if args.cache_dir else None
    errors=[];missing=[];warnings=[];sources={};objects=[];components=[];all_conflicts=[];source_triangles={};proxy_triangles=0
    report={'schema':'ang.blender-export.v1','source_kind':'cue_bound_art','contract_sha256':digest(args.contract),'geometry_contract_sha256':contract['geometry_contract_sha256'],'components':components,'validation':{'errors':errors,'missing_art':missing,'warnings':warnings,'conflicts':all_conflicts},'sources':sources,'budget':contract['budget']}
    reflection=Matrix(contract['convention']['model_reflection'])
    for key,row in sorted(contract['identities'].items()):
        if not row['required'] or row['greybox'] or (row.get('rendered_by') and not row.get('model_ref')):continue
        if row.get('procedural') and not row.get('model_ref'):
            batch=[]
            cube_indices=[0,2,1,0,3,2,4,5,6,4,6,7,0,1,5,0,5,4,1,2,6,1,6,5,2,3,7,2,7,6,3,0,4,3,4,7]
            for i,box in enumerate(row['blockout']):
                x,south,up=box['center_blender_m'];w,d,h=box['size_blender_m']
                vertices=[(x+dx*w/2,up+dz*h/2,-south-dy*d/2) for dx,dy,dz in [(-1,-1,-1),(1,-1,-1),(1,1,-1),(-1,1,-1),(-1,-1,1),(1,-1,1),(1,1,1),(-1,1,1)]]
                obj=MeshObject(key+'/procedural/'+str(i),vertices,cube_indices,Matrix(),[row['material_role']]);batch.append(obj)
                all_conflicts.extend(validate_geometry(contract,[obj],Matrix(),None,errors,instance_id=key))
            components.append({'id':key,'model_ref':'','source':'CUE/procedural','triangles':sum(len(o.loop_triangles) for o in batch),'objects':[{'name':o.name,'triangles':len(o.loop_triangles),'materials':o.materials} for o in batch]})
            proxy_triangles+=sum(len(o.loop_triangles) for o in batch)
            continue
        asset=Path(row.get('asset',''))
        if not asset.is_file():
            missing.append(f'CUE instance {key!r}: no bound art; set its existing kit asset, an authored procedural role or explicit greybox with reason')
            continue
        world=Matrix(row['world'])
        if row.get('asset_basis')=='godot':world=world@reflection
        asset_sha=digest(asset)
        entry_key=cache.key(key,row,asset,asset_sha) if cache else None
        hit=cache.load(entry_key,asset) if cache else None
        if hit is not None:
            for name,value in hit['sources']:sources[name]=value
            for item in hit['objects']:all_conflicts.extend(item['conflicts'])
            components.append({'id':key,'model_ref':row['model_ref'],'source':str(asset),'sha256':asset_sha,'triangles':sum(item['triangles'] for item in hit['objects']),'objects':[{'name':item['name'],'triangles':item['triangles'],'materials':item['materials']} for item in hit['objects']]})
            source_triangles[key]=sum(item['triangles'] for item in hit['objects'])
            continue
        read={}
        try:batch=read_asset(asset,asset.parent,key,world,read)
        except (ValueError,KeyError,IndexError,struct.error) as e:
            sources.update(read)
            errors.append(f'CUE instance {key!r}, source {str(asset)!r}: {e}');continue
        sources.update(read)
        if not batch:missing.append(f'CUE instance {key!r}: model has no mesh, fix the bound art {str(asset)!r}')
        verdicts=[];failed=False
        for obj in batch:
            before=len(errors)
            conflicts=validate_geometry(contract,[obj],Matrix(),None,errors,instance_id=key)
            failed=failed or len(errors)>before
            # A copy before check_known_conflicts annotates the rows.
            if cache:verdicts.append({'name':obj.name,'triangles':len(obj.loop_triangles),'materials':list(obj.materials),'conflicts':json.loads(json.dumps(conflicts))})
            all_conflicts.extend(conflicts)
        components.append({'id':key,'model_ref':row['model_ref'],'source':str(asset),'sha256':asset_sha,'triangles':sum(len(o.loop_triangles) for o in batch),'objects':[{'name':o.name,'triangles':len(o.loop_triangles),'materials':o.materials} for o in batch]})
        source_triangles[key]=sum(len(o.loop_triangles) for o in batch)
        objects.extend(batch)
        if cache and batch and not failed:cache.store(entry_key,list(read.items()),verdicts)
    report['validation']['known_conflicts']=check_known_conflicts(contract,all_conflicts,errors,warnings)
    if contract.get('light_metrics_status')!='cue_declared':
        warnings.append('Light budget UNMEASURED: CUE has no authored light list; declared count zero is not a complete scene measurement. Supply the actual producer/native counters before acceptance.')
    report['light_metrics_status']=contract.get('light_metrics_status','unmeasured')
    triangle_budget=installed_triangle_budget(contract,source_triangles,warnings)
    triangle_budget['validation_proxy_triangles_excluded']=proxy_triangles
    report['triangle_budget']=triangle_budget
    report['triangle_metrics_status']=triangle_budget['status']
    totals={'triangles':triangle_budget['triangles'],'lights':len(contract['lights']),'shadowed_lights':sum(bool(l['shadows']) for l in contract['lights'])};report['totals']=totals
    for key,value in totals.items():
        if value>contract['budget'][key]:warnings.append(f'pipeline.budget.{key}: {value} exceeds {contract["budget"][key]}; review content or the explicit authoring budget. No silent cap')
    attachments=json.loads(Path(args.attachments_existing).read_text()) if args.attachments_existing else {'schema':'ang.model-attachments.v1','source_frame':'model-local','sources':{},'models':{}}
    attachments.setdefault('instances',{})
    for loc in contract['locators']:
        key=loc.get('instance_id');row=contract['identities'].get(key,{})
        if not row:continue
        nodes=attachments['models'].setdefault(row['model_ref'],[]) if row.get('model_ref') else attachments['instances'].setdefault(key,[])
        if any(v['name']==loc['name'] for v in nodes):continue
        x,y,z=loc['position_plan_m'];point=Matrix(row['world']).inverse()@Point((x,y,-z))
        nodes.append({'name':loc['name'],'position':list(point),'source_path':'CUE/'+key+'/'+loc['name']})
    attachments['sources'].update(sources)
    write_json(out/'provenance.json',report);write_json(out/'models.json',{});write_json(out/'model-attachments.json',attachments)
    if errors or missing:raise ValueError('ANG authoring validation failed:\n'+'\n'.join(errors+missing))
    if cache:print('ANG_ART_CACHE',cache.hits,'reused;',cache.stored,'stored;',len(cache.mismatches),'stored verdicts differed from fresh ones',*cache.mismatches[:5])
    print('ANG_CUE_ART_PASS',len(components),'instances;',totals)

if __name__=='__main__':main()
