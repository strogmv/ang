package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

// A finder field that is present but does not decode must be an error, not an
// empty string: the finder would be generated without its sql.
func TestExtractRepoFindersReportsUndecodableField(t *testing.T) {
	val := cuecontext.New().CompileString(`
Repositories: {
	Tender: {
		finders: [{
			name:    "Search"
			returns: "many"
			sql:     42
		}]
	}
}
`)
	_, err := New().ExtractRepoFinders(val)
	if err == nil {
		t.Fatalf("expected an error for sql: 42")
	}
	for _, want := range []string{"Tender", `finder "Search"`, "sql"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

func TestExtractRepoFindersKeepsOptionalFieldsEmpty(t *testing.T) {
	val := cuecontext.New().CompileString(`
#Finder: {name: string, returns: *"many" | "one", sql?: string, order_by?: string}
Repositories: Tender: finders: [#Finder & {name: "List"}]
`)
	got, err := New().ExtractRepoFinders(val)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got["Tender"]) != 1 || got["Tender"][0].Returns != "many" || got["Tender"][0].CustomSQL != "" {
		t.Fatalf("unexpected finders: %#v", got)
	}
}
