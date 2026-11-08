# Protojson WebSocket for bidi RPCs

Set `GRPC.websocket: true` on the ordinary CUE GRPC block. At least one bidi RPC
is required. ANG emits `websocket.gen.go` alongside the gRPC transport. A route
`/<package>.<service>/<method>` invokes the same generated typed service handler
through grpc.GenericServerStream. Authentication/session/subscription logic stays
in that handler; the adapter only marshals/unmarshals protobuf envelopes.

`ServeWebSocket(ctx, listener, handlers, WebSocketOptions{})` defaults to loopback
bind and peers. `AllowRemote: true` explicitly opts into external access. Packets
are text protojson with proto names, bounded at 16 MiB; writes have a deadline.
Cancellation closes upgraded sockets. No application message is extracted or
coalesced. Transport headers/trailers have no protojson envelope representation.

For CUE delivery annotations, ViewProjector.WantsField(fullName) lets a producer
skip constructing unrequested diagnostic fields. Live/static fields are always
wanted; nil subscription preserves the full profile. Check an annotated ancestor
before constructing its children, and serialize calls with Project/SetSubscription.
Projection still enforces the wire contract independently.

The GamePOC host exposes --ws-listen and --allow-remote and shares its original
Runtime.Play with gRPC. --parent-stdin ties local-server lifetime to a client-owned
pipe. See the generated real-WS tests for Hello/new/Life/move/save/load/reconnect,
invalid token and non-loopback refusal.
