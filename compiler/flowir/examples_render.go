package flowir

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
)

// ExampleCUE returns the example step of an action written as a CUE flow step,
// the way it appears in impl files.
func ExampleCUE(name string) (string, bool) {
	example, ok := actionExamples[name]
	if !ok {
		return "", false
	}
	return stepCUE(example), true
}

var cueIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func cueLabel(name string) string {
	if cueIdentifier.MatchString(name) {
		return name
	}
	return strconv.Quote(name)
}

func stepCUE(step normalizer.FlowStep) string {
	keys := make([]string, 0, len(step.Args))
	for key := range step.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{"action: " + strconv.Quote(step.Action)}
	for _, key := range keys {
		// Nested steps are written without the underscore the parser adds.
		parts = append(parts, cueLabel(strings.TrimPrefix(key, "_"))+": "+valueCUE(step.Args[key]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func valueCUE(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case []string:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, strconv.Quote(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, valueCUE(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case []normalizer.FlowStep:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, stepCUE(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string][]normalizer.FlowStep:
		return mapCUE(len(v), func(yield func(string, string)) {
			for key, steps := range v {
				yield(key, valueCUE(steps))
			}
		})
	case map[string]map[string]string:
		return mapCUE(len(v), func(yield func(string, string)) {
			for key, inner := range v {
				fields := make(map[string]any, len(inner))
				for k, s := range inner {
					fields[k] = s
				}
				yield(key, valueCUE(fields))
			}
		})
	case map[string]any:
		return mapCUE(len(v), func(yield func(string, string)) {
			for key, item := range v {
				yield(key, valueCUE(item))
			}
		})
	case nil:
		return "null"
	}
	return strconv.Quote(fmt.Sprint(value))
}

func mapCUE(size int, each func(yield func(string, string))) string {
	rendered := make(map[string]string, size)
	each(func(key, value string) { rendered[key] = value })
	keys := make([]string, 0, len(rendered))
	for key := range rendered {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, cueLabel(key)+": "+rendered[key])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
