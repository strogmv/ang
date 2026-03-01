package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepControlCollections(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, _ func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "list.Filter":
		from := arg("from")
		as := arg("as")
		cond := arg("condition")
		out := arg("output")
		if from == "" || out == "" || cond == "" {
			return "", true
		}
		if as == "" {
			as = "item"
		}
		assign := ":="
		if st.declared[out] {
			assign = "="
		}
		st.declared[out] = true
		return fmt.Sprintf("%s%s %s %s[:0:0]\n%sfor _, %s := range %s {\n%s\tif %s {\n%s\t\t%s = append(%s, %s)\n%s\t}\n%s}\n",
			pad, out, assign, from,
			pad, as, from,
			pad, cond,
			pad, out, out, as,
			pad, pad), true

	case "list.Paginate":
		in := arg("input")
		off := arg("offset")
		lim := arg("limit")
		out := arg("output")
		if in == "" || off == "" || lim == "" || out == "" {
			return "", true
		}
		assign := ":="
		if st.declared[out] {
			assign = "="
		}
		st.declared[out] = true
		st.pointers[out] = false
		defaultLimit := 50
		if v, ok := step.Args["defaultLimit"]; ok {
			switch n := v.(type) {
			case int:
				defaultLimit = n
			case int64:
				defaultLimit = int(n)
			case float64:
				defaultLimit = int(n)
			}
		}
		ov, lv, sv, ev := "_off"+sfx, "_lim"+sfx, "_start"+sfx, "_end"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, ov, off))
		b.WriteString(fmt.Sprintf("%sif %s < 0 { %s = 0 }\n", pad, ov, ov))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, lv, lim))
		b.WriteString(fmt.Sprintf("%sif %s <= 0 { %s = %d }\n", pad, lv, lv, defaultLimit))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, sv, ov))
		b.WriteString(fmt.Sprintf("%sif %s > len(%s) { %s = len(%s) }\n", pad, sv, in, sv, in))
		b.WriteString(fmt.Sprintf("%s%s := %s + %s\n", pad, ev, sv, lv))
		b.WriteString(fmt.Sprintf("%sif %s > len(%s) { %s = len(%s) }\n", pad, ev, in, ev, in))
		b.WriteString(fmt.Sprintf("%s%s %s %s[%s:%s]\n", pad, out, assign, in, sv, ev))
		return b.String(), true

	case "list.Append":
		to := arg("to")
		item := arg("item")
		if to == "" || item == "" {
			return "", true
		}
		return fmt.Sprintf("%s%s = append(%s, %s)\n", pad, to, to, item), true

	case "list.Sort":
		items := arg("items")
		by := arg("by")
		order := arg("order") // raw: "asc" | "desc" | runtime expr e.g. "req.SortOrder"
		if items == "" || by == "" {
			return "", true
		}
		var b strings.Builder
		orderLower := strings.ToLower(order)
		isDynamic := order != "" && orderLower != "asc" && orderLower != "desc"
		if isDynamic {
			b.WriteString(fmt.Sprintf("%ssort.Slice(%s, func(i, j int) bool {\n", pad, items))
			b.WriteString(fmt.Sprintf("%s\tif strings.ToLower(%s) == \"desc\" { return %s[i].%s > %s[j].%s }\n", pad, order, items, by, items, by))
			b.WriteString(fmt.Sprintf("%s\treturn %s[i].%s < %s[j].%s\n", pad, items, by, items, by))
			b.WriteString(fmt.Sprintf("%s})\n", pad))
		} else {
			cmp := "<"
			if orderLower == "desc" {
				cmp = ">"
			}
			b.WriteString(fmt.Sprintf("%ssort.Slice(%s, func(i, j int) bool { return %s[i].%s %s %s[j].%s })\n", pad, items, items, by, cmp, items, by))
		}
		return b.String(), true

	case "str.Normalize":
		in := arg("input")
		mode := strings.ToLower(arg("mode"))
		out := arg("output")
		if in == "" || out == "" {
			return "", true
		}
		if !st.declared[out] {
			st.declared[out] = true
		}
		switch mode {
		case "trim":
			return fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, out, in), true
		default:
			return fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, out, in), true
		}
	}

	return "", false
}
