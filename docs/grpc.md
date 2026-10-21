# gRPC, CUE-first

The gRPC contract is declared once, in CUE (`GRPC:` in `cue/api`). ANG writes the
`.proto`, the Go message and service code, and the server from it, so contract,
server and clients cannot drift apart. The `.proto` is an output: never edit it.

## Declaration

```cue
import "github.com/strogmv/ang/cue/schema"

GRPC: schema.#GRPC & {
	package:          "game.v1"                       // last element must be a version
	csharp_namespace: "AngelsDemons.Game.V1"          // optional, for Unity / .NET
	handlers:         "internal/game"                 // hand-written package, see below
	copy_proto_to:    ["../client-unity/Assets/Proto"] // optional, relative to the project
	listen:           ":50051"                        // env GRPC_ADDR overrides

	services: GameService: Play: {stream: "bidi", request: "ClientMessage", response: "ServerMessage"}

	enums: TileKind: values: [{name: "FLOOR", n: 1}, {name: "WALL", n: 2}]
	messages: {
		Hello: fields: player_name: {n: 1, type: "string"}
		ClientMessage: oneofs: msg: hello: {n: 1, type: "Hello"}
		...
	}
}
```

- `stream`: `none` (unary, default), `server`, `client`, `bidi`.
- Field `type`: `double float int32 int64 uint32 uint64 bool string bytes` or the name
  of a message or enum of the same declaration. `repeated`, `optional`, `map_key`
  (a `map<key, type>`), `deprecated` are flags of the field.
- **Field numbers (`n`) are explicit and are the wire contract.** Never renumber or
  reuse one; list a retired number or name in `reserved` / `reserved_names`.
  Fields are written in number order, so reordering CUE changes nothing on the wire.
- Every enum gets `<NAME>_UNSPECIFIED = 0` automatically; value numbers start at 1.

## What is generated

| Output | Content |
|---|---|
| `api/grpc/<pkg dirs>/<name>.proto` | the contract (`game.v1` -> `api/grpc/game/v1/game.proto`) |
| `internal/transport/grpc/pb/<pkg dirs>/<name>.pb.go` | messages: protoc-gen-go output, produced in process (no `protoc`) |
| `.../<name>_grpc.pb.go` | client, server interface, `ServiceDesc`, stream aliases (`GameService_PlayServer`) |
| `internal/transport/grpc/server.gen.go` | `Handlers`, `NewServer`, `ListenAndServe`, recovery/logging interceptors, reflection |
| `cmd/server/main.go` | starts the gRPC server next to HTTP and stops it on shutdown |
| `copy_proto_to` dirs | a copy of the `.proto`, for clients that build from it |
| `csharp_out` dir | the C# client (below) |

A `GRPC:` declaration enables these steps whatever `target.framework` says.

## Handlers (hand-written)

`handlers` names a package in your module that ANG never touches. It must export:

```go
func New(ctx context.Context, rt *bootstrap.RuntimeContainer) (grpc.Handlers, error)
```

with `grpc` = `<module>/internal/transport/grpc`. Embed
`pb.Unimplemented<Service>Server` in each handler so a new rpc added in CUE compiles.
`go test` can run the generated server in memory with `bufconn`.

## Errors

`CUE_GRPC_NORMALIZE_ERROR`, with file:line: duplicate or reserved field number,
number in protobuf's 19000-19999 range, unknown type, request/response that is not a
declared message, repeated or map inside `oneof`, a name that is not
lower_snake_case (fields) or PascalCase (types).

## C# client (Unity / .NET)

`csharp_out: "../client-unity/Assets/Generated/MyGame"` makes `ang build` write, after the
staged build is promoted (so the directory may lie outside the project):

| File | Content |
|---|---|
| `Messages/<Name>.cs` | message classes: `protoc --csharp_out` (protoc must be on PATH; the build fails with a clear message otherwise) |
| `Grpc/<Service>Grpc.cs` | method descriptors and `<Service>Client` for all four rpc kinds, written against `Grpc.Core.Api`'s `CallInvoker` |
| `Sessions/<Service><Rpc>Session.cs` | for every **bidi** rpc whose request and response are each a single `oneof`: a typed session |
| `ang-generated.txt` | the files of the last build; a file the contract no longer produces is removed, nothing else is touched |

The session has no `UnityEngine` dependency. Network work runs on pool threads; every event is
raised from `Pump()`, which the game calls once per frame (`Update()`):

```csharp
var s = new GameServicePlaySession(channel.CreateCallInvoker(),
    () => new ClientMessage { Hello = new Hello { PlayerName = "Hero" } }); // first message of every (re)connect
s.OnWelcome += w => ...;   // one On<Member> event per member of the response oneof
s.OnSnapshot += snap => ...;
s.Start();                 // reconnects with exponential backoff (s.Reconnect)
s.SendMoveTo(new MoveTo { X = 3, Y = 4 });   // one Send<Member> per member of the request oneof
// in Update(): s.Pump();
```

Needs `Google.Protobuf` and `Grpc.Core.Api` (and `Grpc.Net.Client` + an HTTP/2 handler such as
`Cysharp/YetAnotherHttpHandler` for Unity's `GrpcChannel`). Verified: the generated sources compile
with `netstandard2.1`, and a .NET client built on the session plays against the generated Go server.
