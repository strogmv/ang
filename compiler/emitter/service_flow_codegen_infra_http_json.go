package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepInfraHTTPAndSerialization(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "http.Call":
		if out, ok := renderFlowHTTPCallAST(st, step, indent, sfx, arg); ok {
			return out, true
		}
		method := arg("method")
		url := arg("url")
		body := arg("body")
		output := arg("output")
		statusVar := arg("statusVar")
		if method == "" || url == "" {
			return "", true
		}
		failOnError := true
		if v, ok := step.Args["failOnError"].(bool); ok {
			failOnError = v
		}
		httpReqV, httpReqErrV := "_httpReq"+sfx, "_httpReqErr"+sfx
		httpResV, httpErrV := "_httpRes"+sfx, "_hErr"+sfx
		httpBodyV := "_httpBody" + sfx
		var b strings.Builder
		bodyExpr := "nil"
		if body != "" {
			bodyExpr = fmt.Sprintf("strings.NewReader(%s)", body)
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(ctx, %q, %s, %s)\n", pad, httpReqV, httpReqErrV, method, url, bodyExpr))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, httpReqErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http: request: %w\", "+httpReqErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if hdrs, ok := step.Args["headers"].(map[string]string); ok {
			for hk, hv := range hdrs {
				b.WriteString(fmt.Sprintf("%s%s.Header.Set(%q, %s)\n", pad, httpReqV, hk, hv))
			}
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, httpResV, httpErrV, httpReqV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, httpErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http: %w\", "+httpErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, httpResV))
		b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, httpBodyV, httpResV))
		if statusVar != "" {
			assign := ":="
			if st.declared[statusVar] {
				assign = "="
			}
			st.declared[statusVar] = true
			st.pointers[statusVar] = false
			b.WriteString(fmt.Sprintf("%s%s %s %s.StatusCode\n", pad, statusVar, assign, httpResV))
		}
		if failOnError {
			b.WriteString(fmt.Sprintf("%sif %s.StatusCode >= 400 {\n", pad, httpResV))
			b.WriteString(errReturn(st, pad+"\t", "errors.New("+httpResV+".StatusCode, \"HTTP_ERROR\", string("+httpBodyV+"))"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, httpBodyV))
		}
		return b.String(), true

	case "rand.Code":
		output := arg("output")
		if output == "" {
			return "", true
		}
		length := 6
		if v, ok := step.Args["length"]; ok {
			switch n := v.(type) {
			case int:
				length = n
			case int64:
				length = int(n)
			case float64:
				length = int(n)
			}
		}
		modBase := "_codeBase" + sfx
		codeNVar := "_codeN" + sfx
		codeBufVar := "_codeBuf" + sfx
		outV := "_codeOut" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := 1\n", pad, modBase))
		b.WriteString(fmt.Sprintf("%sfor _i := 0; _i < %d; _i++ { %s *= 10 }\n", pad, length, modBase))
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, 8)\n", pad, codeBufVar))
		b.WriteString(fmt.Sprintf("%sif _, _cErr := cryptorand.Read(%s); _cErr != nil {\n", pad, codeBufVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"rand.Code: %w\", _cErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := int(binary.BigEndian.Uint64(%s) %% uint64(%s))\n", pad, codeNVar, codeBufVar, modBase))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"%%0%dd\", %s)\n", pad, outV, length, codeNVar))
		b.WriteString(renderFlowAssignTarget(st, pad, output, outV, "string"))
		return b.String(), true

	case "rand.Token":
		output := arg("output")
		if output == "" {
			return "", true
		}
		nbytes := 32
		if v, ok := step.Args["bytes"]; ok {
			switch n := v.(type) {
			case int:
				nbytes = n
			case int64:
				nbytes = int(n)
			case float64:
				nbytes = int(n)
			}
		}
		rbv, rerrv := "_rb"+sfx, "_rbErr"+sfx
		outV := "_tokenOut" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, %d)\n", pad, rbv, nbytes))
		b.WriteString(fmt.Sprintf("%s_, %s := cryptorand.Read(%s)\n", pad, rerrv, rbv))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rerrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"rand.Token: %w\", "+rerrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := hex.EncodeToString(%s)\n", pad, outV, rbv))
		b.WriteString(renderFlowAssignTarget(st, pad, output, outV, "string"))
		return b.String(), true

	case "str.Format":
		tmpl := arg("template")
		output := arg("output")
		if tmpl == "" || output == "" {
			return "", true
		}
		var fmtArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				fmtArgs = x
			case []interface{}:
				for _, it := range x {
					if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
						fmtArgs = append(fmtArgs, s)
					}
				}
			case string:
				if x != "" {
					fmtArgs = []string{x}
				}
			}
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		callArgs := tmpl
		if len(fmtArgs) > 0 {
			callArgs += ", " + strings.Join(fmtArgs, ", ")
		}
		return fmt.Sprintf("%s%s %s fmt.Sprintf(%s)\n", pad, output, assign, callArgs), true

	case "str.Concat":
		output := arg("output")
		if output == "" {
			return "", true
		}
		sep := arg("sep")
		var parts []string
		if v, ok := step.Args["parts"]; ok {
			switch x := v.(type) {
			case []string:
				parts = x
			case []interface{}:
				for _, it := range x {
					if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
						parts = append(parts, s)
					}
				}
			case string:
				if strings.TrimSpace(x) != "" {
					parts = []string{x}
				}
			}
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		if len(parts) == 0 {
			return fmt.Sprintf("%s%s %s \"\"\n", pad, output, assign), true
		}
		if sep == "" {
			concat := make([]string, 0, len(parts))
			for _, p := range parts {
				concat = append(concat, fmt.Sprintf("fmt.Sprint(%s)", p))
			}
			return fmt.Sprintf("%s%s %s %s\n", pad, output, assign, strings.Join(concat, " + ")), true
		}
		tmp := "_concatParts" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := []string{}\n", pad, tmp))
		for _, p := range parts {
			b.WriteString(fmt.Sprintf("%s%s = append(%s, fmt.Sprint(%s))\n", pad, tmp, tmp, p))
		}
		b.WriteString(fmt.Sprintf("%s%s %s strings.Join(%s, %s)\n", pad, output, assign, tmp, sep))
		return b.String(), true

	case "str.StripMarkdown":
		input := arg("input")
		output := arg("output")
		if input == "" {
			return "", true
		}
		if output == "" {
			output = input
		}
		sfxVar := "_sm" + sfx
		assign := ":="
		if output != input && st.declared[output] {
			assign = "="
		}
		if output != input {
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "string"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// str.StripMarkdown\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, sfxVar, input))
		b.WriteString(fmt.Sprintf("%sif strings.HasPrefix(%s, \"```\") {\n", pad, sfxVar))
		b.WriteString(fmt.Sprintf("%s\t%sLines := strings.SplitN(%s, \"\\n\", 2)\n", pad, sfxVar, sfxVar))
		b.WriteString(fmt.Sprintf("%s\tif len(%sLines) > 1 {\n", pad, sfxVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %sLines[1]\n", pad, sfxVar, sfxVar))
		b.WriteString(fmt.Sprintf("%s\t\tif _smEnd := strings.LastIndex(%s, \"```\"); _smEnd >= 0 {\n", pad, sfxVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = strings.TrimSpace(%s[:_smEnd])\n", pad, sfxVar, sfxVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if output == input {
			b.WriteString(fmt.Sprintf("%s%s = strings.TrimSpace(%s)\n", pad, input, sfxVar))
		} else {
			b.WriteString(fmt.Sprintf("%s%s %s strings.TrimSpace(%s)\n", pad, output, assign, sfxVar))
		}
		return b.String(), true

	case "cast.ToString":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return "", true
		}
		format := arg("format")
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		if format != "" {
			return fmt.Sprintf("%s%s %s fmt.Sprintf(%s, %s)\n", pad, output, assign, format, input), true
		}
		return fmt.Sprintf("%s%s %s fmt.Sprint(%s)\n", pad, output, assign, input), true

	case "json.Parse":
		input := arg("input")
		into := arg("into")
		output := arg("output")
		if input == "" || into == "" || output == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, into))
		b.WriteString(fmt.Sprintf("%sif _jErr := json.Unmarshal([]byte(%s), &%s); _jErr != nil {\n", pad, input, output))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"json: %w\", _jErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		st.declared[output] = true
		st.pointers[output] = false
		return b.String(), true

	case "json.Marshal":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return "", true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		jbv, jerrv := "_jb"+sfx, "_jErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(%s)\n", pad, jbv, jerrv, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, jerrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"json: %w\", "+jerrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, jbv))
		return b.String(), true
	}

	return "", false
}
