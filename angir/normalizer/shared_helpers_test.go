package normalizer

import (
	"reflect"
	"testing"
)

func TestHelperReferencesReadsNamesAndMethodCalls(t *testing.T) {
	byName := map[string]*Helper{
		"countryMarkets": {Name: "countryMarkets", Kind: HelperFunc},
		"market":         {Name: "market", Kind: HelperType},
		"writeAudit":     {Name: "writeAudit", Kind: HelperMethod},
		"unused":         {Name: "unused", Kind: HelperFunc},
	}
	code := `
		var m market
		for _, m = range countryMarkets() {
			_ = m.countryMarkets // a field, not the helper
		}
		_, _ = s.writeAudit(ctx, "x")
		other.unused()
		// writeAudit in a comment is no call
		_ = "unused"
		_ = writeAudit(ctx)
	`
	refs, bare := HelperReferences(code, byName)
	if want := []string{"countryMarkets", "market", "writeAudit"}; !reflect.DeepEqual(refs, want) {
		t.Errorf("refs = %v, want %v", refs, want)
	}
	if want := []string{"writeAudit"}; !reflect.DeepEqual(bare, want) {
		t.Errorf("bare method calls = %v, want %v", bare, want)
	}
}

func TestHelperFileName(t *testing.T) {
	for name, want := range map[string]string{
		"countryMarkets": "helper__country_markets.gen.go",
		"x":              "helper__x.gen.go",
		"eInvoiceXML":    "helper__e_invoice_xml.gen.go",
		"parseHTTPBody":  "helper__parse_http_body.gen.go",
		"tg2Keyboard":    "helper__tg2_keyboard.gen.go",
	} {
		if got := HelperFileName(name); got != want {
			t.Errorf("HelperFileName(%q) = %q, want %q", name, got, want)
		}
	}
}
