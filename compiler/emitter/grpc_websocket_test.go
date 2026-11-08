package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGRPCWebSocketGeneratedRuntime(t *testing.T) {
	def := deliveryFixture()
	def.WebSocket = true
	imp, pkg := GRPCGoPackage(def, "example.test")
	ws, err := RenderGRPCWebSocket(def, imp, pkg)
	if err != nil {
		t.Fatal(err)
	}
	_, pb, err := GeneratePBGo(def, "example.test")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pbDir := filepath.Join(dir, "internal/transport/grpc/pb/sample/v1")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		filepath.Join(pbDir, "sample.pb.go"):      pb,
		filepath.Join(pbDir, "sample_grpc.pb.go"): RenderGRPCStubs(def, pkg),
		filepath.Join(dir, "server.go"):           RenderGRPCServer(def, imp, pkg),
		filepath.Join(dir, "websocket.go"):        ws,
		filepath.Join(dir, "websocket_test.go"):   []byte(webSocketRuntimeTest),
		filepath.Join(dir, "go.mod"):              []byte("module example.test\n\ngo 1.24.0\nrequire (\ngithub.com/gorilla/websocket v1.5.3\ngoogle.golang.org/grpc v1.84.0\ngoogle.golang.org/protobuf v1.36.12\n)\n"),
	}
	for path, data := range files {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-mod=mod", ".", "-count=1")
	cmd.Dir = dir
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated native WS: %v\n%s", err, data)
	}
}

const webSocketRuntimeTest = `package grpc
import(
 "context"
 "net"
 "testing"
 "time"
 "strings"
 pb "example.test/internal/transport/grpc/pb/sample/v1"
 "github.com/gorilla/websocket"
 "google.golang.org/grpc"
 "google.golang.org/protobuf/encoding/protojson"
)
type echo struct{pb.UnimplementedPipeServer}
func(echo)Play(s grpc.BidiStreamingServer[pb.Envelope,pb.Envelope])error{m,err:=s.Recv();if err!=nil{return err};return s.Send(m)}
func TestWire(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 lis,err:=net.Listen("tcp","0.0.0.0:0");if err!=nil{t.Fatal(err)};defer lis.Close()
 if ValidateWebSocketAddress(lis.Addr(),WebSocketOptions{})==nil{t.Fatal("wildcard accepted by default")}
 done:=make(chan error,1);go func(){done<-ServeWebSocket(ctx,lis,Handlers{Pipe:echo{}},WebSocketOptions{AllowRemote:true})}()
 _,port,_:=net.SplitHostPort(lis.Addr().String())
 c,_,err:=websocket.DefaultDialer.Dial("ws://127.0.0.1:"+port+"/sample.v1.Pipe/Play",nil);if err!=nil{t.Fatal(err)};defer c.Close()
 _=c.SetReadDeadline(time.Now().Add(5*time.Second))
 if err:=c.WriteMessage(websocket.TextMessage,[]byte("{\"live\":true,\"tiles_revision\":\"9007199254740993\"}"));err!=nil{t.Fatal(err)}
 _,data,err:=c.ReadMessage();if err!=nil{t.Fatal(err)}
 if !strings.Contains(string(data),"tiles_revision"){t.Fatal("proto names not preserved")}
 m:=new(pb.Envelope);if err:=protojson.Unmarshal(data,m);err!=nil{t.Fatal(err)}
 if !m.Live||m.TilesRevision!=9007199254740993{t.Fatal(m)}
 cancel();if err:=<-done;err!=nil{t.Fatal(err)}
}
`
