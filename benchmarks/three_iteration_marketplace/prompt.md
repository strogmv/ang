# 3-Iteration Backend Prompt (Marketplace)

Создай backend для маркетплейса: товары с категориями, корзина, заказы с состояниями `draft -> paid -> shipped -> delivered`, платежи через Stripe webhook, уведомления продавцу по email при новом заказе.

## Success Criteria

- Реализовано в 3 итерации или меньше.
- После итерации 3: `ang build` успешен.
- Сгенерирован полный backend артефакт (domain, transport, repos, OpenAPI, SDK).

## Notes

- Этот benchmark фиксирует воспроизводимый сценарий для демо и CI.
- Исполнение: `make benchmark-3iter`.
