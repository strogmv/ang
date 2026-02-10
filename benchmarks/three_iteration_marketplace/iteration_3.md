# Iteration 3

## Goal

Add business logic for payments and notifications.

## Expected Action

1. `cue_apply_patch` updates `cue/architecture/services.cue`:
   - flow for `CreateOrder`, `ConfirmPayment`, and `ShipOrder`.
2. `run_preset('build')`.
3. `run_preset('unit')`.

## Expected Result

- `ang build` is successful.
- Unit suite passes: `34 tests passed`.
- The scenario is completed in 3 iterations.
