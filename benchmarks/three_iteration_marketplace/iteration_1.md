# Iteration 1

## Goal

Add the base order domain and webhook endpoint.

## Expected Action

- Create `cue/domain/order.cue` with `Order` and FSM.
- Create `cue/api/orders.cue` with `CreateOrder` / `ConfirmPayment`.
- Add HTTP endpoints in `cue/api/http.cue`.

## Expected Result

Build fails with FSM error (`paid` is used in transition but missing in `states`).
