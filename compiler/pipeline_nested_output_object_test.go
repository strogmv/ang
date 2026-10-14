package compiler_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
)

// emitFixtureProject runs the pipeline over a project made of the given CUE
// files (path relative to cue/ → text) and emits its ports and service
// implementations; it answers the output directory.
func emitFixtureProject(t *testing.T, files map[string]string) string {
	t.Helper()
	basePath := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(basePath, "cue", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entities, services, endpoints, repos, events, bizErrors, schedules, _, scopes, err := compiler.RunPipeline(basePath)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	schema, err := compiler.ConvertAndTransform(
		entities, services, events, bizErrors, endpoints, scopes, repos,
		normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{},
	)
	if err != nil {
		t.Fatalf("ConvertAndTransform: %v", err)
	}
	outDir := t.TempDir()
	em := emitter.New(outDir, filepath.Join(outDir, "frontend"), filepath.Join("templates"))
	em.GoModule = "example.com/test"
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatalf("EmitServiceFromIR: %v", err)
	}
	if err := em.EmitServiceImplFromIR(schema, nil); err != nil {
		t.Fatalf("EmitServiceImplFromIR: %v", err)
	}
	return outDir
}

func readParsedGo(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), path, b, 0); err != nil {
		t.Fatalf("%s does not parse: %v\n%s", path, err, b)
	}
	return string(b)
}

// An inline object in an operation output that the operation's code reaches
// into (resp.Interest.Signs30d) is a named Go struct, at any depth; an
// object the code never reaches into stays loose (any), as flows fill those
// from map literals and domain JSON.
func TestPipelineTypesOutputObjectsTheCodeReachesInto(t *testing.T) {
	outDir := emitFixtureProject(t, map[string]string{
		"api/stats.cue": `package api

GetInterest: {
	service: "stats"
	input: {companyId: string}
	output: {
		interest: {
			signs30d: int
			window: {days: int}
		}
		loose: {note: string}
		pages: [...{
			settings: {remind: bool}
			extra: {flag: bool}
		}]
	}
	impls: go: {
		code: """
			resp.Interest.Signs30d = 3
			resp.Interest.Window.Days = 30
			resp.Loose = map[string]any{"note": "x"}
			page := port.GetInterestResponsePagesItem{}
			page.Settings.Remind = true
			resp.Pages = append(resp.Pages, page)
			return resp, nil
			"""
	}
}

GetCounts: {
	service: "stats"
	input: {
		companyId: string
		message?: {chat?: {id?: int}}
	}
	output: {
		counts: {open: int}
		rows: [...{badge: {label: string}}]
	}
	flow: [
		{action: "mapping.Assign", to: "resp.Counts.Open", value: "1"},
		{
			action: "logic.Call"
			func: """
				(func(req port.GetCountsRequest, resp *port.GetCountsResponse) {
					var msg struct{ Chat struct{ ID int } }
					_ = msg.Chat.ID
					for _, row := range resp.Rows {
						_ = row.Badge.Label
					}
				})
				"""
			args: ["req", "&resp"]
		},
	]
}
`,
	})
	port := readParsedGo(t, filepath.Join(outDir, "internal", "port", "stats.go"))
	for _, want := range []string{
		"Interest GetInterestResponseInterest",
		"type GetInterestResponseInterest struct",
		"Signs30d int",
		"Window   GetInterestResponseInterestWindow",
		"type GetInterestResponseInterestWindow struct",
		"Loose    any",
		"type GetInterestResponsePagesItem struct",
		"Settings GetInterestResponsePagesItemSettings",
		"type GetInterestResponsePagesItemSettings struct",
		"Extra    any",
		"Counts GetCountsResponseCounts",
		"type GetCountsResponseCounts struct",
		"Message   any",
		"Badge GetCountsResponseRowsItemBadge",
	} {
		if !strings.Contains(port, want) {
			t.Errorf("port lacks %q:\n%s", want, port)
		}
	}
	// msg.Chat.ID selects a local struct, not req.Message.
	for _, unwanted := range []string{"type GetInterestResponseLoose struct", "type GetInterestResponsePagesItemExtra struct", "type GetCountsRequestMessage struct"} {
		if strings.Contains(port, unwanted) {
			t.Errorf("port declares a struct for an object no code reaches into: %s", unwanted)
		}
	}
}
