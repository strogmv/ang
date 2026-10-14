package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Item 4 of docs/tech-debt-backlog.md: the imports of raw Go were checked
// only by go build of the output, after goimports had dropped the unused ones
// and guessed the missing ones.
func TestSemanticPhasesCheckRawGoImports(t *testing.T) {
	const op = `package api

RenameUser: {
	service: "user"
	input: {id: string, name: string}
	output: {ok: bool}
	impls: go: {
		lang: "go"
		imports: ["encoding/json", "strings"]
		code: """
			name := strings.TrimSpace(req.Name)
			if name == "" {
				return resp, nil
			}
			n, _ := strconv.Atoi(req.ID)
			_ = n
			resp.Ok = true
			return resp, nil
			"""
	}
}

TokenForUser: {
	service: "user"
	input: {id: string}
	output: {token: string}
	flow: [
		{
			action: "logic.Call"
			func: """
				(func() (string, error) {
					buf := make([]byte, 16)
					if _, err := rand.Read(buf); err != nil {
						return "", err
					}
					return string(buf), nil
				})
				"""
			output: "token"
		},
		{action: "mapping.Assign", to: "resp.Token", value: "token"},
	]
}
`
	base := copyMinimalProject(t)
	if err := os.WriteFile(filepath.Join(base, "cue", "api", "rename.cue"), []byte(op), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err == nil {
		t.Fatalf("expected import errors, got none")
	}
	var ce *ContractError
	if !asContractError(err, &ce) || ce.Code != ErrCodeRawGoImport {
		t.Fatalf("expected ContractError %s, got %#v", ErrCodeRawGoImport, err)
	}
	msg := err.Error()
	for _, want := range []string{
		`rename.cue:9: User.RenameUser: import "encoding/json" is not used`,
		`rename.cue:15: User.RenameUser: strconv.Atoi uses package strconv, which is not in imports; add "strconv"`,
		`rename.cue:33: User.TokenForUser: rand.Read exists in "crypto/rand" and "math/rand"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, `"strings"`) {
		t.Errorf("strings is used, it must not be reported:\n%s", msg)
	}
}

// Declaring what the code uses passes; imports: on a logic.Call is how a
// lambda names the package goimports would otherwise guess.
func TestSemanticPhasesAcceptDeclaredRawGoImports(t *testing.T) {
	const op = `package api

RenameUser: {
	service: "user"
	input: {id: string, name: string}
	output: {ok: bool}
	impls: go: {
		lang: "go"
		imports: ["strconv", "strings"]
		code: """
			name := strings.TrimSpace(req.Name)
			n, _ := strconv.Atoi(req.ID)
			resp.Ok = name != "" && n > 0
			return resp, nil
			"""
	}
}

TokenForUser: {
	service: "user"
	input: {id: string}
	output: {token: string}
	flow: [
		{
			action: "logic.Call"
			imports: ["crypto/rand"]
			func: """
				(func() (string, error) {
					buf := make([]byte, 16)
					if _, err := rand.Read(buf); err != nil {
						return "", err
					}
					return strconv.Quote(string(buf)), nil
				})
				"""
			output: "token"
		},
		{action: "mapping.Assign", to: "resp.Token", value: "token"},
	]
}
`
	base := copyMinimalProject(t)
	if err := os.WriteFile(filepath.Join(base, "cue", "api", "rename.cue"), []byte(op), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunSemanticPhasesWithOptions(base, PipelineOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
