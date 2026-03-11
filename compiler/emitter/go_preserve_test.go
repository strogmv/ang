package emitter

import (
	"strings"
	"testing"
)

func TestMergeGoCustomBlocksPreservesExistingBody(t *testing.T) {
	generated := `package bootstrap
// ANG:BEGIN_CUSTOM runtime_container.after_init
// default
// ANG:END_CUSTOM runtime_container.after_init
`
	existing := `package bootstrap
// ANG:BEGIN_CUSTOM runtime_container.after_init
fmt.Println("keep")
// ANG:END_CUSTOM runtime_container.after_init
`
	merged := mergeGoCustomBlocks(generated, existing)
	if !strings.Contains(merged, `fmt.Println("keep")`) {
		t.Fatalf("expected preserved custom body, got:\n%s", merged)
	}
}

func TestShouldPreserveGoCustomBlocks(t *testing.T) {
	if !shouldPreserveGoCustomBlocks("internal/bootstrap/runtime_container.go") {
		t.Fatal("runtime container should preserve custom blocks")
	}
	if shouldPreserveGoCustomBlocks("internal/service/user.gen.go") {
		t.Fatal("generated service methods must not preserve custom blocks")
	}
}
