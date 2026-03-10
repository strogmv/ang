package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitPortMocks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	em := New(root, "", "templates")
	em.GoModule = "example.com/project"

	portDir := filepath.Join(root, "internal", "port")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatalf("mkdir port dir: %v", err)
	}
	src := `package port

import (
	"context"
	"time"
)

type GetPostRequest struct {
	ID string
}

type GetPostResponse struct {
	OK bool
}

type Blog interface {
	GetPost(ctx context.Context, req GetPostRequest) (GetPostResponse, error)
	Stream(ctx context.Context, chunks chan<- string, ttl time.Duration) error
}
`
	if err := os.WriteFile(filepath.Join(portDir, "blog.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write port file: %v", err)
	}

	if err := em.EmitPortMocks(); err != nil {
		t.Fatalf("EmitPortMocks failed: %v", err)
	}

	mockPath := filepath.Join(root, "internal", "adapter", "mock", "blog.gen.go")
	data, err := os.ReadFile(mockPath)
	if err != nil {
		t.Fatalf("read mock file: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		`type MockBlog struct {`,
		`var _ port.Blog = (*MockBlog)(nil)`,
		`func NewBlog() *MockBlog`,
		`req port.GetPostRequest`,
		`ttl time.Duration`,
		`type MockBlogGetPostCall struct {`,
		`var ret0 port.GetPostResponse`,
		`var ret1 error`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated mock missing %q:\n%s", want, out)
		}
	}
}
