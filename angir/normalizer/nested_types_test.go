package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func compileOutput(t *testing.T, src string) cue.Value {
	t.Helper()
	val := cuecontext.New().CompileString(src, cue.Filename("cue/api/x.cue"))
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	return val.LookupPath(cue.ParsePath("output"))
}

func fieldByName(t *testing.T, fields []Field, name string) Field {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %q in %v", name, fields)
	return Field{}
}

// A list of objects inside a list of objects inside an object (three levels)
// keeps a named type at every level; it used to come out as []string.
func TestNestedListsOfObjectsGetNamedTypesAtAnyDepth(t *testing.T) {
	out := compileOutput(t, `
output: {
	version: string
	settings: {
		remind: bool
		days:   int
	}
	pages: [...{
		pageKey: string
		steps: [...{
			anchor: string
			isNew:  bool
			hints: [...{text: string}]
		}]
		labels: [string]: string
		counts: [string]: int
		flags: [string]: [string]: bool
		tagsBy: [string]: [...string]
		grid: [...[...{x: int}]]
		rows: [...{[string]: string}]
		rowsBy: [string]: [...{n: int}]
	}]
	data: [...{id: string}]
	tags: [...string]
	extra: _
	open: {...}
}
`)
	ent, err := New().parseEntity("GetToursResponse", out)
	if err != nil {
		t.Fatal(err)
	}

	settings := fieldByName(t, ent.Fields, "settings")
	if settings.Type != "GetToursResponseSettings" || len(settings.ItemFields) != 2 || settings.IsList {
		t.Fatalf("settings = %q list=%v fields=%d", settings.Type, settings.IsList, len(settings.ItemFields))
	}

	pages := fieldByName(t, ent.Fields, "pages")
	if pages.Type != "[]GetToursResponsePagesItem" || pages.ItemTypeName != "GetToursResponsePagesItem" {
		t.Fatalf("pages = %q / %q", pages.Type, pages.ItemTypeName)
	}
	steps := fieldByName(t, pages.ItemFields, "steps")
	if steps.Type != "[]GetToursResponsePagesItemStepsItem" || !steps.IsList {
		t.Fatalf("pages[].steps = %q list=%v", steps.Type, steps.IsList)
	}
	hints := fieldByName(t, steps.ItemFields, "hints")
	if hints.Type != "[]GetToursResponsePagesItemStepsItemHintsItem" {
		t.Fatalf("pages[].steps[].hints = %q", hints.Type)
	}
	if got := fieldByName(t, hints.ItemFields, "text").Type; got != "string" {
		t.Fatalf("hints[].text = %q", got)
	}
	if got := fieldByName(t, pages.ItemFields, "labels").Type; got != "map[string]string" {
		t.Fatalf("pages[].labels = %q", got)
	}
	if got := fieldByName(t, pages.ItemFields, "counts").Type; got != "map[string]int" {
		t.Fatalf("pages[].counts = %q", got)
	}
	if got := fieldByName(t, pages.ItemFields, "flags").Type; got != "map[string]map[string]bool" {
		t.Fatalf("pages[].flags = %q", got)
	}
	if got := fieldByName(t, pages.ItemFields, "tagsBy").Type; got != "map[string][]string" {
		t.Fatalf("pages[].tagsBy = %q", got)
	}
	grid := fieldByName(t, pages.ItemFields, "grid")
	if grid.Type != "[][]GetToursResponsePagesItemGridItemItem" || len(grid.ItemFields) != 1 {
		t.Fatalf("pages[].grid = %q", grid.Type)
	}
	if got := fieldByName(t, pages.ItemFields, "rows").Type; got != "[]map[string]string" {
		t.Fatalf("pages[].rows = %q", got)
	}
	rowsBy := fieldByName(t, pages.ItemFields, "rowsBy")
	if rowsBy.Type != "map[string][]GetToursResponsePagesItemRowsByValueItem" || len(rowsBy.ItemFields) != 1 {
		t.Fatalf("pages[].rowsBy = %q", rowsBy.Type)
	}

	// Unchanged shapes.
	if got := fieldByName(t, ent.Fields, "data").Type; got != "[]GetToursResponseData" {
		t.Fatalf("data = %q", got)
	}
	if got := fieldByName(t, ent.Fields, "tags").Type; got != "[]string" {
		t.Fatalf("tags = %q", got)
	}
	if got := fieldByName(t, ent.Fields, "extra").Type; got != "any" {
		t.Fatalf("extra = %q", got)
	}
	if got := fieldByName(t, ent.Fields, "open").Type; got != "map[string]any" {
		t.Fatalf("open = %q", got)
	}

	names := []string{}
	for _, nested := range CollectNestedTypes(ent.Fields) {
		names = append(names, nested.Name)
	}
	want := "GetToursResponseSettings,GetToursResponsePagesItem,GetToursResponsePagesItemStepsItem,GetToursResponsePagesItemStepsItemHintsItem,GetToursResponsePagesItemGridItemItem,GetToursResponsePagesItemRowsByValueItem,GetToursResponseData"
	if strings.Join(names, ",") != want {
		t.Fatalf("nested types = %v, want %s", names, want)
	}
}

// Shapes ANG cannot type fail the build with the CUE position instead of
// degrading to []string or any.
func TestUntypableShapesFailLoudly(t *testing.T) {
	cases := map[string]string{
		"union map value":       `output: { byKey: [string]: int | string }`,
		"union in list of maps": `output: { rows: [...{[string]: int | {x: int}}] }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New().parseEntity("XResponse", compileOutput(t, src))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "x.cue") {
				t.Fatalf("error does not point at the CUE file: %v", err)
			}
		})
	}
}

// Domain entities keep their storage types: an inline object stays
// map[string]any (a JSON column), only lists of objects get item types.
func TestDomainEntityInlineObjectStaysUntyped(t *testing.T) {
	out := compileOutput(t, `
output: {
	meta: {a: string}
	lines: [...{sku: string, parts: [...{n: int}]}]
}
`)
	ent, err := New().parseEntity("Order", out)
	if err != nil {
		t.Fatal(err)
	}
	if got := fieldByName(t, ent.Fields, "meta").Type; got != "map[string]any" {
		t.Fatalf("meta = %q", got)
	}
	lines := fieldByName(t, ent.Fields, "lines")
	if got := fieldByName(t, lines.ItemFields, "parts").Type; got != "[]OrderLinesItemPartsItem" {
		t.Fatalf("lines[].parts = %q", got)
	}
}

// A map of objects gets a value type; a CUE definition inside it (#Column,
// #Transform) is typed inline rather than taken for a domain entity.
func TestMapOfObjectsGetsAValueType(t *testing.T) {
	val := cuecontext.New().CompileString(`
#Transform: {op: string, arg?: string}
#Column: {
	header: string
	transforms?: [...#Transform]
}
output: {
	data: [...{
		id: string
		columns: [string]: #Column
	}]
}
`, cue.Filename("cue/api/x.cue"))
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	ent, err := New().parseEntity("ListTemplatesResponse", val.LookupPath(cue.ParsePath("output")))
	if err != nil {
		t.Fatal(err)
	}
	data := fieldByName(t, ent.Fields, "data")
	columns := fieldByName(t, data.ItemFields, "columns")
	if columns.Type != "map[string]ListTemplatesResponseDataColumnsValue" || columns.ItemTypeName != "ListTemplatesResponseDataColumnsValue" {
		t.Fatalf("columns = %q / %q", columns.Type, columns.ItemTypeName)
	}
	transforms := fieldByName(t, columns.ItemFields, "transforms")
	if transforms.Type != "[]ListTemplatesResponseDataColumnsValueTransformsItem" || !transforms.IsOptional {
		t.Fatalf("columns{}.transforms = %q optional=%v", transforms.Type, transforms.IsOptional)
	}
	if got := fieldByName(t, transforms.ItemFields, "op").Type; got != "string" {
		t.Fatalf("transforms[].op = %q", got)
	}
}
