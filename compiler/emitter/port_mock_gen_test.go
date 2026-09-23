package emitter

import (
	"os"
	"os/exec"
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
		`GetPostFunc  func(ctx context.Context, req port.GetPostRequest) (port.GetPostResponse, error)`,
		`StreamCalls  []MockBlogStreamCall`,
		`var _ port.Blog = (*MockBlog)(nil)`,
		`func NewBlog() *MockBlog`,
		`req port.GetPostRequest`,
		`ttl time.Duration`,
		`type MockBlogGetPostCall struct {`,
		`type MockBlogStreamCall struct {`,
		`func (m *MockBlog) Stream(ctx context.Context, chunks chan<- string, ttl time.Duration) error {`,
		`var ret0 port.GetPostResponse`,
		`var ret1 error`,
		`callsMu sync.Mutex`,
		`m.callsMu.Lock()`,
		`func (m *MockBlog) GetPostCallsSnapshot() []MockBlogGetPostCall {`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated mock missing %q:\n%s", want, out)
		}
	}
}

// A mock shared by goroutines records its calls under a mutex: a test that
// calls it concurrently passes under the race detector, and the exported
// Calls slices a single-goroutine test reads are still there.
func TestEmitPortMocksConcurrentCallsPassRace(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a generated module with -race")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not found")
	}
	root := t.TempDir()
	em := New(root, "", "templates")
	em.GoModule = "example.com/mockrace"

	portDir := filepath.Join(root, "internal", "port")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatalf("mkdir port dir: %v", err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(filepath.Join(portDir, "store.go"), `package port

import "context"

type Store interface {
	Save(ctx context.Context, id string) error
	Ping()
}
`)
	if err := em.EmitPortMocks(); err != nil {
		t.Fatalf("EmitPortMocks failed: %v", err)
	}
	write(filepath.Join(root, "go.mod"), "module example.com/mockrace\n\ngo 1.21\n")
	write(filepath.Join(root, "internal", "adapter", "mock", "race_test.go"), `package mock

import (
	"context"
	"sync"
	"testing"
)

func TestConcurrentCalls(t *testing.T) {
	m := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Save(context.Background(), "x")
			m.Ping()
			_ = m.SaveCallsSnapshot()
		}()
	}
	wg.Wait()
	if len(m.SaveCalls) != 16 || len(m.PingCallsSnapshot()) != 16 {
		t.Fatalf("recorded %d saves, %d pings", len(m.SaveCalls), len(m.PingCalls))
	}
}
`)
	cmd := exec.Command(goBin, "test", "-race", "-count=1", "./internal/adapter/mock/")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "-race requires cgo") {
			t.Skipf("race detector unavailable: %s", out)
		}
		t.Fatalf("generated mock fails under -race: %v\n%s", err, out)
	}
}
