package emitter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strogmv/ang/angir/ir"
)

func deliveryFixture() *ir.GRPC {
	return &ir.GRPC{Package: "sample.v1", ViewSubscription: "Subscription", Messages: []ir.GRPCMessage{
		{Name: "Subscription", Fields: []ir.GRPCField{{Name: "diagnostics", Number: 1, Type: "string", Repeated: true}, {Name: "diagnostics_all", Number: 2, Type: "bool"}, {Name: "static_on_change", Number: 3, Type: "bool"}}},
		{Name: "Envelope", Fields: []ir.GRPCField{
			{Name: "diagnostic", Number: 1, Type: "string", Delivery: "diagnostic"},
			{Name: "tiles", Number: 2, Type: "string", Repeated: true, Delivery: "static_on_change", Revision: "tiles_revision"},
			{Name: "tiles_revision", Number: 3, Type: "uint64"},
			{Name: "child", Number: 4, Type: "Row"}, {Name: "rows", Number: 5, Type: "Row", Repeated: true},
			{Name: "lookup", Number: 6, Type: "Row", MapKey: "string"}, {Name: "subscription", Number: 7, Type: "Subscription"},
			{Name: "live", Number: 8, Type: "bool"},
			{Name: "settings", Number: 9, Type: "Settings", Delivery: "static_on_change", Revision: "settings_revision"}, {Name: "settings_revision", Number: 10, Type: "uint64"},
			{Name: "tags", Number: 11, Type: "int64", MapKey: "string", Delivery: "static_on_change", Revision: "tags_revision"}, {Name: "tags_revision", Number: 12, Type: "uint64"},
			{Name: "flag", Number: 13, Type: "bool", Optional: true, Delivery: "static_on_change", Revision: "flag_revision"}, {Name: "flag_revision", Number: 14, Type: "uint64"},
		}},
		{Name: "Settings", Fields: []ir.GRPCField{{Name: "mode", Number: 1, Type: "string"}}},
		{Name: "Row", Fields: []ir.GRPCField{{Name: "label", Number: 1, Type: "string", Delivery: "static_on_change", Revision: "label_revision"}, {Name: "label_revision", Number: 2, Type: "uint64"}, {Name: "nested", Number: 6, Type: "Row", MapKey: "string"}}},
	}, Services: []ir.GRPCService{{Name: "Pipe", Methods: []ir.GRPCMethod{{Name: "Play", Request: "Envelope", Response: "Envelope", Stream: "bidi"}}}}}
}

func TestGRPCDeliveryGoldenAndRuntime(t *testing.T) {
	def := deliveryFixture()
	if err := def.Validate(); err != nil {
		t.Fatal(err)
	}
	gd, err := RenderGDScriptContract(def, "example.test")
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := RenderGRPCDelivery(def, "samplev1")
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string][]byte{"grpc_delivery.go.golden": delivery, "grpc_delivery.gd": gd} {
		p := filepath.Join("testdata", name)
		if os.Getenv("UPDATE_GDSCRIPT_GOLDEN") == "1" {
			if err := os.WriteFile(p, got, 0644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s differs from golden", name)
		}
	}
	dir := t.TempDir()
	_, pb, err := GeneratePBGo(def, "example.test")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"contract.pb.go": pb, "delivery.go": delivery, "delivery_test.go": []byte(deliveryRuntimeTest), "go.mod": []byte("module example.test\n\ngo 1.24.0\n\nrequire google.golang.org/protobuf v1.36.6\n"), "contract.gd": gd, "test.gd": []byte(deliveryGDScriptTest), "project.godot": []byte("config_version=5\n[application]\nconfig/use_custom_user_dir=true\nconfig/custom_user_dir_name=\"godot-link-private\"\n")}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-mod=mod", ".", "-count=1")
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated projection: %v\n%s", err, b)
	}
	godot := os.Getenv("ANG_GODOT_BIN")
	if godot == "" {
		t.Log("set ANG_GODOT_BIN for native delta-cache checks")
		return
	}
	cmd = exec.CommandContext(ctx, godot, "--headless", "--path", dir, "--log-file", filepath.Join(dir, "engine.log"), "--script", "test.gd")
	b, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(b), "ERROR:") || !strings.Contains(string(b), "DELIVERY_RUNTIME_PASS") {
		t.Fatalf("native cache: %v\n%s", err, b)
	}
}

const deliveryRuntimeTest = `package samplev1
import (
 "encoding/json"
 "os"
 "testing"
 "google.golang.org/protobuf/proto"
 "google.golang.org/protobuf/encoding/protojson"
)
func TestProjection(t *testing.T) {
 flag:=false
 in:=&Envelope{Settings:&Settings{Mode:"sample"},Tags:map[string]int64{"a":9007199254740993},Flag:&flag,Diagnostic:"private",Tiles:[]string{"map"},Child:&Row{Label:"child"},Rows:[]*Row{{Label:"row"}},Lookup:map[string]*Row{"a":{Label:"lookup",Nested:map[string]*Row{"b":{Label:"nested"}}},"a/6/b":{Label:"slash"}},Live:true}
 original:=proto.Clone(in)
 legacy:=NewViewProjector(nil)
 for i:=0;i<2;i++ {raw,err:=legacy.Project(in);if err!=nil {t.Fatal(err)};s:=raw.(*Envelope);if s.Diagnostic!="private"||len(s.Tiles)!=1||s.TilesRevision!=1 {t.Fatal("legacy full",s)}}
 p:=NewViewProjector(&Subscription{StaticOnChange:true})
 var frames []json.RawMessage
 project:=func()*Envelope {t.Helper();raw,err:=p.Project(in);if err!=nil {t.Fatal(err)};s:=raw.(*Envelope);b,err:=(protojson.MarshalOptions{UseProtoNames:true}).Marshal(s);if err!=nil {t.Fatal(err)};frames=append(frames,b);return s}
 first:=project();if first.Settings==nil||first.Settings.Mode!="sample"||first.Tags["a"]!=9007199254740993||first.Flag==nil||*first.Flag {t.Fatal("static message/map/optional",first)};if first.Diagnostic!=""||len(first.Tiles)!=1||first.TilesRevision!=1||first.Child.Label!="child" {t.Fatal("first",first)}
 second:=project();if second.Settings!=nil||len(second.Tags)!=0||second.Flag!=nil||second.FlagRevision!=1 {t.Fatal("message/map/optional resent",second)};if len(second.Tiles)!=0||second.TilesRevision!=1||second.Child.Label!=""||second.Rows[0].Label!=""||second.Lookup["a"].Label!=""||!second.Live {t.Fatal("unchanged",second)}
 if !proto.Equal(original,in) {t.Fatal("projection mutated source")}
 in.Tiles=[]string{"changed"};in.Child.Label="new child"
 changed:=project();if len(changed.Tiles)!=1||changed.TilesRevision!=2||changed.Child.LabelRevision!=2 {t.Fatal("change",changed)}
 if s:=project();len(s.Tiles)!=0||s.TilesRevision!=2 {t.Fatal("resent unchanged",s)}
 in.Tiles=nil;in.Child.Label="";in.Settings=nil;in.Tags=nil;in.Flag=nil
 clear:=project();if len(clear.Tiles)!=0||clear.TilesRevision!=3||clear.Child.LabelRevision!=3 {t.Fatal("clear",clear)}
 if s:=project();s.TilesRevision!=3||len(s.Tiles)!=0 {t.Fatal("empty unchanged",s)}
 // The same content after a world replacement must still send a full snapshot.
 in.Tiles=[]string{"map"};in.Child.Label="child";project()
 p.ResetSnapshots();s:=project();if len(s.Tiles)!=1||s.TilesRevision!=4||s.Child.Label!="child" {t.Fatal("reset",s)}
 p.SetSubscription(&Subscription{Diagnostics:[]string{"sample.v1.Envelope.diagnostic"},StaticOnChange:true})
 if s:=project();s.Diagnostic!="private"||len(s.Tiles)!=1||s.TilesRevision!=4 {t.Fatal("exact subscription",s)}
 p.SetSubscription(&Subscription{DiagnosticsAll:true})
 for i:=0;i<2;i++ {if s:=project();s.Diagnostic!="private"||len(s.Tiles)!=1 {t.Fatal("all/full",s)}}
 reconnect:=NewViewProjector(&Subscription{StaticOnChange:true});raw,err:=reconnect.Project(in);if err!=nil {t.Fatal(err)};if s:=raw.(*Envelope);len(s.Tiles)!=1||s.Child.Label!="child" {t.Fatal("reconnect",s)}
 b,err:=json.Marshal(frames);if err!=nil {t.Fatal(err)};if err:=os.WriteFile("frames.json",b,0600);err!=nil {t.Fatal(err)}
}
`

const deliveryGDScriptTest = `extends SceneTree
const C = preload("res://contract.gd")
func _init():
 var cache = C.ViewCache.new()
 var frames = JSON.parse_string(FileAccess.get_file_as_string("res://frames.json"))
 var first = cache.decode_Envelope(frames[0])
 var second = cache.decode_Envelope(frames[1])
 assert(first.tiles == ["map"] and second.tiles == first.tiles)
 assert(is_same(first.tiles, second.tiles))
 assert(is_same(first.settings, second.settings) and second.settings.mode == "sample")
 assert(second.tags.a == 9007199254740993 and second.flag == false)
 assert(second.child.label == "child" and second.rows[0].label == "row" and second.lookup.a.label == "lookup")
 assert(second.lookup.a.nested.b.label == "nested" and second.lookup["a/6/b"].label == "slash")
 var changed = cache.decode_Envelope(frames[2])
 assert(changed.tiles == ["changed"] and changed.child.label == "new child")
 assert(first.tiles == ["map"] and first.child.label == "child")
 var unchanged = cache.decode_Envelope(frames[3])
 assert(unchanged.tiles == changed.tiles and unchanged.child.label == "new child")
 var cleared = cache.decode_Envelope(frames[4])
 assert(cleared.tiles.is_empty() and cleared.child.label.is_empty())
 assert(cleared.settings == null and cleared.tags.is_empty() and cleared.flag == null)
 assert(cache.decode_Envelope(frames[5]).tiles.is_empty())
 for index in range(6,frames.size()): cache.decode_Envelope(frames[index])
 cache.reset()
 var reconnected = cache.decode_Envelope(frames[0])
 assert(reconnected.tiles == ["map"] and reconnected.child.label == "child")
 # Each stream owns its own cache; plain from_dict retains legacy defaults.
 assert(C.Envelope.from_dict(frames[1]).tiles.is_empty())
 print("DELIVERY_RUNTIME_PASS")
 quit()
`
