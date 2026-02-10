package normalizer

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestDetectType_ListStringAliasesStayStringSlice(t *testing.T) {
	ctx := cuecontext.New()
	v := ctx.CompileString(`
package test

#MutedType: "email" | "sms" | "digest"

notificationMutedTypes: [#MutedType]
scopes: [...string]
`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	muted := n.detectType("notificationMutedTypes", v.LookupPath(cuePath("notificationMutedTypes")))
	if muted != "[]string" {
		t.Fatalf("notificationMutedTypes type = %q, want []string", muted)
	}

	scopes := n.detectType("scopes", v.LookupPath(cuePath("scopes")))
	if scopes != "[]string" {
		t.Fatalf("scopes type = %q, want []string", scopes)
	}
}

func cuePath(path string) cue.Path {
	return cue.ParsePath(path)
}
