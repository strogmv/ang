package main

import "testing"

func TestParseGoListDirOutput_LastPathLineWins(t *testing.T) {
	t.Parallel()

	out := []byte("go: downloading x/y v1.2.3\n/home/strog/work/sendbox/cmd/server\n")
	got, err := parseGoListDirOutput(out)
	if err != nil {
		t.Fatalf("parseGoListDirOutput: %v", err)
	}
	want := "/home/strog/work/sendbox/cmd/server"
	if got != want {
		t.Fatalf("unexpected parsed dir: got=%q want=%q", got, want)
	}
}

func TestParseGoListDirOutput_EmptyFails(t *testing.T) {
	t.Parallel()

	if _, err := parseGoListDirOutput([]byte("\n \n")); err == nil {
		t.Fatalf("expected error for empty output")
	}
}

