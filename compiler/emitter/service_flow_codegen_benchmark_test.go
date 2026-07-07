package emitter

import (
	"fmt"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func BenchmarkRenderLargeFlow(b *testing.B) {
	steps := make([]normalizer.FlowStep, 0, 1000)
	for i := 0; i < 1000; i++ {
		steps = append(steps, normalizer.FlowStep{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    fmt.Sprintf("resp.Field%d", i),
				"value": fmt.Sprintf("req.Field%d", i),
			},
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderFlowForService("Benchmark", steps)
	}
}
