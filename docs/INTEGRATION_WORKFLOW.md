# Integration workflow (PM ticket → CUE → template → Expert)

Instrumental base for payment-provider integrations.

**Knowledge base lives in deal/expert** (`knowledge/data/*.json` + `*.research.cue`), not in provider repos.

## Artifacts per provider

```text
payment_providers/<sid>/
  ang.yaml              # build config + expert_knowledge_id
  .cue/
    provider.cue        # ANG intent
    cue.mod/
  *.go                  # hand-written OR ang build output
```

**No** `integration/api-notes.md`, checklists, or markdown in provider trees.

## Expert knowledge (deal/expert)

```text
knowledge/
  data/
    wallet-checkout.json       # cross-provider wallet patterns
    centrobill-mx6.json        # per-integration ticket + API facts
  payment-provider.core.cue
  wallet-checkout.research.cue
  centrobill-mx6.research.cue
```

## Agent order of work

1. Read Expert `knowledge/data/<id>.json` (via `ang pp brief` with `ANG_EXPERT_ROOT`).
2. Draft or update `.cue/provider.cue`.
3. `ang pp vet` → `ang pp facts` → Expert with research packs.
4. `ang build` **only** when generation-first (not investigation / hand-written).

## Commands

```bash
export ANG_EXPERT_ROOT=/path/to/deal/expert

ang pp init payment_providers/mx6_centrobill \
  --sid mx6 --label "MX-6" --package mx6_centrobill \
  --ticket "Integrate Apple Pay and Google Pay for Centrobill (EUR redirect checkout)"

ang pp brief payment_providers/mx6_centrobill
ang pp brief payment_providers/mx6_centrobill --json

ang pp pack validate $ANG_EXPERT_ROOT/knowledge/centrobill-mx6.research.cue

ang pp vet payment_providers/mx6_centrobill
ang pp facts payment_providers/mx6_centrobill --json
ang pp expert payment_providers/mx6_centrobill \
  --mode advise \
  --expert-base-url http://127.0.0.1:8787 \
  --expert-pack payment-provider.core \
  --expert-pack centrobill-mx6.research
```

## Hand-written vs generated Go

| Mode | When | `ang build` |
|------|------|-------------|
| Investigation | CUE scaffold only | dry-run only |
| Hand-written | incas-style pilots | **never** overwrite |
| Generated | greenfield | yes |

Mode is in Expert `knowledge/data/<id>.json` field `implementation`.

## Checkout / redirect

- [CHECKOUT_PROFILE_DSL.md](./CHECKOUT_PROFILE_DSL.md) — CUE vs template boundaries
- Expert `knowledge/data/wallet-checkout.json` — wallet pattern catalog
- Expert `knowledge/data/centrobill-mx6.json` — MX-6 investigation
