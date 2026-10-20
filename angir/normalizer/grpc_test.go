package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

const grpcCUEHead = `GRPC: {
	package: "game.v1"
	services: GameService: Play: {stream: "bidi", request: "In", response: "Out"}
`

func extractGRPC(t *testing.T, body string) (*GRPCDef, error) {
	t.Helper()
	v := cuecontext.New().CompileString(grpcCUEHead + body + "\n}")
	if err := v.Err(); err != nil {
		t.Fatalf("cue: %v", err)
	}
	return (&Normalizer{}).ExtractGRPC(v)
}

func TestExtractGRPCSortsByNumberAndName(t *testing.T) {
	t.Parallel()
	def, err := extractGRPC(t, `
	enums: Kind: values: [{name: "B", n: 2}, {name: "A", n: 1}]
	messages: {
		Out: fields: {z: {n: 2, type: "Kind"}, a: {n: 1, type: "string"}}
		In: oneofs: msg: {b: {n: 2, type: "Out"}, a: {n: 1, type: "Out"}}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if def.Messages[0].Name != "In" || def.Messages[1].Name != "Out" {
		t.Fatalf("messages not sorted by name: %+v", def.Messages)
	}
	if got := def.Messages[1].Fields[0].Name; got != "a" {
		t.Fatalf("fields not sorted by number: first = %s", got)
	}
	if got := def.Messages[0].Oneofs[0].Members[0].Name; got != "a" {
		t.Fatalf("oneof members not sorted by number: first = %s", got)
	}
	if def.Enums[0].Values[0].Name != "A" {
		t.Fatalf("enum values not sorted by number: %+v", def.Enums[0].Values)
	}
	if !def.Reflection || def.Listen != ":50051" {
		t.Fatalf("defaults wrong: reflection=%v listen=%q", def.Reflection, def.Listen)
	}
}

func TestExtractGRPCAbsent(t *testing.T) {
	t.Parallel()
	v := cuecontext.New().CompileString(`Other: 1`)
	def, err := (&Normalizer{}).ExtractGRPC(v)
	if err != nil || def != nil {
		t.Fatalf("want nil,nil got %v,%v", def, err)
	}
}

func TestExtractGRPCRejects(t *testing.T) {
	t.Parallel()
	msgs := `messages: In: fields: x: {n: 1, type: "string"}
	`
	cases := map[string]struct{ body, want string }{
		"duplicate number": {msgs + `messages: Out: fields: {a: {n: 1, type: "string"}, b: {n: 1, type: "string"}}`, "number 1 used by both"},
		"unknown type":     {msgs + `messages: Out: fields: a: {n: 1, type: "Nope"}`, `unknown type "Nope"`},
		"reserved number":  {msgs + `messages: Out: {reserved: [3], fields: a: {n: 3, type: "string"}}`, "reserved"},
		"protobuf range":   {msgs + `messages: Out: fields: a: {n: 19500, type: "string"}`, "reserved by protobuf"},
		"missing response": {`messages: In: fields: x: {n: 1, type: "string"}`, "must be declared messages"},
		"bad field name":   {msgs + `messages: Out: fields: PlayerName: {n: 1, type: "string"}`, "lower_snake_case"},
		"map and repeated": {msgs + `messages: Out: fields: a: {n: 1, type: "string", map_key: "string", repeated: true}`, "map_key excludes repeated"},
		"repeated oneof":   {msgs + `messages: Out: oneofs: o: a: {n: 1, type: "string", repeated: true}`, "not allowed in oneof"},
		"enum clash":       {msgs + `messages: Out: fields: a: {n: 1, type: "string"}` + "\nenums: Out: values: [{name: \"A\", n: 1}]", "both an enum and a message"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := extractGRPC(t, tc.body)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}
