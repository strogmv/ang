# Go written in CUE: what the code sees

An operation can carry Go in three places: `impls: go: code`, the function
literal of a `logic.Call` step, and shared helpers (docs/helpers.md). ANG
copies that Go into the generated method verbatim. It does not rewrite it.

## `impls: go:` code

The generated method declares five names, and the code runs as a block of its
own inside it:

```go
func (s *OrdersImpl) RenameOrder(ctx context.Context, req port.RenameOrderRequest) (resp port.RenameOrderResponse, err error) {
	{
		// your code, line for line, mapped to the CUE file with //line
	}
}
```

| Name | What it is |
|---|---|
| `s` | the service implementation: repositories, `s.txManager`, helpers |
| `ctx` | the request context (the transaction's under `transactional: true`) |
| `req` | the operation's input |
| `resp`, `err` | the named results; `resp` starts as the zero value |
| `chunks` | streaming operations only; they have no named results |

Any Go name is legal in the code: `err := …`, `var err error`, `resp := …`,
`var resp port.X`, a variable `l`, a field `ExternalId`. A `resp` or `err`
that the code declares shadows the result, as a nested scope does in Go.
Return it explicitly, as in `return resp, err`. Operation metrics
(`target.operation_metrics`) and `transactional: true` add no names that the
code can see.

Two warnings point out where shadowing changes what the code means. Both name
the CUE line:

- `IMPL_RESULT_SHADOWED` on a naked `return` where the code's own `resp` or
  `err` is in scope. Go refuses it with "result parameter err not in scope at
  return".
- `IMPL_RESULT_SHADOWED` on a deferred function literal that reads or sets the
  code's own `resp` or `err`. It sees that variable, not the result the
  operation returns. To set the returned error from a defer, leave `err` to
  the method.

Until 2026-10 the code was pasted into the method body and rewritten line by
line. `err :=` became `err =`, `l.` became `slog.`, `.UserId` became
`.UserID` (inside string literals too), and `var err error` and
`var resp port.X` were deleted. Validation refused `err :=`, `resp :=` and
`l.` outright. All of that is gone. The rules above are plain Go.

## `logic.Call` function literals

A `logic.Call` literal is called inside the generated flow:

```go
token, err := (func(ctx context.Context) (string, error) { … })(ctx)
```

It has its own scope. It can also see the flow around it: `s`, `ctx`, `req`,
`resp` and the outputs of earlier steps. That access is how a literal reads a
flow variable without an argument. The flow's `err` is also visible, so a
literal that writes `x, err = f()` without declaring `err` sets the error
variable of the generated method. `LOGIC_CALL_OUTER_ERR` (a warning) points
at that line. Declare `err` in the literal with `var err error` or
`x, err := f()`.
