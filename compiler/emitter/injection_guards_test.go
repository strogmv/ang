package emitter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestGetRepoEntitiesSkipsDTOEntities(t *testing.T) {
	em := &Emitter{}
	fn, ok := em.getSharedFuncMap()["getRepoEntities"].(func(normalizer.Service, []normalizer.Entity) []string)
	if !ok {
		t.Fatalf("getRepoEntities has unexpected function signature")
	}

	service := normalizer.Service{
		Name: "Tender",
		Methods: []normalizer.Method{
			{
				Sources: []normalizer.Source{
					{Entity: "TenderParticipantReport"},
					{Entity: "Tender"},
				},
				Flow: []normalizer.FlowStep{
					{Action: "repo.Find", Args: map[string]any{"source": "TenderBidHistoryItem"}},
					{Action: "repo.List", Args: map[string]any{"source": "Tender"}},
				},
			},
		},
	}
	entities := []normalizer.Entity{
		{Name: "Tender", Metadata: map[string]any{}},
		{Name: "TenderParticipantReport", Metadata: map[string]any{"dto": true}},
		{Name: "TenderBidHistoryItem", Metadata: map[string]any{"dto": true}},
	}

	got := fn(service, entities)
	want := []string{"Tender"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getRepoEntities mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestHasRepoEntitiesExcludesDTOAndMongoOnly(t *testing.T) {
	em := &Emitter{}
	fn, ok := em.getAppFuncMap()["HasRepoEntities"].(func([]normalizer.Service, []normalizer.Entity) bool)
	if !ok {
		t.Fatalf("HasRepoEntities has unexpected function signature")
	}

	services := []normalizer.Service{
		{
			Name: "Tender",
			Methods: []normalizer.Method{
				{
					Sources: []normalizer.Source{
						{Entity: "TenderParticipantReport"},
						{Entity: "MongoAudit"},
					},
				},
			},
		},
	}
	entities := []normalizer.Entity{
		{Name: "TenderParticipantReport", Metadata: map[string]any{"dto": true}},
		{Name: "MongoAudit", Metadata: map[string]any{"storage": "mongo"}},
	}

	if fn(services, entities) {
		t.Fatalf("expected HasRepoEntities=false for DTO+mongo-only sources")
	}

	services[0].Methods[0].Sources = append(services[0].Methods[0].Sources, normalizer.Source{Entity: "Tender"})
	entities = append(entities, normalizer.Entity{Name: "Tender", Metadata: map[string]any{}})
	if !fn(services, entities) {
		t.Fatalf("expected HasRepoEntities=true when postgres-compatible entity is present")
	}
}

func TestEmitSystemRepositoryUsesErrorsIsForNoRows(t *testing.T) {
	tmp := t.TempDir()
	em := &Emitter{
		OutputDir: tmp,
		GoModule:  "github.com/example/project",
	}
	if err := em.EmitSystemRepository(); err != nil {
		t.Fatalf("EmitSystemRepository failed: %v", err)
	}

	path := filepath.Join(tmp, "internal", "adapter", "repository", "postgres", "systemrepository.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated system repository: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "errors.Is(err, pgx.ErrNoRows)") {
		t.Fatalf("generated repository must use errors.Is for no rows detection")
	}
	if strings.Contains(text, `err.Error() == "no rows in result set"`) {
		t.Fatalf("generated repository must not compare err.Error() for no rows detection")
	}
}
