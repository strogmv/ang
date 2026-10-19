package flowsem

import (
	"reflect"
	"testing"
)

func TestExprRootsSkipsFunctionsAndPackageCalls(t *testing.T) {
	cases := map[string][]string{
		`isValidIBAN(req.Iban)`: {"req"},
		`utf8.RuneCountInString(strings.TrimSpace(req.Name)) <= max`: {"max", "req"},
		`quote.Total > 0 && quote.Valid()`:                           {"quote"},
		`helper(total) + total`:                                      {"total"},
		`pkgVar.Field.Call()`:                                        {"pkgVar"},
		`cfg.Limit(x) > cfg.Base`:                                    {"cfg", "x"},
	}
	for expr, want := range cases {
		if got := exprRoots(expr); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: roots %v, want %v", expr, got, want)
		}
	}
}
