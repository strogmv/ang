package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowStepInfraHTTPAndSerialization(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "http.Call":
		typed, decodeErr := flowir.DecodeAs[flowir.HTTPCall](step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, decodeErr.Error()), true
		}
		if out, ok := renderFlowHTTPCallAST(st, step, indent, sfx, arg); ok {
			return out, true
		}
		method, url, body, output, statusVar, failOnError := typed.Method, normalizeFlowExpr(typed.URL.Source), normalizeFlowExpr(typed.Body.Source), typed.Output, typed.StatusVar, typed.FailOnError
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
		if hdrs := typed.Headers; len(hdrs) > 0 {
			keys := make([]string, 0, len(hdrs))
			for k := range hdrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, hk := range keys {
				b.WriteString(fmt.Sprintf("%s%s.Header.Set(%q, %s)\n", pad, httpReqV, hk, normalizeFlowExpr(hdrs[hk].Source)))
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
		typed, err := flowir.DecodeAs[flowir.RandomCode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, length := typed.Output, typed.Length
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
		typed, err := flowir.DecodeAs[flowir.RandomToken](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, nbytes := typed.Output, typed.Bytes
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
		typed, err := flowir.DecodeAs[flowir.StringFormat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		tmpl, output := normalizeFlowExpr(typed.Template.Source), typed.Output
		fmtArgs := make([]string, 0, len(typed.Arguments))
		for _, item := range typed.Arguments {
			fmtArgs = append(fmtArgs, normalizeFlowExpr(item.Source))
		}
		callArgs := tmpl
		if len(fmtArgs) > 0 {
			callArgs += ", " + strings.Join(fmtArgs, ", ")
		}
		if strings.Contains(output, ".") {
			tmpVar := "_fmt" + sfx
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(%s)\n", pad, tmpVar, callArgs))
			b.WriteString(renderFlowAssignTarget(st, pad, output, tmpVar, "string"))
			return b.String(), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		return fmt.Sprintf("%s%s %s fmt.Sprintf(%s)\n", pad, output, assign, callArgs), true

	case "str.Concat":
		typed, err := flowir.DecodeAs[flowir.StringConcat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, sep := typed.Output, normalizeFlowExpr(typed.Separator.Source)
		parts := make([]string, 0, len(typed.Parts))
		for _, item := range typed.Parts {
			parts = append(parts, normalizeFlowExpr(item.Source))
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
		typed, err := flowir.DecodeAs[flowir.StringStripMarkdown](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
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

	case "str.ReplaceAll":
		typed, err := flowir.DecodeAs[flowir.StringReplaceAll](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, old, newS, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Old.Source), normalizeFlowExpr(typed.New.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("strings.ReplaceAll(%s, %s, %s)", input, old, newS), "string"), true

	case "str.TrimSpace":
		typed, err := flowir.DecodeAs[flowir.StringTrimSpace](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("strings.TrimSpace(%s)", input), "string"), true

	case "cast.ToString":
		typed, err := flowir.DecodeAs[flowir.CastToString](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output, format := normalizeFlowExpr(typed.Input.Source), typed.Output, normalizeFlowExpr(typed.Format)
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
		typed, err := flowir.DecodeAs[flowir.JSONParse](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, into, output := normalizeFlowExpr(typed.Input.Source), typed.Into, typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = into
		var b strings.Builder
		if assign == ":=" {
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, into))
		}
		b.WriteString(fmt.Sprintf("%sif _jErr := json.Unmarshal([]byte(%s), &%s); _jErr != nil {\n", pad, input, output))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"json: %w\", _jErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "json.Marshal":
		typed, err := flowir.DecodeAs[flowir.JSONMarshal](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
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

	case "json.Stringify":
		typed, err := flowir.DecodeAs[flowir.JSONMarshal](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		jbv, jerrv := "_jsb"+sfx, "_jsErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(%s)\n", pad, jbv, jerrv, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, jerrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"json.Stringify: %w\", "+jerrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, jbv))
		return b.String(), true

	case "template.Render":
		tmpl := arg("template")
		data := arg("data")
		output := arg("output")
		if tmpl == "" || data == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "template.Render", "template.Render requires template, data, and output"), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		tmplVar := "_tmpl" + sfx
		bufVar := "_tmplBuf" + sfx
		errVar := "_tmplErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := template.New(\"flow\").Parse(%s)\n", pad, tmplVar, errVar, tmpl))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"template.Render parse: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s bytes.Buffer\n", pad, bufVar))
		b.WriteString(fmt.Sprintf("%sif %s := %s.Execute(&%s, %s); %s != nil {\n", pad, errVar, tmplVar, bufVar, data, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"template.Render execute: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s.String()\n", pad, output, assign, bufVar))
		return b.String(), true

	case "stream.Emit":
		data := arg("data")
		if data == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sselect {\n", pad))
		b.WriteString(fmt.Sprintf("%scase <-ctx.Done():\n", pad+"\t"))
		b.WriteString(errReturn(st, pad+"\t\t", "ctx.Err()"))
		b.WriteString(fmt.Sprintf("%scase chunks <- fmt.Sprint(%s):\n", pad+"\t", data))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}
