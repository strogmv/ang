package main

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestResolveBackendDirForTarget_InPlaceIgnoresOutputDir(t *testing.T) {
	td := normalizer.TargetDef{Name: "go-service", Lang: "go", OutputDir: "dist/release/go-service"}
	got := resolveBackendDirForTarget("in_place", ".", td, false)
	if got != "." {
		t.Fatalf("expected '.', got %q", got)
	}
}

func TestResolveBackendDirForTarget_InPlaceGoMultiTargetStaysRoot(t *testing.T) {
	td := normalizer.TargetDef{Name: "go-service", Lang: "go"}
	got := resolveBackendDirForTarget("in_place", ".", td, true)
	if got != "." {
		t.Fatalf("expected '.', got %q", got)
	}
}

func TestResolveBackendDirForTarget_InPlaceNonGoMultiTargetIsNamespaced(t *testing.T) {
	td := normalizer.TargetDef{Name: "python-service", Lang: "python"}
	got := resolveBackendDirForTarget("in_place", ".", td, true)
	if got != "python-service" {
		t.Fatalf("expected 'python-service', got %q", got)
	}
}

func TestResolveBackendDirForTarget_ReleaseUsesOutputDir(t *testing.T) {
	td := normalizer.TargetDef{Name: "go-service", OutputDir: "dist/release/go-service"}
	got := resolveBackendDirForTarget("release", "internal", td, false)
	if got != "dist/release/go-service" {
		t.Fatalf("expected dist/release/go-service, got %q", got)
	}
}

func TestValidateBuildMode_RejectsMixedReleaseConfig(t *testing.T) {
	opts := OutputOptions{}
	targets := []normalizer.TargetDef{
		{Name: "go", OutputDir: "dist/release/go"},
		{Name: "python", OutputDir: ""},
	}
	err := validateBuildMode("release", opts, targets)
	if err == nil {
		t.Fatal("expected mixed release config error")
	}
}

func TestResolveBuildModePriority(t *testing.T) {
	ctx := cuecontext.New()
	project := ctx.CompileString(`package project
build: { mode: "release" }
`)
	if err := project.Err(); err != nil {
		t.Fatalf("compile project cue: %v", err)
	}

	if got := resolveBuildMode("in_place", project, false); got != "in_place" {
		t.Fatalf("cli mode must win, got %q", got)
	}
	if got := resolveBuildMode("", project, true); got != "in_place" {
		t.Fatalf("explicit backend-dir must force in_place, got %q", got)
	}
	if got := resolveBuildMode("", project, false); got != "release" {
		t.Fatalf("project build.mode should be used, got %q", got)
	}
}

func TestParseOutputOptions_BackendDirExplicit(t *testing.T) {
	opts, err := parseOutputOptions([]string{"--backend-dir=."})
	if err != nil {
		t.Fatalf("parseOutputOptions: %v", err)
	}
	if !opts.BackendDirExplicit {
		t.Fatal("expected BackendDirExplicit=true")
	}
}
