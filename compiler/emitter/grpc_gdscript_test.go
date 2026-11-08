package emitter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bufbuild/protocompile"
	"github.com/strogmv/ang/angir/ir"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

const gdProto = `syntax="proto3"; package sample.v1;
service Pipe { rpc Play(Envelope) returns (Envelope); }
enum Mode { MODE_UNSPECIFIED=0; MODE_READY=1; }
message Unused { string ignored=1; }
message Envelope {
 message Child { string label=1; int64 huge=2; }
 int64 signed_value=1; uint64 unsigned_value=2; Mode mode=3;
 Child child=4; repeated Child children=5; map<string,Child> lookup=6;
 repeated int64 counts=7; map<bool,int64> flags=8;
 optional int32 present=9; bool active=10; bytes payload=11;
 oneof msg { Child selected=12; string text=13; }
 double ratio=14; uint64 maximum=15; map<uint64,string> ids=16; Object object_value=17;
}
message Object { string class=1; string pb_class=2; string get_instance_id=3; int32 and=4;
}`

func TestGDScriptGoldenAndRuntime(t *testing.T) {
	c := protocompile.Compiler{Resolver: &protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(map[string]string{"sample.proto": gdProto})}}
	files, err := c.Compile(context.Background(), "sample.proto")
	if err != nil {
		t.Fatal(err)
	}
	got := renderGDDescriptor(files[0])
	golden := filepath.Join("testdata", "grpc_contract.gd")
	if os.Getenv("UPDATE_GDSCRIPT_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("GDScript differs from golden")
	}
	if string(got) != string(renderGDDescriptor(files[0])) {
		t.Fatal("non-deterministic contract")
	}
	godot := os.Getenv("ANG_GODOT_BIN")
	if godot == "" {
		t.Log("set ANG_GODOT_BIN to also execute the golden with Godot")
		return
	}
	dir := t.TempDir()
	for n, b := range map[string][]byte{"contract.gd": got, "project.godot": []byte("config_version=5\n[application]\nconfig/name=\"Contract test\"\nconfig/use_custom_user_dir=true\nconfig/custom_user_dir_name=\"godot-link-private\"\n")} {
		if err := os.WriteFile(filepath.Join(dir, n), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	input := `{"signed_value":"-9007199254740993","unsigned_value":"9007199254740993","mode":"MODE_READY","child":{"label":"nested","huge":"9223372036854775807"},"children":[{"label":"list"}],"lookup":{"a":{"huge":"-4"}},"counts":["9007199254740993"],"flags":{"true":"2","false":"-1"},"present":0,"active":true,"payload":"YWJj","selected":{"label":"oneof"},"ratio":"Infinity","maximum":"18446744073709551615","ids":{"18446744073709551615":"large"}}`
	if err := os.WriteFile(filepath.Join(dir, "input.json"), []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	script := `extends SceneTree
const C = preload("res://contract.gd")
func _init():
 var d = C.Envelope.from_dict(JSON.parse_string(FileAccess.get_file_as_string("res://input.json")))
 var special = C._PB_Object.from_dict({"class":"keyword","pb_class":"prefix","get_instance_id":"method","and":3})
 assert(special.pb_class == "keyword" and special.pb_pb_class == "prefix" and special.pb_get_instance_id == "method" and special.pb_and == 3)
 assert(special.to_dict() == {"class":"keyword","pb_class":"prefix","get_instance_id":"method","and":3})
 var numeric_enum = C.Envelope.new()
 numeric_enum.mode = 1
 assert(numeric_enum.to_dict().mode == C.Mode.MODE_READY)
 assert(C.Envelope.from_dict({"mode":1}).mode == C.Mode.MODE_READY)
 assert(C.Envelope.from_dict({"mode":99}).to_dict().mode == 99)
 assert(d.maximum == "18446744073709551615" and d.ids["18446744073709551615"] == "large")
 assert(d.signed_value == -9007199254740993)
 assert(d.unsigned_value == 9007199254740993)
 assert(d.mode == C.Mode.MODE_READY)
 assert(d.child.huge == 9223372036854775807)
 assert(d.children[0].label == "list" and d.lookup.a.huge == -4)
 assert(d.counts[0] == 9007199254740993 and d.flags[true] == 2 and d.flags[false] == -1)
 assert(d.present == 0 and d.selected.label == "oneof" and d.text == null)
 assert(d.payload.get_string_from_utf8() == "abc" and is_inf(d.ratio))
 var empty = C.Envelope.from_dict({"child":null,"counts":null})
 assert(empty.child == null and empty.children.is_empty() and empty.lookup.is_empty())
 assert(empty.signed_value == 0 and empty.unsigned_value == 0 and not empty.active)
 assert(empty.mode == C.Mode.MODE_UNSPECIFIED and empty.present == null and empty.text == null)
 assert(empty.payload.is_empty() and empty.ratio == 0.0 and empty.to_dict().is_empty())
 assert(C.Envelope.from_dict({"signedValue":"17"}).signed_value == 17)
 var out = FileAccess.open("res://output.json",FileAccess.WRITE)
 out.store_string(JSON.stringify(d.to_dict()))
 out.close()
 d.text = "changed"
 assert(d.selected == null and d.to_dict().text == "changed")
 d.text = null
 assert(not d.to_dict().has("text"))
 print("CONTRACT_RUNTIME_PASS")
 quit()
`
	if err := os.WriteFile(filepath.Join(dir, "test.gd"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, godot, "--headless", "--path", dir, "--log-file", filepath.Join(dir, "engine.log"), "--script", "test.gd")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Godot: %v\n%s", err, b)
	}
	if strings.Contains(string(b), "ERROR:") || !strings.Contains(string(b), "CONTRACT_RUNTIME_PASS") {
		t.Fatalf("Godot runtime diagnostics: %s", b)
	}
	out, err := os.ReadFile(filepath.Join(dir, "output.json"))
	if err != nil {
		t.Fatalf("runtime did not finish: %v\n%s", err, b)
	}
	desc := files[0].Messages().ByName("Envelope")
	a, z := dynamicpb.NewMessage(desc), dynamicpb.NewMessage(desc)
	if err := protojson.Unmarshal([]byte(input), a); err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(out, z); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(a, z) {
		t.Fatalf("protojson roundtrip changed message: %s", out)
	}
}

func TestGDScriptFromIRAndOption(t *testing.T) {
	def := &ir.GRPC{Package: "test.v1", GDScriptOut: "client/contract.gd", Messages: []ir.GRPCMessage{{Name: "Root", Fields: []ir.GRPCField{{Name: "value", Number: 1, Type: "int64"}}}}, Services: []ir.GRPCService{{Name: "API", Methods: []ir.GRPCMethod{{Name: "Call", Request: "Root", Response: "Root", Stream: "none"}}}}}
	b, err := RenderGDScriptContract(def, "example.test")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	e := &Emitter{OutputDir: dir}
	if err := e.emitGRPCContract(def); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, def.GDScriptOut))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(out) {
		t.Fatal("normal pipeline did not emit contract")
	}
}
