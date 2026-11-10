"""ANG generic ID exporter. Run through the serial wrapper; never saves source."""
import argparse
import hashlib
import json
import math
import shutil
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from runtime import recorded_asset, apply_legacy, digest, fail, identity_tags, import_asset, model_matrix, signature, validate_geometry, check_known_conflicts, inherited_properties, resolve_wall_role, art_consumers, installed_triangle_budget, write_json
import bpy
from mathutils import Matrix

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
    component = {'id': base, 'lod':lod,'model_ref': row['model_ref'], 'mesh_signature': mesh_hash, 'triangles': sum(item['triangles'] for item in audit), 'objects': audit,'applied_material_roles':{o.name:object_roles[o.name] for o in objects if o.name in object_roles}, 'scale': [1, 1, 1], 'units': 'metre'}
    # An exact unchanged kit GLB is retained byte for byte. The signature covers
    # evaluated geometry, material identities, UV and normals at the CUE pivot.
    source_row = mapping.get('ids', {}).get(base, sources.get(base, {})) if lod==0 else {}
    asset = recorded_asset(source_row, row)
    passthrough = not any(obj.name in object_roles for obj in objects) and source_row.get('source_signature') == mesh_hash and asset is not None
    if passthrough:
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
            uv = mesh.uv_layers.active
            expected = mapping.get('objects', {}).get(obj.name, {}).get('materials') or obj.get('ang_source_materials')
            if isinstance(expected, str):
                expected = json.loads(expected)
            normal = (transform @ ev.matrix_world).to_3x3().inverted().transposed()
            for triangle in mesh.loop_triangles:
                fallback = obj.material_slots[triangle.material_index].material if triangle.material_index < len(obj.material_slots) else None
                name = 'ANG_ROLE__'+object_roles[obj.name] if obj.name in object_roles else expected[min(triangle.material_index, len(expected) - 1)] if expected else fallback.name if fallback else ''
                batch = batches.setdefault(name, {'mat': material(name, fallback), 'vertices': [], 'uv': [], 'normals': []})
                for li in triangle.loops:
                    batch['vertices'].append(tuple(transform @ ev.matrix_world @ mesh.vertices[mesh.loops[li].vertex_index].co))
                    batch['normals'].append(tuple((normal @ mesh.corner_normals[li].vector).normalized()))
                    batch['uv'].append(tuple(uv.data[li].uv) if uv else (0, 0))
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
            layer = mesh.uv_layers.new(name='UVMap')
            for loop in mesh.loops:
                layer.data[loop.index].uv = batch['uv'][loop.vertex_index]
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
