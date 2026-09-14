package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func exitedProcessPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot start a short-lived process: %v", err)
	}
	return cmd.ProcessState.Pid()
}

func makeScratchDir(t *testing.T, parent, name string, owner *buildScratchOwner, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	writeTestFile(t, filepath.Join(dir, "stage", "payload.go"), "payload")
	if owner != nil {
		data, err := json.Marshal(owner)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, buildScratchOwnerFile), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if age > 0 {
		stamp := time.Now().Add(-age)
		if err := os.Chtimes(dir, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSweepAbandonedBuildScratch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("liveness is not probed on windows; every owner counts as alive")
	}
	parent := t.TempDir()
	host, _ := os.Hostname()
	dead := exitedProcessPID(t)

	deadOwner := makeScratchDir(t, parent, ".ang-build-transaction-dead", &buildScratchOwner{PID: dead, Host: host}, 0)
	legacyOld := makeScratchDir(t, parent, ".ang-build-transaction-legacy-old", nil, 48*time.Hour)
	live := makeScratchDir(t, parent, ".ang-build-transaction-live", &buildScratchOwner{PID: os.Getpid(), Host: host}, 0)
	legacyYoung := makeScratchDir(t, parent, ".ang-build-transaction-legacy-young", nil, time.Hour)
	kept := makeScratchDir(t, parent, ".ang-build-transaction-keep", &buildScratchOwner{PID: dead, Host: host, Keep: true}, 72*time.Hour)
	otherHost := makeScratchDir(t, parent, ".ang-build-transaction-other-host", &buildScratchOwner{PID: dead, Host: host + "-elsewhere"}, 72*time.Hour)
	unrelated := makeScratchDir(t, parent, ".ang-cache-not-scratch", nil, 72*time.Hour)

	var result buildScratchSweep
	sweepAbandonedBuildScratch(parent, transactionScratchPrefix, time.Now(), &result)

	for _, gone := range []string{deadOwner, legacyOld} {
		assertTestFileMissing(t, gone)
	}
	for _, stays := range []string{live, legacyYoung, kept, otherHost, unrelated} {
		if _, err := os.Stat(stays); err != nil {
			t.Fatalf("%s was removed but must be kept: %v", filepath.Base(stays), err)
		}
	}
	if result.Removed != 2 {
		t.Fatalf("removed %d directories, want 2", result.Removed)
	}
	if result.Bytes <= 0 {
		t.Fatalf("freed %d bytes, want the payload sizes", result.Bytes)
	}
	if len(result.Kept) != 1 || result.Kept[0] != kept {
		t.Fatalf("kept = %v, want [%s]", result.Kept, kept)
	}
}

func TestBuildTransactionRecordsOwnerAndSweepsAbandonedScratch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("liveness is not probed on windows")
	}
	root := t.TempDir()
	host, _ := os.Hostname()
	abandonedTx := makeScratchDir(t, root, ".ang-build-transaction-crashed", &buildScratchOwner{PID: exitedProcessPID(t), Host: host}, 0)
	abandonedWorkspace := makeScratchDir(t, filepath.Dir(root), ".ang-build-workspace-crashed-"+filepath.Base(root), &buildScratchOwner{PID: exitedProcessPID(t), Host: host}, 0)
	writeTestFile(t, filepath.Join(root, "internal", "a.go"), "a")

	tx, err := beginBuildTransaction([]string{filepath.Join(root, "internal")})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.CreateWorkspace(root); err != nil {
		t.Fatal(err)
	}

	assertTestFileMissing(t, abandonedTx)
	assertTestFileMissing(t, abandonedWorkspace)
	if swept := tx.Swept(); swept.Removed != 2 {
		t.Fatalf("swept %+v, want 2 removed", swept)
	}

	var owner buildScratchOwner
	data, err := os.ReadFile(filepath.Join(tx.entries[0].workDir, buildScratchOwnerFile))
	if err != nil {
		t.Fatalf("new transaction directory has no owner: %v", err)
	}
	if err := json.Unmarshal(data, &owner); err != nil || owner.PID != os.Getpid() || owner.Keep {
		t.Fatalf("owner = %+v err=%v", owner, err)
	}
	if _, err := os.Stat(filepath.Join(tx.workspaces[0], buildScratchOwnerFile)); err != nil {
		t.Fatalf("new workspace has no owner: %v", err)
	}
}
