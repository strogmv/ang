package compiler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEntityIntegrity_PartialSelectProducesErrorDiagnostic(t *testing.T) {
	var got []normalizer.Warning
	opts := PipelineOptions{
		WarningSink: func(w normalizer.Warning) { got = append(got, w) },
	}

	entityFieldMap := map[string]map[string]string{
		"User": {
			"id":    "string",
			"email": "string",
		},
	}
	finders := []normalizer.RepositoryFinder{
		{
			Name:   "FindByEmail",
			Returns: "one",
			Select: []string{"id"},
			Source: "cue/repo/repositories.cue:12",
		},
	}

	emitSelectProjectionDiagnostics("User", finders, entityFieldMap, opts)
	if len(got) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(got))
	}
	if got[0].Code != "ENTITY_PARTIAL_SELECT_ERROR" {
		t.Fatalf("unexpected code: %s", got[0].Code)
	}
	if strings.ToUpper(got[0].Severity) != "ERROR" {
		t.Fatalf("expected ERROR severity, got %s", got[0].Severity)
	}
}

func TestProjectionRules_ImplicitProjectionDedupBySortedFields(t *testing.T) {
	entity := normalizer.Entity{
		Name: "User",
		Fields: []normalizer.Field{
			{Name: "ID", Type: "string"},
			{Name: "Email", Type: "string"},
			{Name: "Bio", Type: "string"},
		},
	}
	entityByName := map[string]normalizer.Entity{"User": entity}

	finders := []normalizer.RepositoryFinder{
		{Name: "A", Returns: "many", Select: []string{"id", "email"}},
		{Name: "B", Returns: "many", Select: []string{"email", "id"}},
	}
	projNameByKey := map[string]string{}
	outFinders, projections := synthesizeImplicitProjections("User", finders, entityByName, projNameByKey)

	if len(projections) != 1 {
		t.Fatalf("expected one deduplicated projection, got %d", len(projections))
	}
	if outFinders[0].ReturnType == "" || outFinders[1].ReturnType == "" {
		t.Fatalf("expected finder return types to be rewritten to projection")
	}
	if outFinders[0].ReturnType != outFinders[1].ReturnType {
		t.Fatalf("expected deduplicated return type, got %s vs %s", outFinders[0].ReturnType, outFinders[1].ReturnType)
	}
}

func TestProjectionRules_DeterministicSynthesis(t *testing.T) {
	entity := normalizer.Entity{
		Name: "User",
		Fields: []normalizer.Field{
			{Name: "ID", Type: "string"},
			{Name: "Email", Type: "string"},
			{Name: "Bio", Type: "string"},
		},
	}
	entityByName := map[string]normalizer.Entity{"User": entity}
	finders := []normalizer.RepositoryFinder{
		{Name: "A", Returns: "many", Select: []string{"id", "email"}},
		{Name: "B", Returns: "one", Select: []string{"email", "id"}},
	}

	runs := make([][]normalizer.RepositoryFinder, 0, 2)
	for i := 0; i < 2; i++ {
		in := append([]normalizer.RepositoryFinder(nil), finders...)
		out, _ := synthesizeImplicitProjections("User", in, entityByName, map[string]string{})
		runs = append(runs, out)
	}
	if !reflect.DeepEqual(runs[0], runs[1]) {
		t.Fatalf("projection synthesis must be deterministic across runs")
	}
}

