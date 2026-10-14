package mcp

import (
	"os/exec"
	"testing"
)

func TestValidateCuePath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"cue/domain/user.cue", false},
		{"cue/api/http.cue", false},
		{"internal/mcp/server.go", true},
		{"main.go", true},
		{"../outside.cue", true},
		{"cue/invalid.txt", true},
	}

	for _, tt := range tests {
		err := validateCuePath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateCuePath(%s) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestValidateReadPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"cue/domain/user.cue", false},
		{"internal/mcp/server.go", false},
		{"go.mod", false},
		{"../outside.go", true},
	}

	for _, tt := range tests {
		err := validateReadPath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateReadPath(%s) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestGitStatusLogic(t *testing.T) {
	// Проверяем, что команда git status --porcelain работает и возвращает что-то вменяемое
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		t.Skip("Git not available or not a git repo")
	}
	t.Logf("Git status output:\n%s", string(out))
}
