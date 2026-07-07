package main

import (
	"testing"
	"testing/fstest"
)

func TestCalculateTemplateFSHashIsStableAndTracksContent(t *testing.T) {
	first := fstest.MapFS{
		"nested/b.tmpl": {Data: []byte("beta")},
		"a.tmpl":        {Data: []byte("alpha")},
	}
	second := fstest.MapFS{
		"a.tmpl":        {Data: []byte("alpha")},
		"nested/b.tmpl": {Data: []byte("beta")},
	}

	h1, err := calculateTemplateFSHash(first)
	if err != nil {
		t.Fatalf("hash first filesystem: %v", err)
	}
	h2, err := calculateTemplateFSHash(second)
	if err != nil {
		t.Fatalf("hash second filesystem: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hash depends on traversal order: %s != %s", h1, h2)
	}

	second["nested/b.tmpl"] = &fstest.MapFile{Data: []byte("changed")}
	h3, err := calculateTemplateFSHash(second)
	if err != nil {
		t.Fatalf("hash changed filesystem: %v", err)
	}
	if h3 == h1 {
		t.Fatal("template content change did not invalidate hash")
	}
}

func TestCalculateTemplateFSHashRejectsEmptyFilesystem(t *testing.T) {
	if _, err := calculateTemplateFSHash(fstest.MapFS{}); err == nil {
		t.Fatal("expected empty template filesystem to fail")
	}
}
