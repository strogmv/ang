package normalizer

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// A field written as domain.X keeps its entity type even when the referenced
// value is not resolved. The real loader leaves the imported domain package's
// definitions at top; a local domain struct of tops reproduces that.
func TestDetectTypeReadsDomainReferenceFromSyntax(t *testing.T) {
	src := `
domain: {
	APIKey:      _
	Application: _
}
output: {
	data: domain.APIKey
	items: [...domain.Application]
	name: string
	extra: _
}
`
	val := cuecontext.New().CompileString(src, cue.Filename("cue/api/x.cue"))
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	out := val.LookupPath(cue.ParsePath("output"))
	n := New()
	cases := map[string]string{
		"data":  "domain.APIKey",
		"items": "[]domain.Application",
		"name":  "string",
		"extra": "any",
	}
	for field, want := range cases {
		got := n.detectType(field, out.LookupPath(cue.ParsePath(field)))
		if got != want {
			t.Errorf("detectType(%s) = %q, want %q", field, got, want)
		}
	}
}
