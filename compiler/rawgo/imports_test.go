package rawgo

import (
	"reflect"
	"testing"
)

func TestPackageName(t *testing.T) {
	for path, want := range map[string]string{
		"strconv":                      "strconv",
		"math/rand/v2":                 "rand",
		"encoding/json/v2":             "json",
		"github.com/redis/go-redis/v9": "redis",
		"gopkg.in/yaml.v3":             "yaml",
		"github.com/xuri/excelize/v2":  "excelize",
		"github.com/google/uuid":       "uuid",
	} {
		if got := PackageName(path); got != want {
			t.Errorf("PackageName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestGuessedStdlib(t *testing.T) {
	if got := GuessedStdlib("rand", "Read"); !reflect.DeepEqual(got, []string{"crypto/rand", "math/rand"}) {
		t.Errorf("rand.Read: %v", got)
	}
	// Only math/rand has Intn; math/rand and math/rand/v2 are one family.
	for _, sel := range []string{"Intn", "Perm"} {
		if got := GuessedStdlib("rand", sel); got != nil {
			t.Errorf("rand.%s: %v", sel, got)
		}
	}
	if got := GuessedStdlib("template", "New"); !reflect.DeepEqual(got, []string{"html/template", "text/template"}) {
		t.Errorf("template.New: %v", got)
	}
	if got := GuessedStdlib("strconv", "Itoa"); got != nil {
		t.Errorf("strconv.Itoa: %v", got)
	}
}

func TestFuncBodyRefsSkipsLocalNames(t *testing.T) {
	refs, ok := FuncBodyRefs(`
rand := newSource()
_ = rand.Next()
n, _ := strconv.Atoi(req.ID)
for _, item := range req.Items {
	_ = item.Name
}
_ = n
`)
	if !ok {
		t.Fatal("does not parse")
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.Name+"."+r.Sel)
	}
	want := []string{"strconv.Atoi", "req.ID", "req.Items"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("refs = %v, want %v", got, want)
	}
	if refs[0].Line != 4 {
		t.Errorf("strconv on line %d, want 4", refs[0].Line)
	}
}

func TestStdlibExports(t *testing.T) {
	for _, ref := range [][2]string{{"utf8", "RuneCountInString"}, {"slices", "Contains"}, {"rand", "Read"}, {"strconv", "Itoa"}} {
		if !StdlibExports(ref[0], ref[1]) {
			t.Errorf("%s.%s is a standard package call", ref[0], ref[1])
		}
	}
	for _, ref := range [][2]string{{"rand", "NoSuchFunc"}, {"utf8", "lower"}, {"quote", "Total"}} {
		if StdlibExports(ref[0], ref[1]) {
			t.Errorf("%s.%s is no standard package call", ref[0], ref[1])
		}
	}
}
