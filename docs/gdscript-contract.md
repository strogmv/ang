# GDScript protojson contract

Add a file path to the CUE GRPC block:

```cue
GRPC: schema.#GRPC & {
    package: "game.v1"
    gdscript_out: "../godot/generated/game_contract.gd"
    // messages, enums, services remain the ordinary gRPC declaration
}
```

`ang build` and `ang game-poc` emit the file relative to the backend root. The
latter also synchronizes the vendored canonical CUE schema before compilation.
No external protoc/GDScript plugin is required. Existing Go/proto/C# outputs are
unaffected when the option is absent.

`RenderGDScriptContract` compiles `RenderGRPCProto` with the same protocompile
library used for Go PB generation, then walks protobuf descriptors from every
RPC input/output. Reachability follows message fields and map values, including
nested messages and cycles; only reachable messages/enums are emitted. Messages
and enums are sorted, fields retain proto declaration order, and output has no
timestamps or filesystem-specific data. There are no game-specific names.

Preload the script and use its inner message classes:

```gdscript
const Contract = preload("res://generated/game_contract.gd")
var envelope = Contract.ServerEnvelope.from_dict(parsed_json)
var command = Contract.ClientEnvelope.new()
# Set its generated typed members, then:
var json_text = JSON.stringify(command.to_dict())
```

The message names above are examples, not required roots. Each class has typed
properties, `static from_dict(Dictionary)` and `to_dict()`. Nested messages are
nullable; repeated values use typed arrays, maps use dictionaries, optional and
oneof scalar members use nullable Variant. Assigning oneof members clears other
members of that group. Decoding multiple set members asserts invalid input.
Input is expected to be valid protojson from a trusted protobuf transport; this
is not a replacement for server-side protobuf validation.

Wire names stay proto field names, with standard lowerCamel aliases accepted on
input. Absent/null fields get proto3 defaults. Enum constants are canonical names;
known numeric input normalizes to names, unknown enum numbers remain numeric.
Bytes use base64. Nonfinite floats use protojson's string tokens. Serialization
omits scalar defaults, empty arrays/maps/bytes and absent messages, while
preserving optional/oneof presence, including explicitly set zero/false/empty.
All 64-bit integers serialize as decimal strings. Signed values are converted
exactly to Godot int. uint64 values within INT64_MAX become int; larger values
remain decimal strings (also for map keys), because Godot cannot represent their
mathematical value as an int. They round-trip losslessly. Do not use floating
point for these values.

For identifiers, nested class levels are joined by `__`, original underscores
become `_u`, conflicting builtin class names receive `_PB_`, and conflicting
field names/Object members receive `pb_` (an existing `pb_` prefix is escaped
again). This affects GDScript identifiers, never wire names. Opaque JSON stored
inside a proto `string` stays a string: declaring real protobuf messages is the
way to obtain generated typed access to its contents.

`TestGDScriptGoldenAndRuntime` compares a checked-in synthetic golden. With
`ANG_GODOT_BIN=/path/to/Godot`, it also executes the file in native headless
Godot, checks defaults/presence/oneof reassignment, nested/repeated/map values,
reserved names, bytes, nonfinite floats, known/unknown enums, integer precision
and maximum uint64, then validates the JSON round-trip with protobuf dynamic
messages. `TestGDScriptFromIRAndOption` covers the IR entry point and normal
emitter output option. To intentionally refresh the synthetic golden, use
`UPDATE_GDSCRIPT_GOLDEN=1`; review its diff.

## Delivery subscriptions

A contract can opt into generated delivery projection. Declare a subscription
message in CUE, set `GRPC.view_subscription` to its type name, and carry it in the
project's Hello and/or a dedicated command. That message must declare
`diagnostics` (repeated string), `diagnostics_all` (bool), and
`static_on_change` (bool). An absent subscription means full legacy delivery.
A present subscription selects diagnostic fields by their fully qualified proto
field name; `diagnostics_all` selects all. `static_on_change: false` sends static
values every frame, while true sends them at first publication and on change.

Each ordinary field has optional `delivery: "live" | "diagnostic" |
"static_on_change"`; absent means live. A static field additionally declares
`revision: "some_revision"`, naming an exclusive plain uint64 sibling with its
own explicit wire number. The compiler rejects unknown classes, invalid/shared
revision fields and annotations on oneof members. Static values are atomic;
their descendants cannot also have delivery annotations. An unannotated parent
can contain independently annotated children, including repeated/map children.

`RenderGRPCDelivery` generates `<proto>_delivery.pb.go` in the PB package for both
ordinary builds and `game-poc`. `NewViewProjector(subscription)` owns per-stream
state; serialize its calls with publication. `Project` returns a projection
without mutating the input. Source messages must remain immutable after
publication, as with ordinary protobuf sends. Diagnostic fields are skipped
before traversal. Static values use SHA-256 over their deterministic protobuf
encoding, not JSON ordering or pointer identity. Each field occurrence has a
revision starting at one and increasing only on content change. Revisions are
present in both static modes even when the value is empty or unchanged. A
changed revision with an absent payload represents the protobuf default/empty
value. No patch operations or ACKs are required on the ordered reliable stream.

Call `ResetSnapshots` after world replacement/save-load: full static values are
sent again, while revisions remain monotonic. A new connection owns a new
projector/revision domain and always gets a full snapshot. `SetSubscription`
also forces a full snapshot without resetting revisions. Project only messages
that will be published; after a transport failure reconnect with a new
projector. Revisions are scoped by message occurrence path (field numbers,
list index/map key), rather than being a persistent world version.

GDScript output adds a `ViewCache` with typed `decode_<Response>(Dictionary)`
methods and `reset()`. Keep one cache per ordered stream; reset on reconnect.
The generated decoder reuses the previous **typed value** when a payload is
absent and its revision matches. New revisions clear/replace values, including
empty lists/maps, absent messages and optional false/null. Live fields retain
ordinary per-frame defaults. Treat returned values as immutable. Calling the
ordinary `Message.from_dict(d)` without a cache keeps the stateless legacy API.
No network adapter needs field knowledge.

`TestGRPCDeliveryGoldenAndRuntime` checks generated Go/GDScript goldens, executes
the generated Go projection, then (with `ANG_GODOT_BIN`) feeds its actual
protojson frames through native Godot. It covers diagnostic selection and the
legacy profile, revision monotonicity, exactly-on-change delivery, forced
snapshots, independent reconnect, typed reuse, clearing, maps, optional presence,
nested/repeated values and integer precision. The game host additionally tests
real gRPC Hello, commands, disk save/load and reconnect.
