# Planner Contract (ScanPlan / RenderPlan)

Templates should consume precomputed plans, not embed selection/scanning logic.

Core contracts:

- `compiler/planner/contracts.go`
  - `ScanVariable`
  - `ScanPlan`
  - `RenderPlan`

## Intent

- `ScanPlan`: deterministic mapping from selected DB columns to target fields.
- `RenderPlan`: normalized template envelope with precomputed data.

## Current Use

- Postgres repository emitter uses:
  - `planner.ScanPlan` for finder scans
  - `planner.RenderPlan` as template input envelope

This is the base for moving more emitter logic from templates to planners.

