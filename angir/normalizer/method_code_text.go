package normalizer

import (
	"sort"
	"strings"
)

// MethodCodeText is every piece of Go an operation carries: its impl code and
// each string argument of its flow steps (logic.Call funcs, values,
// conditions, targets), nested branches included, one per line.
func MethodCodeText(m Method) string {
	var b strings.Builder
	if m.Impl != nil {
		b.WriteString(m.Impl.Code)
		b.WriteByte('\n')
	}
	for _, step := range m.Flow {
		writeFlowStepText(&b, step)
	}
	return b.String()
}

func writeFlowStepText(b *strings.Builder, step FlowStep) {
	keys := make([]string, 0, len(step.Args))
	for k := range step.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeFlowArgText(b, step.Args[k])
	}
}

func writeFlowArgText(b *strings.Builder, v any) {
	switch value := v.(type) {
	case string:
		b.WriteString(value)
		b.WriteByte('\n')
	case []string:
		for _, s := range value {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	case []any:
		for _, item := range value {
			writeFlowArgText(b, item)
		}
	case map[string]any:
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeFlowArgText(b, value[k])
		}
	case map[string]string:
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(value[k])
			b.WriteByte('\n')
		}
	case []FlowStep:
		for _, step := range value {
			writeFlowStepText(b, step)
		}
	case map[string][]FlowStep:
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, step := range value[k] {
				writeFlowStepText(b, step)
			}
		}
	}
}
