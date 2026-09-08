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
