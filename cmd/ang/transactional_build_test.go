package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// transactionalCUE has the three ways an operation gets a transaction:
// transactional: true on a flow, on impls.go code, and s.Tx in raw Go.
const transactionalCUE = `package api

MarkInvoicePaid: {
	service:       "billing"
	transactional: true
	input: {id: string}
	output: {ok: bool}
	sources: {invoice: {kind: "sql", entity: "Invoice"}}
	flow: [
		{action: "repo.Find", source: "Invoice", input: "req.ID", output: "inv", error: "Not Found"},
		{action: "flow.If", condition: "inv.Status == \"paid\"", then: [
			{action: "flow.Return"},
		]},
		{action: "mapping.Assign", to: "inv.Status", value: "\"paid\""},
		{action: "repo.Save", source: "Invoice", input: "inv"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

VoidInvoice: {
	service:       "billing"
	transactional: true
	input: {id: string}
	output: {ok: bool}
	impls: go: {
		lang: "go"
		code: """
			inv, lockErr := s.InvoiceRepo.LockByID(ctx, req.ID)
			if lockErr != nil {
				return resp, lockErr
			}
			if inv == nil {
				return resp, fmt.Errorf("invoice %s not found", req.ID)
			}
			inv.Status = "void"
			if saveErr := s.InvoiceRepo.Save(ctx, inv); saveErr != nil {
				return resp, saveErr
			}
			resp.Ok = true
			return resp, nil
			"""
	}
}

CopyInvoices: {
	service: "billing"
	input: {ids: [...string]}
	output: {copied: int}
	impls: go: {
		lang: "go"
		code: """
			txErr := s.Tx.WithTx(ctx, func(ctx context.Context) error {
				for _, id := range req.IDs {
					inv, findErr := s.InvoiceRepo.FindByID(ctx, id)
					if findErr != nil {
						return findErr
					}
					if inv == nil {
						continue
					}
					inv.ID = uuid.NewString()
					if saveErr := s.InvoiceRepo.Save(ctx, inv); saveErr != nil {
						return saveErr
					}
					resp.Copied++
				}
				return nil
			})
			return resp, txErr
			"""
	}
}
`

// Item 18 of docs/tech-debt-backlog.md: transactional: true wraps an
// operation in one transaction (its flow through the tx.Block renderer, its
// Go in s.txManager.WithTx), and s.Tx is the TxManager under a public name.
// The generated service and repository packages must compile.
func TestTransactionalOperationsCompile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	projectDir := filepath.Join(t.TempDir(), "txapp")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "txapp",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/txapp",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectDir, "cue", "api", "tx.cue"), transactionalCUE)
	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}); err != nil {
		t.Fatalf("build: %v", err)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(projectDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for file, wants := range map[string][]string{
		"internal/service/billing__mark_invoice_paid.gen.go": {"if err := s.txManager.WithTx(ctx, func(ctx context.Context) error {"},
		"internal/service/billing__void_invoice.gen.go": {
			"err = s.txManager.WithTx(ctx, func(ctx context.Context) error {",
			"resp, txErr = func() (resp port.VoidInvoiceResponse, err error) {",
		},
		"internal/service/billing_impl.gen.go":    {"Tx          port.TxManager", "txManager: txManager, Tx: txManager}"},
		"internal/bootstrap/runtime_container.go": {"repoInvoice,\n\t\ttxManager,"},
	} {
		got := read(file)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s lacks %q:\n%s", file, want, got)
			}
		}
	}

	// Compile what the operations generate, offline, with ANG's own
	// dependency versions (the module cache has them).
	repoRoot, _ := filepath.Abs("../..")
	angMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	requires := string(angMod)[strings.Index(string(angMod), "require"):]
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/txapp\n\ngo 1.27.0\n\n"+requires)
	sum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectDir, "go.sum"), string(sum))
	cmd := exec.Command("go", "vet", "./internal/service/", "./internal/adapter/repository/postgres/")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "module lookup disabled") {
			t.Skipf("module cache lacks a dependency: %s", out)
		}
		t.Fatalf("generated packages do not compile: %v\n%s", err, out)
	}
}
