# ANG diagnostics

## Diagnostic codes

Every structured diagnostic contains a stable `code`, source location when
available, a `docsURL`, and machine-applicable `suggestedFix` entries when ANG
can determine a safe replacement.

Use the code directly:

```bash
ang doctor --code DTO_FIELD_UNKNOWN
ang build --log-format json
ang validate --json
```

Code families:

- `CUE_*`: intent loading, normalization, and policy validation. Fix the
  referenced source below `cue/`, then run `ang validate`.
- `DTO_FIELD_UNKNOWN`: a Go selector does not exist in the normalized DTO.
  Apply the `before`/`after` replacement from `suggestedFix`.
- `REPO_REF_UNKNOWN`: Go in an operation reaches `s.<X>Repo`, but no entity `X`
  has a repository (unknown name or a DTO). A repository of a real entity
  needs no `sources` entry: the service implementation gets every repository
  its operations' Go reaches.
- `REPO_FINDER_INVALID`: a finder's hand-written `sql:` and its generated
  method disagree — a `$n` without an argument or an argument the SQL never
  uses, a select list with other columns than the method scans (count,
  order), a timestamp/date/bool column scanned into a Go string without
  `::text`, a `select: "auto"` without `*`, an upsert whose `on_conflict`
  is not a unique index (docs/repositories.md).
- `HELPER_INVALID`: a shared helper (`Helpers:` in cue/api) that cannot work.
  This covers code that is not one declaration named after its key, a method
  not written `func (s *Service) name`, a name the code leaves undeclared, an
  operation that calls a method helper without `s.`, and two helpers that would
  write the same file (docs/helpers.md).
- `HELPER_UNUSED` (warning): no operation reaches the helper. An unused method
  helper is not generated at all.
- `REPO_FINDER_PAGED_IMPLICIT` (warning): `limit:` > 0 or a name containing
  `paginate` adds `(limit int, offset int)` to a finder; write `paged: true`.
- `SQL_INDEX_NAME_TRUNCATED` (warning): a generated index name is longer than
  63 bytes and Postgres stores its first 63. Pin that name with `name:` in the
  index, or use `target.sql_index_names: "hashed"` (docs/sql-schema.md).
- `SQL_INDEX_NAME_COLLISION` (build error): two different indexes, or an index
  and a table, would be stored under the same name; give one a `name:`.
- `AUTH_PUBLIC_MUTATION` (warning): an endpoint without `auth` changes state
  (POST/PUT/PATCH/DELETE, or a GET whose operation writes) and anyone can call
  it. Require a caller (`auth: {required: true}`) or say it is public on
  purpose: `auth: {public: true, reason: "..."}` (docs/http.md).
- `AUTH_PUBLIC_NO_REASON`: `auth: {public: true}` on an endpoint that changes
  state, without a `reason`. A warning by default, a build error under
  `target.auth_default: "deny"`.
- `AUTH_DEFAULT_INVALID` (build error): `target.auth_default` is neither
  `"allow"` nor `"deny"`.
- `CONFIG_LOAD_IN_OPERATION` (warning, `target.service_config` only): an
  operation's Go calls `config.Load()`; the service holds the loaded
  configuration as `s.Cfg` (docs/config.md).
- `IR_*`: canonical IR conversion, migration, or semantic validation.
- `TRANSFORMER_*` and `HOOK_*`: extension execution failures.
- `EMITTER_*`: capability resolution, output validation, or generation failure.
- `W_RAW_BODY_*`: request-body decoding. See below.

## Raw request bodies

An input field named `body` makes the generated handler read the raw HTTP body
into that string instead of decoding a JSON object into the request struct.
Clients generated from the same spec send an object, so the call fails with
`400 cannot unmarshal object into Go value of type string`.

Declare the intent on the operation instead of relying on the field name:

```cue
HandleTelegramWebhook: schema.#Operation & {
	@rawBody()
	input: {
		botId: string
		body:  string
	}
}
```

- `W_RAW_BODY_IMPLICIT`: the operation decodes a raw body only because a field
  is named `body`. Add `@rawBody()` if that is intended (webhooks, signature
  verification over exact bytes), or rename the field — `text`, `message`,
  `payload` — to decode the body as JSON.
- `W_RAW_BODY_ATTRIBUTE_IGNORED`: `@rawBody()` is declared but the input has no
  `body` field, so the attribute does nothing.
- `W_RAW_BODY_FIELDS_IGNORED`: a raw-body operation declares input fields that
  are neither the body nor path parameters. Nothing populates them.


`ang doctor --code <CODE>` is the canonical source for code-specific guidance;
it is generated from the same registry used by the compiler and CLI.
