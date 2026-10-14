package normalizer

import (
	"strings"
	"testing"
)

func TestExtractServicesReadsTypedQueryParams(t *testing.T) {
	t.Parallel()

	svcs, err := extractTestServices(t, `
ListOrders: {
	service: "orders"
	input: {
		companyId: string
		status:    string
		from:      string @query(type="date")
		at:        string @query(type="time", format="2006-01-02 15:04")
		page:      int @query(required=true)
		ids:       [...string] @query()
		tags:      string @query(type="[]string", format="csv", name="tag")
	}
	output: {total: int}
}`)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]Field{}
	for _, f := range svcs[0].Methods[0].Input.Fields {
		fields[f.Name] = f
	}
	for name, want := range map[string]struct {
		goType, kind, format, key string
		required                  bool
	}{
		"from": {"time.Time", QueryTypeDate, "2006-01-02", "", false},
		"at":   {"time.Time", QueryTypeTime, "2006-01-02 15:04", "", false},
		"page": {"int", QueryTypeInt, "", "", true},
		"ids":  {"[]string", QueryTypeStrings, "", "", false},
		"tags": {"[]string", QueryTypeStrings, "csv", "tag", false},
	} {
		f := fields[name]
		spec := FieldQueryParam(f.Metadata)
		if spec == nil || f.Type != want.goType || spec.Type != want.kind || spec.Format != want.format || spec.Name != want.key || spec.Required != want.required {
			t.Fatalf("%s: type %q spec %+v, want %+v", name, f.Type, spec, want)
		}
	}
	for _, plain := range []string{"companyId", "status"} {
		if f := fields[plain]; f.Type != "string" || f.Metadata != nil {
			t.Fatalf("%s has no @query and stays a plain string: %+v", plain, f)
		}
	}
}

func TestExtractServicesRejectsBadQueryParams(t *testing.T) {
	t.Parallel()

	for name, field := range map[string]string{
		"unknown type":      `x: string @query(type="uuid")`,
		"format on int":     `x: int @query(format="2006")`,
		"bad list format":   `x: [...string] @query(format="pipes")`,
		"bad required flag": `x: string @query(required=yes)`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := extractTestServices(t, `List: {service: "orders", input: {`+field+`}, output: {n: int}}`)
			if err == nil || !strings.Contains(err.Error(), "@query") {
				t.Fatalf("want an @query error, got %v", err)
			}
		})
	}
}
