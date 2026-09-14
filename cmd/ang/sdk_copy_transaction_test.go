package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// copyFrontendSDK empties its target before writing, so a failure part-way
// through would leave the application without an SDK. During a build the
// target is never the real directory: build.go hands the copy step the
// transaction's stage for --frontend-app-dir, and the real directory changes
// only in Commit. A copy that fails mid-way therefore fails the build, the
// transaction rolls back, and the application's SDK is untouched.
func TestFrontendSDKCopyFailureDuringBuildLeavesAppSDKUntouched(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("relies on a read-only directory refusing writes, which windows and root ignore")
	}
	appSDK := filepath.Join(t.TempDir(), "front", "src", "@sdk")
	writeTestFile(t, filepath.Join(appSDK, sdkManifestName), "{\"old\":true}")
	writeTestFile(t, filepath.Join(appSDK, "index.ts"), "old sdk")

	generated := t.TempDir()
	writeTestFile(t, filepath.Join(generated, sdkManifestName), "{\"new\":true}")
	writeTestFile(t, filepath.Join(generated, "a.ts"), "new a")
	unreadable := filepath.Join(generated, "z.ts")
	writeTestFile(t, unreadable, "new z")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

	tx, err := beginBuildTransaction([]string{appSDK})
	if err != nil {
		t.Fatal(err)
	}
	staged := tx.StagePath(appSDK)
	if staged == appSDK {
		t.Fatal("the SDK target was not staged")
	}
	if err := copyFrontendSDK(generated, staged); err == nil {
		t.Fatal("copy succeeded although z.ts is unreadable")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	assertTestFile(t, filepath.Join(appSDK, "index.ts"), "old sdk")
	assertTestFile(t, filepath.Join(appSDK, sdkManifestName), "{\"old\":true}")
	assertTestFileMissing(t, filepath.Join(appSDK, "a.ts"))
}

// The same copy that succeeds reaches the application only through Commit's
// merge: stale SDK files are removed, and a file someone added to the SDK
// directory while the build ran is kept.
func TestFrontendSDKCopyPublishesThroughMerge(t *testing.T) {
	appSDK := filepath.Join(t.TempDir(), "front", "src", "@sdk")
	writeTestFile(t, filepath.Join(appSDK, sdkManifestName), "{\"old\":true}")
	writeTestFile(t, filepath.Join(appSDK, "stale.ts"), "removed endpoint")

	generated := t.TempDir()
	writeTestFile(t, filepath.Join(generated, sdkManifestName), "{\"new\":true}")
	writeTestFile(t, filepath.Join(generated, "index.ts"), "new sdk")

	tx, err := beginBuildTransaction([]string{appSDK})
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFrontendSDK(generated, tx.StagePath(appSDK)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(appSDK, "local-note.md"), "added during the build")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	assertTestFile(t, filepath.Join(appSDK, "index.ts"), "new sdk")
	assertTestFile(t, filepath.Join(appSDK, sdkManifestName), "{\"new\":true}")
	assertTestFileMissing(t, filepath.Join(appSDK, "stale.ts"))
	assertTestFile(t, filepath.Join(appSDK, "local-note.md"), "added during the build")
}
