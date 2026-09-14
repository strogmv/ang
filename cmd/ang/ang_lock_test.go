package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	pinnedRevision = "050e46d0fe4536bb10be54e33604e449012ce642"
	otherRevision  = "0bb3b928b547d63d3db5782d9471d772c7a776ac"
)

func TestReadANGLock(t *testing.T) {
	dir := t.TempDir()
	if _, pinned, err := readANGLock(dir); err != nil || pinned {
		t.Fatalf("project without ang.lock: pinned=%v err=%v, want not pinned", pinned, err)
	}

	lock := "# generator pin\nrepository=https://github.com/strogmv/ang.git\n\ncommit= " + strings.ToUpper(pinnedRevision) + " \n"
	if err := os.WriteFile(filepath.Join(dir, angLockFileName), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	pin, pinned, err := readANGLock(dir)
	if err != nil || !pinned {
		t.Fatalf("pinned=%v err=%v", pinned, err)
	}
	if pin.Commit != pinnedRevision || pin.Repository != "https://github.com/strogmv/ang.git" {
		t.Fatalf("pin = %+v", pin)
	}

	if err := os.WriteFile(filepath.Join(dir, angLockFileName), []byte("repository=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readANGLock(dir); err == nil {
		t.Fatal("a lock without commit= must be an error, not an unpinned project")
	}
}

func TestCheckANGLock(t *testing.T) {
	pin := angLockPin{Commit: pinnedRevision}
	cases := []struct {
		name         string
		pin          angLockPin
		running      compilerRevision
		allow        bool
		wantErr      bool
		wantWarnings int
		mustMention  []string
	}{
		{name: "same revision", pin: pin, running: compilerRevision{Revision: pinnedRevision}},
		{name: "abbreviated pin", pin: angLockPin{Commit: "050e46d0"}, running: compilerRevision{Revision: pinnedRevision}},
		{name: "same revision, dirty checkout", pin: pin, running: compilerRevision{Revision: pinnedRevision, Modified: true}, wantWarnings: 1, mustMention: []string{"uncommitted"}},
		{name: "different revision", pin: pin, running: compilerRevision{Revision: otherRevision}, wantErr: true, mustMention: []string{"050e46d0", "0bb3b928", "--allow-lock-mismatch", pinnedRevision}},
		{name: "different revision, allowed", pin: pin, running: compilerRevision{Revision: otherRevision}, allow: true, wantWarnings: 1, mustMention: []string{"050e46d0", "0bb3b928"}},
		{name: "different and dirty, allowed", pin: pin, running: compilerRevision{Revision: otherRevision, Modified: true}, allow: true, wantWarnings: 2},
		{name: "no VCS information", pin: pin, running: compilerRevision{}, wantWarnings: 1, mustMention: []string{"no VCS revision"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warnings, err := checkANGLock(tc.pin, tc.running, tc.allow)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if len(warnings) != tc.wantWarnings {
				t.Fatalf("warnings = %q, want %d", warnings, tc.wantWarnings)
			}
			text := strings.Join(warnings, "\n")
			if err != nil {
				text = err.Error()
			}
			for _, fragment := range tc.mustMention {
				if !strings.Contains(text, fragment) {
					t.Fatalf("%q does not mention %q", text, fragment)
				}
			}
		})
	}
}

func TestSameRevisionRejectsShortOrEmptyPrefixes(t *testing.T) {
	if sameRevision("050e46", pinnedRevision) {
		t.Fatal("a 6-digit prefix is too short to identify a revision")
	}
	if sameRevision("", pinnedRevision) || sameRevision(pinnedRevision, "") {
		t.Fatal("empty revisions never match")
	}
}
