package main

import (
	"reflect"
	"testing"
)

// findTopLevelListItems must return each item of a CUE list once; a comma at
// the top level ends the item (backlog #7: the break left only the switch).
func TestFindTopLevelListItemsSplitsThreeItems(t *testing.T) {
	body := `
	{ action: "a", args: ["x", "y"] },
	{ action: "b", value: "f(1, 2)" },
	{ action: "c", then: [{ action: "d" }, { action: "e" }] },
`
	var got []string
	for _, r := range findTopLevelListItems(body) {
		got = append(got, body[r[0]:r[1]])
	}
	want := []string{
		`{ action: "a", args: ["x", "y"] }`,
		`{ action: "b", value: "f(1, 2)" }`,
		`{ action: "c", then: [{ action: "d" }, { action: "e" }] }`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("items:\n got  %q\n want %q", got, want)
	}

	got = got[:0]
	plain := `"a", "b", "c"`
	for _, r := range findTopLevelListItems(plain) {
		got = append(got, plain[r[0]:r[1]])
	}
	if want := []string{`"a"`, `"b"`, `"c"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plain items: got %q want %q", got, want)
	}
}
