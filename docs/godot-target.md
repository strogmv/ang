# Godot location target: Phase 0 + 1

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
  `pose_override` requires a reason and preserves `source_transform` in the output.
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

`examples/godot/cue/market` is a frozen, engine-neutral export of the market's
actual CUE authoring. It retains the three previously approved service-band
overrides and computes the train's static epoch pose in CUE. The regression test
compiles it and compares all 215 identities, positions, model choices and complete
bases against an independent compact oracle in
`compiler/placement/testdata/market-oracle.json`. It also verifies deterministic
regeneration, reference resolution, source edits and failed-build preservation.

For verification against the live game source and its unchanged prototype:

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

See [verification evidence](godot-market-verification.json) for measured results.
All 215 placements match: maximum numerical position error `7.11e-15 m`, no model
or basis mismatches. Godot 4.7.2 loads all 70 model instances using 58 resources;
native transform, authored camera focus, west-crane/right and train/lower-right
checks pass with zero engine errors. The three service overrides are explicit CUE.
`go build ./...`, `go vet ./...`, `go test ./... -count=1`, and `make lint` pass.
The generator uses Godot's [TSCN format](https://docs.godotengine.org/en/4.7/engine_details/file_formats/tscn.html).
