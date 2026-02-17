package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type contractField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Tag  string `json:"tag,omitempty"`
}

func TestSchemaContractGoldenV1(t *testing.T) {
	assertSchemaContractGolden(t, "golden_ir_v1.json")
}

func TestSchemaContractGoldenCurrent(t *testing.T) {
	assertSchemaContractGolden(t, "golden_ir_v2.json")
}

func assertSchemaContractGolden(t *testing.T, filename string) {
	t.Helper()

	contract := buildIRContract()
	path := filepath.Join("testdata", filename)
	if os.Getenv("UPDATE_IR_CONTRACT") == "1" {
		data, err := json.MarshalIndent(contract, "", "  ")
		if err != nil {
			t.Fatalf("marshal contract: %v", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write golden contract: %v", err)
		}
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden contract: %v", err)
	}

	var want map[string][]contractField
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("decode golden contract: %v", err)
	}

	if !reflect.DeepEqual(want, contract) {
		gotBytes, _ := json.MarshalIndent(contract, "", "  ")
		t.Fatalf("IR contract changed.\nIf intentional, bump ir version/migrations and refresh %s.\nGot:\n%s", path, string(gotBytes))
	}
}

func buildIRContract() map[string][]contractField {
	types := []reflect.Type{
		reflect.TypeOf(Schema{}),
		reflect.TypeOf(Project{}),
		reflect.TypeOf(Target{}),
		reflect.TypeOf(Entity{}),
		reflect.TypeOf(Field{}),
		reflect.TypeOf(Service{}),
		reflect.TypeOf(Method{}),
		reflect.TypeOf(Source{}),
		reflect.TypeOf(Event{}),
		reflect.TypeOf(Endpoint{}),
		reflect.TypeOf(Repository{}),
		reflect.TypeOf(Finder{}),
	}

	out := make(map[string][]contractField, len(types))
	for _, typ := range types {
		fields := make([]contractField, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			cf := contractField{
				Name: f.Name,
				Type: f.Type.String(),
			}
			if tag := f.Tag.Get("json"); tag != "" {
				cf.Tag = tag
			}
			fields = append(fields, cf)
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		out[typ.Name()] = fields
	}
	return out
}
