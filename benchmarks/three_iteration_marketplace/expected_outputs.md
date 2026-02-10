# Expected Outputs

После `make benchmark-3iter` должны появиться:

- `benchmarks/three_iteration_marketplace/output/summary.md`
- `benchmarks/three_iteration_marketplace/output/iter1.build.log`
- `benchmarks/three_iteration_marketplace/output/iter2.build.log`
- `benchmarks/three_iteration_marketplace/output/iter3.build.log`

## Pass Criteria

- Iteration 1: `build` = FAIL (ожидаемая диагностическая ошибка).
- Iteration 2: `build` = SUCCESS.
- Iteration 3: `build` = SUCCESS.
- В финале сгенерировано `>= 30` файлов в `dist/release/go-service`.
