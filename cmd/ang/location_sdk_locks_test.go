package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func TestLocationSDKFingerprintIsReadOnlyAndExplicitAcceptanceUnlocksChangedScene(t *testing.T) {
	dir := t.TempDir()
	input := locationSDKTestInput()
	var stdout, stderr bytes.Buffer
	writeSDKTestCUE(t, dir, input)
	sourcePath := filepath.Join(dir, "layout.cue")
	before, _ := os.ReadFile(sourcePath)
	if err := runLocationSDK([]string{"-fingerprint", "clinic", dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var lock locationsdk.LayoutLock
	if err := json.Unmarshal(stdout.Bytes(), &lock); err != nil || lock.LayoutID != "clinic" || len(lock.ExpectedHash) != 64 {
		t.Fatal("missing ready-to-paste lock candidate", err)
	}
	files, _ := os.ReadDir(dir)
	after, _ := os.ReadFile(sourcePath)
	if len(files) != 1 || !bytes.Equal(before, after) || stderr.Len() != 0 {
		t.Fatal("fingerprint wrote files or changed the CUE source")
	}
	input.Locks = []locationsdk.LayoutLock{lock}
	input.Layouts[0].Instances[0].Reason += " intentional change"
	writeSDKTestCUE(t, dir, input)
	stdout.Reset()
	if err := runLocationSDK([]string{"-check", dir}, &stdout, &stderr); err == nil || !strings.Contains(stderr.String(), "lock_mismatch") {
		t.Fatal("locked edit was silently accepted", err)
	}
	stdout.Reset()
	if err := runLocationSDK([]string{"-fingerprint", "clinic", dir}, &stdout, &stderr); err != nil {
		t.Fatal("read-only candidate unavailable for explicit review", err)
	}
	var candidate locationsdk.LayoutLock
	if err := json.Unmarshal(stdout.Bytes(), &candidate); err != nil || candidate.ExpectedHash == lock.ExpectedHash {
		t.Fatal("new candidate was not computed", err)
	}
	still, _ := os.ReadFile(sourcePath)
	if !bytes.Contains(still, []byte(lock.ExpectedHash)) || bytes.Contains(still, []byte(candidate.ExpectedHash)) {
		t.Fatal("candidate calculation rewrote the old expectation")
	}
	input.Locks[0] = candidate
	writeSDKTestCUE(t, dir, input)
	if err := runLocationSDK([]string{"-check", dir}, &stdout, &stderr); err != nil {
		t.Fatal("explicit candidate replacement was not accepted", err)
	}
}

func TestLocationSDKLockChecksPrecedeEveryOutputWriteWhileOtherLayoutsRemainEditable(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "inspection")
	input := locationSDKTestInput()
	other := input.Layouts[0]
	other.ID = "separate"
	input.Layouts = append(input.Layouts, other)
	hash, err := locationsdk.LayoutFingerprint(input, "clinic")
	if err != nil {
		t.Fatal(err)
	}
	input.Locks = []locationsdk.LayoutLock{{LayoutID: "clinic", ExpectedHash: hash}}
	var stdout, stderr bytes.Buffer
	writeSDKTestCUE(t, dir, input)
	if err := runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	input.Layouts[1].Seed++
	writeSDKTestCUE(t, dir, input)
	if err := runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err != nil {
		t.Fatal("other layout was unnecessarily frozen", err)
	}
	validJSON, _ := os.ReadFile(filepath.Join(out, "compiled.json"))
	validHTML, _ := os.ReadFile(filepath.Join(out, "index.html"))
	// Shared light is a genuine visual dependency of the locked room even when
	// its terrain and structural hash do not change.
	input.Catalog.Times[1].Brightness -= .1
	writeSDKTestCUE(t, dir, input)
	stderr.Reset()
	if err := runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err == nil {
		t.Fatal("changed shared view bypassed the lock")
	}
	var diagnostic struct {
		Diagnostics []locationsdk.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &diagnostic); err != nil || len(diagnostic.Diagnostics) != 1 || diagnostic.Diagnostics[0].Code != "lock_mismatch" || diagnostic.Diagnostics[0].Path != "locks[0].expected_hash" || !strings.Contains(diagnostic.Diagnostics[0].Message, "-fingerprint") {
		t.Fatal("lock failure was not machine-readable and actionable", err)
	}
	unchangedJSON, _ := os.ReadFile(filepath.Join(out, "compiled.json"))
	unchangedHTML, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !bytes.Equal(validJSON, unchangedJSON) || !bytes.Equal(validHTML, unchangedHTML) {
		t.Fatal("lock failure replaced a previously valid output")
	}
	absent := filepath.Join(dir, "must-not-exist")
	if err := runLocationSDK([]string{"-out", absent, dir}, &stdout, &stderr); err == nil {
		t.Fatal("expected locked compile rejection")
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("locked rejection created an output directory")
	}
}

func TestLocationSDKFingerprintRejectsUnknownLayoutAndOutputFlags(t *testing.T) {
	dir := t.TempDir()
	writeSDKTestCUE(t, dir, locationSDKTestInput())
	for _, args := range [][]string{
		{"-fingerprint", "absent", dir},
		{"-fingerprint", "clinic", "-check", dir},
		{"-fingerprint", "clinic", "-out", filepath.Join(dir, "forbidden"), dir},
	} {
		var stdout, stderr bytes.Buffer
		if err := runLocationSDK(args, &stdout, &stderr); err == nil || stdout.Len() != 0 {
			t.Fatal("invalid fingerprint invocation wrote a successful candidate", args, err)
		}
	}
}
