# Iteration 2

## Goal

Fix the FSM error from iteration 1.

## Expected Action

1. `ang_doctor` reads the log and returns a suggestion:
   - `FSM state 'paid' not in states list, add it`.
2. `cue_apply_patch` updates `cue/domain/order.cue`:
   - add `paid` to `fsm.states`.
3. `run_preset('build')`.
4. `repo_diff`/artifact verification:
   - capture generated file count.

## Expected Result

- `ang build` is successful.
- Backend and API artifacts are generated (domain, HTTP, repos, events, OpenAPI, SDK).
- For the current benchmark run, at least `47` generated files are expected in the final artifact.
