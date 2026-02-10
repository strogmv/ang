package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler"
)

func TestFormatStageFailureSnapshot(t *testing.T) {
	got := formatStageFailure(
		"Validation FAILED",
		compiler.StageCUE,
		compiler.ErrCodeCUEPipeline,
		"run pipeline",
		errors.New("boom"),
	)

	goldenPath := filepath.Join("testdata", "cli_error_snapshot.txt")
	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if got+"\n" != want {
		t.Fatalf("snapshot mismatch\nwant: %q\ngot:  %q", want, got+"\n")
	}
}
