package main

import (
	"strings"
	"testing"

	"cuelang.org/go/cue"
	cueast "cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cueparser "cuelang.org/go/cue/parser"
)

func fieldValue(t *testing.T, src, name string) cueast.Expr {
	t.Helper()
	file, err := cueparser.ParseFile("x.cue", src, cueparser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var found cueast.Expr
	cueast.Walk(file, func(n cueast.Node) bool {
		if f, ok := n.(*cueast.Field); ok {
			if label, _, _ := cueast.LabelName(f.Label); label == name {
				found = f.Value
				return false
			}
		}
		return true
	}, nil)
	if found == nil {
		t.Fatalf("field %s not found", name)
	}
	return found
}

// The raw string must evaluate to exactly the same value in CUE.
func TestToRawCUEStringKeepsTheValue(t *testing.T) {
	src := "package api\n\n#Helper: \"helperFn\"\n\nA: {\n\tfunc: \"\"\"\n\t\tmsg := \"a\\\\n\\\"b\\\"\"\n\t\tx := \\(#Helper)(msg)\n\n\t\ttab := \"\\t\"\n\t\t\"\"\"\n}\n"
	value := fieldValue(t, src, "func")
	raw, ok, reason := toRawCUEString([]byte(src), value, "\t\t")
	if !ok {
		t.Fatalf("not converted: %s", reason)
	}
	if !strings.HasPrefix(raw, "#\"\"\"\n") || !strings.HasSuffix(raw, "\n\t\t\"\"\"#") || !strings.Contains(raw, `\#(#Helper)`) {
		t.Fatalf("raw literal:\n%s", raw)
	}
	if strings.Contains(raw, `\\n`) || !strings.Contains(raw, `"a\n"b""`) {
		t.Fatalf("escapes must be decoded:\n%s", raw)
	}

	start, end := value.Pos().Offset(), value.End().Offset()
	converted := src[:start] + raw + src[end:]
	before, _ := cuecontext.New().CompileString(src).LookupPath(cue.ParsePath("A.func")).String()
	afterValue := cuecontext.New().CompileString(converted)
	if err := afterValue.Err(); err != nil {
		t.Fatalf("converted file does not compile: %v\n%s", err, converted)
	}
	after, _ := afterValue.LookupPath(cue.ParsePath("A.func")).String()
	if before != after {
		t.Fatalf("value changed:\nbefore %q\nafter  %q", before, after)
	}
}

func TestToRawCUEStringSkipsWhatItCannotConvert(t *testing.T) {
	src := "package api\n\nA: {\n\tfunc: \"\"\"\n\t\ts := \"\\\\#\"\n\t\t\"\"\"\n\tdone: #\"\"\"\n\t\talready raw\n\t\t\"\"\"#\n}\n"
	if _, ok, reason := toRawCUEString([]byte(src), fieldValue(t, src, "func"), "\t\t"); ok || !strings.Contains(reason, `\#`) {
		t.Fatalf("text with \\# must be skipped with a reason, got ok=%v reason=%q", ok, reason)
	}
	if _, ok, reason := toRawCUEString([]byte(src), fieldValue(t, src, "done"), "\t\t"); ok || reason != "" {
		t.Fatalf("an already raw string is left alone silently, got ok=%v reason=%q", ok, reason)
	}
}
