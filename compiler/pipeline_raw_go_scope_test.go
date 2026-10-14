package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// Item 11 of docs/tech-debt-backlog.md: raw Go may declare any name. The
// names the template used to reserve (err :=, var err error, resp :=,
// var resp, a variable l) validate without a diagnostic; only the two places
// where shadowing the results changes meaning, and a logic.Call func that
// sets the method's err, are warnings with a hint.
func TestSemanticPhasesWarnOnlyWhereRawGoLeansOnTemplateNames(t *testing.T) {
	const op = `package api

RenameUser: {
	service: "user"
	input: {id: string, name: string}
	output: {ok: bool}
	impls: go: {
		lang: "go"
		code: """
			var err error
			resp := port.RenameUserResponse{}
			l := struct{ Name string }{Name: req.Name}
			if l.Name == "" {
				var resp port.RenameUserResponse
				return resp, err
			}
			resp.Ok = true
			return resp, nil
			"""
	}
}

DeleteUser: {
	service: "user"
	input: {id: string}
	output: {ok: bool}
	impls: go: {
		lang: "go"
		code: """
			err := s.check(req.ID)
			defer func() {
				if err != nil {
					resp.Ok = false
				}
			}()
			if err != nil {
				return
			}
			resp.Ok = true
			return resp, nil
			"""
	}
}

TokenForUser: {
	service: "user"
	input: {id: string}
	output: {token: string}
	flow: [
		{
			action: "logic.Call"
			func: """
				(func() (string, error) {
					var token string
					token, err = s.issue(req.ID)
					return token, err
				})
				"""
			output: "token"
		},
		{
			action: "logic.Call"
			func: """
				(func() (string, error) {
					token, err := s.issue(req.ID)
					return token, err
				})
				"""
			output: "again"
		},
		{action: "mapping.Assign", to: "resp.Token", value: "token + again"},
	]
}
`
	base := copyMinimalProject(t)
	if err := os.WriteFile(filepath.Join(base, "cue", "api", "scope.cue"), []byte(op), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(w normalizer.Warning) {
		if w.Kind == "impl" || w.Code == CodeLogicCallOuterErr || strings.HasPrefix(w.Code, "IMPL_") {
			got = append(got, filepath.Base(w.File)+":"+strconv.Itoa(w.Line)+" "+w.Code+" "+w.Severity)
		}
	}})
	if err != nil {
		t.Fatalf("raw Go that declares err, resp and l validates: %v", err)
	}
	sort.Strings(got)
	want := []string{
		"scope.cue:32 IMPL_RESULT_SHADOWED warn", // the deferred func reads the code's own err
		"scope.cue:37 IMPL_RESULT_SHADOWED warn", // naked return under the code's own err
		"scope.cue:55 LOGIC_CALL_OUTER_ERR warn", // token, err = … with no err of its own
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
