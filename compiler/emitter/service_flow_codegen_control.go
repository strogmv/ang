package emitter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepControlLegacyDeprecatedMonolith(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "flow.If":
		cond := arg("condition")
		if cond == "" {
			return ""
		}
		thenSteps := child("_then")
		elseSteps := child("_else")
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
		b.WriteString(renderFlowSteps(cloneFlowState(st), thenSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}", pad))
		if len(elseSteps) > 0 {
			b.WriteString(" else {\n")
			b.WriteString(renderFlowSteps(cloneFlowState(st), elseSteps, indent+1))
			b.WriteString(fmt.Sprintf("%s}", pad))
		}
		b.WriteString("\n")
		return b.String()

	case "flow.For":
		each := arg("each")
		as := arg("as")
		if each == "" {
			return ""
		}
		if as == "" {
			as = "item"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, each))
		b.WriteString(renderFlowSteps(cloneFlowState(st), child("_do"), indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Block", "tx.Block":
		return renderFlowSteps(st, child("_do"), indent)

	case "list.Filter":
		from := arg("from")
		as := arg("as")
		cond := arg("condition")
		out := arg("output")
		if from == "" || out == "" || cond == "" {
			return ""
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
			pad, pad)

	case "list.Paginate":
		in := arg("input")
		off := arg("offset")
		lim := arg("limit")
		out := arg("output")
		if in == "" || off == "" || lim == "" || out == "" {
			return ""
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
		return b.String()

	case "list.Append":
		to := arg("to")
		item := arg("item")
		if to == "" || item == "" {
			return ""
		}
		return fmt.Sprintf("%s%s = append(%s, %s)\n", pad, to, to, item)

	case "list.Sort":
		items := arg("items")
		by := arg("by")
		order := arg("order") // raw: "asc" | "desc" | runtime expr e.g. "req.SortOrder"
		if items == "" || by == "" {
			return ""
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
		return b.String()

	case "str.Normalize":
		in := arg("input")
		mode := strings.ToLower(arg("mode"))
		out := arg("output")
		if in == "" || out == "" {
			return ""
		}
		if !st.declared[out] {
			st.declared[out] = true
		}
		switch mode {
		case "trim":
			return fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, out, in)
		default:
			return fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, out, in)
		}

	case "mapping.Map":
		from := arg("from")
		to := arg("to")
		input := arg("input")
		output := arg("output")
		entity := arg("entity")
		// from/to form: { from: "list", to: "resp.data" } or { from: "req", to: "newAtt", entity: "Foo" }
		if from != "" && to != "" {
			// Export struct field accesses in dot-path
			toRef := to
			if strings.Contains(to, ".") {
				parts := strings.Split(to, ".")
				parts[len(parts)-1] = ExportName(parts[len(parts)-1])
				toRef = strings.Join(parts, ".")
			}
			if entity != "" && !st.declared[to] {
				st.declared[to] = true
				st.pointers[to] = false // value type created via var declaration
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, to, entity))
				b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, to, from))
				b.WriteString(errReturn(st, pad+"\t", "err"))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				return b.String()
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, toRef, from))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String()
		}
		// input/output form: { input: "req.Data", output: "item", entity: "User" }
		if input != "" && output != "" {
			outputRef := output
			if strings.Contains(output, ".") {
				parts := strings.Split(output, ".")
				parts[len(parts)-1] = ExportName(parts[len(parts)-1])
				outputRef = strings.Join(parts, ".")
			}
			if entity != "" && !st.declared[output] {
				st.declared[output] = true
				st.pointers[output] = false // value type created via var declaration
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, output, entity))
				b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, output, input))
				b.WriteString(errReturn(st, pad+"\t", "err"))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				return b.String()
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, outputRef, input))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String()
		}
		// declaration-only form: { output: "newFoo", entity: "Foo" } — just declares var
		if output != "" && entity != "" && !st.declared[output] {
			st.declared[output] = true
			st.pointers[output] = false
			return fmt.Sprintf("%svar %s domain.%s\n", pad, output, entity)
		}
		return ""

	case "event.Publish":
		name := arg("name")
		payload := renderEventPayloadExpr(st, step, name, arg)
		if name == "" || payload == "" {
			return ""
		}
		return fmt.Sprintf("%sif s.publisher != nil {\n%s\t_ = s.publisher.Publish%s(ctx, %s)\n%s}\n",
			pad, pad, ExportName(name), payload, pad)

	case "logic.Call":
		call, callErr := decodeFlowCall(step)
		if callErr != nil || call.Function == "" {
			return ""
		}
		callStr := call.Function + "(" + strings.Join(call.Arguments, ", ") + ")"
		if call.IgnoreError {
			return fmt.Sprintf("%s_, _ = %s\n", pad, callStr)
		}
		if call.Output != "" {
			assign := ":="
			if st.declared[call.Output] {
				assign = "="
			}
			st.declared[call.Output] = true
			st.pointers[call.Output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, call.Output+", err", assign, callStr))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String()
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "service.Call":
		serviceName := strings.TrimSpace(arg("service"))
		methodName := strings.TrimSpace(arg("method"))
		output := arg("output")
		ignoreErr, _ := step.Args["ignoreErr"].(bool)
		if serviceName == "" || methodName == "" {
			return ""
		}
		var callArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				callArgs = append(callArgs, x...)
			case string:
				callArgs = append(callArgs, x)
			}
		}
		if len(callArgs) == 0 || strings.TrimSpace(callArgs[0]) != "ctx" {
			callArgs = append([]string{"ctx"}, callArgs...)
		}
		callStr := fmt.Sprintf("s.%sService.%s(%s)", ExportName(serviceName), ExportName(methodName), strings.Join(callArgs, ", "))
		if ignoreErr {
			return fmt.Sprintf("%s_, _ = %s\n", pad, callStr)
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output+", err", assign, callStr))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String()
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "exec.Run", "exec.Stream":
		cmd := arg("cmd")
		output := arg("output")
		exitCodeVar := arg("exitCodeVar")
		isStream := step.Action == "exec.Stream"
		timeout := arg("timeout")
		if timeout == "" {
			if timeoutMS := flowIntArg(step.Args, "timeoutMs", 0); timeoutMS > 0 {
				timeout = fmt.Sprintf("time.Duration(%d) * time.Millisecond", timeoutMS)
			} else {
				timeout = "120 * time.Second"
			}
		} else if strings.HasPrefix(timeout, `"`) && strings.HasSuffix(timeout, `"`) {
			// string literal like "120s" — parse at codegen time, emit nanosecond constant
			if d, err := time.ParseDuration(timeout[1 : len(timeout)-1]); err == nil {
				timeout = fmt.Sprintf("%d * time.Nanosecond", d.Nanoseconds())
			}
		}
		failOnError := true
		if v, ok := step.Args["failOnError"].(bool); ok {
			failOnError = v
		}
		if cmd == "" {
			return ""
		}
		var cmdArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				cmdArgs = x
			case string:
				if x != "" {
					cmdArgs = []string{x}
				}
			}
		}
		execCtxVar, execCancelVar := "_execCtx"+sfx, "_execCancel"+sfx
		ecv, eov, eerv := "_execCmd"+sfx, "_execOut"+sfx, "_execErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, %s)\n", pad, execCtxVar, execCancelVar, timeout))
		b.WriteString(fmt.Sprintf("%sdefer %s()\n", pad, execCancelVar))
		b.WriteString(fmt.Sprintf("%s%s := exec.CommandContext(%s, %s", pad, ecv, execCtxVar, cmd))
		for _, a := range cmdArgs {
			b.WriteString(fmt.Sprintf(", %s", a))
		}
		b.WriteString(")\n")
		if stdin := arg("stdin"); stdin != "" {
			b.WriteString(fmt.Sprintf("%s%s.Stdin = strings.NewReader(%s)\n", pad, ecv, stdin))
		}
		if isStream {
			pipeReadVar, pipeWriteVar := "_execPipeR"+sfx, "_execPipeW"+sfx
			doneVar, linesVar := "_execDone"+sfx, "_execLines"+sfx
			waitErrVar := "_execWaitErr" + sfx
			b.WriteString(fmt.Sprintf("%s%s, %s := io.Pipe()\n", pad, pipeReadVar, pipeWriteVar))
			b.WriteString(fmt.Sprintf("%s%s.Stdout = %s\n", pad, ecv, pipeWriteVar))
			b.WriteString(fmt.Sprintf("%s%s.Stderr = %s\n", pad, ecv, pipeWriteVar))
			b.WriteString(fmt.Sprintf("%sif %s := %s.Start(); %s != nil {\n", pad, eerv, ecv, eerv))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"exec.Stream start: %%w\", %s)", eerv)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s := make([]string, 0, 64)\n", pad, linesVar))
			b.WriteString(fmt.Sprintf("%s%s := make(chan struct{})\n", pad, doneVar))
			b.WriteString(fmt.Sprintf("%sgo func() {\n", pad))
			b.WriteString(fmt.Sprintf("%s\tdefer close(%s)\n", pad, doneVar))
			b.WriteString(fmt.Sprintf("%s\t_scanner := bufio.NewScanner(%s)\n", pad, pipeReadVar))
			b.WriteString(fmt.Sprintf("%s\t_scannerBuf := make([]byte, 0, 64*1024)\n", pad))
			b.WriteString(fmt.Sprintf("%s\t_scanner.Buffer(_scannerBuf, 1024*1024)\n", pad))
			b.WriteString(fmt.Sprintf("%s\tfor _scanner.Scan() {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t_line := _scanner.Text()\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tslog.Info(\"exec.stream\", \"line\", _line)\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t%s = append(%s, _line)\n", pad, linesVar, linesVar))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s\tif _scanErr := _scanner.Err(); _scanErr != nil {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tslog.Warn(\"exec.stream.scanner\", \"error\", _scanErr)\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t%s = append(%s, \"scanner error: \"+_scanErr.Error())\n", pad, linesVar, linesVar))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s}()\n", pad))
			b.WriteString(fmt.Sprintf("%s%s := %s.Wait()\n", pad, waitErrVar, ecv))
			b.WriteString(fmt.Sprintf("%s_ = %s.Close()\n", pad, pipeWriteVar))
			b.WriteString(fmt.Sprintf("%s<-%s\n", pad, doneVar))
			b.WriteString(fmt.Sprintf("%s_ = %s.Close()\n", pad, pipeReadVar))
			b.WriteString(fmt.Sprintf("%s%s := strings.Join(%s, \"\\n\")\n", pad, eov, linesVar))
			b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, eerv, waitErrVar))
		} else {
			b.WriteString(fmt.Sprintf("%s%s, %s := %s.CombinedOutput()\n", pad, eov, eerv, ecv))
		}
		if exitCodeVar != "" {
			assign := ":="
			if st.declared[exitCodeVar] {
				assign = "="
			}
			st.declared[exitCodeVar] = true
			st.pointers[exitCodeVar] = false
			b.WriteString(fmt.Sprintf("%s%s %s 0\n", pad, exitCodeVar, assign))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n%s\tif _ee, _ok := %s.(*exec.ExitError); _ok { %s = _ee.ExitCode() }\n%s}\n", pad, eerv, pad, eerv, exitCodeVar, pad))
		}
		if failOnError {
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, eerv))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf(`fmt.Errorf("exec: %%s: %%w", string(%s), %s)`, eov, eerv)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, eov))
		}
		return b.String()

	case "fs.TempDir":
		output := arg("output")
		if output == "" {
			return ""
		}
		pattern := arg("pattern")
		if pattern == "" {
			pattern = `"ang-tmp-*"`
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		tdv, tdev := "_tmpDir"+sfx, "_tmpDirErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := os.MkdirTemp(\"\", %s)\n", pad, tdv, tdev, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, tdev))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"temp dir: %w\", "+tdev+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, tdv))
		return b.String()

	case "fs.WriteFile":
		path := arg("path")
		data := arg("data")
		if path == "" || data == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _mkErr := os.MkdirAll(filepath.Dir(%s), 0o755); _mkErr != nil {\n", pad, path))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"mkdir: %w\", _mkErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _wErr := os.WriteFile(%s, []byte(%s), 0o644); _wErr != nil {\n", pad, path, data))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"write file: %w\", _wErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "fs.ReadFile":
		path := arg("path")
		output := arg("output")
		if path == "" || output == "" {
			return ""
		}
		optional := false
		if v, ok := step.Args["optional"].(bool); ok {
			optional = v
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		rfbv, rferrv := "_rfBytes"+sfx, "_rfErr"+sfx
		var b strings.Builder
		if optional {
			b.WriteString(fmt.Sprintf("%s%s, %s := os.ReadFile(%s)\n", pad, rfbv, rferrv, path))
			if assign == ":=" {
				b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, output))
			}
			b.WriteString(fmt.Sprintf("%sif %s == nil { %s = string(%s) }\n", pad, rferrv, output, rfbv))
		} else {
			b.WriteString(fmt.Sprintf("%s%s, %s := os.ReadFile(%s)\n", pad, rfbv, rferrv, path))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rferrv))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"read file: %w\", "+rferrv+")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, rfbv))
		}
		return b.String()

	case "fs.Remove":
		path := arg("path")
		if path == "" {
			return ""
		}
		errVar := "_rmErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s := os.RemoveAll(%s); %s != nil {\n", pad, errVar, path, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"remove path: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Switch":
		value := arg("value")
		if value == "" {
			return ""
		}
		cases, _ := step.Args["_cases"].(map[string][]normalizer.FlowStep)
		defaultSteps := child("_default")
		keys := make([]string, 0, len(cases))
		for k := range cases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, value))
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%scase %q:\n", pad, k))
			b.WriteString(renderFlowSteps(cloneFlowState(st), cases[k], indent+1))
		}
		if len(defaultSteps) > 0 {
			b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
			b.WriteString(renderFlowSteps(cloneFlowState(st), defaultSteps, indent+1))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.While":
		cond := arg("condition")
		if cond == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor %s {\n", pad, cond))
		b.WriteString(renderFlowSteps(cloneFlowState(st), child("_do"), indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Checkpoint":
		name := arg("name")
		if name == "" {
			return ""
		}
		data := arg("data")
		if data == "" {
			data = "map[string]any{\"resp\": resp}"
		}
		keyLit := fmt.Sprintf("%q", name)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _flowCheckpoints == nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_flowCheckpoints = make(map[string]any)\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s_flowCheckpoints[%s] = %s\n", pad, keyLit, data))
		return b.String()

	case "flow.Resume":
		name := arg("name")
		if name == "" {
			return ""
		}
		output := arg("output")
		onMissing := child("_onMissing")
		keyLit := fmt.Sprintf("%q", name)
		ckptValV, ckptOKV := "_ckptVal"+sfx, "_ckptOK"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s any\n", pad, ckptValV))
		b.WriteString(fmt.Sprintf("%s%s := false\n", pad, ckptOKV))
		b.WriteString(fmt.Sprintf("%sif _flowCheckpoints != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s, %s = _flowCheckpoints[%s]\n", pad, ckptValV, ckptOKV, keyLit))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, ckptOKV))
		if len(onMissing) > 0 {
			b.WriteString(renderFlowSteps(cloneFlowState(st), onMissing, indent+1))
		} else {
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"CHECKPOINT_NOT_FOUND\", \"checkpoint %s not found\")", name)))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "any"
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, ckptValV))
		}
		return b.String()

	case "flow.Validate":
		cond := arg("condition")
		if cond == "" {
			return ""
		}
		message := arg("message")
		if message == "" {
			message = arg("throw")
		}
		if message == "" {
			message = "validation failed"
		}
		if hint := arg("hint"); hint != "" {
			message = message + " (hint: " + hint + ")"
		}
		code := arg("code")
		if code == "" {
			code = "VALIDATION_FAILED"
		}
		status := arg("status")
		if status == "" {
			status = "http.StatusBadRequest"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, cond))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", status, code, message)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Try":
		doSteps := child("_do")
		catchSteps := child("_catch")
		if len(doSteps) == 0 {
			return ""
		}
		retries := 0
		if v, ok := step.Args["retries"]; ok {
			switch n := v.(type) {
			case int:
				retries = n
			case int64:
				retries = int(n)
			case float64:
				retries = int(n)
			}
		}
		backoffMs := 0
		if v, ok := step.Args["backoffMs"]; ok {
			switch n := v.(type) {
			case int:
				backoffMs = n
			case int64:
				backoffMs = int(n)
			case float64:
				backoffMs = int(n)
			}
		}

		outerDeclared := make(map[string]bool, len(st.declared))
		for k, v := range st.declared {
			outerDeclared[k] = v
		}
		type newVar struct {
			typ   string
			isPtr bool
		}
		newVars := make(map[string]newVar)
		for _, branch := range [][]normalizer.FlowStep{doSteps, catchSteps} {
			probeState := cloneFlowState(st)
			probeState.returnErrOnly = true
			_ = renderFlowSteps(probeState, branch, indent+1)
			for varName := range probeState.declared {
				if outerDeclared[varName] {
					continue
				}
				goType := probeState.types[varName]
				if goType == "" {
					goType = "any"
				}
				newVars[varName] = newVar{goType, probeState.pointers[varName]}
			}
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		newVarNames := make([]string, 0, len(newVars))
		for n := range newVars {
			newVarNames = append(newVarNames, n)
		}
		sort.Strings(newVarNames)
		for _, varName := range newVarNames {
			v := newVars[varName]
			b.WriteString(fmt.Sprintf("%s\tvar %s %s\n", pad, varName, v.typ))
			st.declared[varName] = true
			st.pointers[varName] = v.isPtr
			st.types[varName] = v.typ
		}

		tryRunV, tryErrV, tryMaxV, tryBackoffV := "_tryRun"+sfx, "_tryErr"+sfx, "_tryMax"+sfx, "_tryBackoff"+sfx
		b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, tryMaxV, retries))
		b.WriteString(fmt.Sprintf("%s\tif %s < 0 { %s = 0 }\n", pad, tryMaxV, tryMaxV))
		b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, tryBackoffV, backoffMs))
		b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, tryRunV))
		tryState := cloneFlowState(st)
		tryState.returnErrOnly = true
		b.WriteString(renderFlowSteps(tryState, doSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, tryErrV))
		b.WriteString(fmt.Sprintf("%s\tfor _tryAttempt := 0; _tryAttempt <= %s; _tryAttempt++ {\n", pad, tryMaxV))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %s()\n", pad, tryErrV, tryRunV))
		b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, tryErrV))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _tryAttempt < %s && %s > 0 {\n", pad, tryMaxV, tryBackoffV))
		b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n", pad, tryBackoffV))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, tryErrV))
		b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, tryErrV))
		if len(catchSteps) > 0 {
			b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+2))
		} else {
			b.WriteString(errReturn(st, pad+"\t\t", tryErrV))
		}
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Catch":
		catchSteps := child("_do")
		if len(catchSteps) == 0 {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _flowLastError != nil {\n", pad))
		b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s\t_flowLastError = nil\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Retry":
		doSteps := child("_do")
		catchSteps := child("_catch")
		if len(doSteps) == 0 {
			return ""
		}
		attempts := 3
		if v, ok := step.Args["attempts"]; ok {
			switch n := v.(type) {
			case int:
				attempts = n
			case int64:
				attempts = int(n)
			case float64:
				attempts = int(n)
			}
		} else if v, ok := step.Args["retries"]; ok {
			switch n := v.(type) {
			case int:
				attempts = n + 1
			case int64:
				attempts = int(n) + 1
			case float64:
				attempts = int(n) + 1
			}
		}
		if attempts <= 0 {
			attempts = 1
		}
		backoffMs := 0
		if v, ok := step.Args["backoffMs"]; ok {
			switch n := v.(type) {
			case int:
				backoffMs = n
			case int64:
				backoffMs = int(n)
			case float64:
				backoffMs = int(n)
			}
		}

		runV, errV, attemptsV, backoffV := "_retryRun"+sfx, "_retryErr"+sfx, "_retryAttempts"+sfx, "_retryBackoff"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, attemptsV, attempts))
		b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, backoffV, backoffMs))
		b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, runV))
		retryState := cloneFlowState(st)
		retryState.returnErrOnly = true
		b.WriteString(renderFlowSteps(retryState, doSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, errV))
		b.WriteString(fmt.Sprintf("%s\tfor _tryAttempt := 0; _tryAttempt < %s; _tryAttempt++ {\n", pad, attemptsV))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %s()\n", pad, errV, runV))
		b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, errV))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _tryAttempt+1 < %s && %s > 0 {\n", pad, attemptsV, backoffV))
		b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n", pad, backoffV))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errV))
		b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, errV))
		if len(catchSteps) > 0 {
			b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+2))
		} else {
			b.WriteString(errReturn(st, pad+"\t\t", errV))
		}
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Fallback":
		mainSteps := child("_do")
		fallbackSteps := child("_fallback")
		if len(mainSteps) == 0 || len(fallbackSteps) == 0 {
			return ""
		}
		runV, errV := "_fbRun"+sfx, "_fbErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, runV))
		fbState := cloneFlowState(st)
		fbState.returnErrOnly = true
		b.WriteString(renderFlowSteps(fbState, mainSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := %s()\n", pad, errV, runV))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errV))
		b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, errV))
		b.WriteString(renderFlowSteps(cloneFlowState(st), fallbackSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Timeout":
		duration := arg("duration")
		doSteps := child("_do")
		onTimeout := child("_onTimeout")
		if duration == "" || len(doSteps) == 0 {
			return ""
		}
		toCtxV, toCancelV, toRunV, toErrV := "_toCtx"+sfx, "_toCancel"+sfx, "_toRun"+sfx, "_toErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := context.WithTimeout(ctx, %s)\n", pad, toCtxV, toCancelV, duration))
		b.WriteString(fmt.Sprintf("%s\tdefer %s()\n", pad, toCancelV))
		b.WriteString(fmt.Sprintf("%s\t%s := func(ctx context.Context) error {\n", pad, toRunV))
		toState := cloneFlowState(st)
		toState.returnErrOnly = true
		b.WriteString(renderFlowSteps(toState, doSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := %s(%s)\n", pad, toErrV, toRunV, toCtxV))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, toErrV))
		b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, toErrV))
		b.WriteString(fmt.Sprintf("%s\t\tif %s.Err() == context.DeadlineExceeded {\n", pad, toCtxV))
		if len(onTimeout) > 0 {
			b.WriteString(renderFlowSteps(cloneFlowState(st), onTimeout, indent+3))
		} else {
			b.WriteString(errReturn(st, pad+"\t\t\t", "errors.New(http.StatusGatewayTimeout, \"TIMEOUT\", \"flow step timed out\")"))
		}
		b.WriteString(fmt.Sprintf("%s\t\t} else {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t\t", toErrV))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Defer":
		doSteps := child("_do")
		if len(doSteps) == 0 {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
		deferState := cloneFlowState(st)
		deferState.deferMode = true
		b.WriteString(renderFlowSteps(deferState, doSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}()\n", pad))
		return b.String()

	case "flow.SuggestNext":
		output := arg("output")
		var options []string
		if v, ok := step.Args["options"]; ok {
			switch x := v.(type) {
			case []string:
				options = append(options, x...)
			case string:
				if strings.TrimSpace(x) != "" {
					options = []string{x}
				}
			}
		}
		if len(options) == 0 {
			return ""
		}
		quoted := make([]string, 0, len(options))
		for _, opt := range options {
			quoted = append(quoted, fmt.Sprintf("%q", opt))
		}
		listExpr := "[]string{" + strings.Join(quoted, ", ") + "}"
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "[]string"
			return fmt.Sprintf("%s%s %s %s\n", pad, output, assign, listExpr)
		}
		return fmt.Sprintf("%sslog.Info(\"flow.suggest_next\", \"options\", %s)\n", pad, listExpr)

	case "flow.ExplainError":
		errExpr := arg("error")
		if errExpr == "" {
			errExpr = "_flowLastError"
		}
		output := arg("output")
		message := arg("message")
		hint := arg("hint")
		expMsgV := "_expMsg" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errExpr))
		b.WriteString(fmt.Sprintf("%s\t%s := fmt.Sprintf(\"flow error: %%v\", %s)\n", pad, expMsgV, errExpr))
		if message != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %q + \": \" + %s\n", pad, expMsgV, message, expMsgV))
		}
		if hint != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s + %q\n", pad, expMsgV, expMsgV, " | hint: "+hint))
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "string"
			b.WriteString(fmt.Sprintf("%s\t%s %s %s\n", pad, output, assign, expMsgV))
		} else {
			b.WriteString(fmt.Sprintf("%s\tslog.Warn(\"flow.explain_error\", \"message\", %s)\n", pad, expMsgV))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	default:
		return ""
	}
}
