package compiler

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func bareErrorService(name, method string) normalizer.Service {
	return normalizer.Service{Name: name, Methods: []normalizer.Method{{
		Name: method,
		Flow: []normalizer.FlowStep{{
			Action: "flow.If",
			Args: map[string]any{"condition": "req.Force", "_then": []normalizer.FlowStep{{
				Action: "logic.Call", File: "cue/api/impl_x.cue", Line: 7, Column: 4,
				Args: map[string]any{"func": "(func() error { return fmt.Errorf(\"bad input\") })"},
			}}},
		}},
	}}}
}

func TestBareErrorWarnsOnlyForHTTPOperations(t *testing.T) {
	services := []normalizer.Service{
		bareErrorService("Company", "ImportProducts"),
		bareErrorService("Search", "ReindexCompanyProductCatalog"),
	}
	endpoints := []normalizer.Endpoint{{Method: "POST", Path: "/api/products/import", ServiceName: "company", RPC: "ImportProducts"}}

	got, opts := collectWarnings()
	emitBareErrorDiagnostics(services, endpoints, opts)
	if len(*got) != 1 {
		t.Fatalf("want one warning for the HTTP operation only, got %#v", *got)
	}
	w := (*got)[0]
	if w.Code != "LAMBDA_BARE_ERROR" || w.File != "cue/api/impl_x.cue" || w.Line != 7 {
		t.Fatalf("warning = %#v", w)
	}
}

// dealingi-back: Tender.CreateTender's logic.Call had no position of its own,
// and the warning came out without a file.
func TestBareErrorFallsBackToOperationPosition(t *testing.T) {
	svc := normalizer.Service{Name: "Tender", Methods: []normalizer.Method{{
		Name:   "CreateTender",
		Source: "cue/api/tender_create.cue:12",
		Flow:   []normalizer.FlowStep{{Action: "logic.Call", Args: map[string]any{"func": "(func() error { return fmt.Errorf(\"x\") })"}}},
	}}}
	got, opts := collectWarnings()
	emitBareErrorDiagnostics([]normalizer.Service{svc}, []normalizer.Endpoint{{Method: "POST", ServiceName: "tender", RPC: "CreateTender"}}, opts)
	if len(*got) != 1 || (*got)[0].File != "cue/api/tender_create.cue" || (*got)[0].Line != 12 {
		t.Fatalf("warning must fall back to the operation position, got %#v", *got)
	}
}
