package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

const finderParamsDomain = `package domain

import "time"

Event: {
	id:        string    @db(type="UUID", primary_key=true)
	companyId: string    @db(type="UUID")
	day:       time.Time @db(type="TIMESTAMPTZ")
	title:     string    @db(type="TEXT")
}
`

func writeFinderParamsProject(t *testing.T, repoCUE string) string {
	t.Helper()
	base := copyMinimalProject(t)
	if err := os.WriteFile(filepath.Join(base, "cue", "domain", "event.cue"), []byte(finderParamsDomain), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "cue", "repo", "events.cue"), []byte(repoCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	return base
}

func eventFinder(t *testing.T, out NormalizePhaseOutput, name string) normalizer.RepositoryFinder {
	t.Helper()
	for _, r := range out.Repos {
		if r.Entity != "Event" {
			continue
		}
		for _, f := range r.Finders {
			if f.Name == name {
				return f
			}
		}
	}
	t.Fatalf("finder Event.%s not found", name)
	return normalizer.RepositoryFinder{}
}

// Item 14 of docs/tech-debt-backlog.md: an explicit param_type stays as
// written; only an unwritten one takes the type of the entity field. A
// time-typed field used to turn param_type: "string" into time.Time.
func TestSemanticPhasesKeepExplicitWhereParamType(t *testing.T) {
	base := writeFinderParamsProject(t, `package repo

Repositories: {
	Event: {
		finders: [{
			name:    "ByDayText"
			returns: "many"
			where: [
				{field: "day", op: "=", param: "day", param_type: "string"},
				{field: "day", op: ">", param: "after"},
			]
			sql: "SELECT id, companyid, day, title FROM events WHERE day::date = $1::date AND day > $2"
		}]
	}
}
`)
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f := eventFinder(t, out, "ByDayText")
	if got := f.Where[0].ParamType; got != "string" {
		t.Errorf("explicit param_type: \"string\" became %q", got)
	}
	if got := f.Where[1].ParamType; got != "time.Time" {
		t.Errorf("unwritten param_type took %q, want the field's time.Time", got)
	}
}

func TestSemanticPhasesReadFinderParamsAndPaged(t *testing.T) {
	base := writeFinderParamsProject(t, `package repo

Repositories: {
	Event: {
		finders: [{
			name:    "Page"
			returns: "many"
			paged:   true
			select:  "auto"
			params: [{name: "companyID", type: "uuid"}, {name: "ids", type: "[]uuid"}, {name: "from", type: "time"}, {name: "score", type: "float"}]
			sql: "SELECT * FROM events WHERE companyid = $1 AND id = ANY($2) AND day >= $3 AND $4 > 0 LIMIT $5 OFFSET $6"
		}]
	}
}
`)
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f := eventFinder(t, out, "Page")
	want := []normalizer.FinderParam{
		{Name: "companyID", Type: "string", Spec: "uuid"},
		{Name: "ids", Type: "[]string", Spec: "[]uuid"},
		{Name: "from", Type: "time.Time", Spec: "time"},
		{Name: "score", Type: "float64", Spec: "float"},
	}
	if len(f.Params) != len(want) {
		t.Fatalf("params = %+v", f.Params)
	}
	for i := range want {
		if f.Params[i] != want[i] {
			t.Errorf("params[%d] = %+v, want %+v", i, f.Params[i], want[i])
		}
	}
	if f.Paged == nil || !*f.Paged || !normalizer.FinderPaged(f) {
		t.Errorf("paged: true not read: %v", f.Paged)
	}
	if !f.SelectAuto || len(f.Select) != 0 {
		t.Errorf("select: \"auto\" not read: auto=%v select=%v", f.SelectAuto, f.Select)
	}
}

func TestSemanticPhasesRejectInvalidFinderKeys(t *testing.T) {
	cases := []struct {
		name   string
		finder string
		want   string
	}{
		{"unknown type", `{name: "A", returns: "many", params: [{name: "x", type: "uuid[]"}], sql: "SELECT 1"}`, `unknown type "uuid[]"; use one of`},
		{"params and where", `{name: "A", returns: "many", where: [{field: "title", op: "="}], params: [{name: "x", type: "string"}], sql: "SELECT 1"}`, "params: and where: both declare parameters"},
		{"params without sql", `{name: "A", returns: "many", params: [{name: "x", type: "string"}]}`, "params: is for sql: finders"},
		{"bad name", `{name: "A", returns: "many", params: [{name: "type", type: "string"}], sql: "SELECT 1"}`, `name "type" is not a Go identifier`},
		{"duplicate name", `{name: "A", returns: "many", params: [{name: "x", type: "string"}, {name: "x", type: "int"}], sql: "SELECT 1"}`, `name "x" is declared twice`},
		{"paged with a limit param", `{name: "A", returns: "many", paged: true, params: [{name: "limit", type: "int"}], sql: "SELECT 1"}`, `params: must not declare "limit"`},
		{"select auto without sql", `{name: "A", returns: "many", select: "auto"}`, `select: "auto" is for sql: finders`},
		{"upsert without on_conflict", `{name: "A", kind: "upsert"}`, `kind: "upsert" needs on_conflict`},
		{"on_conflict without upsert", `{name: "A", returns: "many", on_conflict: ["title"]}`, `on_conflict: needs kind: "upsert"`},
		{"upsert with sql", `{name: "A", kind: "upsert", on_conflict: ["title"], sql: "SELECT 1"}`, `it takes no sql:`},
		{"unknown kind", `{name: "A", kind: "merge"}`, `kind: "merge" is unknown`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := writeFinderParamsProject(t, "package repo\n\nRepositories: {\n\tEvent: {\n\t\tfinders: [\n\t\t\t"+tc.finder+",\n\t\t]\n\t}\n}\n")
			_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
			if err == nil {
				t.Fatalf("expected an error")
			}
			msg := err.Error()
			for _, want := range []string{tc.want, "events.cue:6:", `finder "A"`} {
				if !strings.Contains(msg, want) {
					t.Errorf("error does not mention %q:\n%s", want, msg)
				}
			}
			var ce *ContractError
			if !asContractError(err, &ce) || ce.Code != ErrCodeCUERepoNormalize {
				t.Errorf("expected ContractError %s, got %#v", ErrCodeCUERepoNormalize, err)
			}
		})
	}
}

// Item 15: the legacy rule (limit: > 0 or "paginate" in the name adds
// limit/offset) still works but says so; paged: makes it explicit.
func TestSemanticPhasesWarnAboutImplicitPaging(t *testing.T) {
	base := writeFinderParamsProject(t, `package repo

Repositories: {
	Event: {
		finders: [
			{name: "Recent", returns: "many", limit: 50, order_by: "day DESC"},
			{name: "PaginateByCompany", returns: "many", where: [{field: "companyId", op: "="}]},
			{name: "Explicit", returns: "many", paged: true},
			{name: "PaginateNot", returns: "many", paged: false},
		]
	}
}
`)
	var diags []normalizer.Warning
	_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(w normalizer.Warning) {
		if w.Code == "REPO_FINDER_PAGED_IMPLICIT" {
			diags = append(diags, w)
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 2 {
		t.Fatalf("want 2 REPO_FINDER_PAGED_IMPLICIT warnings, got %+v", diags)
	}
	if !strings.Contains(diags[0].Message, "limit: 50 does not limit the query to 50 rows") || !strings.HasSuffix(diags[0].File, "events.cue") || diags[0].Line != 6 {
		t.Errorf("first warning = %+v", diags[0])
	}
	if !strings.Contains(diags[1].Message, "Event.PaginateByCompany") {
		t.Errorf("second warning = %+v", diags[1])
	}
}

func TestRenderFinderSignatureShowsParamsPagingAndUpsert(t *testing.T) {
	paged := true
	got := RenderFinderSignature("Event", normalizer.RepositoryFinder{
		Name:       "Page",
		ReturnType: "[]domain.Event",
		Params:     []normalizer.FinderParam{{Name: "ids", Type: "[]string"}},
		Paged:      &paged,
	})
	if want := "Page(ctx context.Context, ids []string, limit int, offset int) ([]domain.Event, error)"; got != want {
		t.Errorf("signature = %q, want %q", got, want)
	}
	got = RenderFinderSignature("Event", normalizer.RepositoryFinder{Name: "UpsertByDay", Kind: "upsert"})
	if want := "UpsertByDay(ctx context.Context, entities []*domain.Event) error"; got != want {
		t.Errorf("signature = %q, want %q", got, want)
	}
}

// Item 18: transactional: is read per operation; a value that is not a
// bool, or a streaming operation, fails with the file:line.
func TestSemanticPhasesReadTransactional(t *testing.T) {
	op := func(extra string) string {
		return "package api\n\nMoveUser: {\n\tservice: \"user\"\n\t" + extra + "\n\tinput: {id: string}\n\toutput: {ok: bool}\n\tflow: [{action: \"mapping.Assign\", to: \"resp.Ok\", value: \"true\"}]\n}\n"
	}
	run := func(extra string) (NormalizePhaseOutput, error) {
		base := copyMinimalProject(t)
		if err := os.WriteFile(filepath.Join(base, "cue", "api", "move.cue"), []byte(op(extra)), 0o644); err != nil {
			t.Fatal(err)
		}
		return RunSemanticPhasesWithOptions(base, PipelineOptions{})
	}
	out, err := run("transactional: true")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, svc := range out.Services {
		for _, m := range svc.Methods {
			if m.Name == "MoveUser" {
				found = m.Transactional
			}
		}
	}
	if !found {
		t.Fatal("transactional: true not read")
	}
	for extra, want := range map[string]string{
		`transactional: "yes"`:                "transactional: must be true or false",
		"stream: true\n\ttransactional: true": "transactional: true does not apply to a streaming operation",
	} {
		_, err := run(extra)
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "move.cue:") {
			t.Errorf("%s: error = %v, want %q with move.cue:line", extra, err, want)
		}
	}
}
