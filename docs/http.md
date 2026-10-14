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
