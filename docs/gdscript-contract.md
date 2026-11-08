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
