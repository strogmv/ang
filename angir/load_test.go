package angir

import "testing"

func TestLoadSchema_MinimalProject(t *testing.T) {
	result, err := Load("testdata/minimal")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if result.Schema == nil {
		t.Fatalf("schema is nil")
	}
	if len(result.Schema.Entities) != 1 || result.Schema.Entities[0].Name != "User" {
		t.Fatalf("unexpected entities: %#v", result.Schema.Entities)
	}
	if len(result.Schema.Services) != 1 || result.Schema.Services[0].Name != "User" {
		t.Fatalf("unexpected services: %#v", result.Schema.Services)
	}
	if len(result.Schema.Endpoints) != 1 || result.Schema.Endpoints[0].Path != "/users" {
		t.Fatalf("unexpected endpoints: %#v", result.Schema.Endpoints)
	}
	if len(result.Schema.Repos) != 1 || result.Schema.Repos[0].Entity != "User" {
		t.Fatalf("unexpected repos: %#v", result.Schema.Repos)
	}
}
