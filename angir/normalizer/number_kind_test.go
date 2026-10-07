package normalizer

import (
	"strings"
	"testing"
)

// CUE's `number` (int | float) is a float64 everywhere — top level, in a list
// item and in a nested object — never a string (billing's VAT rate and churn).
func TestCueNumberIsFloat64AtEveryLevel(t *testing.T) {
	out := compileOutput(t, `
output: {
	churnRate: number
	opt?:      number
	byDefault: number | *0
	rows: [...{vatRatePercent: number, maybe?: number}]
	totals: {rate: number}
}
`)
	ent, err := New().parseEntity("BillingResponse", out)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"churnRate", "opt", "byDefault"} {
		if got := fieldByName(t, ent.Fields, name).Type; got != "float64" {
			t.Errorf("%s = %q, want float64", name, got)
		}
	}
	rows := fieldByName(t, ent.Fields, "rows")
	for _, name := range []string{"vatRatePercent", "maybe"} {
		if got := fieldByName(t, rows.ItemFields, name).Type; got != "float64" {
			t.Errorf("rows[].%s = %q, want float64", name, got)
		}
	}
	totals := fieldByName(t, ent.Fields, "totals")
	if got := fieldByName(t, totals.ItemFields, "rate").Type; got != "float64" {
		t.Errorf("totals.rate = %q, want float64", got)
	}
}

// Next to a field named `number` (an invoice number), CUE resolves a bare
// `number` to that sibling field, not to the type: billing's invoice rows
// typed vatRatePercent as a string. ANG stops with the fix instead.
func TestSiblingFieldShadowingAPredeclaredTypeIsAnError(t *testing.T) {
	out := compileOutput(t, `
output: {
	invoices: [...{
		number:         string
		vatRatePercent: number
	}]
}
`)
	_, err := New().parseEntity("BillingResponse", out)
	if err == nil {
		t.Fatal("want an error for vatRatePercent: number next to a field named number")
	}
	for _, want := range []string{"vatRatePercent", "number", "__number"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

// The field named number itself and the escaped __number next to it are fine.
func TestEscapedPredeclaredTypeNextToItsNamesake(t *testing.T) {
	out := compileOutput(t, `
output: {
	invoices: [...{
		number:         string
		vatRatePercent: __number
		count:          int
	}]
}
`)
	ent, err := New().parseEntity("BillingResponse", out)
	if err != nil {
		t.Fatal(err)
	}
	rows := fieldByName(t, ent.Fields, "invoices")
	if got := fieldByName(t, rows.ItemFields, "vatRatePercent").Type; got != "float64" {
		t.Errorf("vatRatePercent = %q, want float64", got)
	}
	if got := fieldByName(t, rows.ItemFields, "number").Type; got != "string" {
		t.Errorf("number = %q, want string", got)
	}
}
