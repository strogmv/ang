package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// renderFlowStepControlLegacy is a thin compatibility dispatch:
// keep the actively maintained legacy handlers in a compact core, then
// fallback to the historical monolith for any remaining edge action.
func renderFlowStepControlLegacy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	if out, ok := renderFlowStepControlLegacyCore(st, step, indent, sfx, arg); ok {
		return out
	}
	return renderFlowStepControlLegacyDeprecated(st, step, indent, sfx, arg, child)
}

func renderFlowStepControlLegacyCoreMonolith(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "mapping.Map":
		from := arg("from")
		to := arg("to")
		input := arg("input")
		output := arg("output")
		entity := arg("entity")
		if from != "" && to != "" {
			toRef := to
			if strings.Contains(to, ".") {
				parts := strings.Split(to, ".")
				parts[len(parts)-1] = ExportName(parts[len(parts)-1])
				toRef = strings.Join(parts, ".")
			}
			if entity != "" && !st.declared[to] {
				st.declared[to] = true
				st.pointers[to] = false
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, to, entity))
				b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, to, from))
				b.WriteString(errReturn(st, pad+"\t", "err"))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				return b.String(), true
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, toRef, from))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		if input != "" && output != "" {
			outputRef := output
			if strings.Contains(output, ".") {
				parts := strings.Split(output, ".")
				parts[len(parts)-1] = ExportName(parts[len(parts)-1])
				outputRef = strings.Join(parts, ".")
			}
			if entity != "" && !st.declared[output] {
				st.declared[output] = true
				st.pointers[output] = false
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, output, entity))
				b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, output, input))
				b.WriteString(errReturn(st, pad+"\t", "err"))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				return b.String(), true
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, outputRef, input))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		if output != "" && entity != "" && !st.declared[output] {
			st.declared[output] = true
			st.pointers[output] = false
			return fmt.Sprintf("%svar %s domain.%s\n", pad, output, entity), true
		}
		return "", true

	case "event.Publish":
		name := arg("name")
		payload := arg("payload")
		if name == "" || payload == "" {
			return "", true
		}
		payload = normalizePayloadExpr(payload)
		return fmt.Sprintf("%sif s.publisher != nil {\n%s\t_ = s.publisher.Publish%s(ctx, %s)\n%s}\n",
			pad, pad, ExportName(name), payload, pad), true

	case "logic.Call":
		funcExpr := arg("func")
		output := arg("output")
		if funcExpr == "" {
			return "", true
		}
		var callArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				callArgs = x
			case string:
				callArgs = []string{x}
			}
		}
		callStr := funcExpr + "(" + strings.Join(callArgs, ", ") + ")"
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
			return b.String(), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := %s; err != nil {\n", pad, callStr))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "exec.Run":
		cmd := arg("cmd")
		output := arg("output")
		exitCodeVar := arg("exitCodeVar")
		failOnError := true
		if v, ok := step.Args["failOnError"].(bool); ok {
			failOnError = v
		}
		if cmd == "" {
			return "", true
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
		ecv, eov, eerv := "_execCmd"+sfx, "_execOut"+sfx, "_execErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := exec.CommandContext(ctx, %s", pad, ecv, cmd))
		for _, a := range cmdArgs {
			b.WriteString(fmt.Sprintf(", %s", a))
		}
		b.WriteString(")\n")
		if stdin := arg("stdin"); stdin != "" {
			b.WriteString(fmt.Sprintf("%s%s.Stdin = strings.NewReader(%s)\n", pad, ecv, stdin))
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := %s.CombinedOutput()\n", pad, eov, eerv, ecv))
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
			b.WriteString(fmt.Sprintf("%s\t"+`return resp, fmt.Errorf("exec: %%s: %%w", string(%s), %s)`+"\n", pad, eov, eerv))
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
		return b.String(), true

	case "fs.TempDir":
		output := arg("output")
		if output == "" {
			return "", true
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
		tdv, tdev := "_tmpDir"+sfx, "_tmpDirErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := os.MkdirTemp(\"\", %s)\n", pad, tdv, tdev, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, tdev))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"temp dir: %w\", "+tdev+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, tdv))
		return b.String(), true

	case "fs.WriteFile":
		path := arg("path")
		data := arg("data")
		if path == "" || data == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _mkErr := os.MkdirAll(filepath.Dir(%s), 0o755); _mkErr != nil {\n", pad, path))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"mkdir: %w\", _mkErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _wErr := os.WriteFile(%s, []byte(%s), 0o644); _wErr != nil {\n", pad, path, data))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"write file: %w\", _wErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "fs.ReadFile":
		path := arg("path")
		output := arg("output")
		if path == "" || output == "" {
			return "", true
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
		return b.String(), true

	case "fs.Remove":
		path := arg("path")
		if path == "" {
			return "", true
		}
		return fmt.Sprintf("%sdefer os.RemoveAll(%s)\n", pad, path), true
	}

	return "", false
}
