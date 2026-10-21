package emitter

import (
	"strings"
	"testing"
)

func TestCSharpNamespace(t *testing.T) {
	t.Parallel()
	def := testGRPCDef()
	def.CSharpNamespace = ""
	if got := CSharpNamespace(def); got != "Game.V1" {
		t.Fatalf("default namespace = %q", got)
	}
	def.CSharpNamespace = "My.Game"
	if got := CSharpNamespace(def); got != "My.Game" {
		t.Fatalf("explicit namespace = %q", got)
	}
}

func TestSessionMethodsOnlyBidiOneofWrappers(t *testing.T) {
	t.Parallel()
	got := SessionMethods(testGRPCDef())
	if len(got) != 1 || got[0].Method.Name != "Play" {
		t.Fatalf("session methods = %+v, want only Play (Look is unary, Watch is server-streaming)", got)
	}
}

func TestRenderCSharpStubsCoversAllRpcKinds(t *testing.T) {
	t.Parallel()
	def := testGRPCDef()
	out := RenderCSharpStubs(def, def.Services[0])
	for _, want := range []string{
		"namespace Game.V1",
		`public const string ServiceName = "game.v1.GameService";`,
		"Method<ClientMessage, ServerMessage> PlayMethod = new Method<ClientMessage, ServerMessage>(MethodType.DuplexStreaming",
		"Method<Ping, ServerMessage> WatchMethod = new Method<Ping, ServerMessage>(MethodType.ServerStreaming",
		"Method<Ping, Hello> LookMethod = new Method<Ping, Hello>(MethodType.Unary",
		"AsyncDuplexStreamingCall<ClientMessage, ServerMessage> Play(CallOptions options) => CallInvoker.AsyncDuplexStreamingCall(GameServiceGrpc.PlayMethod, null, options);",
		"AsyncServerStreamingCall<ServerMessage> Watch(Ping request, CallOptions options)",
		"Hello Look(Ping request, CallOptions options) => CallInvoker.BlockingUnaryCall(",
		"public class GameServiceClient : ClientBase<GameServiceClient>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stubs lack %q", want)
		}
	}
	// A marshaller per distinct message, not per use.
	if n := strings.Count(out, "Marshaller<Ping> MarshallerPing"); n != 1 {
		t.Errorf("Ping marshaller declared %d times", n)
	}
}

func TestRenderCSharpSessionTypedEventsAndSenders(t *testing.T) {
	t.Parallel()
	def := testGRPCDef()
	sm := SessionMethods(def)[0]
	out := RenderCSharpSession(def, sm.Service, sm.Method)
	for _, want := range []string{
		"public sealed class GameServicePlaySession : IDisposable",
		"public event Action<MapLoad> OnMapLoad;",
		"public void SendHello(Hello value) { Send(new ClientMessage { Hello = value }); }",
		"public void SendPing(Ping value) { Send(new ClientMessage { Ping = value }); }",
		"case ServerMessage.MsgOneofCase.MapLoad:",
		"public int Pump(int maxMessages = int.MaxValue)",
		"public GameServicePlaySessionReconnectPolicy Reconnect",
		"CancellationTokenSource.CreateLinkedTokenSource(ct)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("session lacks %q", want)
		}
	}
	if strings.Contains(out, "UnityEngine") {
		t.Error("the session must not depend on UnityEngine (testable under plain dotnet)")
	}
	if strings.Contains(out, "%!") {
		t.Errorf("format verb mismatch in template:\n%s", out)
	}
}
