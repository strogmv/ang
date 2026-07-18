# Wallet checkout knowledge (moved)

Structured knowledge lives in **deal/expert**, not in ang docs or provider repos:

- `deal/expert/knowledge/data/wallet-checkout.json` — cross-provider patterns
- `deal/expert/knowledge/data/centrobill-mx6.json` — MX-6 ticket + API facts + PM options
- `deal/expert/knowledge/wallet-checkout.research.cue` — audit rules
- `deal/expert/knowledge/centrobill-mx6.research.cue` — MX-6 audit rules

See `deal/expert/knowledge/README.md`.

Load brief:

```bash
export ANG_EXPERT_ROOT=/path/to/deal/expert
ang pp brief payment_providers/mx6_centrobill
```
