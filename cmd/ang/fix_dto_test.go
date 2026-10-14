package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Item 36 of docs/tech-debt-backlog.md: `ang fix dto` turns `_dto: true` into
// the attribute @dto(only="true") and touches nothing else.
func TestFixDTOMarkersRewritesTheHiddenField(t *testing.T) {
	root := t.TempDir()
	src := `package domain

// Both markers: the field goes.
#OrderRowProj: {
	_dto:  true
	id:    string
	total: int
} @dto(only="true")

// Only the field: it goes and the attribute comes.
Settlement: {
	_dto:        true
	type:        string
	customText?: string
}

Inline: {_dto: true, a: int}

// _dto was the longest label: the others align again.
#IDProj: {
	_dto: true
	id:   string
} @dto(only="true")

Order: {
	id: string
	meta: {_dto: true, note: string}
}
`
	path := filepath.Join(root, "domain", "orders.cue")
	writeTestFile(t, path, src)

	res, err := fixDTOMarkers(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesChanged != 1 {
		t.Fatalf("check reports %d changed files", res.FilesChanged)
	}
	if data, _ := os.ReadFile(path); string(data) != src {
		t.Fatal("--check wrote the file")
	}

	res, err = fixDTOMarkers(root, false)
	if err != nil {
		t.Fatal(err)
	}
	want := `package domain

// Both markers: the field goes.
#OrderRowProj: {
	id:    string
	total: int
} @dto(only="true")

// Only the field: it goes and the attribute comes.
Settlement: {
	type:        string
	customText?: string
} @dto(only="true")

Inline: {a: int} @dto(only="true")

// _dto was the longest label: the others align again.
#IDProj: {
	id: string
} @dto(only="true")

Order: {
	id: string
	meta: {_dto: true, note: string}
}
`
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("rewritten:\n%s\nwant:\n%s", data, want)
	}
	file := filepath.ToSlash(path)
	if wantMarked := []string{file + ":11 Settlement", file + ":17 Inline"}; !reflect.DeepEqual(res.Marked, wantMarked) {
		t.Errorf("marked = %v, want %v", res.Marked, wantMarked)
	}
	if len(res.Removed) != 4 {
		t.Errorf("removed = %v", res.Removed)
	}
	if wantLeft := []string{file + ":27"}; !reflect.DeepEqual(res.Left, wantLeft) {
		t.Errorf("left = %v, want %v", res.Left, wantLeft)
	}

	// A second run finds nothing to do.
	res, err = fixDTOMarkers(root, true)
	if err != nil || res.FilesChanged != 0 {
		t.Fatalf("second run: %+v, %v", res, err)
	}
}
