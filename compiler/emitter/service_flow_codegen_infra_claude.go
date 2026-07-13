package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepClaude handles claude.Chat directly from TypedStep.
func renderTypedStepClaude(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "claude.Chat":
		typed, decodeErr := typedActionAs[flowir.ClaudeChat](step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, decodeErr.Error()), true
		}
		system, systemContext, userMessage, history, model := normalizeFlowExpr(typed.System.Source), normalizeFlowExpr(typed.SystemContext.Source), normalizeFlowExpr(typed.UserMessage.Source), normalizeFlowExpr(typed.History.Source), normalizeFlowExpr(typed.Model.Source)
		output, maxTokens := typed.Output, typed.MaxTokens
		localeExpr, timezoneExpr := normalizeFlowExpr(typed.Locale.Source), normalizeFlowExpr(typed.Timezone.Source)
		var systemExpr string
		if system != "" && systemContext != "" {
			systemExpr = fmt.Sprintf(`%s + "\n\n== Current project CUE content ==\n" + %s`, system, systemContext)
		} else if system != "" {
			systemExpr = system
		} else if systemContext != "" {
			systemExpr = systemContext
		} else {
			systemExpr = `""`
		}
		if localeExpr != "" || timezoneExpr != "" {
			var parts []string
			if localeExpr != "" {
				parts = append(parts, fmt.Sprintf(`"locale=" + fmt.Sprint(%s)`, localeExpr))
			}
			if timezoneExpr != "" {
				parts = append(parts, fmt.Sprintf(`"timezone=" + fmt.Sprint(%s)`, timezoneExpr))
			}
			localePrefix := fmt.Sprintf(`"[" + %s + "]\n"`, strings.Join(parts, ` + ", " + `))
			if systemExpr == `""` {
				systemExpr = localePrefix
			} else {
				systemExpr = localePrefix + ` + ` + systemExpr
			}
		}
		if userMessage == "" {
			return renderInvalidFlowStepConfig(st, pad, "claude.Chat", "claude.Chat requires user_message"), true
		}

		msgsVar := "_claudeMsgs" + sfx
		bodyVar := "_claudeReqBody" + sfx
		ctxVar := "_claudeCtx" + sfx
		cancelVar := "_claudeCancel" + sfx
		reqVar := "_claudeReq" + sfx
		reqErrVar := "_claudeReqErr" + sfx
		resVar := "_claudeRes" + sfx
		resErrVar := "_claudeResErr" + sfx
		rawVar := "_claudeRaw" + sfx
		parsedVar := "_claudeParsed" + sfx

		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// claude.Chat\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := make([]map[string]string, 0)\n", pad, msgsVar))

		if history != "" {
			b.WriteString(fmt.Sprintf("%sfor _, _hMsg%s := range %s {\n", pad, sfx, history))
			b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]string{\"role\": _hMsg%s.Role, \"content\": _hMsg%s.Content})\n", pad, msgsVar, msgsVar, sfx, sfx))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}

		b.WriteString(fmt.Sprintf("%s%s = append(%s, map[string]string{\"role\": \"user\", \"content\": %s})\n", pad, msgsVar, msgsVar, userMessage))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(map[string]any{\"model\": %s, \"max_tokens\": %d, \"system\": %s, \"messages\": %s})\n", pad, bodyVar, model, maxTokens, systemExpr, msgsVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, 60*time.Second)\n", pad, ctxVar, cancelVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, \"POST\", \"https://api.anthropic.com/v1/messages\", bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, reqErrVar))
		b.WriteString(fmt.Sprintf("%s\t%s()\n", pad, cancelVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"claude.Chat: %%w\", %s)", reqErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"x-api-key\", os.Getenv(\"ANTHROPIC_API_KEY\"))\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"anthropic-version\", \"2023-06-01\")\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"content-type\", \"application/json\")\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, resVar, resErrVar, reqVar))
		b.WriteString(fmt.Sprintf("%s%s()\n", pad, cancelVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, resErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"claude.Chat: %%w\", %s)", resErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, resVar))
		b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, rawVar, resVar))
		b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, parsedVar))
		b.WriteString(fmt.Sprintf("%s\tContent []struct{ Text string `json:\"text\"` } `json:\"content\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tError *struct{ Message string `json:\"message\"` } `json:\"error,omitempty\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, parsedVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"claude.Chat parse: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s.StatusCode != 200 {\n", pad, resVar))
		b.WriteString(fmt.Sprintf("%s\t_claudeErrMsg%s := \"claude API error\"\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\tif %s.Error != nil { _claudeErrMsg%s = %s.Error.Message }\n", pad, parsedVar, sfx, parsedVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"claude.Chat (%%d): %%s\", %s.StatusCode, _claudeErrMsg%s)", resVar, sfx)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif len(%s.Content) == 0 {\n", pad, parsedVar))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusInternalServerError, \"CLAUDE_EMPTY\", \"empty response from Claude\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := %s.Content[0].Text\n", pad, output, parsedVar))
		return b.String(), true
	}
	return "", false
}
