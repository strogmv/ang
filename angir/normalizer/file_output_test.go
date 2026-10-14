package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func extractTestServices(t *testing.T, src string) ([]Service, error) {
	t.Helper()
	v := cuecontext.New().CompileString(src)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}
	return New().ExtractServices(v, nil)
}

func TestExtractServicesReadsFileOutput(t *testing.T) {
	t.Parallel()

	svcs, err := extractTestServices(t, `
ExportOrders: {
	service: "orders"
	input: {companyId: string}
	output: {kind: "file", contentType: "text/csv; charset=utf-8", filename: "orders.csv"}
}
ShowInvoice: {
	service: "orders"
	output: {kind: "file", contentType: "application/pdf", disposition: "inline"}
}
ListOrders: {
	service: "orders"
	output: {kind: string, total: int}
}`)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]Method{}
	for _, m := range svcs[0].Methods {
		methods[m.Name] = m
	}
	export := MethodFileOutput(methods["ExportOrders"].Metadata)
	if export == nil || export.ContentType != "text/csv; charset=utf-8" || export.Filename != "orders.csv" || export.Disposition != "attachment" {
		t.Fatalf("ExportOrders file output = %+v", export)
	}
	var names []string
	for _, f := range methods["ExportOrders"].Output.Fields {
		if !f.IsSecret {
			t.Fatalf("file fields never travel as JSON, %s is not marked", f.Name)
		}
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "body,content,contentType,filename,size" {
		t.Fatalf("file output fields = %v", names)
	}
	if show := MethodFileOutput(methods["ShowInvoice"].Metadata); show == nil || show.Disposition != "inline" || show.Filename != "" {
		t.Fatalf("ShowInvoice file output = %+v", show)
	}
	if MethodFileOutput(methods["ListOrders"].Metadata) != nil || len(methods["ListOrders"].Output.Fields) != 2 {
		t.Fatalf("a response with a kind field stays a JSON response: %+v", methods["ListOrders"].Output)
	}
}

func TestExtractServicesRejectsBadFileOutput(t *testing.T) {
	t.Parallel()

	for name, output := range map[string]string{
		"extra field":     `{kind: "file", rows: int}`,
		"bad disposition": `{kind: "file", disposition: "download"}`,
		"filename slash":  `{kind: "file", filename: "a/b.csv"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := extractTestServices(t, `Export: {service: "orders", output: `+output+`}`)
			if err == nil || !strings.Contains(err.Error(), "output for Export") {
				t.Fatalf("want an error naming Export, got %v", err)
			}
		})
	}
}
