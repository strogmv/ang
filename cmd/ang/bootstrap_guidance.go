package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler"
)

func printBootstrapGuidanceIfNeeded(projectPath string, stage compiler.Stage, code string, err error) {
	if err == nil {
		return
	}
	if stage != compiler.StageCUE {
		return
	}
	if code != compiler.ErrCodeCUEAPILoad && code != compiler.ErrCodeCUEDomainLoad {
		return
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "no such file") && !errors.Is(err, os.ErrNotExist) {
		return
	}
	apiPath := filepath.Join(projectPath, "cue", "api", "http.cue")
	domainPath := filepath.Join(projectPath, "cue", "domain", "entities.cue")
	_, apiErr := os.Stat(apiPath)
	_, domainErr := os.Stat(domainPath)
	if apiErr == nil && domainErr == nil {
		return
	}
	fmt.Println("Guided fix: missing CUE contract bootstrap for this project.")
	fmt.Println("  Initialize a scaffold: ang init --template saas (or write cue/api/http.cue and cue/domain/entities.cue)")
}
