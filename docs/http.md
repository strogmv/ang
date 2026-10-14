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
parameter, field by field:

```cue
ListOrders: schema.#Operation & {
	service: "orders"
	input: {
		companyId: string                                        // unchanged: a string
		from:      string      @query(type="date")               // ?from=2026-10-14 -> time.Time
		at:        string      @query(type="time", format="2006-01-02 15:04")
		page:      int         @query(required=true)             // missing -> 400
		ids:       [...string] @query(name="id")                 // ?id=a&id=b or ?id=a,b
		tags:      [...string] @query(format="csv")              // ?tags=a,b only
	}
	output: {...}
}
```

| Key | Meaning |
|---|---|
| `type` | `string`, `int`, `int64`, `float`, `bool`, `date`, `time`, `[]string`. Default: the field's CUE type. `date` and `time` make the field `time.Time`. |
| `format` | A Go time layout for `date` (default `2006-01-02`) and `time` (default RFC 3339); for `[]string`, `"multi"` (repeated keys), `"csv"` (one comma-separated value) or unset (both). |
| `name` | The query key; default the field's JSON name. |
| `required` | `true`: a missing or blank value is a 400. |

A value that does not parse, or a missing required one, gets one answer shape,
the same problem+json the handlers already use for an `int` that is not one:

```json
{"status": 400, "title": "Invalid query", "errors": {"from": "must be a date like 2006-01-02"}}
```

`@query` works on POST/PUT/PATCH/DELETE too: the field is filled from the query
string after the JSON body is decoded. The parsing helpers live in
`internal/transport/http/query_params.go`, generated with the first `@query`
field. OpenAPI lists each one as a query parameter with its type (`format:
date`, `date-time`, `int64`; arrays with `style: form` and `explode`).
