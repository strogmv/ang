package main

import "testing"

func TestSplitPPProjectPath(t *testing.T) {
	path, flags := splitPPProjectPath([]string{"/tmp/provider", "--mode", "advise", "--expert-base-url", "http://127.0.0.1:8787"})
	if path != "/tmp/provider" {
		t.Fatalf("path = %q", path)
	}
	if len(flags) != 4 {
		t.Fatalf("flags = %v", flags)
	}
	path, flags = splitPPProjectPath([]string{"--json"})
	if path != "." || len(flags) != 1 {
		t.Fatalf("default path = %q flags = %v", path, flags)
	}
}
