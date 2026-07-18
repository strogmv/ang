package ppfacts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyStubAndImplemented(t *testing.T) {
	dir := t.TempDir()
	writeTestGoFile(t, dir, "provider.go", `package sample

import providers "example.com/providers"

type PP struct{}

func (pp *PP) InitPay(mc, ps, tx any) (*any, error) {
	return nil, providers.ErrNotImplemented
}

func (pp *PP) InitPayout(mc, ps, tx any) (*any, error) {
	if true {
		return nil, nil
	}
	return nil, nil
}
`)
	methods, err := classifyProviderMethods(dir, "PP")
	if err != nil {
		t.Fatalf("classifyProviderMethods: %v", err)
	}
	if methods["init_pay"] != "stub" {
		t.Fatalf("init_pay = %q", methods["init_pay"])
	}
	if methods["init_payout"] != "implemented" {
		t.Fatalf("init_payout = %q", methods["init_payout"])
	}
}

func TestCryptoBehaviorRequiresImport(t *testing.T) {
	dir := t.TempDir()
	writeTestGoFile(t, dir, "sign.go", `package sample

import (
	"crypto/rsa"
)

func verify(key *rsa.PublicKey, hash []byte, sig []byte) error {
	return rsa.VerifyPKCS1v15(key, 0, hash, sig)
}

func noise() {
	_ = "rsa.VerifyPKCS1v15"
}
`)
	methods, err := classifyProviderMethods(dir, "PP")
	if err != nil {
		t.Fatalf("classifyProviderMethods: %v", err)
	}
	if methods["behavior:rsa_pkcs1v15_callback_verification"] != "present" {
		t.Fatal("expected rsa callback behavior")
	}
}

func writeTestGoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
