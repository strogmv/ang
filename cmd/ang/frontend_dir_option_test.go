package main

import (
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// Where the SDK is generated comes from CUE unless the command says otherwise.
func TestFrontendDirOptionForTarget(t *testing.T) {
	cue := normalizer.TargetDef{FrontendDir: "../app/src/@sdk"}

	if got := frontendDirOptionForTarget(OutputOptions{FrontendDir: "sdk"}, cue); got != "../app/src/@sdk" {
		t.Errorf("without the flag the target's frontend_dir wins, got %q", got)
	}
	explicit := OutputOptions{FrontendDir: "build/sdk", FrontendDirExplicit: true}
	if got := frontendDirOptionForTarget(explicit, cue); got != "build/sdk" {
		t.Errorf("--frontend-dir must win, got %q", got)
	}
	if got := frontendDirOptionForTarget(OutputOptions{FrontendDir: "sdk"}, normalizer.TargetDef{}); got != "sdk" {
		t.Errorf("without frontend_dir the flag default stays, got %q", got)
	}
}
