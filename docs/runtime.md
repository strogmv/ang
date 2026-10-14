# Runtime: schedules, process roles, lifecycle hooks, queues

What the generated `cmd/server` runs besides HTTP routes. Everything on this
page is opt-in: a project that declares none of it generates what it generated
before.

## Schedules

One declaration runs an operation on a timer:

```cue
Schedules: {
	// every day at 06:00 in SCHEDULER_TIMEZONE (default UTC)
	DailyDigest: {action: "SendDailyDigest", at: "06:00"}
	// an interval
	SweepExpired: {action: "SweepExpiredOffers", every: "15m"}
	// cron: minute hour day-of-month month day-of-week
	MonthlyClose: {action: "CloseMonth", cron: "0 3 1 * *"}
	// payload keys are input fields of the action
	AuditPurge: {service: "audit", action: "PurgeAuditLogs", every: "24h", payload: {olderThanDays: 365}}
}
```

ANG creates the trigger event `<Name>Requested` (owner: the action's service,
fields: the payload), subscribes the action to it (a NATS queue group, so one
replica runs each tick) and has the scheduler publish it.

- `action` is the operation; `service` is needed only when two services have
  an operation of that name.
- `at: "06:00"` alone is every day at that time. `at` with `every` keeps its
  old meaning: first run at `at`, then every interval (a 24h ticker, which
  drifts by an hour across a DST change; `cron` does not). An RFC 3339 `at`
  runs once.
- `cron` is five fields; each is `*`, `n`, `a-b`, `*/n`, `a-b/n` or a list
  `a,b`. Day of week 0 and 7 are Sunday; when both day fields are restricted a
  day matching either runs. It excludes `at` and `every`.
- `payload` values are integers, bools and strings; a key that is not an input
  field of the action is an error.

Errors, with file:line (`CUE_SCHEDULE_NORMALIZE_ERROR`): unknown or ambiguous
action, no timing, a bad `cron`/`every`/`at`, an event of that name already in
`cue/events`.

The three-part form still works and is left as it is: with `publish:` the
schedule only publishes that event, and the event (`cue/events`) and the
subscription (`subscribes:` on the operation) are declared by hand.
`cron:` works there too.

## Process roles: `SERVER_ROLE`

```cue
// cue/project
state: target: {
	server_roles:     true
	shutdown_timeout: "30s" // optional; SHUTDOWN_TIMEOUT overrides it at run time
}
```

The generated `cmd/server` then reads `SERVER_ROLE`:

| `SERVER_ROLE` | HTTP routes | subscriptions, schedules, queue workers |
|---|---|---|
| `all` (default, also when unset) | yes | yes |
| `api` | yes | no |
| `worker` | no (`/health`, `/health/ready`, `/metrics` only) | yes |

Any other value exits with an error. WebSocket fan-out subscriptions belong to
the API. One image serves both: deploy it twice with different
`SERVER_ROLE`, and a worker no longer needs its own hand-written `cmd/`.

### `subscribe.when`

```cue
SweepExpiredOffers: schema.#Operation & {
	service: "b2b"
	subscribes: {SweepExpiredOffersRequested: "SweepExpiredOffers"}
	subscribe: {when: "cfg.JobsEnabled"}
	...
}
```

The operation's subscriptions are made only when the Go expression holds at
start (`cfg` is the loaded config): a replica with `JOBS_ENABLED=false` does
not join the queue group, so it cannot take ticks it would then skip. It works
without `server_roles`. Every `cfg.<Field>` must be a config field (cue/infra
`#AppConfig`), checked when the main is generated.

## Lifecycle hooks

```cue
WarmPriceCache: schema.#Operation & {
	service: "pricing"
	hook:    "OnStart"
	input: {}
	output: {ok: bool}
	flow: [...]
}

FlushPriceCache: schema.#Operation & {
	service: "pricing"
	hook: {on: "OnShutdown", role: "worker", when: "cfg.JobsEnabled"}
	input: {}
	output: {ok: bool}
	flow: [...]
}
```

A hook is an operation with an empty input that the main calls:

- `OnStart` after the services, subscriptions and queue workers are set up and
  before the HTTP server listens, with a context that ends on SIGTERM (a hook
  may start a loop on it). An error exits the process.
- `OnShutdown` on SIGTERM/SIGINT after the drain, in reverse service order,
  with its own `shutdownTimeout` budget; errors are logged.
- `role` (`all` default, `api`, `worker`) and `when` limit where it runs.

With roles, hooks, queues or `shutdown_timeout` the shutdown is a drain: the
HTTP server stops taking requests (`srv.Shutdown`), the run context ends
(schedules and queue claims stop), jobs in flight finish, all within
`shutdownTimeout` (default 30s), then the `OnShutdown` hooks run. Set the pod's
`terminationGracePeriodSeconds` above twice that. Without any of these the
main keeps its old 5-second HTTP shutdown.
