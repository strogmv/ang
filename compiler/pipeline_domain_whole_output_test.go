package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
)

func runPipelineFiles(t *testing.T, files map[string]string) ([]normalizer.Entity, []normalizer.Service, error) {
	t.Helper()
	root := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entities, services, _, _, _, _, _, _, _, err := compiler.RunPipeline(root)
	return entities, services, err
}

func methodByName(services []normalizer.Service, name string) *normalizer.Method {
	for si := range services {
		for mi := range services[si].Methods {
			if services[si].Methods[mi].Name == name {
				return &services[si].Methods[mi]
			}
		}
	}
	return nil
}

func fieldNames(e normalizer.Entity) string {
	names := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		names = append(names, f.Name+":"+f.Type)
	}
	return strings.Join(names, ",")
}

// An operation whose whole input or output is a domain DTO (domain.#X) gets
// its fields; the loader stubs the domain package while it reads cue/api, so
// the response used to be an empty struct (backlog 48).
func TestPipelineReadsWholeDomainInputAndOutput(t *testing.T) {
	_, services, err := runPipelineFiles(t, map[string]string{
		"cue.mod/module.cue": "module: \"example.com/app\"\nlanguage: version: \"v0.9.0\"\n",
		"cue/domain/quote.cue": `package domain

#QuoteView: {
	@dto(only="true")
	id:    string
	total: float
	lines: [...#QuoteLine]
}

#QuoteLine: {
	@dto(only="true")
	sku: string
	qty: int
}
`,
		"cue/api/quotes.cue": `package api

import "example.com/app/cue/domain"

#Op: {
	service: string
	input?:  _
	output?: _
	...
}

GetQuote: #Op & {
	service: "quotes"
	input: {id: string}
	output: domain.#QuoteView
}

GetQuoteMerged: #Op & {
	service: "quotes"
	input: {id: string}
	output: domain.#QuoteView & {currency: string}
}

PutQuote: {
	service: "quotes"
	input: domain.#QuoteView
	output: {ok: bool}
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"GetQuote.output":       "id:string,total:float64,lines:[]domain.QuoteLine",
		"GetQuoteMerged.output": "id:string,total:float64,lines:[]domain.QuoteLine,currency:string",
		"PutQuote.input":        "id:string,total:float64,lines:[]domain.QuoteLine",
	} {
		op, side, _ := strings.Cut(name, ".")
		m := methodByName(services, op)
		if m == nil {
			t.Fatalf("no operation %s", op)
		}
		ent := m.Output
		if side == "input" {
			ent = m.Input
		}
		if got := fieldNames(ent); got != want {
			t.Errorf("%s fields = %s, want %s", name, got, want)
		}
	}
}

func TestPipelineRefusesWholeOutputOfNoDomainDTO(t *testing.T) {
	_, _, err := runPipelineFiles(t, map[string]string{
		"cue.mod/module.cue":   "module: \"example.com/app\"\nlanguage: version: \"v0.9.0\"\n",
		"cue/domain/quote.cue": "package domain\n\n#QuoteView: {\n\t@dto(only=\"true\")\n\tid: string\n}\n",
		"cue/api/quotes.cue": `package api

import "example.com/app/cue/domain"

GetQuote: {
	service: "quotes"
	input: {id: string}
	output: domain.#QuoteViw
}
`,
	})
	if err == nil || !strings.Contains(err.Error(), "QuoteViw") {
		t.Fatalf("expected an error naming domain.#QuoteViw, got %v", err)
	}
}

// A definition of cue/api used as a field type is a DTO of its own; it used
// to be "unknown entity type" unless moved to cue/domain (backlog 45).
func TestPipelineRegistersAPIDefinitionsUsedAsTypes(t *testing.T) {
	entities, services, err := runPipelineFiles(t, map[string]string{
		"cue/domain/quote.cue": "package domain\n\nQuote: {\n\tid: string\n}\n",
		"cue/api/quotes.cue": `package api

#QuoteLine: {
	sku: string
	qty: int
	tax?: #TaxLine
}

#TaxLine: {
	rate: float
}

#Unused: {x: string}

SubmitQuote: {
	service: "quotes"
	input: {
		quoteId: string
		lines: [...#QuoteLine]
	}
	output: {ok: bool}
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]normalizer.Entity{}
	for _, e := range entities {
		byName[e.Name] = e
	}
	for name, want := range map[string]string{
		"QuoteLine": "sku:string,qty:int,tax:domain.TaxLine",
		"TaxLine":   "rate:float64",
	} {
		e, ok := byName[name]
		if !ok {
			t.Fatalf("%s is not registered; entities: %v", name, entities)
		}
		if dto, _ := e.Metadata["dto"].(bool); !dto {
			t.Errorf("%s must be a DTO (no table)", name)
		}
		if got := fieldNames(e); got != want {
			t.Errorf("%s fields = %s, want %s", name, got, want)
		}
	}
	if _, ok := byName["Unused"]; ok {
		t.Error("a definition no operation uses stays out")
	}
	m := methodByName(services, "SubmitQuote")
	if got := fieldNames(m.Input); got != "quoteId:string,lines:[]domain.QuoteLine" {
		t.Errorf("SubmitQuote input = %s", got)
	}
}
