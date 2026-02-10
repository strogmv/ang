package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
)

func detectReleaseRootModuleMismatch(projectPath string) (string, bool) {
	p := parser.New()
	n := normalizer.New()

	var projectVal cue.Value
	var targets []normalizer.TargetDef
	if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/project")); err == nil && ok {
		projectVal = val
		parsed, err := n.ExtractTargets(val)
		if err == nil {
			targets = parsed
		}
	}
	if len(targets) == 0 {
		return "", false
	}
	mode := resolveBuildMode("", projectVal, false)
	if mode != "release" {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err != nil {
		return "", false
	}

	for _, td := range targets {
		if strings.ToLower(strings.TrimSpace(td.Lang)) != "go" {
			continue
		}
		out := strings.TrimSpace(td.OutputDir)
		if out == "" {
			out = filepath.ToSlash(filepath.Join("dist/release", safeTargetDirName(td.Name)))
		}
		out = filepath.ToSlash(filepath.Clean(out))
		if strings.HasPrefix(out, "dist/") || strings.HasPrefix(out, "./dist/") {
			return fmt.Sprintf("Generated code is not used by runtime build: release mode Go target outputs to %s while root go.mod exists; fix by using --mode=in_place, or keep --mode=release and run build/compile from %s module", out, out), true
		}
	}
	return "", false
}
