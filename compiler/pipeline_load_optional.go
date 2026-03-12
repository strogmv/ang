package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	cueparser "cuelang.org/go/cue/parser"
	"github.com/strogmv/ang-ir/parser"
)

func LoadOptionalDomain(p *parser.Parser, path string) (cue.Value, bool, error) {
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	if len(matches) == 0 {
		return cue.Value{}, false, nil
	}
	val, err := p.LoadDomain(path)
	if err != nil {
		filtered, skipped, filterErr := filterValidCUEFiles(matches)
		if filterErr != nil || len(skipped) == 0 || len(filtered) == 0 {
			return cue.Value{}, false, err
		}

		tmpDir, mkErr := os.MkdirTemp("", "ang-partial-cue-*")
		if mkErr != nil {
			return cue.Value{}, false, err
		}
		defer os.RemoveAll(tmpDir)
		for _, src := range filtered {
			data, readErr := os.ReadFile(src)
			if readErr != nil {
				return cue.Value{}, false, err
			}
			dst := filepath.Join(tmpDir, filepath.Base(src))
			if writeErr := os.WriteFile(dst, data, 0o644); writeErr != nil {
				return cue.Value{}, false, err
			}
		}

		val2, err2 := p.LoadDomain(tmpDir)
		if err2 != nil {
			return cue.Value{}, false, err
		}
		printSkippedCueWarnings(path, skipped, len(filtered), len(matches))
		return val2, true, nil
	}
	return val, true, nil
}

func filterValidCUEFiles(files []string) (valid []string, skipped map[string]error, err error) {
	valid = make([]string, 0, len(files))
	skipped = make(map[string]error)
	for _, file := range files {
		src, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, nil, readErr
		}
		if _, parseErr := cueparser.ParseFile(file, src, cueparser.ParseComments); parseErr != nil {
			skipped[file] = parseErr
			continue
		}
		valid = append(valid, file)
	}
	return valid, skipped, nil
}

func printSkippedCueWarnings(dir string, skipped map[string]error, built, total int) {
	if len(skipped) == 0 {
		return
	}
	ordered := make([]string, 0, len(skipped))
	for file := range skipped {
		ordered = append(ordered, file)
	}
	sort.Strings(ordered)

	for _, file := range ordered {
		rel := file
		if cwd, err := os.Getwd(); err == nil {
			if r, relErr := filepath.Rel(cwd, file); relErr == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		fmt.Fprintf(os.Stderr, "File %s: syntax error: %v\n", filepath.ToSlash(rel), skipped[file])
		fmt.Fprintln(os.Stderr, "Skipping this file and continuing with remaining files...")
	}
	fmt.Fprintf(os.Stderr, "Built %d/%d files successfully for %s. Fix skipped files and rebuild.\n", built, total, filepath.ToSlash(dir))
}
