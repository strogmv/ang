package main

import (
	"os/exec"
	"testing"
)

func TestWithGoParallelism_ReplacesExistingPFlag(t *testing.T) {
	got := withGoParallelism("-mod=mod -p=8 -tags=contract", 2)
	want := "-mod=mod -tags=contract -p=2"
	if got != want {
		t.Fatalf("unexpected GOFLAGS: got %q want %q", got, want)
	}
}

func TestWithGoParallelism_AppendsPFlag(t *testing.T) {
	got := withGoParallelism("-mod=mod", 2)
	want := "-mod=mod -p=2"
	if got != want {
		t.Fatalf("unexpected GOFLAGS: got %q want %q", got, want)
	}
}

func TestConfigureBuildSubprocess_AppliesCaps(t *testing.T) {
	cmd := exec.Command("go", "build", "./...")
	cmd.Env = []string{"GOFLAGS=-mod=mod -p=8"}
	configureBuildSubprocess(cmd)
	gotGoFlags := envValue(cmd.Env, "GOFLAGS")
	if gotGoFlags != "-mod=mod -p=2" {
		t.Fatalf("unexpected GOFLAGS after cap: %q", gotGoFlags)
	}
	if got := envValue(cmd.Env, "GOMAXPROCS"); got != "2" {
		t.Fatalf("unexpected GOMAXPROCS after cap: %q", got)
	}
}
