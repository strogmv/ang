package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func authTestSchema() *ir.Schema {
	return &ir.Schema{
		Services: []ir.Service{{
			Name: "Shop",
			Methods: []ir.Method{
				{Name: "ListProducts", Flow: []ir.FlowStep{{Action: "repo.List"}}},
				{Name: "TrackOpen", Flow: []ir.FlowStep{{Action: "flow.If", Then: []ir.FlowStep{{Action: "repo.Save"}}}}},
				{Name: "OpenShare", Flow: []ir.FlowStep{{Action: "logic.Call", Args: map[string]any{"func": "(func(ctx context.Context) error { return s.ShareRepo.Save(ctx, x) })"}}}},
				{Name: "Checkout"},
				{Name: "Webhook"},
				{Name: "Signup"},
				{Name: "Me"},
				{Name: "Stream"},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/products", Service: "Shop", RPC: "ListProducts"},
			{Method: "GET", Path: "/t/{token}/pixel.gif", Service: "Shop", RPC: "TrackOpen", Source: "cue/api/shop.cue:10:2"},
			{Method: "GET", Path: "/share/{token}", Service: "Shop", RPC: "OpenShare"},
			{Method: "POST", Path: "/checkout", Service: "Shop", RPC: "Checkout", Source: "cue/api/shop.cue:20:2"},
			{Method: "POST", Path: "/webhook", Service: "Shop", RPC: "Webhook",
				Metadata: map[string]any{normalizer.EndpointMetaAuthPublic: true, normalizer.EndpointMetaAuthPublicReason: "signed by the provider"}},
			{Method: "POST", Path: "/signup", Service: "Shop", RPC: "Signup", Source: "cue/api/shop.cue:30:2",
				Metadata: map[string]any{normalizer.EndpointMetaAuthPublic: true}},
			{Method: "PUT", Path: "/me", Service: "Shop", RPC: "Me", Auth: &ir.EndpointAuth{Type: "jwt"}},
			{Method: "WS", Path: "/ws", Service: "Shop", RPC: "Stream"},
		},
	}
}

func warningCodes(ws []normalizer.Warning) map[string]string {
	out := map[string]string{}
	for _, w := range ws {
		out[w.Op] = w.Code
	}
	return out
}

func TestApplyAuthDefaultAllowReportsPublicMutations(t *testing.T) {
	t.Parallel()

	schema := authTestSchema()
	for _, mode := range []string{"", AuthDefaultAllow} {
		out, res, err := ApplyAuthDefault(schema, mode)
		if err != nil || res.Err() != nil {
			t.Fatalf("mode %q: %v %v", mode, err, res.Err())
		}
		if out != schema {
			t.Fatalf("mode %q must emit the schema as is", mode)
		}
		got := warningCodes(res.Warnings)
		want := map[string]string{
			"TrackOpen": CodeAuthPublicMutation, // GET whose flow saves
			"OpenShare": CodeAuthPublicMutation, // GET whose Go saves
			"Checkout":  CodeAuthPublicMutation, // POST
			"Signup":    CodeAuthPublicNoReason, // public on purpose, no reason
		}
		if len(got) != len(want) {
			t.Fatalf("mode %q: warnings %v, want %v", mode, got, want)
		}
		for op, code := range want {
			if got[op] != code {
				t.Fatalf("mode %q: %s got %q, want %q (all: %v)", mode, op, got[op], code, got)
			}
		}
	}
	for _, w := range func() []normalizer.Warning { _, r, _ := ApplyAuthDefault(schema, ""); return r.Warnings }() {
		if w.Op == "Checkout" && (w.Path != "cue/api/shop.cue:20:2" || !strings.Contains(w.Message, "POST /checkout")) {
			t.Fatalf("warning must name the endpoint and its CUE source: %+v", w)
		}
	}
}

func TestApplyAuthDefaultDenyRequiresACaller(t *testing.T) {
	t.Parallel()

	schema := authTestSchema()
	schema.Endpoints[5].Metadata[normalizer.EndpointMetaAuthPublicReason] = "anyone signs up"
	out, res, err := ApplyAuthDefault(schema, AuthDefaultDeny)
	if err != nil || res.Err() != nil {
		t.Fatalf("%v %v", err, res.Err())
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("deny reports nothing when every public endpoint says why: %v", res.Warnings)
	}
	byRPC := map[string]ir.Endpoint{}
	for _, ep := range out.Endpoints {
		byRPC[ep.RPC] = ep
	}
	for _, rpc := range []string{"ListProducts", "TrackOpen", "OpenShare", "Checkout", "Stream"} {
		if a := byRPC[rpc].Auth; a == nil || a.Type != "jwt" {
			t.Fatalf("%s: deny must require a caller, got %+v", rpc, a)
		}
	}
	for _, rpc := range []string{"Webhook", "Signup"} {
		if byRPC[rpc].Auth != nil {
			t.Fatalf("%s is public on purpose and stays so", rpc)
		}
	}
	for _, ep := range schema.Endpoints {
		if ep.RPC == "Checkout" && ep.Auth != nil {
			t.Fatal("deny must not change the schema it was given")
		}
	}
}

func TestApplyAuthDefaultDenyRefusesPublicMutationWithoutReason(t *testing.T) {
	t.Parallel()

	_, res, err := ApplyAuthDefault(authTestSchema(), AuthDefaultDeny)
	if err != nil {
		t.Fatal(err)
	}
	e := res.Err()
	if e == nil || !strings.Contains(e.Error(), CodeAuthPublicNoReason) || !strings.Contains(e.Error(), "Signup POST /signup (is a POST): cue/api/shop.cue:30:2") {
		t.Fatalf("want an error naming Signup and its source, got %v", e)
	}
	if _, _, err := ApplyAuthDefault(authTestSchema(), "closed"); err == nil || !strings.Contains(err.Error(), CodeAuthDefaultInvalid) {
		t.Fatalf("an unknown auth_default must fail, got %v", err)
	}
}

func TestCheckConfigLoadsNamesOperationsThatReloadConfig(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{Services: []ir.Service{{
		Name: "Mail",
		Methods: []ir.Method{
			{Name: "Send", Source: "cue/api/mail.cue:3:1", Impl: &ir.Impl{Code: "cfg, err := config.Load()\n_ = cfg"}},
			{Name: "Notify", Flow: []ir.FlowStep{{Action: "flow.If", Then: []ir.FlowStep{{Action: "logic.Call", Args: map[string]any{"func": "if c, e := config.Load(); e == nil { _ = c }"}}}}}},
			{Name: "Quiet", Impl: &ir.Impl{Code: "cfg := s.Cfg\n_ = cfg"}},
		},
	}}}
	ws := CheckConfigLoads(schema)
	if len(ws) != 2 {
		t.Fatalf("want Send and Notify, got %+v", ws)
	}
	if ws[0].Code != CodeConfigLoadInOperation || ws[0].Op != "Send" || ws[0].Path != "cue/api/mail.cue:3:1" || ws[1].Op != "Notify" {
		t.Fatalf("got %+v", ws)
	}
}

// A POST marked read: true is no mutation unless its operation writes; one
// that writes is reported (backlog 49).
func TestReadEndpointsDoNotMutateUnlessTheyWrite(t *testing.T) {
	t.Parallel()
	schema := &ir.Schema{
		Services: []ir.Service{{
			Name: "Catalog",
			Methods: []ir.Method{
				{Name: "SearchProducts", Flow: []ir.FlowStep{{Action: "repo.List"}}},
				{Name: "PreviewImport", Flow: []ir.FlowStep{{Action: "repo.Save"}}},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/search", Service: "Catalog", RPC: "SearchProducts", Read: true},
			{Method: "POST", Path: "/preview", Service: "Catalog", RPC: "PreviewImport", Read: true, Source: "cue/api/catalog.cue:9:2"},
		},
	}
	methods := methodIndex(schema)
	if mutates, why := EndpointMutates(schema.Endpoints[0], methods["Catalog.SearchProducts"]); mutates {
		t.Fatalf("a read POST that only lists does not mutate (%s)", why)
	}
	if mutates, _ := EndpointMutates(schema.Endpoints[1], methods["Catalog.PreviewImport"]); !mutates {
		t.Fatal("a read POST whose flow saves mutates")
	}
	_, res, err := ApplyAuthDefault(schema, AuthDefaultAllow)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string][]string{}
	for _, w := range res.Warnings {
		codes[w.Op] = append(codes[w.Op], w.Code)
	}
	if strings.Join(codes["PreviewImport"], ",") != CodeReadEndpointWrites+","+CodeAuthPublicMutation {
		t.Fatalf("PreviewImport warnings = %v", codes["PreviewImport"])
	}
	if len(codes["SearchProducts"]) != 0 {
		t.Fatalf("a public read POST is no public mutation: %v", codes["SearchProducts"])
	}
}
