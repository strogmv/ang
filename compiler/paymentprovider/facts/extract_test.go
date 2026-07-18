package ppfacts_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	ppfacts "github.com/strogmv/ang/compiler/paymentprovider/facts"
)

func TestExtractDeterministicFromMinimalFixture(t *testing.T) {
	dir := t.TempDir()
	if err := copyTree(filepath.Join("..", "testdata", "minimal"), dir); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	goSrc := `package testpp

import providers "example.com/providers"

type PPTest struct{}

func (pp *PPTest) InitPay(mc, ps, tx any) (*any, error) {
	return nil, providers.ErrNotImplemented
}
`
	if err := os.WriteFile(filepath.Join(dir, "testpp.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "behavior_test.go"), []byte("package testpp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := ppfacts.Extract(ppfacts.ExtractOptions{ProjectPath: dir, CueRoot: ".cue"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	second, err := ppfacts.Extract(ppfacts.ExtractOptions{ProjectPath: dir, CueRoot: ".cue"})
	if err != nil {
		t.Fatalf("Extract second: %v", err)
	}
	json1, err := ppfacts.CanonicalJSON(first)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	json2, err := ppfacts.CanonicalJSON(second)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	if string(json1) != string(json2) {
		t.Fatalf("facts not deterministic")
	}
	assertHasSemanticFact(t, first, "pp_provider", "testpp", "testpp")
	assertHasSemanticFact(t, first, "pp_capability", "payin", "true")
	assertHasSemanticFact(t, first, "pp_operation", "init_pay", "stub")
}

func TestExtractMarksCallbackOpsDeclaredFromCallbackIntent(t *testing.T) {
	dir := t.TempDir()
	if err := copyTree(filepath.Join("..", "testdata", "pumpp2p"), dir); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	goSrc := `package pumpp2p

import (
	"net/http"

	"gitlab.q-tech.host/transferty/backend/tnx_processor/model"
)

type PPPumpP2P struct{}

func (pp *PPPumpP2P) ParseCallback(r *http.Request) (*model.CallbackData, error) { return nil, nil }
func (pp *PPPumpP2P) ValidateCallback(mc *model.MerchantCreds, tx *model.Transaction, cd *model.CallbackData) error {
	return nil
}
func (pp *PPPumpP2P) FinishCallback(mc *model.MerchantCreds, tx *model.Transaction, cd *model.CallbackData) (*model.StatusResponse, error) {
	return nil, nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "pumpp2p.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := ppfacts.Extract(ppfacts.ExtractOptions{ProjectPath: dir, CueRoot: ".cue"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, operation := range []string{"parse_callback", "validate_callback", "finish_callback"} {
		assertOperationFact(t, env, operation, "declared", "implemented")
	}
}

func TestExtractMarksCheckStatusDeclaredFromProfileConfig(t *testing.T) {
	incas := "/Users/m.ramanchak/Desktop/transferty/tnx_processor/payment_providers/incas_n692"
	if _, err := os.Stat(incas); err != nil {
		t.Skip("incas_n692 fixture not available")
	}
	goSrc := `package incas_n692

import "gitlab.q-tech.host/transferty/backend/tnx_processor/model"

type PPIncasN692 struct{}

func (pp *PPIncasN692) CheckStatus(mc *model.MerchantCreds, tx *model.Transaction) (*model.StatusResponse, error) {
	return nil, nil
}
`
	if err := os.WriteFile(filepath.Join(incas, "checkstatus_probe.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(incas, "checkstatus_probe.go")) })

	env, err := ppfacts.Extract(ppfacts.ExtractOptions{ProjectPath: incas, CueRoot: ".cue"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	assertOperationFact(t, env, "check_status", "declared", "implemented")
}

func assertOperationFact(t *testing.T, env ppfacts.Envelope, operation, declaration, implementation string) {
	t.Helper()
	for _, fact := range env.Facts {
		if fact.Predicate != "pp_operation" {
			continue
		}
		var op, decl, impl string
		for _, term := range fact.Terms {
			switch term.Sort {
			case "operation":
				op = term.Value
			case "declaration":
				decl = term.Value
			case "implementation":
				impl = term.Value
			}
		}
		if op == operation && decl == declaration && impl == implementation {
			return
		}
	}
	t.Fatalf("missing pp_operation operation=%s declaration=%s implementation=%s", operation, declaration, implementation)
}

func assertHasSemanticFact(t *testing.T, env ppfacts.Envelope, predicate, key, value string) {
	t.Helper()
	for _, fact := range env.Facts {
		if fact.Predicate != predicate {
			continue
		}
		hasKey := false
		hasValue := false
		for _, term := range fact.Terms {
			if term.Value == key {
				hasKey = true
			}
			if term.Value == value {
				hasValue = true
			}
		}
		if hasKey && hasValue {
			return
		}
	}
	t.Fatalf("missing fact predicate=%s key=%s value=%s", predicate, key, value)
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
