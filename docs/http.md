# HTTP endpoints: who may call, files, query parameters

Endpoints are declared in the `HTTP` block of `cue/api`, keyed by the operation
name. This page covers what the generated handlers do beyond decoding JSON:
authorization, file responses and typed query parameters. Every feature here is
opt-in: a project that uses none of it generates exactly what it did before.

## Who may call an endpoint

```cue
HTTP: {
	PlaceBid: {
		method: "POST", path: "/api/bids"
		auth: {required: true, scope: "bids:write"}  // a signed-in caller with that scope
	}
	ListOrders: {
		method: "GET", path: "/api/orders"
		auth: {type: "jwt", roles: ["owner", "admin"], inject: ["companyId"]}
	}
	ReceiveStripeWebhook: {
		method: "POST", path: "/api/public/stripe/webhook"
		auth: {public: true, reason: "Stripe calls it; the signature is checked against the raw body"}
	}
}
```

| Key | Meaning |
|---|---|
| `type: "jwt"` | A signed-in caller (cookie session or bearer token): `AuthMiddleware`. |
| `required: true` | The same as `type: "jwt"`. |
| `roles`, `action`/`permission`, `scope` | Further checks after sign-in (`RequireRoles`, `RequirePermission`, `RequireScopeMiddleware`). |
| `check` | An operation that decides per request (`{Allowed: bool}`). |
| `inject` | Request fields filled from the caller (`userId`, `companyId`, `locale`, `timezone`). |
| `public: true` (or `required: false`) | Anyone may call, on purpose. Cannot be combined with `type`, `roles`, `scope`, `check`, `inject`, `permission`. |
| `reason` | Why a public endpoint is public. Needed when it changes state (below). |

### Default: allow or deny

`target.auth_default` in `cue/project` decides what an endpoint **without** an
`auth` block means:

- `"allow"` (or unset, the historical behaviour): it is public.
- `"deny"`: it requires a signed-in caller, as if it said `auth: {required: true}`.
  Public endpoints must say `auth: {public: true}`. WebSocket streams already
  authenticate on the socket and are unaffected.

```cue
#Target: {
	auth_default?: "allow" | "deny"   // add to the project's #Target if it is closed
}
state: target: auth_default: "deny"
```

### Public endpoints that change state

An endpoint *changes state* when it is a POST, PUT, PATCH or DELETE, or when its
operation writes even though it is a GET: a `repo.Save`/`repo.Delete`/
`repo.Upsert`, `event.Publish`, `mail.Send`, `storage.Upload`, ... step, or a
repository write (`XRepo.Save(`, `Delete`, `Update`, `Insert`, `Upsert`) or an
event publish (`.PublishX(`) in its Go.

- `"allow"`: a public endpoint that changes state is reported by the build with
  `AUTH_PUBLIC_MUTATION` unless it says `auth: {public: true, reason: "..."}`;
  `public: true` without a reason gets `AUTH_PUBLIC_NO_REASON`. Both are warnings.
- `"deny"`: `public: true` without a reason on such an endpoint fails the build
  with `AUTH_PUBLIC_NO_REASON`, one line per endpoint with its CUE file:line.

The reason is for the reader of the CUE (and of the review): what protects the
endpoint instead of a session — a token in the path, a webhook signature, an
API key in the body, a rate limit.

### Moving a project to deny

1. Build once with the default; every `AUTH_PUBLIC_MUTATION` warning is an
   endpoint anyone can call that changes state.
2. For each endpoint that must stay public (sign-in, webhooks, share links),
   write `auth: {public: true, reason: "..."}`; read-only public endpoints need
   only `public: true`.
3. Set `auth_default: "deny"`. Everything else now needs a caller.

### Rights beyond the role: `auth.right`

A role that opens a whole console (`admin`) usually comes in grades: billing,
maintenance, reference data. The right a route needs is part of its binding:

```cue
HTTP: AdminListInvoices: {method: "GET", path: "/api/admin/billing/invoices",
	auth: {type: "jwt", roles: ["admin"], right: "billing"}}
```

- The route is wrapped in `RequireRolesRight(roles, right)`; the role gate
  calls the project's `CheckRoleAccess` with `right` in its input (add a
  `right: string` input field) and refuses with its reason (403
  `ROLE_CONDITION_UNMET`). Routes without a right keep `RequireRoles`.
- `internal/transport/http/route_rights.go` has `RouteRight(method, pattern)`
  for tests and tools that check a gate without the router.
- `target.route_rights: {roles: ["admin"], rights: ["view", "billing", …]}`
  makes the right mandatory: a route whose `auth.roles` names one of `roles`
  without `right` fails the build (`AUTH_RIGHT_MISSING`, file:line per route),
  a right outside `rights` is `AUTH_RIGHT_UNKNOWN`. Any `auth.right` without a
  `CheckRoleAccess` that takes a right is `AUTH_RIGHT_UNCHECKED`.
- `right` needs `roles`, and is refused on public and WebSocket routes.

## File responses

An operation whose output is `kind: "file"` answers with the bytes of a file,
not with a JSON object:

```cue
ExportCustomersCSV: schema.#Operation & {
	service: "customers"
	input: {companyId: string}
	output: {kind: "file", contentType: "text/csv; charset=utf-8", filename: "customers.csv"}
	// or: output: schema.#FileOutput & {contentType: "application/pdf", disposition: "inline"}
}
```

| Key | Meaning |
|---|---|
| `contentType` | `Content-Type` unless the operation sets `resp.ContentType`; `application/octet-stream` when neither does. |
| `filename` | The name in `Content-Disposition` unless the operation sets `resp.Filename`. |
| `disposition` | `"attachment"` (default: the browser saves the file) or `"inline"` (it shows it, e.g. a PDF). |

The response type in `internal/port` is fixed:

```go
type ExportCustomersCSVResponse struct {
	Body        io.ReadCloser // streamed to the client and closed; when nil, Content is written
	Content     []byte
	ContentType string // replaces the declared one when set
	Filename    string // replaces the declared one when set
	Size        int64  // length of Body when known (Content-Length)
}
```

so the operation's Go sets `resp.Content = csvBytes` (or `resp.Body = object`
for a file read from storage, never held in memory) and, for a name that
depends on the data, `resp.Filename = "invoice-" + number + ".pdf"`.

The handler (`writeFileResponse` in `internal/transport/http/file_response.go`,
generated with the first file endpoint) sets `Content-Type`,
`X-Content-Type-Options: nosniff`, `Cache-Control: private`, `Content-Length`
when known, and `Content-Disposition` per RFC 6266: an ASCII stand-in in
`filename="..."` and the real name in `filename*=UTF-8''...` when it is not
ASCII (`Rechnung März.pdf`). Quotes, slashes and line breaks in a name never
reach the header. Errors before the first byte are the usual problem+json
answers. OpenAPI describes the 200 answer as `format: binary` in that content
type, and no JSON schema is generated for the response.

The TypeScript SDK still reads every answer as text, so download a file
endpoint by its URL (a link or `window.open`); the browser sends the session
cookie. This replaces the JSON `{fileName, contentType, contentBase64}` shape:
no base64 (a third larger), and the file is not held in memory twice.

The older convention — output fields named `rawContent`/`rawContentType`
(+ `rawDisposition`, `rawETag`) — keeps working unchanged.

## Typed query parameters

A GET handler reads its request from the query string. Without `@query` a
field is read as before: `int`, `float64` and `bool` fields are parsed, every
other field is copied as a string. `@query` on an input field makes it a typed
parameter, field by field. The query key is the field's JSON name, the key the
TypeScript SDK sends:

```cue
ListOrders: schema.#Operation & {
	service: "orders"
	input: {
		companyId: string                                        // unchanged: a string
		from:      string      @query(type="date")               // ?from=2026-10-14 -> time.Time
		at:        string      @query(type="time", format="2006-01-02 15:04")
		page:      int         @query(required=true)             // missing -> 400
		ids:       [...string] @query()                          // ?ids=a&ids=b or ?ids=a,b
		tags:      [...string] @query(format="csv")              // ?tags=a,b only
	}
	output: {...}
}
```

| Key | Meaning |
|---|---|
| `type` | `string`, `int`, `int64`, `float`, `bool`, `date`, `time`, `[]string`. Default: the field's CUE type. `date` and `time` make the field `time.Time`. |
| `format` | A Go time layout for `date` (default `2006-01-02`) and `time` (default RFC 3339); for `[]string`, `"multi"` (repeated keys), `"csv"` (one comma-separated value) or unset (both). |
| `required` | `true`: a missing or blank value is a 400. |

The TypeScript SDK sends a list as repeated keys (`?ids=a&ids=b`), which the
default and `"multi"` read; `"csv"` is for clients that send one value.

A value that does not parse, or a missing required one, gets one answer shape,
the same problem+json the handlers already use for an `int` that is not one:

```json
{"type": "validation-error", "title": "VALIDATION_FAILED", "status": 400, "detail": "Invalid query",
 "code": 40010, "errors": {"from": "must be a date like 2006-01-02"}}
```

`@query` works on POST/PUT/PATCH/DELETE too: the field is filled from the query
string after the JSON body is decoded. The parsing helpers live in
`internal/transport/http/query_params.go`, generated with the first `@query`
field. OpenAPI lists each one as a query parameter with its type (`format:
date`, `date-time`, `int64`; arrays with `style: form` and `explode`).

## Inputs from a header: `@header`

A credential the caller sends as a header — a webhook secret, an API key —
does not belong in the path (it ends up in access logs, proxies and the
sender's dashboards) or the body. `@header` on an input field binds it to a
request header:

```cue
TelegramWebhook: {
	input: {
		secretToken: string @header(name="X-Telegram-Bot-Api-Secret-Token")
		message?: {...}
	}
}
```

- The handler sets the field from the header after reading the body, path and
  query, always: an absent header is `""`, and a body or query value with the
  same name never counts. A GET does not read the field from the query.
- Only `string` fields; `@header("X-Name")` is the short form; `@header` and
  `@query` exclude each other.
- OpenAPI lists it as `in: header` (required unless the field is optional);
  the TypeScript SDK sends it as that header and leaves it out of the body
  and query.

## Three-state inputs: `@nullable()`

An optional `bool` is `false` when the client leaves it out, so a PATCH
cannot tell "leave it" from "set it to false" (dealingi-back worked around it
with `clearX?: bool` flags and "only true is applied"). Mark such a field:

```cue
UpdateTender: schema.#Operation & {
    input: {
        id:         string
        openEntry?: bool @nullable()   // absent/null = unchanged, true, false
    }
}
```

- The port struct field is a pointer: `OpenEntry *bool`. JSON `null` or an
  absent key is `nil`; GET query and path parameters (plain or `@query`) store
  the parsed value's address. Code reads it as
  `if req.OpenEntry != nil { tender.OpenEntry = *req.OpenEntry }`; the flow
  checker types `req.OpenEntry` as a pointer.
- TypeScript `openEntry?: boolean | null`, Zod `.nullable().optional()`,
  OpenAPI `nullable: true`. The SDK drops `null`/`undefined` query values and
  sends `false` as `false`, so GET filters work the same way.
- Allowed on optional `bool`, `int`, `int64`, `float`, `string` and time fields
  of operation inputs and outputs. A required field, another type, or a field
  of a cue/domain entity is an error. Fields without the marker do not change.

## Reads that are not GET: `read: true`

A search or a preview often goes out as POST (a large body, a file). The SDK
decided query or mutation by the method only, so such a read got a
`useMutation` hook, no query key, no cache, and invalidated lists after it.
Mark it:

```cue
HTTP: SearchProductsCatalog: {method: "POST", path: "/api/catalog/search", read: true}
```

- SDK: `use<Op>` / `use<Op>Suspense` query hooks, `<op>QueryKey`, query
  options and cache profile as for a GET; the request still goes as a POST
  with a JSON body. No mutation hook, no list invalidation after it. Forms
  and route loaders keep following the method.
- Backend: it does not count as changing state (`AUTH_PUBLIC_MUTATION`) unless
  its operation writes; a read whose flow or Go writes is reported as
  `READ_ENDPOINT_WRITES` (warning). OpenAPI: `x-ang-read: true`.
- Marking an endpoint changes its hook from a mutation to a query, so its
  callers in the front change with it. Not allowed on WS.
