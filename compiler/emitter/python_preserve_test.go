package emitter

import (
	"strings"
	"testing"
)

func TestMergePythonCustomBlocks_PreservesExistingBody(t *testing.T) {
	generated := `class S:
    async def m(self):
        # ANG:BEGIN_CUSTOM S.m
        raise NotImplementedError("Implement S.m")
        # ANG:END_CUSTOM S.m
`
	existing := `class S:
    async def m(self):
        # ANG:BEGIN_CUSTOM S.m
        return 42
        # ANG:END_CUSTOM S.m
`
	merged := mergePythonCustomBlocks(generated, existing)
	if merged == generated {
		t.Fatalf("expected merged content to differ from generated")
	}
	if !strings.Contains(merged, "return 42") {
		t.Fatalf("expected preserved custom body, got:\n%s", merged)
	}
	if !strings.Contains(merged, "ANG:BEGIN_CUSTOM S.m") || !strings.Contains(merged, "ANG:END_CUSTOM S.m") {
		t.Fatalf("expected custom markers to remain, got:\n%s", merged)
	}
}

func TestMergePythonCustomBlocks_KeepsGeneratedWhenNoExistingBlock(t *testing.T) {
	generated := `# ANG:BEGIN_CUSTOM A.m
raise NotImplementedError("x")
# ANG:END_CUSTOM A.m
`
	existing := `# no custom markers`
	merged := mergePythonCustomBlocks(generated, existing)
	if merged != generated {
		t.Fatalf("expected generated unchanged")
	}
}
