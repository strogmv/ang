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

