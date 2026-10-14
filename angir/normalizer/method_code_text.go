package normalizer

import (
	"regexp"
	"sort"
	"strings"
)

var goRepoReference = regexp.MustCompile(`\bs\.([A-Z][A-Za-z0-9_]*)Repo\b`)

// GoRepoReferences lists, once each and in order of appearance, the entity
// part of every repository field Go code reaches on the service: Ticket for
// s.TicketRepo.
func GoRepoReferences(code string) []string {
	seen := map[string]bool{}
	var out []string
	for _, match := range goRepoReference.FindAllStringSubmatch(code, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			out = append(out, match[1])
		}
	}
	return out
}

// MethodCodeText is every piece of Go an operation carries: its impl code and
// each string argument of its flow steps (logic.Call funcs, values,
// conditions, targets), nested branches included, one per line, and the code
// of the helpers it reaches (Method.HelperCode).
func MethodCodeText(m Method) string {
	var b strings.Builder
	if m.Impl != nil {
		b.WriteString(m.Impl.Code)
		b.WriteByte('\n')
	}
	for _, step := range m.Flow {
		writeFlowStepText(&b, step)
	}
	if m.HelperCode != "" {
		b.WriteString(m.HelperCode)
	}
	return b.String()
}

// FlowStepText is the Go of one flow step's own arguments, without the steps
// nested in its branches.
func FlowStepText(step FlowStep) string {
	var b strings.Builder
	writeFlowStepArgs(&b, step, false)
	return b.String()
}

func writeFlowStepText(b *strings.Builder, step FlowStep) {
	writeFlowStepArgs(b, step, true)
}

func writeFlowStepArgs(b *strings.Builder, step FlowStep, nested bool) {
	keys := make([]string, 0, len(step.Args))
	for k := range step.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch step.Args[k].(type) {
		case []FlowStep, map[string][]FlowStep:
			if !nested {
				continue
			}
		}
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
