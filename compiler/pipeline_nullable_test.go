package compiler_test

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestPipelineReadsNullableInputs(t *testing.T) {
	_, services, err := runPipelineFiles(t, map[string]string{
		"cue/domain/tender.cue": "package domain\n\nTender: {\n\tid: string\n}\n",
		"cue/api/tenders.cue": `package api

UpdateTender: {
	service: "tenders"
	input: {
		id:         string
		openEntry?: bool @nullable()
		minLots?:   int @nullable()
		archived?:  bool
		filter?: {onlyMine?: bool @nullable()}
	}
	output: {ok: bool}
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := methodByName(services, "UpdateTender")
	got := map[string]bool{}
	for _, f := range m.Input.Fields {
		got[f.Name] = normalizer.IsNullable(f)
		if f.Name == "openEntry" && f.Type != "bool" {
			t.Errorf("Field.Type stays the scalar, got %s", f.Type)
		}
	}
	if !got["openEntry"] || !got["minLots"] || got["archived"] || got["id"] {
		t.Fatalf("nullable flags = %v", got)
	}
}

func TestPipelineRefusesBadNullable(t *testing.T) {
	cases := map[string]string{
		"not optional": `UpdateTender: {
	service: "tenders"
	input: {id: string, openEntry: bool @nullable()}
	output: {ok: bool}
}`,
		"a list": `UpdateTender: {
	service: "tenders"
	input: {id: string, tags?: [...string] @nullable()}
	output: {ok: bool}
}`,
	}
	for name, api := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := runPipelineFiles(t, map[string]string{
				"cue/domain/tender.cue": "package domain\n\nTender: {\n\tid: string\n}\n",
				"cue/api/tenders.cue":   "package api\n\n" + api + "\n",
			})
			if err == nil || !strings.Contains(err.Error(), "@nullable") {
				t.Fatalf("expected a @nullable error, got %v", err)
			}
		})
	}
	_, _, err := runPipelineFiles(t, map[string]string{
		"cue/domain/tender.cue": "package domain\n\nTender: {\n\tid: string\n\topen?: bool @nullable()\n}\n",
	})
	if err == nil || !strings.Contains(err.Error(), "only operation inputs and outputs") {
		t.Fatalf("an entity field cannot be @nullable, got %v", err)
	}
}
