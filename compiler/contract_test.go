package compiler

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapContractError(t *testing.T) {
	root := errors.New("root")
	err := WrapContractError(StageCUE, "CUE_TEST_ERROR", "load cue/test", root)
	if err == nil {
		t.Fatalf("expected wrapped error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "[CUE:CUE_TEST_ERROR]") {
		t.Fatalf("missing stage/code in error: %s", msg)
	}
	if !strings.Contains(msg, "load cue/test") {
		t.Fatalf("missing op in error: %s", msg)
	}
	if !errors.Is(err, root) {
		t.Fatalf("wrapped error should unwrap to root cause")
	}
}
