"""ANG generic ID exporter. Run through the serial wrapper; never saves source."""
import argparse
import hashlib
import json
import math
import shutil
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from runtime import recorded_asset, apply_legacy, digest, fail, identity_tags, import_asset, model_matrix, signature, validate_geometry, check_known_conflicts, inherited_properties, resolve_wall_role, art_consumers, installed_triangle_budget, write_json, batch_policy_signature, selected_node_names, batch_partition, material_batch_key, hierarchy_signature, artifact_pose_signature, source_pose_unchanged, check_batch_attributes, artifact_material_names, check_retained_material_aliases, mesh_uv_layout, write_batch_uv_layers, SIGNATURE_SCHEMA
import bpy
from mathutils import Matrix, Vector

parser = argparse.ArgumentParser()
parser.add_argument('--contract', required=True)
parser.add_argument('--out', required=True)
parser.add_argument('--resource-prefix', required=True)
parser.add_argument('--legacy-map')
parser.add_argument('--attachments-existing')
parser.add_argument('--only-id', action='append', default=[], help='export a known module/model/instance ID as a partial kit artifact; never validates an entire level')
parser.add_argument('--collect-conflicts', action='store_true', help='collect full diagnostics; failed validation still exits nonzero')
args = parser.parse_args(sys.argv[sys.argv.index('--') + 1:])
contract = json.loads(Path(args.contract).read_text())
source = Path(bpy.data.filepath).resolve()
if not source.is_file() or source.suffix != '.blend':
    fail('open the source .blend before running export.py')
before = digest(source)
out = Path(args.out).resolve()
out.mkdir(parents=True, exist_ok=True)
prefix = args.resource_prefix.rstrip('/') + '/'
if not prefix.startswith('res://') or '..' in prefix.split('/') or '\\' in prefix:
    fail('--resource-prefix must be canonical res:// resource directory')
def merge_mapping(base, overlay):
    result = dict(base)
    for key, value in overlay.items():
        result[key] = merge_mapping(result[key], value) if isinstance(value, dict) and isinstance(result.get(key), dict) else value
    return result

scene = bpy.context.scene
mapping = json.loads(scene.get('ang_export_mapping', '{}'))
if args.legacy_map:
    mapping = merge_mapping(mapping, json.loads(Path(args.legacy_map).read_text()))
known = contract['identities']
scene = bpy.context.scene
errors = []
warnings = []
report = {'schema': 'ang.blender-export.v1', 'contract_sha256': digest(args.contract), 'geometry_contract_sha256': contract['geometry_contract_sha256'], 'source': str(source), 'source_sha256': before, 'components': [], 'tag_audit': [], 'ignored_objects': mapping.get('ignored_objects', []), 'validation': {'errors': errors, 'warnings': warnings, 'missing_art': []}, 'budget': contract['budget'], 'scope': 'Evaluated source geometry provenance; triangle_budget counts required LOD0 placements plus exact ANG primitives. Runtime content and legacy assembly require the native receipt. No source save or silent caps.'}
if scene.unit_settings.system != 'METRIC' or abs(scene.unit_settings.scale_length - 1) > 1e-9:
    errors.append(f'scene units: system={scene.unit_settings.system}, scale_length={scene.unit_settings.scale_length}; set METRIC and 1 metre before export')
if mapping:
    report['persisted_mapping_sha256'] = hashlib.sha256(json.dumps(mapping,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    if args.legacy_map:
        report['legacy_mapping_sha256'] = digest(args.legacy_map)
    report['tag_audit'] = apply_legacy(scene, mapping, known)
source_assets = json.loads(scene.get('ang_source_assets', '{}'))
existing_roots, _ = identity_tags(scene, known)
for key, row in mapping.get('import_assets', {}).items():
    if key in existing_roots:
        continue
    imported, audit = import_asset(scene, contract, key, row['asset'], mapping)
    source_assets[key] = imported
    report['tag_audit'].extend(audit)
roots, memberships = identity_tags(scene, known)
selected = set(args.only_id)
if selected - known.keys():
    fail('unknown --only-id: '+', '.join(sorted(selected-known.keys())))
if selected:
    report['partial_ids'] = sorted(selected)
properties = inherited_properties(scene)
wall_overrides = []
object_roles = {}
report['wall_material_overrides'] = wall_overrides
roles = set(contract['material_roles'])
groups = {}
ignored = set(mapping.get('ignored_objects', []))
for obj in scene.objects:
    if str(obj.get('ang_ignore_reason', '')).strip():
        ignored.add(obj.name)
report['ignored_objects'] = sorted(ignored)
report['ignored_reasons'] = {obj.name: str(obj.get('ang_ignore_reason', 'explicit migration mapping exclusion')) for obj in scene.objects if obj.name in ignored}
for obj in list(scene.objects):
    if obj.type != 'MESH' or obj.hide_render or obj.get('ang_preview_only'):
        continue
    key = memberships.get(obj.name)
    if key is None:
        if selected:
            continue
        if obj.name not in ignored:
            errors.append(f'object {obj.name!r}: visible mesh has no ang_id; tag its object or containing collection with the CUE instance/module ID')
        continue
    if selected and key not in selected:
        continue
    role = resolve_wall_role(contract, obj.name, key, properties[obj.name], errors, wall_overrides)
    if role:
        object_roles[obj.name] = role
    if role and role not in roles:
        errors.append(f'object {obj.name!r}: ang_material_role={role!r} is absent from CUE finishing.materials')
    lod = properties[obj.name].get('ang_lod', 0)
    if not isinstance(lod, int) or isinstance(lod, bool) or lod < 0 or lod > 4:
        errors.append(f'object {obj.name!r}: ang_lod must be an integer 0..4, got {lod!r}')
    expected_scale = obj.get('ang_source_scale', [1, 1, 1])
    if any(abs(abs(float(v)) - abs(float(w))) > 1e-4 for v, w in zip(obj.scale, expected_scale)):
        errors.append(f'object {obj.name!r}: scale={tuple(obj.scale)}; apply object scale in metres, keeping its CUE pose')
    if obj.get('ang_placeholder') and not known[key]['greybox'] and not known[key].get('procedural'):
        errors.append(f'object {obj.name!r}: replace the starting placeholder with art or set pipeline.instances[{key!r}].greybox: true in CUE')
    groups.setdefault(key, []).append(obj)
for key in sorted(selected):
    if known[key]['greybox']:
        errors.append(f'requested ID {key!r}: declared greybox in CUE; assign an art role/model intent before publishing it as a kit')
    if not groups.get(key):
        errors.append(f'requested ID {key!r}: no tagged render mesh in this Blender file; bind/import its art before --only-id export')
# All explicit CUE instances need source art, a reusable kit, or deliberate greybox.
for key, row in known.items():
    if selected:
        continue
    if row['required'] and not row['greybox'] and not row.get('procedural') and not (row.get('rendered_by') and not row.get('model_ref')) and not any(groups.get(alias) for alias in [key, row['module_id'] if 'module_id' in row else '', row['model_ref']]):
        report['validation']['missing_art'].append(f'CUE instance {key!r}: no tagged mesh/kit; assign ang_id to its art or set pipeline.instances[{key!r}].greybox: true')
b2g = Matrix(contract['convention']['blender_to_world'])
source_world = Matrix(mapping.get('source_to_world', contract['convention']['blender_to_world']))
reflection = Matrix(contract['convention']['model_reflection'])
depsgraph = bpy.context.evaluated_depsgraph_get()
# Check each source mesh in world coordinates once. Greybox floors and blockers
# remain subject to the same walkable-body prisms as detailed art.
conflicts = []
validated_objects = set()
consumer_ids = set()
for key, objects in groups.items():
    row = known[key]
    consumers = art_consumers(known,key)
    if consumers:
        validated_objects.update(obj.get('ang_object_id',obj.name) for obj in objects)
    for consumer, target in consumers:
        consumer_ids.add(consumer)
        # Always validate where this art is consumed, including every reuse of a kit.
        matrix = Matrix(target['world']) @ reflection @ b2g @ model_matrix(contract, key, mapping)
        conflicts.extend(validate_geometry(contract, objects, matrix, depsgraph, errors, instance_id=consumer))
if selected:
    selected_objects = validated_objects
    validation_contract = dict(contract)
    validation_contract['known_conflicts'] = [row for row in contract.get('known_conflicts', []) if row['object_id'] in selected_objects]
else:
    validation_contract = contract
report['validation']['consumer_ids'] = sorted(consumer_ids)
report['validation']['conflicts'] = conflicts
report['validation']['known_conflicts'] = check_known_conflicts(validation_contract, conflicts, errors, warnings)
locators = {}
for obj in scene.objects:
    if not obj.get('ang_locator'):
        continue
    name = str(obj['ang_locator'])
    key = str(obj.get('ang_id') or memberships.get(obj.name, ''))
    if key and key not in known:
        errors.append(f'locator object {obj.name!r}: owner ang_id={key!r} is absent from CUE')
        continue
    if obj.type != 'EMPTY':
        errors.append(f'object {obj.name!r}: ang_locator={name!r} belongs on an EMPTY')
        continue
    pair = (key, name)
    if pair in locators:
        errors.append(f'locator object {obj.name!r}: duplicate {key!r}/{name!r}, first on {locators[pair].name!r}')
    locators[pair] = obj
for row in contract['locators']:
    if selected:
        continue
    key, name = row.get('instance_id', ''), row['name']
    obj = locators.get((key, name))
    if obj is None:
        errors.append(f'CUE {row["kind"]} locator {key!r}/{name!r}: missing EMPTY ang_locator={name!r}, ang_id={key!r}; add it at CUE position {row["position_plan_m"]}')
    elif row['kind'] in ('exit', 'service', 'rail', 'anchor'):
        p = source_world @ obj.matrix_world.translation
        want = row['position_plan_m']
        error = math.sqrt((p.x-want[0])**2+(p.y-want[1])**2+(p.z+want[2])**2)
        if error > contract['tolerance_m']:
            errors.append(f'locator object {obj.name!r}: CUE {row["kind"]} {name!r} moved {error:g} m, tolerance {contract["tolerance_m"]:g} m; restore CUE pose {want}')
# Record logical pose roundtrip. Geometry may change locally, never the CUE datum.
pose_roundtrip = {}
for col in bpy.data.collections:
    key = col.get('ang_id')
    if key in known and 'ang_cue_pose' in col:
        pose = json.loads(col['ang_cue_pose'])
        if contract.get('pose_source','cue') == 'cue' and pose != known[key]['world']:
            errors.append(f'collection {col.name!r}: saved CUE pose differs; regenerate starting file after a CUE layout change')
        pose_roundtrip[key] = pose
for obj in scene.objects:
    if not obj.get('ang_pose_datum'):
        continue
    key = memberships.get(obj.name)
    if key not in known:
        continue
    expected = source_world.inverted() @ Matrix(known[key]['world']) @ reflection @ b2g
    error = max(abs(obj.matrix_world[i][j]-expected[i][j]) for i in range(4) for j in range(4))
    if error > 1e-5:
        errors.append(f'object {obj.name!r}: CUE pivot/basis moved (matrix error {error:g}); cue mode requires the CUE pose, blend mode requires re-importing the source geometry')
    pose_roundtrip[key] = known[key]['world']
report['pose_roundtrip'] = pose_roundtrip
attachments = {'schema': 'ang.model-attachments.v1', 'source_frame': 'model-local', 'sources': {}, 'models': {}}
if args.attachments_existing:
    attachments = json.loads(Path(args.attachments_existing).read_text())
    if attachments.get('schema') != 'ang.model-attachments.v1' or attachments.get('source_frame') != 'model-local':
        errors.append('existing attachments: expected ang.model-attachments.v1 model-local')
attachments.setdefault('instances',{})
attachments['sources'][str(source)] = before
for (key, name), obj in sorted(locators.items()):
    if not key:
        continue
    row = known[key]
    ref = row['model_ref']
    local = Matrix(row['world']).inverted() @ source_world @ obj.matrix_world.translation
    chain = []
    node = obj
    while node is not None:
        chain.append(node.name)
        node = node.parent
    node = {'name': name, 'position': list(local), 'source_path': '/'.join(reversed(chain))}
    nodes = attachments['models'].setdefault(ref, []) if ref else attachments['instances'].setdefault(key, [])
    nodes[:] = [item for item in nodes if item['name'] != name]
    nodes.append(node)
triangles = 0
all_lod_triangles = 0
source_triangles = {}
for key,objects in groups.items():
    if not any(properties[obj.name].get('ang_lod',0)==0 for obj in objects):
        errors.append(f'ID {key!r}: no ang_lod=0 mesh; retain a base LOD before authoring reduced variants')
    for obj in objects:
        ev = obj.evaluated_get(depsgraph)
        mesh = ev.to_mesh()
        mesh.calc_loop_triangles()
        all_lod_triangles += len(mesh.loop_triangles)
        if properties[obj.name].get('ang_lod',0)==0:
            triangles += len(mesh.loop_triangles)
            source_triangles[key] = source_triangles.get(key, 0) + len(mesh.loop_triangles)
        ev.to_mesh_clear()
if contract.get('light_metrics_status')!='cue_declared':
    warnings.append('Light budget UNMEASURED: CUE has no authored light list; declared count zero is not a complete scene measurement. Supply the actual producer/native counters before acceptance.')
report['light_metrics_status']=contract.get('light_metrics_status','unmeasured')
lights = len(contract['lights'])
shadows = sum(bool(row['shadows']) for row in contract['lights'])
report['artifact_triangles_all_lods'] = all_lod_triangles
report['artifact_triangles_lod0'] = triangles
triangle_budget = installed_triangle_budget(contract, source_triangles, warnings, bool(selected))
report['triangle_budget'] = triangle_budget
report['triangle_metrics_status'] = triangle_budget['status']
report['totals'] = {'triangles': triangle_budget['triangles'], 'lights': lights, 'shadowed_lights': shadows}
for key, value in report['totals'].items():
    if value > contract['budget'][key]:
        warnings.append(f'pipeline.budget.{key}: {value} exceeds {contract["budget"][key]}; review CUE budget or reduce authored content. No silent cap was applied')
if errors or report['validation']['missing_art']:
    write_json(out / 'provenance.json', report)
    fail('\n'.join(errors + report['validation']['missing_art']))


def material(name, fallback):
    row = mapping.get('materials', {}).get(name)
    if name.startswith('ANG_ROLE__'):
        role = name.removeprefix('ANG_ROLE__')
        row = dict(contract['materials'][role])
        row['exported_name'] = name
        if row.get('resource') or row.get('concrete'):
            # The generated named role binding replaces this import fallback
            # with the exact CUE Godot resource, including concrete shaders.
            row['color'] = [0.5,0.5,0.5,1]
            row['roughness'] = 1
            row['metallic'] = 0
    if row is None:
        return fallback
    exported = row.get('exported_name', name)
    existing = bpy.data.materials.get(exported)
    if existing:
        return existing
    mat = bpy.data.materials.new(exported)
    mat.use_nodes = True
    p = mat.node_tree.nodes.get('Principled BSDF')
    p.inputs['Base Color'].default_value = row['color']
    p.inputs['Roughness'].default_value = row['roughness']
    p.inputs['Metallic'].default_value = row['metallic']
    p.inputs['Alpha'].default_value = row['color'][3]
    p.inputs['Emission Color'].default_value = (*row.get('emission', [0, 0, 0]), 1)
    p.inputs['Emission Strength'].default_value = row.get('emission_energy', 0)
    mat.use_backface_culling = not row.get('double_sided', False)
    return mat



def export_batched_hierarchy(base, objects, transform, file, row):
    """Merge static draw surfaces inside explicitly preserved motion frames.

    A static mesh can never bridge two moving ancestors or two CUE partitions.
    Source names are retained only for declared motion, signal or interaction
    nodes. Exact shader content is shared; no geometry/UV tolerance is used.
    """
    members = set(objects)
    members.update(obj for obj in scene.objects if obj.type == 'EMPTY' and memberships.get(obj.name) == base and not obj.get('ang_ignore_reason'))
    for obj in objects:
        ancestor = obj.parent
        while ancestor is not None and memberships.get(ancestor.name) == base:
            members.add(ancestor)
            ancestor = ancestor.parent
    originals = sorted(members, key=lambda obj: obj.name)
    names = {obj: obj.name for obj in originals}
    selected_names = selected_node_names(names.values(), row.get('preserve_node_names', []))
    kept = {obj for obj in originals if names[obj] in selected_names}
    # Local motion numbers are relative to authored parents. Keep the whole
    # transform chain, but not its static mesh descendants.
    for obj in list(kept):
        ancestor = obj.parent
        while ancestor in members:
            kept.add(ancestor)
            ancestor = ancestor.parent
    temporary = bpy.data.collections.new('ANG_BATCHED_EXPORT')
    scene.collection.children.link(temporary)
    copies = {}
    partitions = {}
    used_partitions = set()
    materials = {}
    material_keys = {}
    source_material_names = set()
    batches = {}
    source_meshes = 0
    source_surfaces = 0
    mesh_cache = {}
    mesh_instances = 0
    def canonical(mat):
        if mat is not None:
            source_material_names.add(mat.name)
        if mat not in material_keys:
            material_keys[mat] = material_batch_key(mat, contract.get('material_bindings', {}))
        identity = material_keys[mat]
        if identity not in materials:
            materials[identity] = mat
        return identity, materials[identity]
    def share_mesh(mesh, allow=True):
        nonlocal mesh_instances
        known_attributes = {'position', '.corner_vert', '.corner_edge', '.edge_verts', 'sharp_face', 'sharp_edge', 'custom_normal', '.select_vert', '.select_edge', '.select_poly', '.uv_select_vert', '.uv_select_edge', '.uv_select_face'} | {uv.name for uv in mesh.uv_layers}
        if allow and all(attr.name in known_attributes for attr in mesh.attributes):
            import struct
            h = hashlib.sha256(json.dumps(mesh_uv_layout(mesh)).encode())
            mesh.calc_loop_triangles()
            for triangle in mesh.loop_triangles:
                h.update(struct.pack('<I', triangle.material_index))
                for li in triangle.loops:
                    h.update(struct.pack('<6f', *mesh.vertices[mesh.loops[li].vertex_index].co, *mesh.corner_normals[li].vector))
                    for uv in mesh.uv_layers:
                        h.update(uv.name.encode())
                        h.update(struct.pack('<2f', *uv.data[li].uv))
            h.update(json.dumps([canonical(mat)[0] for mat in mesh.materials]).encode())
            fingerprint = h.hexdigest()
            if fingerprint in mesh_cache:
                previous = mesh
                mesh = mesh_cache[fingerprint]
                bpy.data.meshes.remove(previous)
                mesh_instances += 1
            else:
                mesh_cache[fingerprint] = mesh
        return mesh

    def nearest(obj):
        ancestor = obj.parent
        while ancestor is not None:
            if ancestor in kept:
                return ancestor
            ancestor = ancestor.parent
        return None
    def depth(obj):
        n = 0
        while obj.parent is not None:
            n += 1
            obj = obj.parent
        return n
    bpy.ops.object.select_all(action='DESELECT')
    try:
        for obj in originals:
            obj.name = 'ANG_SOURCE_' + names[obj]
        for original in sorted(kept, key=lambda obj: (depth(obj), names[obj])):
            if original.type == 'MESH':
                ev = original.evaluated_get(depsgraph)
                mesh = bpy.data.meshes.new_from_object(ev, depsgraph=depsgraph)
                source_meshes += 1
                source_surfaces += len(mesh.materials)
                for index, fallback in enumerate(mesh.materials):
                    mat = material('ANG_ROLE__'+object_roles[names[original]], fallback) if names[original] in object_roles else fallback
                    _, mesh.materials[index] = canonical(mat)
                mesh = share_mesh(mesh, not original.vertex_groups)
                duplicate = bpy.data.objects.new(names[original], mesh)
            else:
                duplicate = bpy.data.objects.new(names[original], None)
            temporary.objects.link(duplicate)
            duplicate['ang_source_node'] = names[original]
            duplicate.matrix_world = transform @ original.matrix_world
            owner = nearest(original)
            if owner is not None:
                matrix = duplicate.matrix_world.copy()
                duplicate.parent = copies[owner]
                duplicate.matrix_world = matrix
            copies[original] = duplicate
        for original in objects:
            if original in kept:
                continue
            ev = original.evaluated_get(depsgraph)
            mesh = ev.to_mesh()
            mesh.calc_loop_triangles()
            source_meshes += 1
            source_surfaces += len(mesh.materials)
            check_batch_attributes(mesh, names[original])
            owner = nearest(original)
            partition = ''
            if row.get('batch_partitions'):
                corners = [transform @ ev.matrix_world @ Vector(point) for point in original.bound_box]
                centre = [(min(p[i] for p in corners)+max(p[i] for p in corners))/2 for i in range(3)]
                partition = batch_partition(centre, row['batch_partitions'])
                used_partitions.add(partition)
                if owner is None and partition not in partitions:
                    anchor = bpy.data.objects.new(partition, None)
                    anchor['ang_source_node'] = partition
                    temporary.objects.link(anchor)
                    partitions[partition] = anchor
            anchor = copies[owner] if owner is not None else partitions.get(partition)
            if owner is not None:
                # Compose the authored local chain directly. Cancelling two
                # large world translations loses float32 bits and prevents
                # exact instancing of otherwise identical repeated mechanisms.
                chain = []
                ancestor = original
                while ancestor != owner:
                    chain.append(ancestor.matrix_local.copy())
                    ancestor = ancestor.parent
                matrix = Matrix.Identity(4)
                for local in reversed(chain):
                    matrix = matrix @ local
            else:
                matrix = (anchor.matrix_world.inverted() if anchor is not None else Matrix.Identity(4)) @ transform @ ev.matrix_world
            normal = matrix.to_3x3().inverted().transposed()
            reflected = matrix.to_3x3().determinant() < 0
            uv_layout = mesh_uv_layout(mesh)
            expected = mapping.get('objects', {}).get(names[original], {}).get('materials') or original.get('ang_source_materials')
            if isinstance(expected, str):
                expected = json.loads(expected)
            for triangle in mesh.loop_triangles:
                fallback = original.material_slots[triangle.material_index].material if triangle.material_index < len(original.material_slots) else None
                role = object_roles.get(names[original])
                name = 'ANG_ROLE__'+role if role else expected[min(triangle.material_index, len(expected)-1)] if expected else fallback.name if fallback else ''
                identity, mat = canonical(material(name, fallback))
                group = (names[owner] if owner is not None else '', partition, identity, uv_layout)
                batch = batches.setdefault(group, {'anchor': anchor, 'mat': mat, 'vertices': [], 'normals': [], 'uv_layout':uv_layout, 'uv_sets': [[] for _ in uv_layout[0]], 'sources': set()})
                batch['sources'].add(names[original])
                loops = list(triangle.loops)
                if reflected:
                    loops.reverse()
                for li in loops:
                    batch['vertices'].append(tuple(matrix @ mesh.vertices[mesh.loops[li].vertex_index].co))
                    batch['normals'].append(tuple((normal @ mesh.corner_normals[li].vector).normalized()))
                    for index, layer in enumerate(mesh.uv_layers):
                        batch['uv_sets'][index].append(tuple(layer.data[li].uv))
            ev.to_mesh_clear()
        for index, (group, batch) in enumerate(sorted(batches.items())):
            mesh = bpy.data.meshes.new('ANG_STATIC_'+str(index))
            vertices = batch['vertices']
            mesh.from_pydata(vertices, [], [tuple(range(i, i+3)) for i in range(0, len(vertices), 3)])
            mesh.update()
            write_batch_uv_layers(mesh, batch['uv_layout'], batch['uv_sets'])
            for poly in mesh.polygons:
                poly.use_smooth = True
            mesh.normals_split_custom_set_from_vertices(batch['normals'])
            if batch['mat'] is not None:
                mesh.materials.append(batch['mat'])
            mesh = share_mesh(mesh)
            duplicate = bpy.data.objects.new('ANG_STATIC_'+str(index), mesh)
            duplicate['ang_static_batch'] = True
            duplicate['ang_merged_source_nodes'] = sorted(batch['sources'])
            temporary.objects.link(duplicate)
            if batch['anchor'] is not None:
                duplicate.parent = batch['anchor']
        marker = bpy.data.objects.new('ANG_ExportPolicy', None)
        marker['ang_batch_material_names'] = sorted(source_material_names)
        marker['ang_batch_policy_sha256'] = batch_policy_signature(row, source_material_names, contract.get('material_bindings', {}))
        temporary.objects.link(marker)
        for obj in temporary.objects:
            obj.select_set(True)
        bpy.ops.export_scene.gltf(filepath=str(file), export_format='GLB', use_selection=True,
                                  export_yup=True, export_animations=False, export_lights=False,
                                  export_cameras=False, export_extras=True)
        return {'preserved_nodes': sorted(names[obj] for obj in kept),
                'static_material_batches': len(batches), 'spatial_partitions': sorted(used_partitions),
                'source_meshes': source_meshes, 'source_material_slots': source_surfaces,
                'canonical_materials': len(materials), 'exact_shared_mesh_instances': mesh_instances,
                'exported_meshes': len(batches)+sum(obj.type == 'MESH' for obj in kept),
                'policy': 'Static surfaces merge by exact material inside the nearest preserved ancestor or authored spatial partition; no motion frames are merged.'}
    finally:
        for duplicate in list(temporary.objects):
            bpy.data.objects.remove(duplicate, do_unlink=True)
        bpy.data.collections.remove(temporary)
        for original in originals:
            original.name = names[original]


def export_named_hierarchy(base, objects, transform, file):
    """Keep CUE-declared mechanism nodes; evaluated geometry stays at its pivot.

    Material batching is valid for static art, but merging a leaf and its frame
    destroys the motion contract. Copies isolate the export from the source.
    The normal ANG adapter still owns its one basis reflection.
    """
    members = set(objects)
    members.update(obj for obj in scene.objects if obj.type == 'EMPTY' and memberships.get(obj.name)==base and not obj.get('ang_ignore_reason'))
    for obj in objects:
        ancestor = obj.parent
        while ancestor is not None and memberships.get(ancestor.name) == base:
            members.add(ancestor)
            ancestor = ancestor.parent
    originals = sorted(members, key=lambda obj: obj.name)
    names = {obj: obj.name for obj in originals}
    copies = {}
    temporary = bpy.data.collections.new('ANG_NAMED_EXPORT')
    scene.collection.children.link(temporary)
    bpy.ops.object.select_all(action='DESELECT')
    try:
        for original in originals:
            original.name = 'ANG_SOURCE_' + names[original]
        for original in originals:
            if original.type == 'MESH':
                evaluated = original.evaluated_get(depsgraph)
                mesh = bpy.data.meshes.new_from_object(evaluated, depsgraph=depsgraph)
                duplicate = bpy.data.objects.new(names[original], mesh)
                if names[original] in object_roles:
                    role = object_roles[names[original]]
                    for index in range(len(mesh.materials)):
                        mesh.materials[index] = material('ANG_ROLE__'+role, mesh.materials[index])
            else:
                duplicate = bpy.data.objects.new(names[original], None)
            temporary.objects.link(duplicate)
            duplicate['ang_source_node'] = names[original]
            duplicate.matrix_world = transform @ original.matrix_world
            copies[original] = duplicate
        for original, duplicate in copies.items():
            world = duplicate.matrix_world.copy()
            if original.parent in copies:
                duplicate.parent = copies[original.parent]
                duplicate.matrix_world = world
            duplicate.select_set(True)
        bpy.ops.export_scene.gltf(filepath=str(file), export_format='GLB', use_selection=True,
                                  export_yup=True, export_animations=False, export_lights=False,
                                  export_cameras=False, export_extras=True)
        return [names[obj] for obj in originals]
    finally:
        for duplicate in copies.values():
            bpy.data.objects.remove(duplicate, do_unlink=True)
        bpy.data.collections.remove(temporary)
        for original in originals:
            original.name = names[original]

sources = source_assets
models = {}
ref_signatures = {}
lod_groups = []
for base,objects in sorted(groups.items()):
    levels = {}
    for obj in objects:
        levels.setdefault(properties[obj.name].get('ang_lod',0),[]).append(obj)
    for lod,items in sorted(levels.items()):
        lod_groups.append((base,lod,items))
for base,lod,objects in lod_groups:
    key = base if lod==0 else base+'.lod'+str(lod)
    row = known[base]
    if row['greybox']:
        continue
    transform = model_matrix(contract, base, mapping)
    mesh_hash, audit = signature(objects, transform, depsgraph)
    component = {'mesh_signature_schema':SIGNATURE_SCHEMA, 'id': base, 'lod':lod,'model_ref': row['model_ref'], 'mesh_signature': mesh_hash, 'triangles': sum(item['triangles'] for item in audit), 'objects': audit,'applied_material_roles':{o.name:object_roles[o.name] for o in objects if o.name in object_roles}, 'scale': [1, 1, 1], 'units': 'metre'}
    # An exact unchanged kit GLB is retained byte for byte. The signature covers
    # evaluated geometry, material identities, UV and normals at the CUE pivot.
    source_row = mapping.get('ids', {}).get(base, sources.get(base, {})) if lod==0 else {}
    asset = recorded_asset(source_row, row)
    selective_policy = bool(row.get('preserve_node_names') or row.get('batch_partitions'))
    binding_names = artifact_material_names(asset, original=True) if asset is not None else []
    if asset is not None and source_row.get('source_batch_policy'):
        check_retained_material_aliases(binding_names, artifact_material_names(asset), contract.get('material_bindings', {}))
    policy_matches = not selective_policy or source_row.get('source_batch_policy') == batch_policy_signature(row, binding_names, contract.get('material_bindings', {}))
    source_frames = [obj for obj in scene.objects if memberships.get(obj.name) == base]
    pose_hash = hierarchy_signature(source_frames, transform)
    artifact_pose_hash = artifact_pose_signature(asset) if asset is not None else ''
    pose_matches = source_pose_unchanged(source_row, row, pose_hash, artifact_pose_hash)
    component.update({'source_pose_signature':pose_hash, 'canonical_artifact_pose_signature':artifact_pose_hash,
                      'canonical_pose_unchanged':pose_matches})
    passthrough = source_row.get('source_signature_schema') == SIGNATURE_SCHEMA and policy_matches and pose_matches and not any(obj.name in object_roles for obj in objects) and source_row.get('source_signature') == mesh_hash and asset is not None
    if passthrough and str(row.get('asset_resource', '')).startswith('res://'):
        # The kit artifact has already passed the same geometry and batching
        # contract. Reuse its registered resource, not a second embedded-texture
        # copy under every consuming level's model directory.
        resource = row['asset_resource']
        retired = [] if resource in [prefix+key+'.glb',prefix+key+'.tscn'] else [key+'.glb',key+'.tscn']
        report.setdefault('retired_artifacts', []).extend(retired)
        for name in retired:
            previous = out / name
            if previous.is_file():
                previous.unlink()
        component.update({'reused_resource':resource, 'unchanged_source':str(asset),
                          'unchanged_source_sha256':digest(asset),
                          'reuse_policy':'Exact source SHA and canonical named parent/TRS graph, unchanged evaluated geometry and imported local-frame memo, matching export policy and no material override'})
    elif passthrough:
        file = out / (key + '.glb')
        shutil.copy2(asset, file)
        if source_row.get('asset_basis', row.get('asset_basis', 'source_plan')) == 'godot':
            adapter = out / (key + '.tscn')
            adapter.write_text('[gd_scene format=3]\n[ext_resource type="PackedScene" path=' + json.dumps(prefix + file.name) + ' id="asset"]\n[node name="Art" type="Node3D"]\n[node name="Source" parent="." instance=ExtResource("asset")]\nscale = Vector3(1, 1, -1)\n')
            resource = prefix + adapter.name
            component.update({'adapter': adapter.name, 'adapter_sha256': digest(adapter)})
        else:
            resource = prefix + file.name
        component.update({'file': file.name, 'sha256': digest(file), 'unchanged_source': str(asset), 'unchanged_source_sha256': digest(asset)})
    elif row.get('preserve_node_names') or row.get('batch_partitions'):
        file = out / (key + '.glb')
        batching = export_batched_hierarchy(base, objects, transform, file, row)
        adapter = out / (key + '.tscn')
        adapter.write_text('[gd_scene format=3]\n[ext_resource type="PackedScene" path=' + json.dumps(prefix + file.name) + ' id="asset"]\n[node name="Art" type="Node3D"]\n[node name="Source" parent="." instance=ExtResource("asset")]\nscale = Vector3(1, 1, -1)\n')
        resource = prefix + adapter.name
        component.update({'file': file.name, 'sha256': digest(file), 'adapter':adapter.name,
                          'adapter_sha256':digest(adapter), **batching,
                          'motion_policy':'Only CUE preserved nodes remain separate; static material batching is generic'})
    elif row.get('preserve_nodes', False):
        file = out / (key + '.glb')
        preserved = export_named_hierarchy(base, objects, transform, file)
        adapter = out / (key + '.tscn')
        adapter.write_text('[gd_scene format=3]\n[ext_resource type="PackedScene" path=' + json.dumps(prefix + file.name) + ' id="asset"]\n[node name="Art" type="Node3D"]\n[node name="Source" parent="." instance=ExtResource("asset")]\nscale = Vector3(1, 1, -1)\n')
        resource = prefix + adapter.name
        component.update({'file':file.name, 'sha256':digest(file), 'adapter':adapter.name,
                          'adapter_sha256':digest(adapter), 'preserved_nodes':preserved,
                          'motion_policy':'server poses named nodes; animations are not exported'})
    else:
        batches = {}
        for obj in objects:
            ev = obj.evaluated_get(depsgraph)
            mesh = ev.to_mesh()
            mesh.calc_loop_triangles()
            check_batch_attributes(mesh, obj.name)
            uv_layout = mesh_uv_layout(mesh)
            expected = mapping.get('objects', {}).get(obj.name, {}).get('materials') or obj.get('ang_source_materials')
            if isinstance(expected, str):
                expected = json.loads(expected)
            normal = (transform @ ev.matrix_world).to_3x3().inverted().transposed()
            for triangle in mesh.loop_triangles:
                fallback = obj.material_slots[triangle.material_index].material if triangle.material_index < len(obj.material_slots) else None
                name = 'ANG_ROLE__'+object_roles[obj.name] if obj.name in object_roles else expected[min(triangle.material_index, len(expected) - 1)] if expected else fallback.name if fallback else ''
                batch = batches.setdefault((name,uv_layout), {'mat': material(name, fallback), 'vertices': [], 'uv_layout':uv_layout, 'uv_sets':[[] for _ in uv_layout[0]], 'normals': []})
                for li in triangle.loops:
                    batch['vertices'].append(tuple(transform @ ev.matrix_world @ mesh.vertices[mesh.loops[li].vertex_index].co))
                    batch['normals'].append(tuple((normal @ mesh.corner_normals[li].vector).normalized()))
                    for index, layer in enumerate(mesh.uv_layers):
                        batch['uv_sets'][index].append(tuple(layer.data[li].uv))
            ev.to_mesh_clear()
        bpy.ops.object.select_all(action='DESELECT')
        exported = []
        for index, (name, batch) in enumerate(sorted(batches.items())):
            mesh = bpy.data.meshes.new(key + str(index))
            vertices = batch['vertices']
            mesh.from_pydata(vertices, [], [tuple(range(i, i + 3)) for i in range(0, len(vertices), 3)])
            mesh.update()
            if transform.to_3x3().determinant() < 0:
                mesh.flip_normals()
            write_batch_uv_layers(mesh, batch['uv_layout'], batch['uv_sets'])
            for poly in mesh.polygons:
                poly.use_smooth = True
            mesh.normals_split_custom_set_from_vertices(batch['normals'])
            if batch['mat']:
                mesh.materials.append(batch['mat'])
            obj = bpy.data.objects.new('SM_' + key.replace('.', '_') + '_' + str(index), mesh)
            scene.collection.objects.link(obj)
            obj.select_set(True)
            exported.append(obj)
        file = out / (key + '.glb')
        bpy.ops.export_scene.gltf(filepath=str(file), export_format='GLB', use_selection=True, export_yup=True, export_animations=False, export_lights=False, export_cameras=False)
        adapter = out / (key + '.tscn')
        adapter.write_text('[gd_scene format=3]\n[ext_resource type="PackedScene" path=' + json.dumps(prefix + file.name) + ' id="asset"]\n[node name="Art" type="Node3D"]\n[node name="Source" parent="." instance=ExtResource("asset")]\nscale = Vector3(1, 1, -1)\n')
        resource = prefix + adapter.name
        component.update({'file': file.name, 'sha256': digest(file), 'adapter': adapter.name, 'adapter_sha256': digest(adapter), 'material_batches': len(batches)})
        for obj in exported:
            obj.hide_set(True)
    ref = row['model_ref'] if lod==0 else ''
    if ref:
        if ref in ref_signatures and ref_signatures[ref] != ('source:'+digest(asset) if passthrough else mesh_hash):
            fail(f'{key!r}: model_ref={ref!r} has different geometry on multiple IDs; select distinct model_ref variants in CUE')
        ref_signatures[ref] = 'source:'+digest(asset) if passthrough else mesh_hash
        models[ref] = resource
    report['components'].append(component)
assert before == digest(source), 'source .blend changed during export'
report['source_preserved'] = True
write_json(out / 'models.json', models)
write_json(out / 'model-attachments.json', attachments)
write_json(out / 'provenance.json', report)
print('ANG_BLENDER_EXPORT_PASS', len(report['components']), 'IDs;', report['totals'])
