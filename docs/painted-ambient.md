# Painted ambient presentation

`PaintedScene.ambient_passages` decorates the current painted scene. It creates no
Life actor, collision, passenger seat, route, boarding opportunity, or sound event.
Boarding and physical travel remain in Life. This book uses the same authored
`transit/c7_loop` day/night calendar for the decorative passage and actual Life
rail departures; presentation never performs a passenger transition.

Every passage has an `id`, Resources `image`, optional source-image `source_rect`,
drawn `width`/`height`, sprite `feet_x`/`feet_y`, a feet `path`, a `visible_strip`
polygon, `observation_cells`, and an `elevation_m` below the scene's explicit
`floor_elevation_m`. Paths, strip vertices, drawn dimensions and feet use logical
scene canvas pixels. Only source rectangles use source image pixels; observation
cells use the authoritative navigation grid. Path speed follows arc length.

The optional `track` has its own image/source rectangle and canvas
`x`, `y`, `width`, `height`. It remains stationary between passages. Omit it when
the approved architecture background already contains rails.

`period_seconds`, `duration_seconds`, and `offset_seconds` use game calendar
seconds. For a day/night schedule additionally set all of
`night_period_seconds`, `day_start_second`, and `day_end_second`:

```cue
period_seconds: 300
night_period_seconds: 3600
day_start_second: 21600 // provisional 06:00
day_end_second: 79200   // provisional 22:00
duration_seconds: 15
offset_seconds: 0
```

The offset delays the first departure from the beginning of **each** window.
Night continues through midnight from the preceding evening. A departure that
cannot finish within its window is suppressed, preventing overlapping boundary
departures. Older passages with no night schedule retain a single periodic clock.

Rendering samples authoritative `GameSeconds` from the fixed boundary of
`state.Tick` using nominal `config.tick_hz` and the existing bounded
`renderedSimNS`. Fractional snapshot `SimTimeNs` and rolling measured server Hz
are not calendar origins. A cursor retains the maximum sample within one
level/epoch, so snapshot jitter, a rate reduction and pause cannot reverse a
passing pose. It has no accumulated timer. Load/level epochs reset the cursor.
Only authoritative `GameSeconds` starts/stops a passage; presentation lead moves
it within an already active departure and cannot flash an early train. Rates
still come from actual `Life.SecondsPerTick`.

Compilation rejects strips intersecting projected walkable-cell interiors,
invalid/nonwalkable observation cells, impossible elevations, bad assets and
out-of-budget geometry. At cold scene preparation, a bounded mask assigns each
strip pixel to the closest observation cell in the same affine projection. Live
train alpha is permitted only where that cell is **currently** visible (`2`).
Remembered architecture does not provide live surveillance. A tiny observation
texture uploads only when those server visibility flags change. Static rails and
train are clipped by the strip and drawn before ordinary fog and actors.

Cold masks, a bounded RenderTexture, and materials belong only to the current
scene. Source art pixels remain untouched. No mask generation or full pixel
processing occurs on moving frames; the GPU composites the existing textures.
The Resources shader must remain included in native builds.

`daylight` is optional visual darkening of the world, before HUD composition:

```cue
daylight: {
    source: "passive_lightguides"
    sunrise_second: 21600
    sunset_second: 79200
    twilight_seconds: 3600
    day_brightness: 1
    night_brightness: 0.65
}
```

Twilight spans symmetrically around each authored sunrise/sunset. This represents
passive collectors with weak backup illumination; it changes no network service,
gameplay perception, stealth rule, or opaque unknown-space privacy.

Compiler and actual C# math tests cover geometry rejection, calendar boundaries,
pause/load/rates, polyline interpolation and viewport transforms. Native Player
captures must additionally verify GPU clipping, UV orientation, asset alignment,
and the actual day/night appearance for each authored placement.

The opt-in windowed Player mode is
`--poc-verify-ambient /absolute/report-directory`. It retains one real ordinary `life`
session (the six canonical authored actors, no synthetic residents) and uses ordinary `goto`, adjacent observer-known `ClickMove`, calendar
rate and pause commands. It never assigns a clock, scene or hero position.
A full unpaused market pass first records Repaint clock/phase/visibility samples
and proof-only GPU body probes, rejecting a reversed phase or a body that blinks
out and back with unchanged observation. The six paused images then cover two
actual market bodies, housing/station by day, and market/housing at night. Pose
times follow the currently observed platform segment rather than a fixed point
in the pass. Separate train-only GPU images and alpha counts prevent a static
rail from satisfying train presence. The selected scenario and actually observed
named actors are recorded and validated; ordinary gameplay, SDK/copper Life100
proofs and population stress budgets retain their own crowd scenarios. Each image includes an actual RenderTexture PNG and RGBA fingerprint;
nonempty colored alpha must remain inside the authored strip and stay identical
through paused render frames. Distinct market phases must have different GPU
images. Pixel readback exists only in this proof partial, with the existing
420-second Life-proof deadline. Visual review of the Player images is still
required; these assertions cannot judge the artistic fit of a train to its rail.
