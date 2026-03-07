package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestLoadFactsEnvelope_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facts.json")
	payload := FactsEnvelope{
		Schema: "ang/facts/v1",
		Operations: []FactOp{
			{
				Name: "CreateOrder",
				InputFields: []FactField{
					{Name: "userId"},
				},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := loadFactsEnvelope(path)
	if err != nil {
		t.Fatalf("loadFactsEnvelope: %v", err)
	}
	if got.Schema != "ang/facts/v1" {
		t.Fatalf("schema = %s, want ang/facts/v1", got.Schema)
	}
}

func TestRunMigrationLints_MissingReqFieldWithoutMarker_Error(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Orders",
		Methods: []normalizer.Method{{
			Name: "CreateOrder",
			Flow: []normalizer.FlowStep{
				{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "req.UserID"}},
			},
		}},
	}}
	facts := &FactsEnvelope{
		Schema: "ang/facts/v1",
		Operations: []FactOp{{
			Name: "CreateOrder",
			InputFields: []FactField{
				{Name: "email"},
			},
		}},
	}

	diags := runMigrationLints(services, facts, lintProfileProd)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d", len(diags))
	}
	if diags[0].Code != codeMigrationGapWithoutQuestion {
		t.Fatalf("code = %s, want %s", diags[0].Code, codeMigrationGapWithoutQuestion)
	}
	if diags[0].Severity != "error" {
		t.Fatalf("severity = %s, want error", diags[0].Severity)
	}
}

func TestRunMigrationLints_MissingReqFieldWithMarker_Warn(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Orders",
		Methods: []normalizer.Method{{
			Name: "CreateOrder",
			Flow: []normalizer.FlowStep{
				{Action: "mapping.Assign", Args: map[string]any{"to": "ctx.Gap", "value": "unknown.userId"}},
				{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "req.UserID"}},
			},
		}},
	}}
	facts := &FactsEnvelope{
		Schema: "ang/facts/v1",
		Operations: []FactOp{{
			Name: "CreateOrder",
			InputFields: []FactField{
				{Name: "email"},
			},
		}},
	}

	diags := runMigrationLints(services, facts, lintProfileProd)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d", len(diags))
	}
	if diags[0].Code != codeMigrationGapMarked {
		t.Fatalf("code = %s, want %s", diags[0].Code, codeMigrationGapMarked)
	}
	if diags[0].Severity != "warn" {
		t.Fatalf("severity = %s, want warn", diags[0].Severity)
	}
}

func TestRunMigrationLints_ReqFieldInFacts_OK(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Orders",
		Methods: []normalizer.Method{{
			Name: "CreateOrder",
			Flow: []normalizer.FlowStep{
				{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "req.UserID"}},
			},
		}},
	}}
	facts := &FactsEnvelope{
		Schema: "ang/facts/v1",
		Operations: []FactOp{{
			Name: "CreateOrder",
			InputFields: []FactField{
				{Name: "userId"},
			},
		}},
	}

	diags := runMigrationLints(services, facts, lintProfileProd)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diags, got %d", len(diags))
	}
}
