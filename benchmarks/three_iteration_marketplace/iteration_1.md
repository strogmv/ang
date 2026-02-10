# Iteration 1

## Goal

Добавить базовый домен заказов и webhook endpoint.

## Expected Action

- Создать `cue/domain/order.cue` с `Order` и FSM.
- Создать `cue/api/orders.cue` с `CreateOrder`/`ConfirmPayment`.
- Добавить HTTP endpoints в `cue/api/http.cue`.

## Expected Result

Build падает на ошибке FSM (`paid` используется в transition, но отсутствует в `states`).
