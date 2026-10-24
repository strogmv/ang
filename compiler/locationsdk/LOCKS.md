# Explicit per-layout locks

`Input.Locks` / CUE `sdk.locks` is an optional list:

```cue
locks: [{layout_id: "example.market_strip", expected_hash: "<64 lowercase hex characters>"}]
```

Use the same command from an ANG checkout or installed binary:

```sh
ang location-sdk -fingerprint example.market_strip /absolute/CUE/package
```

It prints one ready-to-paste JSON object with `layout_id` and `expected_hash`.
It writes no file, changes no lock, and intentionally ignores old expectations
so a changed layout can be reviewed. Normal `-check`, `-out` and game-poc
`CompileRuntime` all call `ValidateLocks` before publishing output.

To accept a revision: inspect the intended layout and dependency changes, run
`-fingerprint` again, manually replace that layout's `expected_hash` in CUE,
then run `ang location-sdk -check /absolute/CUE/package`. Review/version the
source and new expectation together. Printing a candidate does not accept it.
To delete a locked layout, explicitly remove its lock too; a stale ID fails.

The SHA-256 includes the canonical complete authored layout (including seed,
ordered spawn entries and ordered guide points), compiled geometry/metadata,
all composed views and decoration boxes, used module/geometry/decoration
definitions, initial time/event selection and only the selected instance/floor
surface definitions. All catalog time/event states compose into each layout's
views, so changing shared light/events requires acceptance for affected locks.
Valid edits to other layouts and unused surface definitions remain independent.
Unordered record permutations do not alter the fingerprint.

Resources names and metric texture repeats are included; **PNG bytes are not**.
Keep physical asset files under version control and review those changes too.
The core performs no hidden filesystem reads. This is a deterministic change
guard, not an asset snapshot, runtime scene cache, permission system or automatic
approval. Empty/absent locks preserve the previous compiler behavior.
