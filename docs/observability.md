# Observability: metrics and spans per operation and per query

Every generated server exposes Prometheus metrics on `/metrics` (requests by
route, method and status: `http_request_duration_seconds`,
`http_requests_total`, panics, background handler errors) and exports
OpenTelemetry traces when an OTLP endpoint is configured (`internal/pkg/tracing`;
without one the global tracer is OpenTelemetry's no-op).

`target.operation_metrics: true` adds metrics and spans for every operation and
every Postgres query. Without it nothing below is generated and nothing runs.

```cue
state: target: operation_metrics: true   // add operation_metrics?: bool to a closed #Target
```

## Operations

Every service method (except streaming ones, which return no named error)
starts with

```go
ctx, observeDone := observeOperation(ctx, "Orders", "ListOrders")
defer func() { observeDone(err) }()
```

so an operation is measured whoever calls it: an HTTP handler, an event
subscriber, a schedule or another service. `internal/service/operation_metrics.gen.go`:

| Metric | Labels |
|---|---|
| `ang_operation_duration_seconds` (histogram) | `service`, `operation`, `outcome` (`ok`, `error`) |
| `ang_operation_errors_total` | `service`, `operation`, `status` (the HTTP status the error carries; 500 when none) |

The span is named `Service.Operation`, a child of the HTTP request span; an
error is recorded on it, and the span status is Error from status 500 up (a
404 is an answer, not a failure).

## Repository queries

The Postgres pool gets `dbobserve.QueryTracer{}` as its `ConnConfig.Tracer`
(`internal/pkg/dbobserve`). Each query is named after the repository method on
its call stack — `(*TenderRepository).ListByCompany` gives
`repository="TenderRepository", method="ListByCompany"`; a query from anywhere
else is `other/other`. Queries inside a transaction are included.

| Metric | Labels |
|---|---|
| `ang_db_query_duration_seconds` (histogram) | `repository`, `method`, `outcome` |
| `ang_db_query_rows` (histogram) | `repository`, `method` — rows returned (SELECT) or changed |
| `ang_db_query_errors_total` | `repository`, `method` |

The span `db Repository.Method` (kind client, `db.system=postgresql`,
`db.response.returned_rows`) is a child of the operation span. Finding the
caller costs one `runtime.Callers` per query, a few microseconds.

## Cardinality

Labels are operation and method names, fixed at build time: a project with
600 operations has at most 600 × 2 duration series per bucket. No label
carries a path, an id or a user.

## Grafana

```promql
# p95 by operation, last 5 minutes
histogram_quantile(0.95, sum by (service, operation, le) (rate(ang_operation_duration_seconds_bucket[5m])))
# server errors by operation
sum by (service, operation) (rate(ang_operation_errors_total{status=~"5.."}[5m]))
# slowest queries
topk(10, histogram_quantile(0.95, sum by (repository, method, le) (rate(ang_db_query_duration_seconds_bucket[5m]))))
# queries that return too many rows
topk(10, sum by (repository, method) (rate(ang_db_query_rows_sum[5m])) / sum by (repository, method) (rate(ang_db_query_rows_count[5m])))
```
