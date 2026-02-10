# 3-Iteration Backend Prompt (Marketplace)

Create a marketplace backend: products with categories, cart, orders with states `draft -> paid -> shipped -> delivered`, Stripe webhook payments, and seller email notifications on new orders.

## Success Criteria

- Implemented in 3 iterations or less.
- After iteration 3: `ang build` is successful.
- After iteration 3: `run_preset('unit')` reports `34 tests passed`.
- A full backend artifact is generated (domain, transport, repos, OpenAPI, SDK).

## Notes

- This benchmark captures a reproducible scenario for demo and CI.
- Run: `make benchmark-3iter`.
