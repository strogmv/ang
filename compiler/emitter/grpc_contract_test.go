package emitter

import (
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func testGRPCDef() *ir.GRPC {
	return &ir.GRPC{
		Package:         "game.v1",
		CSharpNamespace: "Game.V1",
		Listen:          ":50051",
		Reflection:      true,
		Enums:           []ir.GRPCEnum{{Name: "TileKind", Values: []ir.GRPCEnumValue{{Name: "FLOOR", Number: 1}, {Name: "WALL", Number: 2}}}},
		Messages: []ir.GRPCMessage{
			{Name: "ClientMessage", Oneofs: []ir.GRPCOneof{{Name: "msg", Members: []ir.GRPCField{
				{Name: "hello", Number: 1, Type: "Hello"}, {Name: "ping", Number: 2, Type: "Ping"}}}}},
			{Name: "Hello", Fields: []ir.GRPCField{{Name: "player_name", Number: 1, Type: "string"}}},
			{Name: "MapLoad", Fields: []ir.GRPCField{
				{Name: "tiles", Number: 1, Type: "TileKind", Repeated: true},
				{Name: "labels", Number: 2, Type: "string", MapKey: "string"},
				{Name: "seed", Number: 3, Type: "int64", Optional: true},
			}, Reserved: []int{9}},
			{Name: "Ping", Fields: []ir.GRPCField{{Name: "t", Number: 1, Type: "int64"}}},
			{Name: "ServerMessage", Oneofs: []ir.GRPCOneof{{Name: "msg", Members: []ir.GRPCField{
				{Name: "map_load", Number: 1, Type: "MapLoad"}}}}},
		},
		Services: []ir.GRPCService{{Name: "GameService", Methods: []ir.GRPCMethod{
			{Name: "Look", Stream: "none", Request: "Ping", Response: "Hello"},
			{Name: "Play", Stream: "bidi", Request: "ClientMessage", Response: "ServerMessage"},
			{Name: "Watch", Stream: "server", Request: "Ping", Response: "ServerMessage"},
		}}},
	}
}

func TestRenderGRPCProto(t *testing.T) {
	t.Parallel()
	got := RenderGRPCProto(testGRPCDef(), "example.com/game")
	for _, want := range []string{
		`package game.v1;`,
		`option go_package = "example.com/game/internal/transport/grpc/pb/game/v1;gamev1";`,
		`option csharp_namespace = "Game.V1";`,
		`rpc Play(stream ClientMessage) returns (stream ServerMessage);`,
		`rpc Watch(Ping) returns (stream ServerMessage);`,
		`rpc Look(Ping) returns (Hello);`,
		"  TILE_KIND_UNSPECIFIED = 0;\n  TILE_KIND_FLOOR = 1;",
		"  oneof msg {\n    Hello hello = 1;\n    Ping ping = 2;\n  }",
		`repeated TileKind tiles = 1;`,
		`map<string, string> labels = 2;`,
		`optional int64 seed = 3;`,
		`reserved 9;`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("proto lacks %q:\n%s", want, got)
		}
	}
	if again := RenderGRPCProto(testGRPCDef(), "example.com/game"); again != got {
		t.Fatal("proto output is not deterministic")
	}
}

func TestGRPCProtoPath(t *testing.T) {
	t.Parallel()
	if got := GRPCProtoPath(&ir.GRPC{Package: "dealingi.trade.v1"}); got != "dealingi/trade/v1/trade.proto" {
		t.Fatalf("path = %s", got)
	}
}

// The generated pb code, stubs and server must be valid Go for all rpc kinds.
func TestEmitGRPCServerGeneratesCompilableGo(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/game"}
	def := testGRPCDef()
	if err := em.emitGRPCContract(def); err != nil {
		t.Fatal(err)
	}
	if err := em.emitGRPCServer(def); err != nil {
		t.Fatal(err)
	}
	pbDir := filepath.Join(tmp, "internal", "transport", "grpc", "pb", "game", "v1")
	for _, f := range []string{
		filepath.Join(pbDir, "game.pb.go"),
		filepath.Join(pbDir, "game_grpc.pb.go"),
		filepath.Join(tmp, "internal", "transport", "grpc", "server.gen.go"),
		filepath.Join(tmp, "api", "grpc", "game", "v1", "game.proto"),
	} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(f, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), f, src, parser.AllErrors); err != nil {
				t.Errorf("%s does not parse: %v", f, err)
			}
			if formatted, err := format.Source(src); err != nil || string(formatted) != string(src) {
				t.Errorf("%s is not gofmt-clean (err=%v)", f, err)
			}
		}
	}
	stubs, _ := os.ReadFile(filepath.Join(pbDir, "game_grpc.pb.go"))
	for _, want := range []string{
		"Play(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ClientMessage, ServerMessage], error)",
		"Watch(*Ping, grpc.ServerStreamingServer[ServerMessage]) error",
		"Look(context.Context, *Ping) (*Hello, error)",
		"type GameService_PlayClient = grpc.BidiStreamingClient[ClientMessage, ServerMessage]",
		"type GameService_PlayServer = grpc.BidiStreamingServer[ClientMessage, ServerMessage]",
		`{StreamName: "Play", Handler: _GameService_Play_Handler, ServerStreams: true, ClientStreams: true},`,
		// rpcs are sorted Look, Play, Watch; ServiceDesc.Streams lists only Play (0) and Watch (1).
		"&GameService_ServiceDesc.Streams[0], GameService_Play_FullMethodName",
		"&GameService_ServiceDesc.Streams[1], GameService_Watch_FullMethodName",
	} {
		if !strings.Contains(string(stubs), want) {
			t.Errorf("stubs lack %q", want)
		}
	}
	if !strings.Contains(string(stubs), "grpc.BidiStreamingClient[ClientMessage, ServerMessage]") {
		t.Fatalf("stubs lack the bidi client:\n%s", stubs)
	}
}
