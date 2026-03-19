package ir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestToCanonicalJSON_Golden(t *testing.T) {
	schema := &Schema{
		IRVersion: IRVersionV1,
		Project: Project{
			Name:    "shop",
			Version: "0.1.0",
			Target:  Target{},
		},
		Entities: []Entity{{
			Name: "Product",
			Owns: []string{},
			Fields: []Field{{
				Name:       "id",
				Type:       TypeRef{Kind: KindUUID},
				Attributes: []Attribute{},
				UI: FieldUI{
					Options: []string{},
				},
				Metadata: map[string]any{},
			}},
			Indexes:  []Index{},
			UI:       EntityUI{},
			Metadata: map[string]any{},
			Source:   "cue://domain/Product",
		}},
		Services: []Service{{
			Name: "Catalog",
			Methods: []Method{{
				Name:       "ListProducts",
				Sources:    []Source{},
				CacheTags:  []string{},
				Throws:     []string{},
				Publishes:  []string{},
				Broadcasts: []string{},
				ImplSteps:  []ImplStep{},
				Flow:       []FlowStep{},
				Attributes: []Attribute{},
				Metadata:   map[string]any{},
			}},
			Publishes:  []string{},
			Subscribes: map[string]string{},
			Uses:       []string{},
			Metadata:   map[string]any{},
			Source:     "cue://service/Catalog",
		}},
		Events:    []Event{},
		Errors:    []Error{},
		Endpoints: []Endpoint{},
		Scopes:    []Scope{},
		Repos:     []Repository{},
		Config: Config{
			Fields: []Field{},
		},
		Schedules: []Schedule{},
		Views:     []View{},
		Metadata:  map[string]any{},
	}

	got, err := ToCanonicalJSON(schema)
	if err != nil {
		t.Fatalf("ToCanonicalJSON error: %v", err)
	}

	want, err := os.ReadFile(filepath.Join("testdata", "canonical_minimal_v2.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
		t.Fatalf("canonical json mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestConvertFromNormalizer_DoesNotInventTargetDefaults(t *testing.T) {
	schema := ConvertFromNormalizer(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		normalizer.ConfigDef{},
		nil,
		nil,
		nil,
		nil,
		normalizer.ProjectDef{Name: "shop", Version: "0.1.0"},
	)

	if schema.Project.Target.Lang != "" {
		t.Fatalf("expected empty project.target.lang, got %q", schema.Project.Target.Lang)
	}
	if schema.Project.Target.Framework != "" {
		t.Fatalf("expected empty project.target.framework, got %q", schema.Project.Target.Framework)
	}
	if schema.Project.Target.DB != "" {
		t.Fatalf("expected empty project.target.db, got %q", schema.Project.Target.DB)
	}
}
