package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnsureReleaseGoModule_AllowsIndependentBuild(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "dist", "release", "go-service")
	if err := os.MkdirAll(filepath.Join(target, "cmd", "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "internal", "domain"), 0o755); err != nil {
		t.Fatal(err)
	}

	mainGo := `package main
import "fmt"
func main(){ fmt.Println("ok") }
`
	if err := os.WriteFile(filepath.Join(target, "cmd", "server", "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	if err := ensureReleaseGoModule(target, "github.com/example/release"); err != nil {
		t.Fatalf("ensureReleaseGoModule: %v", err)
	}

	cmdList := exec.Command("go", "list", "-f", "{{.Dir}}", "./cmd/server")
	cmdList.Dir = target
	if out, err := cmdList.CombinedOutput(); err != nil {
		t.Fatalf("go list failed: %v: %s", err, string(out))
	}

	cmdBuild := exec.Command("go", "build", "./cmd/server")
	cmdBuild.Dir = target
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v: %s", err, string(out))
	}
}
