# Godot location target: live CUE input

The entry point is `ang godot`. It compiles the ordinary `sdk` CUE input with
`locationsdk.Build`, resolves engine-neutral placement intent, and renders
`compiler/godot/location.tscn.tmpl`. The Unity target is unchanged.

```sh
go run ./cmd/ang godot \
  -layout example.market_hub \
  -models examples/godot/cue/market/models.json \
  -out examples/godot/market \
  examples/godot/cue/market
```

Outputs are `compiled.json`, `placements.json`, and `location.tscn`. `-check`
validates the complete pipeline and resource mapping without writing files.
Model files stay in the client; no GLB, prefab, or Unity GUID enters the compiler.
The client map selects a `res://` GLB/GLTF or packed scene for each `model_ref`.
Missing mappings fail generation before any existing output is replaced.

## Coordinate contract

`compiler/placement/world.go` owns `ang.world.east-up-north.v1`:

- Plan cells are 500 mm. X is east; plan Y is south; height is up.
- Source metric `(east, up, south)` becomes `(X, Y, -Z)`: east +X, south -Z,
  up +Y, with no axis swap. This is Unity `(X,Y,Z)` → Godot `(X,Y,-Z)`.
- Compiled footprints are already rotated. `Pose` compensates their NW pivot
  for all four quarter turns, and takes structural elevation from compiled slabs,
  obstacles, supports, railings, and connectors.
- Models use the source plan-local numeric basis, matching the oracle kit GLBs.
  World yaw is **+CUE rotation**, with scale `(1,1,-1)`. This is exactly the
  oracle's reflected parent followed by local yaw **-CUE rotation**. The emitter
  writes the complete `Transform3D`; clients consume it without re-deriving it.
- Camera right/up/back vectors pass through that same conversion. The market
  keeps its compiled 35° elevation / 37° azimuth, orthographic size 49 m,
  focus `(28.5,4.6,-22)`, and distance 120 m. No corrective camera yaw is added.

For models authored directly in the canonical world-local basis, use a client
wrapper scene to adapt the asset to the declared source model basis once.

## Manifest and CUE authoring

`ang.placement.v1` has `layout_id`, `world_frame`, `unit_mm`, `structural_hash`,
`compiled_ref`, and an `instances` object keyed by stable `instance_id`:

```json
{
  "world_transform": {
    "position": [3.5, 8.55, -4],
    "rotation_degrees": [0, 0, 0],
    "scale": [1, 1, -1]
  },
  "model_ref": "market.CraneV1_9a_bridge",
  "source_kind": "surface",
  "bindings": {"light": [], "storefront": [], "gate": []},
  "nav_refs": ["#/navigation/surfaces/1"],
  "clearance_refs": []
}
```

References are JSON pointers relative to the compiled level selected by
`compiled_ref` (`compiled.json#/levels/0`). They address footprint/blocked cells,
ports, navigation structures, and compiled clearance records. Empty bindings
are omitted. An empty `model_ref` deliberately carries a reservation or a
structural/batched record without inventing a substitute asset.

Optional `placement` CUE contains:

- `layout_id`, `camera` framing, and `module_models`: module/passport IDs → stable
  model IDs. Without a mapping, a floor instance's module ID is its model ID.
- `instances`: model variants and light/storefront/gate bindings by instance ID.
  Layout instance transforms always come from the compiled SDK. `pose_override`
  is rejected, even when it includes a reason; author geometry changes in CUE.
- `extras`: named static metric poses for external CUE assemblies, such as a
  train sampled at its authored simulation epoch.

A new location supplies `sdk.catalog` + `sdk.layouts`, optionally `placement`,
and a client model map, then runs the command above with its layout ID. A
separate `-placement <CUE package>` overlays presentation intent when the SDK
source is maintained elsewhere. It does not modify the source layout or nav.

The TSCN contains all placement identities, 58 shared external model resources
for the market's 70 model instances, compiled floor slabs and unbound obstacle
bounds, an orthographic camera, and a simple lighting placeholder. Rich lighting,
materials, dressing, runtime bindings, movement and train simulation stay client-side.

## Oracle verification

`examples/godot/cue/market` is ONLY a frozen regression fixture, with independent
numeric expectations in `compiler/placement/testdata/market-oracle.json`. It is
never imported by the production path. Its three service expectations now use
the actual west-wall SDK poses, without presentation position overrides.

The game owns its model identities, asset paths, attachment metadata and epoch
adapter under `content/world/godot/market`. Its `scripts/godot-gen.sh market`
passes the LIVE `content/world/location_sdk` package directly to `ang godot`.
The epoch adapter imports the actual train CUE; it carries no copied layout.

For verification against the live game source and its canonical prototype:

```sh
python3 scripts/verify_godot_market.py \
  --game-root /path/to/angels-demons-book \
  --out /tmp/ang-godot-market \
  --godot /path/to/Godot \
  --report docs/godot-market-verification.json
```

The script generates from the game's CUE, compares against its
`market-placements.json`, copies only the matched kit GLBs into staging, imports
them, and loads the generated scene headlessly. Native checks compare every
global transform with the oracle and verify camera orientation: the west crane
projects screen-right and the actual train model projects lower-right. Source
hashes prove the reference generator and placement/orientation data were preserved.
Omit `--godot` for the numerical comparison alone. Use a private/self-contained
Godot executable when the owner's editor profile must stay untouched.

The comparison checks all 215 identities, model resources and complete transforms,
and rejects visual position overrides. The native step imports the generated
scene and checks the camera orientation and all model transforms. The live-import
CLI regression changes a transitive CUE dependency and verifies regenerated
positions; legacy `pose_override` input must fail without replacing good output.

The game integration report records its independent full-generation/server status;
a passing placement check does not establish that the game's other ungenerated
CUE changes can produce a new server.

## Resolved world lettering

`ang game-poc --godot-only <backend>` reads live CUE and refreshes Godot PO,
Theme and `generated/world_text/` without writing server or legacy client files.
Ordinary `game-poc` delivers the same world-text outputs to each `godot_targets`
entry. The CUE `world_text.metrics_dir` contains the atlas producer's
`assets-manifest.json`: SHA-256 of every metrics JSON, SDF PNG payload, font
manifest and OFL licence. Delivery rejects changes before writing assets; the
native generated consumer rechecks the same hashes. TTF art remains source.

`runtime.json` (`ang.godot.world_text.v1`) has `world_frame`, font tracking/pitch,
and resolved slots. A slot carries its style/surface/ink/wear, plate, authored
anchor/strip node references, per-locale lines/mode/cap height/first strip, logical
line advances and occupied layout width. Its centre/right/up/normal use
`ang.world.east-up-north.v1`, through placement.Pose/PlanTransform/BasisColumns.
Runtime attachment nodes can replace the centre; no state rule is inferred from
a slot's spelling. Missing authored bindings remain missing.

`ang godot -world-text` attaches the generated GDScript renderer.
`-attachments <JSON>` accepts `ang.model-attachments.v1`, `source_frame:model-local`,
and `models:{model_ref:[{name,position:[x,y,z],source_path}]}`. These are source-art
locators in the mesh's own basis, beneath the existing placement transform;
they contain no world poses or text. The game preserves its former mesh-only
art import's locators from its hashed asset sources, without running Unity.

Three MultiMeshes batch ordinary glyph quads by style, with linear SDF and float
data textures; plates share a fourth MultiMesh. One extra ArrayMesh batches strip
glyphs clipped to their original surface triangles (five drawable batches total). Locale notifications rebuild once.
Actual attachment motion updates matrices, visibility signals update their
instances. Strip quads clip to authored pitch/width and actual curved mesh edges.
An explicit `ang_world_text_deformation` node contract can bind a `cloth_v1`
material/phase. A shared generated shader include keeps surface and lettering on
the same deformation and existing material time; it creates no simulation clock.
