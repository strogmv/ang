# Expert report versioning policy

The public expert result is a versioned JSON document. The current schema is
`ang/expert-report/v1`, represented by `compiler/expert.Report`.

## Compatibility rules

1. A producer must always write the exact schema string it implements.
2. A consumer must reject an unknown major version. It must not guess field
   meanings or treat a newer major as v1.
3. Adding an optional field to v1 is backward compatible. The field must use
   `omitempty`, have documented zero-value semantics, and be covered by a
   golden JSON test.
4. Changing a required field, its JSON name, its meaning, an enum value, or
   the ordering/canonicalization rules requires a new major schema, for
   example `ang/expert-report/v2`.
5. A new major schema requires an explicit adapter from every still-supported
   prior major. The adapter belongs outside inference and is tested with input
   fixtures from the old version.
6. Unknown fields may be ignored by readers only when the document has the
   same supported major version. Unknown enum values are errors.

## Determinism rules

- Reports are stored and compared through `expert.CanonicalJSON`, not through
  incidental Go struct order or map iteration.
- IDs are derived from canonical content; random UUIDs and timestamps are not
  part of an expert report contract.
- `Changes` retain declared order because patch application can be
  order-sensitive. Other diagnostic collections are canonicalized by stable
  keys.
- A change to canonical JSON or `expert.Hash` is a contract change and needs
  a golden-test review.

## Safety rules

- `unknown` facts must not contain invented values.
- A `delete` change always sets `requires_approval: true` in v1.
- Every proposed change targets a relative `.cue` file under `cue/` and has a
  non-empty CUE path and rationale. `insert`, `merge`, and `replace` also
  carry an explicit JSON value. A report validator rejects paths outside this
  intent scope before any later apply workflow can see them.
- A report is evidence, advice, or verification output; it does not authorize
  writes. Applying proposals is a later, separately versioned workflow.

## Change checklist

Before changing `compiler/expert`:

1. Decide whether the change is additive or requires a new major schema.
2. Update the constants, model documentation, validation and canonicalization
   together.
3. Add or update a golden JSON fixture and a determinism test.
4. Verify an old v1 fixture still decodes as intended, or add a migration
   adapter and rejection test.
5. Do not couple a schema change to new inference rules, LLM integration, or
   proposal application in the same patch.
