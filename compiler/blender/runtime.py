"""ANG Blender authoring runtime. Geometry selection depends only on ID tags."""
import hashlib
import json
import math
from pathlib import Path


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def write_json(path, value):
    Path(path).write_text(json.dumps(value, indent=2, allow_nan=False) + '\n')


def fail(message):
    raise ValueError('ANG authoring: ' + message)


def identity_tags(scene, known):
    """One authority per ID; locator ID properties are owner references."""
    roots = {}
    memberships = {}
    def root(key, node, members):
        if key not in known:
            fail(f'{node}: ang_id={key!r} is absent from CUE; fix or remove this tag')
        if key in roots:
            fail(f'{node}: duplicate ang_id={key!r}; already owned by {roots[key]}. Keep one tagged root/collection')
        roots[key] = node
        for obj in members:
            memberships.setdefault(obj.name, set()).add(key)
    def collection(col):
        if col.get('ang_id'):
            root(str(col['ang_id']), f'collection {col.name!r}', col.all_objects)
        for child in col.children:
            collection(child)
    collection(scene.collection)
    for obj in scene.objects:
        if obj.get('ang_id') and not obj.get('ang_locator'):
            root(str(obj['ang_id']), f'object {obj.name!r}', [obj, *obj.children_recursive])
    for name, ids in memberships.items():
        if len(ids) > 1:
            fail(f'object {name!r}: inherited conflicting IDs {sorted(ids)}; unlink it from one tagged collection/root')
    return roots, {name: next(iter(ids)) for name, ids in memberships.items()}


def apply_legacy(scene, mapping, identities):
    """Exact-name legacy membership table; every mutation is recorded, never saved."""
    import bpy
    audit = []
    groups = {}
    def existing(col):
        key = col.get('ang_id')
        if key:
            if key in groups:
                fail(f'collection {col.name!r}: duplicate ang_id={key!r}, first collection {groups[key].name!r}')
            groups[key] = col
        for child in col.children:
            existing(child)
    existing(scene.collection)
    for name, row in mapping.get('objects', {}).items():
        obj = scene.objects.get(name)
        if obj is None:
            fail(f'legacy mapping names missing Blender object {name!r}; fix the mapping')
        key = row['ang_id']
        if key not in identities:
            fail(f'object {name!r}: legacy ang_id={key!r} is absent from CUE')
        if key not in groups:
            col = bpy.data.collections.new('COL_ANG_' + key)
            scene.collection.children.link(col)
            col['ang_id'] = key
            groups[key] = col
            audit.append({'node': 'collection:' + col.name, 'property': 'ang_id', 'before': None, 'after': key})
        if obj.name not in groups[key].all_objects:
            groups[key].objects.link(obj)
        for tag in ['ang_material_role', 'ang_lod']:
            if tag in row:
                before = obj.get(tag)
                obj[tag] = row[tag]
                audit.append({'node': 'object:' + obj.name, 'property': tag, 'before': before, 'after': row[tag]})
    for name, row in mapping.get('locators', {}).items():
        obj = scene.objects.get(name)
        if obj is None:
            fail(f'legacy locator {name!r}: expected an existing source object')
        if obj.type != 'EMPTY':
            source = obj
            obj = bpy.data.objects.new('ANG_locator:' + row['ang_locator'], None)
            scene.collection.objects.link(obj)
            obj.matrix_world = source.matrix_world.copy()
            audit.append({'node': 'object:' + obj.name, 'created_from': source.name, 'purpose': 'explicit legacy locator migration to EMPTY'})
        for tag in ['ang_id', 'ang_locator']:
            before = obj.get(tag)
            obj[tag] = row[tag]
            audit.append({'node': 'object:' + obj.name, 'property': tag, 'before': before, 'after': row[tag]})
    from mathutils import Matrix
    for row in mapping.get('create_locators', []):
        obj = bpy.data.objects.new(row['ang_locator'], None)
        scene.collection.objects.link(obj)
        if 'matrix_blender' in row:
            obj.matrix_world = Matrix(row['matrix_blender'])
        else:
            obj.location = row['position_blender_m']
        for tag in ['ang_id', 'ang_locator']:
            obj[tag] = row[tag]
            audit.append({'node': 'object:' + obj.name, 'property': tag, 'before': None, 'after': row[tag], 'created': True})
    return audit


def import_asset(scene, contract, key, asset, mapping=None, collection=None):
    import bpy
    from mathutils import Matrix
    if key not in contract['identities']:
        fail(f'import_assets[{key!r}]: ID absent from CUE')
    asset = Path(asset).resolve()
    if not asset.is_file() or asset.suffix not in ('.glb', '.gltf'):
        fail(f'import_assets[{key!r}]: expected existing glTF source {str(asset)!r}')
    before = set(bpy.data.objects)
    bpy.ops.import_scene.gltf(filepath=str(asset))
    imported = [obj for obj in bpy.data.objects if obj not in before]
    mapping = mapping or {}
    b2g = Matrix(contract['convention']['blender_to_world'])
    source_world = Matrix(mapping.get('source_to_world', contract['convention']['blender_to_world']))
    row = contract['identities'][key]
    basis = row.get('source_asset_basis',row.get('asset_basis','source_plan')) if str(asset)==row.get('source_asset') else row.get('asset_basis','source_plan')
    reflection = Matrix(contract['convention']['model_reflection'])
    asset_basis = reflection if basis == 'godot' else Matrix.Identity(4)
    matrix = source_world.inverted() @ Matrix(contract['identities'][key]['world']) @ asset_basis @ b2g
    # Legacy source may sit on a local floor: compensate its source-to-world
    # offset so all imported meshes remain in that same Blender source frame.
    worlds = {obj: matrix @ obj.matrix_world.copy() for obj in imported}
    col = collection
    if col is None:
        col = bpy.data.collections.new('COL_ANG_Imported_' + key)
        scene.collection.children.link(col)
    col['ang_id'] = key
    col['ang_cue_pose'] = json.dumps(contract['identities'][key]['world'])
    for obj in imported:
        obj.parent = None
        obj.matrix_world = worlds[obj]
        obj['ang_source_scale'] = list(obj.scale)
        for owner in list(obj.users_collection):
            owner.objects.unlink(obj)
        col.objects.link(obj)
    bpy.context.view_layer.update()
    objects = [obj for obj in imported if obj.type == 'MESH']
    mesh_hash, _ = signature(objects, model_matrix(contract, key, mapping), bpy.context.evaluated_depsgraph_get())
    return {'source_asset': str(asset), 'source_sha256': digest(asset), 'source_signature': mesh_hash, 'asset_basis': basis}, [{'node': 'collection:' + col.name, 'property': 'ang_id', 'before': None, 'after': key, 'import_asset': str(asset)}]


def triangle_rect(tri, rect):
    """Closed convex SAT in plan XY; degenerate wall projections remain valid."""
    x0, y0, x1, y1 = rect
    square = [(x0, y0), (x1, y0), (x1, y1), (x0, y1)]
    axes = [(1, 0), (0, 1)]
    for a, b in zip(tri, tri[1:] + tri[:1]):
        axes.append((b[1] - a[1], a[0] - b[0]))
    for axis in axes:
        if abs(axis[0]) + abs(axis[1]) < 1e-14:
            continue
        a = [p[0] * axis[0] + p[1] * axis[1] for p in tri]
        b = [p[0] * axis[0] + p[1] * axis[1] for p in square]
        if max(a) < min(b) or max(b) < min(a):
            return False
    return True


def material_state(material):
    if material is None:
        return None
    row = {'name': material.name, 'diffuse_color': list(material.diffuse_color), 'roughness': material.roughness, 'metallic': material.metallic, 'culling': material.use_backface_culling}
    if material.use_nodes:
        nodes = []
        for node in material.node_tree.nodes:
            inputs = []
            for socket in node.inputs:
                if not hasattr(socket, 'default_value'):
                    continue
                value = socket.default_value
                if not isinstance(value, (str, int, float, bool)):
                    try:
                        value = list(value)
                    except TypeError:
                        value = str(value)
                inputs.append((socket.name, value))
            entry = {'type': node.type, 'name': node.name, 'inputs': inputs}
            if hasattr(node, 'operation'):
                entry['operation'] = node.operation
            if hasattr(node, 'blend_type'):
                entry['blend_type'] = node.blend_type
            if hasattr(node, 'color_ramp'):
                entry['ramp'] = [(item.position, list(item.color)) for item in node.color_ramp.elements]
            image = getattr(node, 'image', None)
            if image:
                entry['image'] = image.name
                if image.packed_file:
                    entry['image_sha256'] = hashlib.sha256(image.packed_file.data).hexdigest()
                elif image.filepath:
                    import bpy
                    path = Path(bpy.path.abspath(image.filepath))
                    entry['image_sha256'] = digest(path) if path.is_file() else 'missing'
                entry['image_dirty'] = image.is_dirty
            nodes.append(entry)
        row['nodes'] = nodes
        row['links'] = [(link.from_node.name, link.from_socket.name, link.to_node.name, link.to_socket.name) for link in material.node_tree.links]
    return row


def triangle_box(triangle, low, high):
    """Exact triangle/AABB separating-axis test, including height correlation."""
    def sub(a, b):
        return tuple(x-y for x, y in zip(a, b))
    def cross(a, b):
        return (a[1]*b[2]-a[2]*b[1], a[2]*b[0]-a[0]*b[2], a[0]*b[1]-a[1]*b[0])
    def dot(a, b):
        return sum(x*y for x, y in zip(a, b))
    edges = [sub(triangle[(i+1) % 3], triangle[i]) for i in range(3)]
    units = [(1, 0, 0), (0, 1, 0), (0, 0, 1)]
    axes = units + [cross(edges[0], edges[1])] + [cross(edge, unit) for edge in edges for unit in units]
    centre = tuple((a+b)/2 for a, b in zip(low, high))
    half = tuple((b-a)/2 for a, b in zip(low, high))
    points = [sub(p, centre) for p in triangle]
    for axis in axes:
        if dot(axis, axis) < 1e-24:
            continue
        values = [dot(p, axis) for p in points]
        radius = sum(abs(a)*h for a, h in zip(axis, half))
        if min(values) > radius + 1e-12 or max(values) < -radius - 1e-12:
            return False
    return True


def signature(objects, model_matrix, depsgraph):
    """Normalised evaluated meshes; object order/names do not select identities."""
    import struct
    h = hashlib.sha256()
    counts = []
    for obj in objects:
        ev = obj.evaluated_get(depsgraph)
        mesh = ev.to_mesh()
        mesh.calc_loop_triangles()
        matrix = model_matrix @ ev.matrix_world
        normal = matrix.to_3x3().inverted().transposed()
        uv = mesh.uv_layers.active
        materials = [s.material.name if s.material else '' for s in obj.material_slots]
        local = hashlib.sha256()
        for tri in mesh.loop_triangles:
            local.update(str(tri.material_index).encode())
            for li in tri.loops:
                p = matrix @ mesh.vertices[mesh.loops[li].vertex_index].co
                n = (normal @ mesh.corner_normals[li].vector).normalized()
                tex = uv.data[li].uv if uv else (0, 0)
                # Blender stores coordinates/normals as float32. Quantisation
                # ignores only import/depsgraph rounding, never visible edits.
                values = [float(v) for v in [*p, *n, *tex]]
                if not all(math.isfinite(v) for v in values):
                    fail(f'object {obj.name!r}: non-finite vertex/normal/UV; repair this mesh')
                local.update(struct.pack('<8d', *(round(v, 6) for v in values)))
        local.update(json.dumps([material_state(slot.material) for slot in obj.material_slots], sort_keys=True).encode())
        counts.append((local.digest(), {'name': obj.name, 'triangles': len(mesh.loop_triangles), 'materials': materials}))
        ev.to_mesh_clear()
    for value, _ in sorted(counts, key=lambda row: row[0]):
        h.update(value)
    return h.hexdigest(), [row for _, row in counts]


def clip_polygon_box(triangle, low, high):
    """Clip a triangle in real 3D; projections never join unrelated heights."""
    polygon = [tuple(p) for p in triangle]
    for axis in range(3):
        for boundary, direction in [(low[axis], 1), (high[axis], -1)]:
            if not polygon:
                return []
            result = []
            previous = polygon[-1]
            previous_distance = direction * (previous[axis] - boundary)
            for point in polygon:
                distance = direction * (point[axis] - boundary)
                if (distance >= -1e-12) != (previous_distance >= -1e-12):
                    fraction = previous_distance / (previous_distance - distance)
                    result.append(tuple(a + fraction * (b-a) for a, b in zip(previous, point)))
                if distance >= -1e-12:
                    result.append(point)
                previous, previous_distance = point, distance
            polygon = result
    return polygon


def projected_intrusion(polygon, rect):
    """Maximum nearest-cell-edge depth over the clipped plan polygon, metres.

    The minimum of the four edge distances is piecewise linear. Its maximum
    occurs on a polygon vertex/edge bisector, or at the cell centre. This exact
    finite candidate set avoids an approximate voxel or bounding-box distance.
    """
    if not polygon:
        return 0.0
    x0, y0, x1, y1 = rect
    plan = [(p[0], p[1]) for p in polygon]
    functions = [(1, 0, -x0), (-1, 0, x1), (0, 1, -y0), (0, -1, y1)]
    def depth(point):
        return max(0.0, min(a*point[0]+b*point[1]+c for a, b, c in functions))
    best = max(depth(p) for p in plan)
    for p, q in zip(plan, plan[1:] + plan[:1]):
        for i, first in enumerate(functions):
            for second in functions[i+1:]:
                a, b, c = (x-y for x, y in zip(first, second))
                dp, dq = a*p[0]+b*p[1]+c, a*q[0]+b*q[1]+c
                if abs(dp-dq) > 1e-14:
                    fraction = dp / (dp-dq)
                    if 0 <= fraction <= 1:
                        best = max(best, depth((p[0]+fraction*(q[0]-p[0]), p[1]+fraction*(q[1]-p[1]))))
    centre = ((x0+x1)/2, (y0+y1)/2)
    # Convex SAT also accepts line/point projections (vertical mesh faces).
    axes = [(q[1]-p[1], p[0]-q[0]) for p, q in zip(plan, plan[1:]+plan[:1])] + [(1, 0), (0, 1)]
    inside = True
    for a, b in axes:
        values = [a*p[0]+b*p[1] for p in plan]
        value = a*centre[0]+b*centre[1]
        if value < min(values)-1e-12 or value > max(values)+1e-12:
            inside = False
            break
    if inside:
        best = max(best, depth(centre))
    return best


def conflict_key(row):
    return (row['object_id'], row['cell']['x'], row['cell']['y'], row['layer'])


def check_known_conflicts(contract, conflicts, errors, warnings):
    """An explicit legacy list can shrink; export never creates or grows it."""
    known = {conflict_key(row): row for row in contract.get('known_conflicts', [])}
    actual = {}
    for row in conflicts:
        if row['kind'] == 'walkable':
            key = conflict_key(row)
            if key not in actual or row['max_intrusion_m'] > actual[key]['max_intrusion_m']:
                actual[key] = row
    if known and not contract.get('legacy', False):
        errors.append('pipeline.known_conflicts: legacy must be explicitly declared; new locations are strict')
    accepted, fresh, grown, stale = [], [], [], []
    for key, row in sorted(actual.items()):
        baseline = known.get(key)
        if baseline is None:
            fresh.append(row)
            errors.append(f'object {row["object_id"]!r}: NEW mesh intrusion at cell ({row["cell"]["x"]},{row["cell"]["y"]}) layer {row["layer"]!r}, measured {row["max_intrusion_m"]:.6f} m; correct art/CUE, never append a baseline during export')
        elif row['max_intrusion_m'] > baseline['max_intrusion_m'] + .01 + 1e-8:
            grown.append(row)
            errors.append(f'object {row["object_id"]!r}: intrusion GREW at cell ({row["cell"]["x"]},{row["cell"]["y"]}) layer {row["layer"]!r}: {row["max_intrusion_m"]:.6f} m versus baseline {baseline["max_intrusion_m"]:.6f} m, allowed growth 0.010000 m; restore or reduce art')
        else:
            accepted.append(row)
    for key, baseline in sorted(known.items()):
        if key not in actual:
            stale.append(baseline)
            errors.append(f'pipeline.known_conflicts: STALE entry {baseline["object_id"]!r} cell ({baseline["cell"]["x"]},{baseline["cell"]["y"]}) layer {baseline["layer"]!r}; remove this resolved baseline entry from CUE')
    for key, baseline in sorted(known.items()):
        measured = actual.get(key)
        status = 'STALE' if measured is None else 'GROWN' if measured['max_intrusion_m']>baseline['max_intrusion_m']+.01+1e-8 else 'ACCEPTED'
        actual_depth = 'absent' if measured is None else f'{measured["max_intrusion_m"]:.6f} m'
        warnings.append(f'Legacy {status}: object {baseline["object_id"]!r}, cell ({baseline["cell"]["x"]},{baseline["cell"]["y"]}), layer {baseline["layer"]!r}; measured {actual_depth}, baseline {baseline["max_intrusion_m"]:.6f} m; reason: {baseline["reason"]}')
    for row in conflicts:
        if row['kind']=='walkable':
            baseline = known.get(conflict_key(row))
            row['baseline_accepted'] = bool(baseline and actual[conflict_key(row)]['max_intrusion_m']<=baseline['max_intrusion_m']+.01+1e-8)
            if baseline:
                row['baseline_max_intrusion_m'] = baseline['max_intrusion_m']
                row['baseline_reason'] = baseline['reason']
    if accepted:
        warnings.append(f'Legacy art conflicts: {len(accepted)} accepted of {len(known)} baseline cells; actual {len(actual)}, new {len(fresh)}, grown {len(grown)}, stale {len(stale)}. Geometry is unchanged by this exception list')
    return {'baseline_count': len(known), 'actual_count': len(actual), 'accepted_count': len(accepted), 'new_count': len(fresh), 'grown_count': len(grown), 'stale_count': len(stale), 'growth_tolerance_m': .01}


def validate_geometry(contract, objects, source_to_world, depsgraph, errors, instance_id=''):
    """All actual object/cell/layer conflicts, never only the first per object."""
    unit, tol = contract['unit_m'], contract['tolerance_m']
    walkable = {}
    for cell in contract['walkable']:
        walkable.setdefault((cell['x'], cell['y']), []).append(cell)
    protected = [l for l in contract['locators'] if l['kind'] in ('service', 'exit')]
    conflicts = {}
    for obj in objects:
        if obj.hide_render:
            continue
        ev = obj.evaluated_get(depsgraph)
        mesh = ev.to_mesh()
        mesh.calc_loop_triangles()
        matrix = source_to_world @ ev.matrix_world
        object_id = str(obj.get('ang_object_id', obj.name))
        for triangle in mesh.loop_triangles:
            points = [matrix @ mesh.vertices[i].co for i in triangle.vertices]
            if not all(math.isfinite(float(value)) for point in points for value in point):
                errors.append(f'object {object_id!r}: non-finite evaluated mesh coordinates; repair this Blender mesh')
                break
            zlo, zhi = min(p[1] for p in points), max(p[1] for p in points)
            prism_triangle = [(p[0], -p[2], p[1]) for p in points]
            xmin, xmax = min(p[0] for p in prism_triangle), max(p[0] for p in prism_triangle)
            ymin, ymax = min(p[1] for p in prism_triangle), max(p[1] for p in prism_triangle)
            for x in range(math.floor(xmin/unit), math.floor(xmax/unit)+1):
                for y in range(math.floor(ymin/unit), math.floor(ymax/unit)+1):
                    for cell in walkable.get((x, y), []):
                        floor = cell['elevation_m']
                        if zhi <= floor+tol or zlo >= floor+1.85:
                            continue
                        rect = (x*unit, y*unit, (x+1)*unit, (y+1)*unit)
                        clipped = clip_polygon_box(prism_triangle, (rect[0], rect[1], floor+tol), (rect[2], rect[3], floor+1.85))
                        intrusion = projected_intrusion(clipped, rect)
                        if intrusion <= tol+1e-9:
                            continue
                        key = (object_id, x, y, cell['surface'])
                        if key not in conflicts or intrusion > conflicts[key]['max_intrusion_m']:
                            conflicts[key] = {'object_id': object_id, 'instance_id': instance_id, 'cell': {'x': x, 'y': y}, 'layer': cell['surface'], 'max_intrusion_m': intrusion, 'kind': 'walkable', 'witness_triangle_world_m': [list(p) for p in points]}
            for locator in protected:
                x, floor, y = locator['position_plan_m']
                radius = max(.05, unit/2-tol)
                if zhi > floor+tol and zlo < floor+1.85 and triangle_box(prism_triangle, (x-radius, y-radius, floor+tol), (x+radius, y+radius, floor+1.85)):
                    key = (object_id, locator['kind'], locator['name'], locator.get('instance_id', ''))
                    if key not in conflicts:
                        conflicts[key] = {'object_id': object_id, 'instance_id': instance_id, 'cell': {'x': math.floor(x/unit), 'y': math.floor(y/unit)}, 'layer': 'pose:'+locator['kind']+':'+locator['name'], 'max_intrusion_m': projected_intrusion(clip_polygon_box(prism_triangle, (x-radius,y-radius,floor+tol), (x+radius,y+radius,floor+1.85)), (x-radius,y-radius,x+radius,y+radius)), 'kind': locator['kind'], 'locator': locator['name'], 'witness_triangle_world_m': [list(p) for p in points]}
                        errors.append(f'object {object_id!r}: blocks {locator["kind"]} pose {locator["name"]!r}, tolerance {tol:g} m; clear this CUE approach')
        ev.to_mesh_clear()
    return sorted(conflicts.values(), key=lambda row: (row['object_id'], row['layer'], row['cell']['x'], row['cell']['y']))


def model_matrix(contract, key, mapping):
    from mathutils import Matrix
    row = mapping.get('ids', {}).get(key, {})
    if 'blender_to_model' in row:
        return Matrix(row['blender_to_model'])
    b2g = Matrix(contract['convention']['blender_to_world'])
    source = Matrix(mapping.get('source_to_world', contract['convention']['blender_to_world']))
    # A proper exported mesh plus a single adapter reflection. ANG placement
    # owns the source-model reflection; artists author ordinary Blender axes.
    reflection = Matrix(contract['convention']['model_reflection'])
    world = Matrix(contract['identities'][key]['world'])
    return b2g.inverted() @ (world @ reflection).inverted() @ source


def inherited_properties(scene):
    """Object > nearest collection/root properties, with conflicting roots rejected."""
    tags = ['ang_material_role', 'ang_lod', 'ang_wall', 'ang_floor', 'ang_material_reason', 'ang_zone', 'ang_tier', 'ang_caste', 'ang_library', 'ang_module', 'ang_layer', 'ang_deck']
    result = {}
    def collection(col, inherited):
        current = dict(inherited)
        current.update({tag: col[tag] for tag in tags if tag in col})
        for obj in col.objects:
            existing = result.setdefault(obj.name, {})
            for tag, value in current.items():
                if tag in existing and existing[tag] != value and tag not in obj:
                    fail(f'object {obj.name!r}: conflicting inherited {tag}; put it in one authoring group or set the property explicitly')
                existing[tag] = value
        for child in col.children:
            collection(child, current)
    collection(scene.collection, {})
    for root in scene.objects:
        if root.get('ang_id') and not root.get('ang_locator'):
            inherited = {tag: root[tag] for tag in tags if tag in root}
            for obj in root.children_recursive:
                result.setdefault(obj.name, {}).update(inherited)
    for obj in scene.objects:
        result.setdefault(obj.name, {}).update({tag: obj[tag] for tag in tags if tag in obj})
    return result


def resolve_wall_role(contract, object_id, instance_id, props, errors, overrides):
    """CUE-owned dome concrete/caste policy; every exception is reported."""
    architecture = contract.get('architecture') or {}
    materials = contract.get('materials', {})
    role = str(props.get('ang_material_role') or contract['identities'].get(instance_id, {}).get('material_role') or '')
    is_wall = bool(props.get('ang_wall')) or role == 'wall'
    if not is_wall or architecture.get('setting') != 'dome_interior':
        return contract.get('surface_roles',{}).get(role,role) if role else ''
    zone = str(props.get('ang_zone') or '')
    zones = architecture.get('wall_zones', {})
    zone_row = zones.get(zone) if zone else zones.get(object_id) or zones.get(instance_id)
    if zone and zone_row is None:
        errors.append(f'wall object {object_id!r}: ang_zone={zone!r} is absent from CUE architecture.wall_zones')
    expected = (zone_row or {}).get('caste', architecture.get('wall_caste'))
    requested = str(props.get('ang_tier') or props.get('ang_caste') or expected or '')
    if requested not in ('copper', 'bronze', 'silver', 'gold'):
        errors.append(f'wall object {object_id!r}: a declared copper/bronze/silver/gold wall caste is required')
    concrete_roles = architecture.get('concrete_roles', {})
    resolved = (zone_row or {}).get('material_role') or concrete_roles.get(object_id) or concrete_roles.get(instance_id) or concrete_roles.get('default')
    if resolved and (not role or role == 'wall' or role in ('concrete.'+str(expected),'concrete.'+requested,architecture.get('concrete_role'))):
        role = resolved
    if not role or role == 'wall':
        default = contract.get('surface_roles',{}).get('wall') or architecture.get('concrete_role', '')
        role = default if materials.get(default, {}).get('surface_caste') == requested else 'concrete.'+requested
    material = materials.get(role)
    if material is None:
        errors.append(f'wall object {object_id!r}: concrete material role {role!r} is absent from CUE finishing.materials')
        return role
    correct = material.get('surface_kind') == 'concrete' and material.get('surface_caste') == expected and requested == expected
    override = architecture.get('wall_overrides', {}).get(object_id) or architecture.get('wall_overrides', {}).get(instance_id)
    if not correct:
        if not override or not str(override.get('reason', '')).strip() or (override.get('material_role') and override['material_role'] != role):
            errors.append(f'wall object {object_id!r}: {role!r} is not concrete of assigned caste {expected!r}; restore its concrete family or declare a per-object CUE wall_overrides reason')
        else:
            overrides.append({'object_id': object_id, 'instance_id': instance_id, 'material_role': role, 'assigned_caste': expected, 'zone': zone, 'reason': override['reason']})
    elif zone_row:
        overrides.append({'object_id': object_id, 'instance_id': instance_id, 'material_role': role, 'assigned_caste': expected, 'zone': zone or object_id, 'reason': zone_row['reason'], 'kind': 'mixed_wall_zone'})
    return role


def art_consumers(identities, key):
    """World validation is scoped to actual placements, not kit library roots."""
    row = identities[key]
    if row['required']:
        return [(key, row)]
    ref = row.get('model_ref')
    return [(name, target) for name, target in identities.items()
            if target['required'] and not target['greybox']
            and (target.get('module_id') == key or target.get('model_ref') == key
                 or bool(ref) and target.get('model_ref') == ref)]


def installed_triangle_budget(contract, source_triangles, warnings, partial=False):
    """LOD0 topology per required model placement, plus the shared emitter plan.

    Source/library artifacts and authoring blockouts are never level draw counts.
    Multiple source IDs for one published model_ref must agree; the exporter also
    checks their full mesh signatures before publishing that shared resource.
    """
    identities = contract['identities']
    by_ref = {}
    for key, count in sorted(source_triangles.items()):
        ref = identities.get(key, {}).get('model_ref')
        if ref:
            by_ref.setdefault(ref, []).append((key, int(count)))
    consumers = {}
    missing = []
    for key, row in sorted(identities.items()):
        ref = row.get('model_ref')
        if not row.get('required') or not ref:
            continue
        candidates = [(key, int(source_triangles[key]))] if key in source_triangles else by_ref.get(ref, [])
        if not candidates:
            if not partial:
                missing.append(key + ' (model_ref ' + ref + ')')
            continue
        if len({count for _, count in candidates}) != 1:
            missing.append(key + ' (conflicting source triangle counts for ' + ref + ')')
            continue
        source_id, count = sorted(candidates)[0]
        consumers[key] = {'source_id': source_id, 'model_ref': ref, 'triangles': count}
    plan = contract.get('scene_geometry')
    if not partial and plan is None:
        missing.append('ANG emitted primitive plan; regenerate the authoring contract')
    procedural = 0 if partial else int((plan or {}).get('triangles', 0))
    models = sum(row['triangles'] for row in consumers.values())
    status = 'partial_export' if partial else 'partial_static_level' if missing else 'static_level'
    if missing:
        warnings.append('Triangle budget UNMEASURED for: ' + '; '.join(missing) + '. Reported subtotal is not a complete installed-scene total.')
    return {'triangles': models + procedural, 'bound_models_lod0': models,
            'emitter_primitives': procedural, 'consumers': consumers, 'status': status,
            'scope': 'Required LOD0 model placements and exact ANG emitted primitives; excludes runtime actors, localized text, particles and legacy assembly additions.'}
