package paymentprovider

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var catalogSourcePattern = regexp.MustCompile(`"([a-z0-9_]+)"`)
var resolveCasePattern = regexp.MustCompile(`(?m)^\tcase "([a-z0-9_]+)"`)

func TestCatalogFieldSourcesMatchResolve(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)

	catalog := mustRead(t, filepath.Join(dir, "schema", "catalogs.cue"))
	start := strings.Index(catalog, "#CatalogFieldSource:")
	end := strings.Index(catalog, "#CatalogOwnerInfoKey:")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("could not slice #CatalogFieldSource")
	}
	catalogSrc := uniqueQuoted(catalogSourcePattern.FindAllStringSubmatch(catalog[start:end], -1))

	resolve := mustRead(t, filepath.Join(dir, "resolve.go"))
	fnStart := strings.Index(resolve, "func resolveSource(")
	if fnStart < 0 {
		t.Fatal("resolveSource not found")
	}
	rest := resolve[fnStart+1:]
	fnEndRel := strings.Index(rest, "\nfunc ")
	if fnEndRel < 0 {
		t.Fatal("resolveSource end not found")
	}
	resolveSrc := uniqueQuoted(resolveCasePattern.FindAllStringSubmatch(resolve[fnStart:fnStart+1+fnEndRel], -1))

	onlyCatalog := missingFrom(catalogSrc, resolveSrc)
	onlyResolve := missingFrom(resolveSrc, catalogSrc)
	if len(onlyCatalog) > 0 || len(onlyResolve) > 0 {
		t.Fatalf("catalog not in resolve: %v\nresolve not in catalog: %v", onlyCatalog, onlyResolve)
	}
}

func TestPaymentMethodGoConst_knownSIDs(t *testing.T) {
	name, err := paymentMethodGoConst("cards")
	if err != nil || name != "CardsMethod" {
		t.Fatalf("cards: %q %v", name, err)
	}
	name, err = paymentMethodGoConst("mastercard")
	if err != nil || name != "MastercarddMethod" {
		t.Fatalf("mastercard: %q %v", name, err)
	}
	if _, err := paymentMethodGoConst("not-a-method"); err == nil {
		t.Fatal("expected error for unknown sid")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func uniqueQuoted(matches [][]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if len(m) < 2 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func missingFrom(have, against []string) []string {
	set := map[string]bool{}
	for _, v := range against {
		set[v] = true
	}
	var out []string
	for _, v := range have {
		if !set[v] {
			out = append(out, v)
		}
	}
	return out
}
