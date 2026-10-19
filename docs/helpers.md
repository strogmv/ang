# Helpers: Go written once, called from any operation

Go that several operations share — a lookup table, a formatter, a check that
reads the database — is a **helper**. You declare it once in `cue/api`. ANG
generates it once. Any operation calls it by name from its `impls: go:` code,
from a `logic.Call` func or from a flow expression.

```cue
package api

import "github.com/strogmv/ang/cue/schema"

Helpers: schema.#Helpers // once per project: the shape of every entry

// Declare the helper next to the operations that use it.
Helpers: countryMarket: code: """
	type countryMarket struct {
		Code       string
		Currencies []string
	}
	"""

Helpers: countryMarkets: code: """
	func countryMarkets() []countryMarket {
		return []countryMarket{{Code: "DE", Currencies: []string{"EUR"}}, {Code: "BY", Currencies: []string{"BYN", "EUR"}}}
	}
	"""

Helpers: writeCompanyAudit: {
	imports: ["crypto/rand"] // only where goimports would have to guess
	code: """
		func (s *Service) writeCompanyAudit(ctx context.Context, companyID, action string) error {
			return s.AuditLogRepo.Save(ctx, &domain.AuditLog{ID: uuid.NewString(), CompanyID: companyID, Action: action})
		}
		"""
}

ListCountryMarkets: schema.#Operation & {
	service: "company"
	impls: go: code: """
		for _, m := range countryMarkets() {
			resp.Codes = append(resp.Codes, m.Code)
		}
		return resp, s.writeCompanyAudit(ctx, req.CompanyID, "markets.listed")
		"""
}
```

## The rules

- The key is the Go name. It starts with a lower-case letter, because helpers
  are private to the service package.
- `code` is **one** top-level Go declaration of that name: a `func`, a `type`,
  a `var` or a `const`, or a **method helper** `func (s *Service) name(…)`.
- A func, type, var or const helper goes to
  `internal/service/helper__<name>.gen.go`, once.
- A method helper reads the service: `s.XRepo`, `s.cfg`, `s.storage`,
  `s.txManager`. ANG puts each service's type in place of `*Service`. The file
  has one copy for each service whose operations reach the helper, directly or
  through another helper. Callers write `s.name(…)`, or use `s.name` as a
  value. It may call the operations of that service (`s.SubmitQuote(ctx, req)`)
  and those of the services it uses (`s.StockService.Reserve(…)`, with
  `uses: ["Stock"]` on one of its operations).
- A repository that a method helper uses becomes a field of every service
  that reaches the helper. `s.Tx` and `s.txManager` work the same way.
- A helper sees only its parameters, packages and other helpers. It cannot see
  an operation's `req`, `resp`, `ctx` or locals, so pass them in as parameters.
- `imports:` lists the packages the code uses beyond the service base imports.
  goimports finds the rest. It is required where goimports would have to
  guess: `rand.Read` is in both crypto/rand and math/rand.
- Each copy carries `//line` directives back to the CUE literal. Compiler
  errors, `go vet`, gopls and panics report the CUE line. This needs `code`
  written as one multi-line `"""` or `#"""` literal.
- `ang fmt` gofmt's helper code as declarations.

## What validate checks

`ang validate` and `ang build` check helpers before any code is generated:

| Problem | Code |
|---|---|
| code that does not parse, more than one declaration, a name that is not the key, an `import` declaration in the code | `HELPER_INVALID` |
| a method helper not written `func (s *Service) name(…)`; a func helper that uses `s` | `HELPER_INVALID` |
| a name the code leaves undeclared: not a helper, a package (`name.Sel`) or a predeclared name (with a did-you-mean) | `HELPER_INVALID` |
| `req` / `resp` inside a helper | `HELPER_INVALID` |
| `s.x(…)` in a method helper where `x` (lower case) is no method helper | `HELPER_INVALID` |
| `s.Op(…)` in a method helper where `Op` is an operation of another service than one that reaches the helper (each reaching service gets its own copy: call `s.<Other>Service.Op(…)` there) | `HELPER_INVALID` |
| `s.<Other>Service` in a method helper reached by a service without `uses: ["<Other>"]` | `HELPER_INVALID` |
| a method helper called without `s.`, from a helper or an operation | `HELPER_INVALID` |
| two helpers that would write the same file (`fooBar`, `foo_bar`) | `HELPER_INVALID` |
| an `imports:` entry the code does not use; `name.Sel` that two standard packages export with nothing importing `name` | `RAW_GO_IMPORT_ERROR` |
| `s.XRepo` of no entity in a method helper | `REPO_REF_UNKNOWN` |
| a helper that no operation reaches (warning). An unused method helper is not generated, so it is not even compiled | `HELPER_UNUSED` |

## Why, and what it replaced

Before helpers, a project kept shared Go in a CUE definition and interpolated
it: `#CountryMarketsFunc: """(func() []… {…})"""`, called as
`\(#CountryMarketsFunc)()`. CUE pastes the whole text at every use, so ANG
generated one copy per operation. In dealingi-back the country-markets table
was copied about 150 times, and the e-invoice download was one 11k-line
function. `compiler/golit` later merged identical closed literals after
generation. It could not merge closures that read `s`, and the generated Go
did not map back to the CUE that held the code.

The dealingi-back migration (2026-10-14) turned 516 definitions into helpers:
272 funcs, 223 method helpers, 12 types and 9 composite tables. It also
rewrote 2093 uses: 1662 interpolations and 431 `func: #XFunc` references.
It found nine definitions that nothing used, and one of them did not compile.

| Measure | Before | After |
|---|---|---|
| `internal/service`, generated Go before golit | 18.6 MB | 11.1 MB |
| `internal/service`, Go on disk (after golit) | 11.4 MB | 7.3 MB |
| `zz_ang_literals.gen.go` | 281 KB (99 literals) | 10 KB (4) |
| largest generated file | 281 KB | 101 KB |
| `channel__download_sales_order_e_invoice.gen.go` | 2122 lines | 127 lines |
| `ang build --dry-run` | 11.6 s | 8.4 s |
| one operation changed: compile `internal/service` | 6.0 s | 3.4 s |
| one operation changed: `go build ./...` | 11.8 s | 8.7 s |

## Converting a definition

| Before | After |
|---|---|
| `#XFunc: """(func(a int) int {…})"""`, used as `\(#XFunc)(1)` | `Helpers: x: code: """func x(a int) int {…}"""`, used as `x(1)` |
| a closure that reads `s.…` | `func (s *Service) x(…)`, used as `s.x(…)` |
| `(func() T {…})()`, called on the spot | `func (s *Service) x() T {…}`, used as `s.x()` |
| `#XGo: """struct {…}"""` | `Helpers: x: code: """type x struct {…}"""` |
| `#XGo: """[]T{…}"""`, a table | `func x() []T { return []T{…} }`, used as `x()` |
| `func: #XFunc` in a `logic.Call` | `func: "x"` or `func: "s.x"` |
