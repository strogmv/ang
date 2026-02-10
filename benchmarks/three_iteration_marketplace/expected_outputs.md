# Expected Outputs

After `make benchmark-3iter`, the following files must exist:

- `benchmarks/three_iteration_marketplace/output/summary.md`
- `benchmarks/three_iteration_marketplace/output/iter1.build.log`
- `benchmarks/three_iteration_marketplace/output/iter2.build.log`
- `benchmarks/three_iteration_marketplace/output/iter3.build.log`
- `benchmarks/three_iteration_marketplace/output/iter3.unit.log`

## Pass Criteria

- Iteration 1: `build` = FAIL (expected diagnostic failure).
- Iteration 2: `build` = SUCCESS.
- Iteration 3: `build` = SUCCESS.
- Iteration 3: `unit` = `34 tests passed`.
- Iteration 2 doctor hint includes the FSM fix (add `paid` to `Order.fsm.states`).
- Final artifact has `>= 47` files in `dist/release/go-service` (repo diff / file count gate).
