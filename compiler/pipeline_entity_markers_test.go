package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// Item 36 of docs/tech-debt-backlog.md: the DTO marker is @dto(only="true").
// A `_dto: true` field marks nothing (CUE paths cannot name hidden fields)
// and is reported once where it is written, even when other entities
// inherit it by embedding. A struct entity without an id and without the
// marker, which the SQL emitter skips silently, is a warning; enums are not
// entities to lint.
func TestSemanticPhasesLintEntityMarkers(t *testing.T) {
	const domain = `package domain

#RowProj: {
	_dto:  true
	total: int
} @dto(only="true")

Report: {
	#RowProj
	id: string
}

Settlement: {
	_dto: true
	kind: string
}

OrderLine: {
	sku: string
	qty: int
}

Status: "open" | "closed"

Archive: {
	key: string
} @storage("mongo")
`
	base := copyMinimalProject(t)
	if err := os.WriteFile(filepath.Join(base, "cue", "domain", "markers.cue"), []byte(domain), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(w normalizer.Warning) {
		if w.Code == normalizer.CodeDTOMarkerDeprecated || w.Code == normalizer.CodeEntityWithoutID {
			got = append(got, filepath.Base(w.File)+":"+strconv.Itoa(w.Line)+" "+w.Code+" "+w.Message)
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	want := []string{
		"markers.cue:13 " + normalizer.CodeEntityWithoutID + " entity Settlement has no id field and no DTO marker, so it gets no table",
		"markers.cue:14 " + normalizer.CodeDTOMarkerDeprecated + " entity Settlement: `_dto: true` is not a DTO marker and ANG does not read it; Settlement is not a DTO through it",
		"markers.cue:18 " + normalizer.CodeEntityWithoutID + " entity OrderLine has no id field and no DTO marker, so it gets no table",
		"markers.cue:4 " + normalizer.CodeDTOMarkerDeprecated + " entity RowProj: `_dto: true` repeats the marker @dto(only=\"true\") and ANG does not read it",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	dto := map[string]bool{}
	for _, e := range out.Entities {
		dto[e.Name] = normalizer.IsDTO(e)
	}
	// The attribute stays on #RowProj; Report embeds the struct but not the
	// marker, and Settlement's hidden field marks nothing.
	if !dto["RowProj"] || dto["Report"] || dto["Settlement"] {
		t.Fatalf("DTOs: %v", dto)
	}
}
