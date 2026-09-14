package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatEmbeddedGoFuncLiteralIsFormattedOnceAndKeepsInterpolation(t *testing.T) {
	src := `package api

#Impls: {
	GetThing: flow: [{
		action: "logic.Call"
		func: """
			(func(ctx context.Context,id string) (string,error) {
			if id=="" {
			return "",nil
			}
			return \(#Helper)(id),nil
			})
			"""
	}]
}
`
	want := `package api

#Impls: {
	GetThing: flow: [{
		action: "logic.Call"
		func: """
			(func(ctx context.Context, id string) (string, error) {
				if id == "" {
					return "", nil
				}
				return \(#Helper)(id), nil
			})
			"""
	}]
}
`
	got, changed, skips := formatEmbeddedGo("impl.cue", []byte(src))
	if len(skips) != 0 || changed != 1 {
		t.Fatalf("changed=%d skips=%+v", changed, skips)
	}
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	again, changed, skips := formatEmbeddedGo("impl.cue", got)
	if changed != 0 || len(skips) != 0 || string(again) != want {
		t.Fatalf("second pass changed=%d skips=%+v:\n%s", changed, skips, again)
	}
}

func TestFormatEmbeddedGoCodeStatements(t *testing.T) {
	src := "package api\n\nX: impl: {\n\tcode: \"\"\"\n\t\tlimit:=req.Limit\n\t\tif limit<=0 {\n\t\tlimit=20\n\t\t}\n\n\t\treturn resp,nil\n\t\t\"\"\"\n}\n"
	want := "package api\n\nX: impl: {\n\tcode: \"\"\"\n\t\tlimit := req.Limit\n\t\tif limit <= 0 {\n\t\t\tlimit = 20\n\t\t}\n\n\t\treturn resp, nil\n\t\t\"\"\"\n}\n"
	got, changed, skips := formatEmbeddedGo("impl.cue", []byte(src))
	if changed != 1 || len(skips) != 0 || string(got) != want {
		t.Fatalf("changed=%d skips=%+v\ngot:\n%s", changed, skips, got)
	}
}

func TestFormatEmbeddedGoDefinitionFuncLiteral(t *testing.T) {
	src := "package api\n\n#AddOneFunc: \"\"\"\n\t(func(a int) int {\n\treturn a+1\n\t})\n\t\"\"\"\n\n#Caption: \"\"\"\n\tHello,   buyer\n\t\"\"\"\n"
	want := "package api\n\n#AddOneFunc: \"\"\"\n\t(func(a int) int {\n\t\treturn a + 1\n\t})\n\t\"\"\"\n\n#Caption: \"\"\"\n\tHello,   buyer\n\t\"\"\"\n"
	got, changed, skips := formatEmbeddedGo("helpers.cue", []byte(src))
	if changed != 1 || len(skips) != 0 || string(got) != want {
		t.Fatalf("changed=%d skips=%+v\ngot:\n%s", changed, skips, got)
	}
}

func TestFormatEmbeddedGoLeavesOtherStringsAlone(t *testing.T) {
	src := `package api

A: {
	// A value assembled from pieces is not Go on its own.
	func: """
		(func() {
		x:=1
		""" + "\n" + #Block + "\n" + """
		})
		"""
	sql: """
		SELECT  *   FROM t
		"""
	code: "x:=1"
}
`
	got, changed, skips := formatEmbeddedGo("impl.cue", []byte(src))
	if changed != 0 || len(skips) != 0 || string(got) != src {
		t.Fatalf("changed=%d skips=%+v\n%s", changed, skips, got)
	}
}

func TestFormatEmbeddedGoReportsWhatItCannotFormat(t *testing.T) {
	src := "package api\n\nA: {\n\tfunc: \"\"\"\n\t\t(func( {\n\t\t\"\"\"\n\tB: code: \"\"\"\n\t\tx:=1\n\n\n\t\treturn x\n\t\t\"\"\"\n}\n"
	got, changed, skips := formatEmbeddedGo("impl.cue", []byte(src))
	if changed != 0 || string(got) != src {
		t.Fatalf("changed=%d\n%s", changed, got)
	}
	if len(skips) != 2 {
		t.Fatalf("skips = %+v", skips)
	}
	if skips[0].Field != "func" || skips[0].Line != 4 || !strings.Contains(skips[0].Reason, "not a Go expression") {
		t.Fatalf("invalid func skip = %+v", skips[0])
	}
	if skips[1].Field != "code" || !strings.Contains(skips[1].Reason, "number of lines") {
		t.Fatalf("line-count skip = %+v", skips[1])
	}
}

func TestSameDecodedGoTokens(t *testing.T) {
	quote := func(body string) string { return "\"\"\"\n" + body + "\n\"\"\"" }
	// Only whitespace between tokens changes: the same program.
	if !sameDecodedGoTokens(quote(`x :=  "a\\n"+y`), quote(`x := "a\\n" + y`)) {
		t.Fatal("whitespace between tokens must count as the same Go")
	}
	// In CUE source `"a\\"` is a closed Go string, but decoded it is `"a\"`:
	// the quote is escaped and the spaces after it belong to the string.
	if sameDecodedGoTokens(quote(`s := "a\\"  +  "b"`), quote(`s := "a\\" + "b"`)) {
		t.Fatal("a change inside a decoded string must not count as the same Go")
	}
}

func TestFormatCueTreeGoOnlyKeepsCueLayout(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "impl.cue")
	// cue fmt would realign lang; --go-only must not.
	src := "package api\n\nA: {\n\tlang:        \"go\"\n\tfunc: \"\"\"\n\t\t(func() int {\n\t\treturn 1\n\t\t})\n\t\t\"\"\"\n}\n"
	want := "package api\n\nA: {\n\tlang:        \"go\"\n\tfunc: \"\"\"\n\t\t(func() int {\n\t\t\treturn 1\n\t\t})\n\t\t\"\"\"\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := formatCueTreeWith(root, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesChanged != 1 || res.EmbeddedGoFormatted != 1 {
		t.Fatalf("result = %+v", res)
	}
	assertTestFile(t, path, want)
}

func TestFormatCueTreeFormatsEmbeddedGo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "impl.cue")
	src := "package api\n\nA: {\n\tfunc: \"\"\"\n\t\t(func() int {\n\t\treturn 1\n\t\t})\n\t\t\"\"\"\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := formatCueTree(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesChanged != 1 || res.EmbeddedGoFormatted != 1 {
		t.Fatalf("check result = %+v", res)
	}
	assertTestFile(t, path, src)

	if _, err := formatCueTree(root, false); err != nil {
		t.Fatal(err)
	}
	formatted, _ := os.ReadFile(path)
	if !strings.Contains(string(formatted), "\t\t(func() int {\n\t\t\treturn 1\n\t\t})\n") {
		t.Fatalf("not formatted:\n%s", formatted)
	}
	res, err = formatCueTree(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesChanged != 0 {
		t.Fatalf("second check result = %+v", res)
	}
}
