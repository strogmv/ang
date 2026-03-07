package flowsem

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	reExampleHeader = regexp.MustCompile(`(?m)^// EXAMPLE\s+(\d+):`)
	reAction        = regexp.MustCompile(`action:\s*"([^"]+)"`)
)

// TestGoldenExamplesCoverCriticalOrchestrationActions ensures that
// cue/GOLDEN_EXAMPLES.cue contains executable reference examples for
// high-risk orchestration actions we rely on in production.
func TestGoldenExamplesCoverCriticalOrchestrationActions(t *testing.T) {
	t.Parallel()

	goldenPath := filepath.Join("..", "..", "cue", "GOLDEN_EXAMPLES.cue")
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read %s: %v", goldenPath, err)
	}
	content := string(raw)

	type exBlock struct {
		number int
		start  int
		end    int
	}

	headers := reExampleHeader.FindAllStringSubmatchIndex(content, -1)
	if len(headers) == 0 {
		t.Fatalf("no EXAMPLE headers found in %s", goldenPath)
	}

	blocks := make([]exBlock, 0, len(headers))
	for i, m := range headers {
		numStr := content[m[2]:m[3]]
		n, convErr := strconv.Atoi(strings.TrimSpace(numStr))
		if convErr != nil {
			t.Fatalf("parse example number %q: %v", numStr, convErr)
		}
		start := m[0]
		end := len(content)
		if i+1 < len(headers) {
			end = headers[i+1][0]
		}
		blocks = append(blocks, exBlock{number: n, start: start, end: end})
	}

	actionExamples := map[string][]int{}
	for _, b := range blocks {
		chunk := content[b.start:b.end]
		matches := reAction.FindAllStringSubmatch(chunk, -1)
		for _, m := range matches {
			action := strings.TrimSpace(m[1])
			if action == "" {
				continue
			}
			actionExamples[action] = append(actionExamples[action], b.number)
		}
	}

	required := []string{
		"flow.Race",
		"flow.Saga",
		"flow.Compensate",
		"flow.Rollback",
		"approval.Wait",
		"fsm.Transition",
	}

	missing := make([]string, 0)
	for _, action := range required {
		nums := actionExamples[action]
		if len(nums) == 0 {
			missing = append(missing, action)
			continue
		}
		sort.Ints(nums)
		actionExamples[action] = uniqInts(nums)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("golden coverage missing actions in %s: %s", goldenPath, strings.Join(missing, ", "))
	}

	// Helpful trace in test logs for CI diagnostics.
	lines := make([]string, 0, len(required))
	for _, action := range required {
		lines = append(lines, fmt.Sprintf("%s -> EXAMPLE %v", action, actionExamples[action]))
	}
	t.Log(strings.Join(lines, "; "))
}

func uniqInts(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	out := []int{in[0]}
	for i := 1; i < len(in); i++ {
		if in[i] != in[i-1] {
			out = append(out, in[i])
		}
	}
	return out
}
