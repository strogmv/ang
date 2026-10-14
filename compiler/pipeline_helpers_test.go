package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
)

const helperDomain = `package domain

Ticket: {
	id:    string
	title: string
}
`

// The helpers (one per kind) and three operations in two services that call
// them. No operation lists Ticket in sources: the service reaches
// s.TicketRepo only through the method helper.
const helperAPI = `package api

Helpers: {
	ticketLabel: {
		code: """
			// ticketLabel is how a ticket is named in messages.
			func ticketLabel(id string, row labelRow) string {
				return strings.ToUpper(id) + ": " + row.Title
			}
			"""
	}
	labelRow: {
		code: """
			type labelRow struct {
				Title string
			}
			"""
	}
	closeTicket: {
		code: """
			func (s *Service) closeTicket(ctx context.Context, id string) (string, error) {
				find := s.findTicket // a method helper as a value
				t, err := find(ctx, id)
				if err != nil || t == nil {
					return "", err
				}
				return ticketLabel(t.ID, labelRow{Title: t.Title}), s.TicketRepo.Delete(ctx, id)
			}
			"""
	}
	findTicket: {
		code: """
			func (s *Service) findTicket(ctx context.Context, id string) (*domain.Ticket, error) {
				return s.TicketRepo.FindByID(ctx, id)
			}
			"""
	}
}

LabelTicket: {
	service: "stats"
	input: {id: string}
	output: {label: string}
	impls: go: {
		code: """
			resp.Label = ticketLabel(req.ID, labelRow{Title: "x"})
			return resp, nil
			"""
	}
}

CloseTicket: {
	service: "stats"
	input: {id: string}
	output: {label: string}
	flow: [
		{action: "logic.Call", func: "s.closeTicket", args: ["ctx", "req.ID"], output: "label"},
		{action: "mapping.Assign", to: "resp.Label", value: "label"},
	]
}

ArchiveTicket: {
	service: "desk"
	input: {id: string}
	output: {label: string}
	impls: go: {
		code: """
			label, err := s.closeTicket(ctx, req.ID)
			resp.Label = label
			return resp, err
			"""
	}
}
`

func emitHelperProject(t *testing.T, files map[string]string) (string, compiler.NormalizePhaseOutput) {
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
	compiled, err := compiler.CompileForEmit(basePath, compiler.PipelineOptions{}, compiler.CompileForEmitOptions{})
	if err != nil {
		t.Fatalf("CompileForEmit: %v", err)
	}
	outDir := t.TempDir()
	em := emitter.New(outDir, filepath.Join(outDir, "frontend"), filepath.Join("templates"))
	em.GoModule = "example.com/test"
	if err := em.EmitServiceFromIR(compiled.IR); err != nil {
		t.Fatalf("EmitServiceFromIR: %v", err)
	}
	if err := em.EmitServiceImplFromIR(compiled.IR, nil); err != nil {
		t.Fatalf("EmitServiceImplFromIR: %v", err)
	}
	return outDir, compiled.Normalized
}

// Item 37 of docs/tech-debt-backlog.md: Go a project wrote once in CUE and
// interpolated into every operation (\(#XFunc)) was emitted once per
// operation. A helper is emitted once: a func/type to its own file, a method
// helper once per service that reaches it; operations only call it.
func TestPipelineEmitsSharedHelpersOnce(t *testing.T) {
	outDir, normalized := emitHelperProject(t, map[string]string{
		"domain/ticket.cue": helperDomain,
		"api/ticket.cue":    helperAPI,
	})

	if len(normalized.Helpers) != 4 {
		t.Fatalf("helpers = %+v", normalized.Helpers)
	}
	kinds := map[string]string{}
	for _, h := range normalized.Helpers {
		kinds[h.Name] = h.Kind
	}
	if kinds["ticketLabel"] != normalizer.HelperFunc || kinds["labelRow"] != normalizer.HelperType || kinds["closeTicket"] != normalizer.HelperMethod {
		t.Fatalf("kinds = %v", kinds)
	}
	for _, svc := range normalized.Services {
		for _, m := range svc.Methods {
			got := strings.Join(m.Helpers, ",")
			want := map[string]string{
				"LabelTicket":   "labelRow,ticketLabel",
				"CloseTicket":   "closeTicket,findTicket,labelRow,ticketLabel",
				"ArchiveTicket": "closeTicket,findTicket,labelRow,ticketLabel",
			}[m.Name]
			if got != want {
				t.Errorf("%s.%s reaches helpers %q, want %q", svc.Name, m.Name, got, want)
			}
		}
	}

	dir := filepath.Join(outDir, "internal", "service")
	label := readParsedGo(t, filepath.Join(dir, "helper__ticket_label.gen.go"))
	if strings.Count(label, "func ticketLabel(") != 1 {
		t.Errorf("helper__ticket_label.gen.go:\n%s", label)
	}
	if !strings.Contains(label, "//line ") || !strings.Contains(label, "ticket.cue:7") {
		t.Errorf("ticketLabel is not mapped to its CUE line (ticket.cue:7):\n%s", label)
	}
	if !strings.Contains(label, "// ticketLabel is how a ticket is named") {
		t.Errorf("the helper's comment is lost:\n%s", label)
	}
	row := readParsedGo(t, filepath.Join(dir, "helper__label_row.gen.go"))
	if !strings.Contains(row, "type labelRow struct") {
		t.Errorf("helper__label_row.gen.go:\n%s", row)
	}
	closeFile := readParsedGo(t, filepath.Join(dir, "helper__close_ticket.gen.go"))
	for _, want := range []string{"func (s *DeskImpl) closeTicket(", "func (s *StatsImpl) closeTicket("} {
		if strings.Count(closeFile, want) != 1 {
			t.Errorf("helper__close_ticket.gen.go lacks one %q:\n%s", want, closeFile)
		}
	}
	// Reached only as a method value from closeTicket, still once per service.
	findFile := readParsedGo(t, filepath.Join(dir, "helper__find_ticket.gen.go"))
	for _, want := range []string{"func (s *DeskImpl) findTicket(", "func (s *StatsImpl) findTicket("} {
		if strings.Count(findFile, want) != 1 {
			t.Errorf("helper__find_ticket.gen.go lacks one %q:\n%s", want, findFile)
		}
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	sawImpl := 0
	for _, path := range matches {
		name := filepath.Base(path)
		text := readParsedGo(t, path)
		if strings.HasPrefix(name, "helper__") {
			continue
		}
		if strings.Contains(text, "func ticketLabel(") || strings.Contains(text, "closeTicket(ctx context.Context") {
			t.Errorf("%s carries a helper's body:\n%s", name, text)
		}
		for _, svc := range []string{"StatsImpl", "DeskImpl"} {
			if strings.Contains(text, "type "+svc+" struct") {
				sawImpl++
				if !strings.Contains(text, "TicketRepo port.TicketRepository") {
					t.Errorf("%s lacks TicketRepo, which its operations reach through closeTicket:\n%s", svc, text)
				}
			}
		}
	}
	if sawImpl != 2 {
		t.Fatalf("found %d of the two service types among %v", sawImpl, matches)
	}
}

// A helper declaration that cannot work fails validate with file:line.
func TestSemanticPhasesRejectInvalidHelpers(t *testing.T) {
	const api = `package api

Helpers: {
	wrongName: {
		code: """
			func otherName() int { return 1 }
			"""
	}
	readsService: {
		code: """
			func readsService() bool {
				return s.TicketRepo != nil
			}
			"""
	}
	capturesLocal: {
		code: """
			func capturesLocal() string {
				return ticketLabl + req.ID
			}
			"""
	}
	ticketLabel: {
		code: """
			func ticketLabel() string { return "t" }
			"""
	}
	callsMethodBare: {
		code: """
			func callsMethodBare() string {
				return closeTicket()
			}
			"""
	}
	closeTicket: {
		code: """
			func (s *Service) closeTicket() string {
				return s.archiveTicket()
			}
			"""
	}
	fooBar: {code: "const fooBar = 1"}
	foo_bar: {code: "const foo_bar = 2"}
}

Noop: {
	service: "stats"
	input: {id: string}
	output: {ok: bool}
	impls: go: {
		code: """
			_ = ticketLabel()
			_ = fooBar + foo_bar
			_ = callsMethodBare()
			_ = s.closeTicket()
			return resp, nil
			"""
	}
}
`
	base := copyMinimalProjectForHelpers(t, api)
	_, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	if err == nil {
		t.Fatal("expected helper errors, got none")
	}
	var ce *compiler.ContractError
	if !asContractErrorForHelpers(err, &ce) || ce.Code != compiler.ErrCodeHelperInvalid {
		t.Fatalf("expected ContractError %s, got %#v", compiler.ErrCodeHelperInvalid, err)
	}
	msg := err.Error()
	for _, want := range []string{
		"helper wrongName: the code declares otherName",
		"helpers.cue:12: helper readsService: uses s, the service; declare it as a method: func (s *Service) readsService(…)",
		"helpers.cue:19: helper capturesLocal: ticketLabl is not declared: no helper, package or predeclared name",
		"did you mean ticketLabel?",
		"helper capturesLocal: req is not declared: it is an operation's variable",
		"helper callsMethodBare: closeTicket is a method helper; call it as s.closeTicket",
		"helper closeTicket: s.archiveTicket is no method helper",
		"helper foo_bar: duplicates helper fooBar (both would be helper__foo_bar.gen.go)",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
}

// An operation calls a method helper as s.name(…); the bare name is a
// mistake caught before the Go compiler says "undefined". A helper no
// operation reaches is a warning.
func TestSemanticPhasesCheckHowOperationsCallHelpers(t *testing.T) {
	const api = `package api

Helpers: {
	closeTicket: {
		code: """
			func (s *Service) closeTicket() string { return "closed" }
			"""
	}
	neverCalled: {code: "const neverCalled = 1"}
}

Noop: {
	service: "stats"
	input: {id: string}
	output: {ok: bool}
	impls: go: {
		code: """
			_ = closeTicket()
			return resp, nil
			"""
	}
}
`
	base := copyMinimalProjectForHelpers(t, api)
	_, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	if err == nil || !strings.Contains(err.Error(), "operation Stats.Noop calls closeTicket(…), a method helper; write s.closeTicket(…)") {
		t.Fatalf("expected the bare method call to fail, got %v", err)
	}

	fixed := strings.Replace(api, "_ = closeTicket()", "_ = s.closeTicket()", 1)
	base = copyMinimalProjectForHelpers(t, fixed)
	var warnings []normalizer.Warning
	if _, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{
		WarningSink: func(w normalizer.Warning) { warnings = append(warnings, w) },
	}); err != nil {
		t.Fatalf("RunSemanticPhases: %v", err)
	}
	unused := 0
	for _, w := range warnings {
		if w.Code == "HELPER_UNUSED" {
			unused++
			if !strings.Contains(w.Message, "helper neverCalled is not called by any operation") || !strings.HasSuffix(w.File, "helpers.cue") {
				t.Errorf("warning = %+v", w)
			}
		}
	}
	if unused != 1 {
		t.Fatalf("HELPER_UNUSED warnings = %d, want 1: %+v", unused, warnings)
	}
}

// A helper's imports are checked like a logic.Call's: an unused entry and an
// ambiguous standard name (rand.Read) fail with the helper's CUE line.
func TestSemanticPhasesCheckHelperImports(t *testing.T) {
	const api = `package api

Helpers: {
	token: {
		imports: ["encoding/hex"]
		code: """
			func token() string {
				buf := make([]byte, 8)
				_, _ = rand.Read(buf)
				return string(buf)
			}
			"""
	}
}

Noop: {
	service: "stats"
	input: {id: string}
	output: {ok: bool}
	impls: go: {
		code: """
			_ = token()
			return resp, nil
			"""
	}
}
`
	base := copyMinimalProjectForHelpers(t, api)
	_, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	var ce *compiler.ContractError
	if err == nil || !asContractErrorForHelpers(err, &ce) || ce.Code != compiler.ErrCodeRawGoImport {
		t.Fatalf("expected %s, got %v", compiler.ErrCodeRawGoImport, err)
	}
	for _, want := range []string{
		`helper token: import "encoding/hex" is not used by the code`,
		`helpers.cue:9: helper token: rand.Read exists in "crypto/rand" and "math/rand"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}

	fixed := strings.Replace(api, `imports: ["encoding/hex"]`, `imports: ["crypto/rand"]`, 1)
	base = copyMinimalProjectForHelpers(t, fixed)
	if _, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{}); err != nil {
		t.Fatalf("declared imports must pass: %v", err)
	}
}

func copyMinimalProjectForHelpers(t *testing.T, api string) string {
	t.Helper()
	base := t.TempDir()
	for rel, text := range map[string]string{
		"cue/domain/ticket.cue": helperDomain,
		"cue/api/helpers.cue":   api,
	} {
		path := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func asContractErrorForHelpers(err error, target **compiler.ContractError) bool {
	for err != nil {
		if ce, ok := err.(*compiler.ContractError); ok {
			*target = ce
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
