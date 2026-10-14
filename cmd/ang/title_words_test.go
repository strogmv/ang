package main

import "testing"

func TestTitleWords(t *testing.T) {
	for in, want := range map[string]string{
		"patch apply":   "Patch Apply",
		"template diff": "Template Diff",
		"":              "",
		"x":             "X",
	} {
		if got := titleWords(in); got != want {
			t.Errorf("titleWords(%q) = %q, want %q", in, got, want)
		}
	}
}
