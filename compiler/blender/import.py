"""ANG Blender-first geometry import. No source save and no engine/world snapshot."""
import argparse
import json
import math
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from runtime import digest, fail, identity_tags, inherited_properties, clip_polygon_box, projected_intrusion, resolve_wall_role, validate_geometry, write_json


def polygon_area(points):
    return abs(sum(a[0]*b[1]-a[1]*b[0] for a, b in zip(points, points[1:]+points[:1]))) / 2


def rectangles(cells, prefix):
    """Deterministic row spans; exact supported cells, never a bounding rectangle."""
    rows = {}
    for x, y in cells:
        rows.setdefault(y, []).append(x)
    spans = []
    for y, columns in sorted(rows.items()):
        columns = sorted(set(columns));start = previous = columns[0]
        for x in columns[1:]+[None]:
            if x != previous+1:
                spans.append({'id': prefix+'.'+str(len(spans)), 'rect': {'x': start, 'y': y, 'w': previous-start+1, 'h': 1}, 'reason': 'Supported by the ID-tagged Blender floor, imported without extending its boundary.'})
                start = x
            previous = x
    # Preserve a single exact rectangular floor ID when possible.
    if spans:
        width = spans[0]['rect']['w'];x = spans[0]['rect']['x'];y = spans[0]['rect']['y']
        if all(row['rect']['x']==x and row['rect']['w']==width and row['rect']['y']==y+i for i, row in enumerate(spans)):
            return [{'id': prefix, 'rect': {'x': x, 'y': y, 'w': width, 'h': len(spans)}, 'reason': spans[0]['reason']}]
    return spans


def aligned(value, unit, tolerance, label):
    cell = round(value/unit)
    tolerance = min(tolerance,1e-5)
    if abs(value-cell*unit) > tolerance:
        fail(f'{label}: {value:g} m is not on the {unit:g} m SDK grid, tolerance {tolerance:g} m; align this Blender datum')
    return cell


def vertical_solid_intervals(triangles, x, y):
    """Signed vertical intersections fill closed mesh volumes and retain holes."""
    hits = []
    # Avoid counting both triangles on a triangulated face diagonal. The offset
    # is far below Blender float32/source tolerance and does not enlarge cells.
    x += 1.234567e-7
    y += 2.345678e-7
    for a,b,c in triangles:
        determinant = (b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0])
        if abs(determinant)<1e-12:
            continue
        u = ((x-a[0])*(c[1]-a[1])-(y-a[1])*(c[0]-a[0]))/determinant
        v = ((b[0]-a[0])*(y-a[1])-(b[1]-a[1])*(x-a[0]))/determinant
        if u < -1e-10 or v < -1e-10 or u+v > 1+1e-10:
            continue
        height = a[2]+u*(b[2]-a[2])+v*(c[2]-a[2])
        hits.append((height,1 if determinant>0 else -1))
    grouped = []
    for height,direction in sorted(hits):
        if grouped and abs(grouped[-1][0]-height)<1e-7:
            grouped[-1][1] += direction
        else:
            grouped.append([height,direction])
    winding,start,intervals = 0,None,[]
    for height,direction in grouped:
        previous = winding
        winding += direction
        if previous==0 and winding!=0:
            start = height
        elif previous!=0 and winding==0:
            intervals.append((start,height))
    return intervals if winding==0 else []


def bind_module_identity(known, key, module, source_name):
    if not key or not module:
        return
    if module not in known or not known[module].get('module_id'):
        fail(f'{source_name}: ang_module={module!r} is not a module in the published CUE kit catalog')
    if key not in known:
        known[key] = dict(known[module],required=True,kind='instance')
    elif known[key].get('module_id') not in ('',module,None):
        fail(f'{source_name}: ang_module={module!r} contradicts the CUE binding {known[key]["module_id"]!r}; update that semantic binding or use a new instance ID')


def protect_source_poses(contract, objects, locators, source_to_world, depsgraph):
    """Protect actual source poses before raster/coarse-cell compilation errors."""
    protected = dict(contract)
    protected['walkable'] = []  # New authored obstacles are legal at import.
    protected['locators'] = locators
    errors = []
    validate_geometry(protected, objects, source_to_world, depsgraph, errors)
    if errors:
        fail('Blender source protected poses:\n'+'\n'.join(errors))


def import_geometry(scene, contract, layout_id, life_unit_mm=0):
    import bpy
    from mathutils import Matrix
    properties = inherited_properties(scene)
    known = dict(contract['identities'])
    # New floor/wall geometry is explicitly class-tagged before its first CUE
    # compilation; semantic kit IDs must already be defined in CUE.
    def architectural(col):
        key = col.get('ang_id')
        module = col.get('ang_module')
        bind_module_identity(known,key,module,f'collection {col.name!r}')
        if key and key not in known and (col.get('ang_floor') or col.get('ang_wall')):
            known[key] = {'kind': 'floor' if col.get('ang_floor') else 'obstacle', 'required': True, 'model_ref': '', 'module_id': '', 'module_size_cells': [0,0,0]}
        if key and key not in known:
            known[key] = {'kind':'authored_mesh','required':True,'model_ref':str(key),'module_id':'','module_size_cells':[0,0,0]}
        for child in col.children:
            architectural(child)
    architectural(scene.collection)
    for obj in scene.objects:
        key = obj.get('ang_id')
        module = properties[obj.name].get('ang_module')
        bind_module_identity(known,key,module,f'object {obj.name!r}')
        if key and key not in known and (properties[obj.name].get('ang_floor') or properties[obj.name].get('ang_wall')):
            known[key] = {'kind': 'floor' if properties[obj.name].get('ang_floor') else 'obstacle', 'required': True, 'model_ref': '', 'module_id': '', 'module_size_cells': [0,0,0]}
        if key and key not in known and obj.type == 'MESH':
            known[key] = {'kind':'authored_mesh','required':True,'model_ref':str(key),'module_id':'','module_size_cells':[0,0,0]}
    _, membership = identity_tags(scene, known)
    unit, tolerance = contract['unit_m'], contract['tolerance_m']
    b2g = Matrix(contract['convention']['blender_to_world'])
    reflection = Matrix(contract['convention']['model_reflection'])
    locators = []
    expected = {(l.get('instance_id',''),l['name']):l for l in contract['locators']}
    for obj in scene.objects:
        if not obj.get('ang_locator'):
            continue
        key = str(obj.get('ang_id') or membership.get(obj.name,''));name = str(obj['ang_locator'])
        previous = expected.get((key,name))
        kind = str(obj.get('ang_locator_kind') or (previous['kind'] if previous else 'anchor'))
        p = b2g @ obj.matrix_world.translation
        locators.append({'name':name,'instance_id':key,'kind':kind,'position_plan_m':[p[0],p[1],-p[2]]})
    groups, datums, root_objects = {}, {}, {}
    for obj in scene.objects:
        key = membership.get(obj.name)
        if key is None or properties[obj.name].get('ang_library'):
            continue
        if obj.get('ang_pose_datum'):
            if key in datums:
                fail(f'object {obj.name!r}: duplicate CUE pose datum for {key!r}')
            datums[key] = obj
        if obj.get('ang_id') and not obj.get('ang_locator'):
            root_objects[key] = obj
        if obj.type == 'MESH' and not obj.hide_render and not obj.get('ang_ignore_reason'):
            groups.setdefault(key, []).append(obj)
    floors, wall_meshes, instances, extras, models, overrides = [], [], [], [], {}, []
    wall_roles, object_zones, object_tiers = {}, {}, {}
    wall_errors = []
    protected_objects = []
    elevations = []
    floor_layers = {}
    object_layers = {}
    depsgraph = bpy.context.evaluated_depsgraph_get()
    for key, objects in sorted(groups.items()):
        info = known[key]
        if info.get('derived'):
            continue
        if info['kind'] == 'kit' and all(properties[o.name].get('ang_library') for o in objects):
            continue
        triangles = []
        for obj in objects:
            layer=properties[obj.name].get('ang_layer') or properties[obj.name].get('ang_deck') or info.get('layer')
            if layer is not None:
                object_layers[obj.name]=str(layer)
        floor = info.get('floor') or info['kind']=='floor' or any(properties[o.name].get('ang_floor') for o in objects)
        if not floor:
            protected_objects.extend(objects)
        wall = any(properties[o.name].get('ang_wall') or properties[o.name].get('ang_material_role')=='wall' for o in objects)
        for obj in objects:
            ev = obj.evaluated_get(depsgraph);mesh = ev.to_mesh();mesh.calc_loop_triangles();matrix = b2g @ ev.matrix_world
            current = [tuple((matrix @ mesh.vertices[i].co)[j] for j in range(3)) for i in range(len(mesh.vertices))]
            triangles.extend([[(current[i][0], -current[i][2], current[i][1]) for i in tri.vertices] for tri in mesh.loop_triangles])
            ev.to_mesh_clear()
        if floor:
            top = max(p[2] for triangle in triangles for p in triangle)
            elevation = aligned(top, unit, tolerance, 'floor '+key+' elevation');elevations.append(elevation)
            layer = str(properties[objects[0].name].get('ang_layer') or properties[objects[0].name].get('ang_deck') or info.get('layer') or key)
            floor_layers[key] = {'layer':layer,'elevation_m':top,'objects':[o.name for o in objects]}
            top_triangles = [t for t in triangles if all(abs(p[2]-top)<1e-5 for p in t)]
            if not top_triangles:
                fail(f'floor {key!r}: SDK requires a flat horizontal top; split sloped geometry into an explicit supported type')
            supported, partial = set(), []
            xmin = math.floor(min(p[0] for t in top_triangles for p in t)/unit);xmax = math.ceil(max(p[0] for t in top_triangles for p in t)/unit)
            ymin = math.floor(min(p[1] for t in top_triangles for p in t)/unit);ymax = math.ceil(max(p[1] for t in top_triangles for p in t)/unit)
            for x in range(xmin, xmax):
                for y in range(ymin, ymax):
                    area = 0
                    for triangle in top_triangles:
                        clipped = clip_polygon_box(triangle, (x*unit,y*unit,top-1e-5), ((x+1)*unit,(y+1)*unit,top+1e-5))
                        if clipped:
                            area += polygon_area([(p[0],p[1]) for p in clipped])
                    if area >= unit*unit-1e-6:
                        supported.add((x,y))
                    elif area > 1e-6:
                        partial.append((x,y))
            if partial:
                fail(f'floor {key!r}: partial grid cells {partial[:8]}; align the floor boundary to the {unit:g} m grid instead of creating unsupported walkable cells')
            if xmin<0 or ymin<0:
                fail(f'floor {key!r}: negative SDK cells; place the source plan at nonnegative east/south coordinates')
            floors.extend(rectangles(supported, key))
            continue
        if wall or (not info.get('module_id') and info['kind']=='authored_mesh'):
            wall_meshes.append((key, objects, triangles))
        if wall:
            for obj in objects:
                props = properties[obj.name]
                role = resolve_wall_role(contract,obj.name,key,props,wall_errors,overrides)
                if role:
                    wall_roles[obj.name] = role
                if props.get('ang_zone'):
                    object_zones[obj.name] = str(props['ang_zone'])
                if props.get('ang_tier') or props.get('ang_caste'):
                    object_tiers[obj.name] = str(props.get('ang_tier') or props['ang_caste'])
        datum = datums.get(key) or root_objects.get(key)
        if datum is None and info['kind'] in ('authored_mesh','obstacle'):
            # A collection has no transform: unique world-authored art uses
            # the canonical world origin and exports its actual coordinates.
            datum = type('WorldOrigin', (), {'name':key, 'matrix_world':Matrix.Identity(4)})()
        if datum is None:
            fail(f'art group {key!r}: add an EMPTY ang_pose_datum at its real pivot (or tag a root object), so Blender-first import can derive its pose')
        world = b2g @ datum.matrix_world @ b2g.inverted()
        east, up, south = world.translation[0], world.translation[1], -world.translation[2]
        yaw = math.degrees(math.atan2(world[0][2],world[0][0])) % 360
        module = info.get('module_id')
        if module:
            rotation = round(yaw/90)*90 % 360
            if abs((yaw-rotation+180)%360-180)>.01:
                fail(f'object {datum.name!r}: SDK module {module!r} requires a quarter turn; current yaw {yaw:g} degrees')
            width, depth, _ = info.get('module_size_cells', [0,0,0])
            if rotation==90:east -= depth*unit
            elif rotation==180:east -= width*unit;south -= depth*unit
            elif rotation==270:south -= width*unit
            instances.append({'id': key, 'module_id': module, 'at': {'x': aligned(east,unit,tolerance,key+' X'), 'y': aligned(south,unit,tolerance,key+' Y')}, 'rotation': rotation, 'reason': 'ID kit instance pose imported from the Blender source.', 'fate': {'policy':'keep'}})
        else:
            extras.append({'instance_id': key, 'model_ref': info.get('model_ref') or key, 'source_kind': info.get('kind') or 'authored_mesh', 'position_plan_m':[east,up,south], 'rotation_degrees':yaw, 'reason':'ID art pose imported from the Blender source.'})
        models[key] = info.get('model_ref') or key
    protect_source_poses(contract,protected_objects,locators,b2g,depsgraph)
    if wall_errors:
        fail('\n'.join(wall_errors))
    if not floors:
        fail('no ID-tagged floor: retain the CUE floor IDs or tag the floor root ang_floor=true and give it an ang_id')
    if len(set(elevations))!=1:
        fail('mixed floor layers need verified SDK deck/connector authoring; no floor was flattened: '+ '; '.join(f'{key} layer={row["layer"]!r} elevation={row["elevation_m"]:g}m objects={row["objects"]}' for key,row in floor_layers.items()))
    elevation = elevations[0];floor_height = elevation*unit
    obstacles = []
    replacements = []
    base_surface = contract.get('base_surface_id') or 'base'
    supported_cells = {(x,y) for area in floors for y in range(area['rect']['y'],area['rect']['y']+area['rect']['h']) for x in range(area['rect']['x'],area['rect']['x']+area['rect']['w'])}
    for key, objects, triangles in wall_meshes:
        by_cell = {}
        triangles_by_cell = {}
        for triangle in triangles:
            xmin = math.floor(min(p[0] for p in triangle)/unit);xmax = math.floor(max(p[0] for p in triangle)/unit)
            ymin = math.floor(min(p[1] for p in triangle)/unit);ymax = math.floor(max(p[1] for p in triangle)/unit)
            for x in range(xmin,xmax+1):
                for y in range(ymin,ymax+1):
                    if (x,y) not in supported_cells:
                        continue
                    triangles_by_cell.setdefault((x,y),[]).append(triangle)
                    rect = (x*unit,y*unit,(x+1)*unit,(y+1)*unit)
                    clipped = clip_polygon_box(triangle,(rect[0]+tolerance,rect[1]+tolerance,-1e6),(rect[2]-tolerance,rect[3]-tolerance,1e6))
                    if not clipped:
                        continue
                    bottom, top = min(p[2] for p in clipped)-floor_height, max(p[2] for p in clipped)-floor_height
                    if top-bottom>1e-5:
                        by_cell.setdefault((x,y),[]).append((bottom,top))
        for (x,y), surfaces in triangles_by_cell.items():
            solid = vertical_solid_intervals(surfaces,(x+.5)*unit,(y+.5)*unit)
            by_cell.setdefault((x,y),[]).extend((bottom-floor_height,top-floor_height) for bottom,top in solid if top-bottom>1e-5)
        for (x,y), intervals in sorted(by_cell.items()):
            merged = []
            for bottom,top in sorted(intervals):
                if merged and bottom <= merged[-1][1]+tolerance:
                    merged[-1][1] = max(top,merged[-1][1])
                else:
                    merged.append([bottom,top])
            for index,(bottom,top) in enumerate(merged):
                if top<=tolerance or bottom>=1.85:
                    continue
                bottom=max(0,bottom)
                obstacle_id=key+'.cell.'+str(x)+'.'+str(y)+'.'+str(index)
                replacements.append({'surface_id':obstacle_id,'instance_id':key,'reason':'Navigation/sight interval is covered by the same ID-tagged source mesh; suppress only the duplicate SDK draw.'})
                obstacles.append({'id':obstacle_id,'surface':base_surface,'rect':{'x':x,'y':y,'w':1,'h':1},'bottom_mm':round(bottom*1000),'top_mm':round(top*1000),'blocks_movement':top>tolerance and bottom<1.85,'blocks_sight':True,'style':'wall' if any(properties[o.name].get('ang_wall') for o in objects) else 'panel','reason':'Actual ID mesh interval imported from Blender; opening gaps remain absent.'})
    geometry = {'source_kind':'blend','source_sha256':digest(bpy.data.filepath),'layout_id':layout_id,'width':max(contract.get('plan_width',0),max(r['rect']['x']+r['rect']['w'] for r in floors)), 'depth':max(contract.get('plan_depth',0),max(r['rect']['y']+r['rect']['h'] for r in floors)), 'elevation':elevation,'floor':floors,'instances':instances,'obstacles':obstacles,'replacements':replacements,'extras':extras,'locators':locators,'model_refs':models,'wall_material_overrides':overrides,'wall_material_roles':wall_roles,'object_zones':object_zones,'object_tiers':object_tiers,'object_layers':object_layers,'floor_layers':floor_layers}
    if obstacles:
        geometry['base_surface_id'] = base_surface
    if life_unit_mm:
        ratio = life_unit_mm/(unit*1000)
        if ratio < 1 or abs(ratio-round(ratio))>1e-9:
            fail('--life-unit-mm must be an integer multiple of the SDK unit')
        ratio = round(ratio)
        if geometry['width']%ratio or geometry['depth']%ratio:
            fail('Blender floor bounds cannot be represented by complete server cells; align level dimensions to --life-unit-mm')
        walkable = {(x,y) for area in floors for y in range(area['rect']['y'],area['rect']['y']+area['rect']['h']) for x in range(area['rect']['x'],area['rect']['x']+area['rect']['w'])}
        for obstacle in obstacles:
            if obstacle['blocks_movement']:
                r = obstacle['rect']
                walkable.difference_update((x,y) for y in range(r['y'],r['y']+r['h']) for x in range(r['x'],r['x']+r['w']))
        rows = []
        for y in range(geometry['depth']//ratio):
            row = ''
            for x in range(geometry['width']//ratio):
                count = sum((x*ratio+dx,y*ratio+dy) in walkable for dx in range(ratio) for dy in range(ratio))
                if count not in (0,ratio*ratio):
                    fail(f'floor near server cell ({x},{y}) has only {count}/{ratio*ratio} SDK cells; --life-unit-mm {life_unit_mm} cannot represent this partial walking area')
                row += '.' if count else '#'
            rows.append(row)
        geometry.update({'life_unit_mm':life_unit_mm,'legacy_terrain':rows,'legacy_width':geometry['width']//ratio,'legacy_depth':geometry['depth']//ratio})
    return geometry


def main():
    import bpy
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--contract',required=True);parser.add_argument('--out',required=True)
    parser.add_argument('--life-unit-mm',type=int,default=0);parser.add_argument('--cue-output');parser.add_argument('--package',default='location_geometry');parser.add_argument('--layout-id',required=True)
    args = parser.parse_args(sys.argv[sys.argv.index('--')+1:])
    if not args.package.replace('_','').isalnum() or not args.package[0].isalpha():
        fail('CUE package must be a simple identifier')
    source = Path(bpy.data.filepath)
    if not source.is_file() or source.suffix!='.blend':fail('open the source .blend before import.py')
    before = digest(source)
    if bpy.context.scene.unit_settings.system!='METRIC' or abs(bpy.context.scene.unit_settings.scale_length-1)>1e-9:
        fail('Blender-first source units must be METRIC, scale_length 1 metre')
    contract = json.loads(Path(args.contract).read_text())
    geometry = import_geometry(bpy.context.scene,contract,args.layout_id,args.life_unit_mm)
    result = {'schema':'ang.blender-location-import.v1','source':str(source),'source_sha256':before,'generated_geometry':geometry}
    assert digest(source)==before,'source .blend changed during import'
    Path(args.out).parent.mkdir(parents=True,exist_ok=True)
    write_json(args.out,result)
    if args.cue_output:
        target = Path(args.cue_output)
        if target.exists() and not target.read_text().startswith('// Generated from the ID-tagged Blender source by ANG; do not hand-edit.'):
            fail(f'{target}: refusing to overwrite hand CUE; move its pose file aside for the Blender-source handover')
        target.parent.mkdir(parents=True,exist_ok=True)
        target.write_text('// Generated from the ID-tagged Blender source by ANG; do not hand-edit.\npackage '+args.package+'\n\ngenerated_geometry: '+json.dumps(geometry,indent=2,allow_nan=False)+'\n')
    print('ANG_BLENDER_IMPORT_PASS',len(geometry['floor']),'floor areas;',len(geometry['instances']),'instances;',len(geometry['obstacles']),'obstacles')

if __name__=='__main__':main()
