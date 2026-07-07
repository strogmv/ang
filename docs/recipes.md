# ANG recipes

## Auth-enabled service

```bash
ang init auth-app --template saas
cd auth-app
ang validate
ang build --log-format json
ang doctor --project-path .
```

Keep auth mode and refresh-store selection in `cue/`; generated bootstrap DI is
validated before the build is committed.

## Event worker

Declare publishers/subscribers in CUE, then run:

```bash
ang validate
ang build
ang events map
```

Subscriptions use queue delivery by default. Choose `delivery: "broadcast"`
only when every replica must receive the event.

## Webhook endpoint

Add the HTTP endpoint and operation under `cue/api/`, then run:

```bash
ang validate
ang build
ang api-diff
```

Review breaking contract output before using `--accept-contract`.
