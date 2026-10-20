package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const managedAccountCUE = `package domain

Account: {
	@owner(auth)
	id:           string
	email:        string
	role:         string
	passwordHash: string @secret()
	adminRoles?: [...string] @managed()
}
`

const managedAccountOpsCUE = `package api

import "github.com/example/keepapp/cue/domain"

AdminUpdateAccount: {
	service: "auth"
	input: {id: string, data: domain.Account}
	output: {ok: bool}
	sources: {account: {kind: "sql", entity: "Account"}}
	flow: [
		{action: "repo.Find", source: "Account", input: "req.ID", output: "item", error: "Not found"},
		{action: "mapping.Map", input: "req.Data", output: "item", entity: "Account"},
		{action: "repo.Save", source: "Account", input: "item"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

AdminCreateAccount: {
	service: "auth"
	input: {data: domain.Account}
	output: {ok: bool}
	sources: {account: {kind: "sql", entity: "Account"}}
	flow: [
		{action: "mapping.Map", input: "req.Data", output: "fresh", entity: "Account"},
		{action: "repo.Save", source: "Account", input: "fresh"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
`

// assignKeepingTest runs in the generated helpers package: the body's values
// land, the kept fields keep the row's, also when the body decodes through
// JSON into the same backing array.
const assignKeepingTest = `package helpers

import "testing"

type account struct {
	ID           string
	Email        string
	PasswordHash string ` + "`json:\"-\"`" + `
	AdminRoles   []string
}

type accountBody struct {
	ID         string
	Email      string
	AdminRoles []string
}

func TestAssignKeepingLeavesProtectedFields(t *testing.T) {
	item := &account{ID: "a1", Email: "old@x", PasswordHash: "$2a$hash", AdminRoles: []string{"support"}}
	body := account{ID: "a1", Email: "new@x", AdminRoles: []string{"superadmin"}}
	if err := AssignKeeping(&item, body, "PasswordHash", "AdminRoles"); err != nil {
		t.Fatal(err)
	}
	if item.Email != "new@x" || item.PasswordHash != "$2a$hash" || len(item.AdminRoles) != 1 || item.AdminRoles[0] != "support" {
		t.Fatalf("same type: %+v", item)
	}
	item = &account{ID: "a1", Email: "old@x", PasswordHash: "$2a$hash", AdminRoles: []string{"support"}}
	if err := AssignKeeping(&item, accountBody{ID: "a1", Email: "new@x", AdminRoles: []string{"superadmin"}}, "PasswordHash", "AdminRoles"); err != nil {
		t.Fatal(err)
	}
	if item.Email != "new@x" || item.PasswordHash != "$2a$hash" || len(item.AdminRoles) != 1 || item.AdminRoles[0] != "support" {
		t.Fatalf("through JSON: %+v", item)
	}
	var none *account
	if err := AssignKeeping(&none, body, "AdminRoles"); err != nil || none == nil || none.AdminRoles[0] != "superadmin" {
		t.Fatalf("no row loaded: nothing to keep, got %+v %v", none, err)
	}
}
`

// dealingi-back's generic console row write (AdminUpdateUser: repo.Find,
// then mapping.Map of the body onto the row) let the body write the console
// roles, and a body of the same type wiped the password hash and second
// factor. mapping.Map onto a loaded row now keeps the entity's @secret and
// @managed() fields; a new row (create) is mapped as before.
func TestMappingABodyOntoALoadedRowKeepsProtectedFields(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and tests a generated project")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	projectDir := filepath.Join(t.TempDir(), "keepapp")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "keepapp",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/keepapp",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectDir, "cue", "domain", "account.cue"), managedAccountCUE)
	writeTestFile(t, filepath.Join(projectDir, "cue", "api", "accounts.cue"), managedAccountOpsCUE)
	writeTestFile(t, filepath.Join(projectDir, "cue", "repo", "accounts.cue"), "package repo\n\nRepositories: Account: {finders: [{name: \"ListAccounts\", returns: \"many\"}]}\n")
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
	update := read("internal/service/auth__admin_update_account.gen.go")
	if !strings.Contains(update, `helpers.AssignKeeping(&item, req.Data, "PasswordHash", "AdminRoles")`) {
		t.Fatalf("the row write keeps the secret and the managed field:\n%s", update)
	}
	create := read("internal/service/auth__admin_create_account.gen.go")
	if strings.Contains(create, "AssignKeeping") || !strings.Contains(create, "helpers.Assign(&fresh, req.Data)") {
		t.Fatalf("a new row is mapped as before:\n%s", create)
	}
	writeTestFile(t, filepath.Join(projectDir, "internal", "pkg", "helpers", "assign_keeping_test.go"), assignKeepingTest)
	runGoOffline(t, projectDir, "github.com/example/keepapp", "test", "-count=1", "-run", "TestAssignKeepingLeavesProtectedFields", "./internal/pkg/helpers/")
	vetGeneratedPackagesOffline(t, projectDir, "github.com/example/keepapp", "./internal/service/")
}
