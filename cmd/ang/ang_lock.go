package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// A project pins the generator revision that produced its committed code in
// ang.lock. The consuming project's Makefile verified the pin before
// `make generate`, but running the binary directly — from a shell, an IDE, or an
// agent through MCP — skipped that check, and a compiler built weeks earlier or
// from uncommitted changes silently generated a different API contract. The
// check now lives in the binary, which already knows its own revision: Go embeds
// vcs.revision and vcs.modified when it builds from a git checkout.
const angLockFileName = "ang.lock"

type angLockPin struct {
	Repository string
	Commit     string
}

type compilerRevision struct {
	Revision string
	Modified bool
}

// readANGLock returns the project's pin. A project without ang.lock is not
// pinned; a lock without a commit is an error rather than "not pinned", so a
// truncated file does not silently switch the check off.
func readANGLock(projectPath string) (angLockPin, bool, error) {
	path := filepath.Join(projectPath, angLockFileName)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return angLockPin{}, false, nil
		}
		return angLockPin{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()

	var pin angLockPin
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "repository":
			pin.Repository = strings.TrimSpace(value)
		case "commit":
			pin.Commit = strings.ToLower(strings.TrimSpace(value))
		}
	}
	if err := scanner.Err(); err != nil {
		return angLockPin{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	if pin.Commit == "" {
		return angLockPin{}, false, fmt.Errorf("%s has no commit= line", path)
	}
	return pin, true, nil
}

func runningCompilerRevision() compilerRevision {
	var revision compilerRevision
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return revision
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision.Revision = strings.ToLower(strings.TrimSpace(setting.Value))
		case "vcs.modified":
			revision.Modified = setting.Value == "true"
		}
	}
	return revision
}

// checkANGLock compares the running binary with the project's pin. A different
// revision is an error unless allowMismatch is set. A binary without VCS
// information (go run, -buildvcs=false) or built with uncommitted changes only
// warns: neither can be shown to produce different output.
func checkANGLock(pin angLockPin, running compilerRevision, allowMismatch bool) ([]string, error) {
	if running.Revision == "" {
		return []string{fmt.Sprintf(
			"this ang binary carries no VCS revision (built with go run or -buildvcs=false), so it cannot be checked against %s commit %s",
			angLockFileName, shortRevision(pin.Commit),
		)}, nil
	}
	var warnings []string
	if !sameRevision(pin.Commit, running.Revision) {
		mismatch := fmt.Sprintf(
			"%s pins ang at %s, but this binary was built from %s; generated code and the API contract may differ from what is committed",
			angLockFileName, shortRevision(pin.Commit), shortRevision(running.Revision),
		)
		if !allowMismatch {
			return nil, errors.New(mismatch + fmt.Sprintf(
				". Rebuild from the pinned revision (make build-ang, or git -C <ang checkout> checkout %s && go build ./cmd/ang), or pass --allow-lock-mismatch to generate anyway",
				pin.Commit,
			))
		}
		warnings = append(warnings, mismatch+" (--allow-lock-mismatch is set)")
	}
	if running.Modified {
		warnings = append(warnings, fmt.Sprintf(
			"this ang binary was built from %s with uncommitted changes; its output may differ from that revision",
			shortRevision(running.Revision),
		))
	}
	return warnings, nil
}

// sameRevision accepts an abbreviated pin (at least 7 hex digits) as a prefix
// of the full revision.
func sameRevision(pinned, running string) bool {
	pinned = strings.ToLower(strings.TrimSpace(pinned))
	running = strings.ToLower(strings.TrimSpace(running))
	if pinned == "" || running == "" {
		return false
	}
	if len(pinned) != len(running) && len(pinned) >= 7 && len(running) >= 7 {
		return strings.HasPrefix(running, pinned) || strings.HasPrefix(pinned, running)
	}
	return pinned == running
}

func shortRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) > 8 {
		return revision[:8]
	}
	return revision
}
