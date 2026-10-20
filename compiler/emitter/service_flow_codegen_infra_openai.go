package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// reasoningEffort keeps the historical default for a step that says nothing.
// "minimal" is what the first gpt-5 models wanted; the later ones in that
// family reject it, and on a small extraction task it measurably degrades the
// answer, so a step that cares says so.
func reasoningEffort(value string) string {
	if value == "" {
		return "minimal"
	}
	return value
}

// renderTypedStepOpenAI handles OpenAI actions directly from TypedStep.
func renderTypedStepOpenAI(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "openai.Chat":
		typed, decodeErr := typedActionAs[flowir.OpenAIChat](step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, decodeErr.Error()), true
		}
		return renderOpenAIChat(st, typed, pad, sfx), true

	case "openai.Embed":
		typed, decodeErr := typedActionAs[flowir.OpenAIEmbed](step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, decodeErr.Error()), true
		}
		input, output, outputUsage, model, dimensions := normalizeFlowExpr(typed.Input.Source), typed.Output, typed.OutputUsage, normalizeFlowExpr(typed.Model.Source), typed.Dimensions
		bodyVar := "_oaiEmbedReqBody" + sfx
		ctxVar := "_oaiEmbedCtx" + sfx
		cancelVar := "_oaiEmbedCancel" + sfx
		reqVar := "_oaiEmbedReq" + sfx
		reqErrVar := "_oaiEmbedReqErr" + sfx
		resVar := "_oaiEmbedRes" + sfx
		resErrVar := "_oaiEmbedResErr" + sfx
		rawVar := "_oaiEmbedRaw" + sfx
		payloadVar := "_oaiEmbedPayload" + sfx
		parsedVar := "_oaiEmbedParsed" + sfx
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]float64"
		var b strings.Builder
		if outputUsage != "" && !st.declared[outputUsage] {
			st.declared[outputUsage] = true
			st.pointers[outputUsage] = false
			st.types[outputUsage] = "struct{ PromptTokens int; TotalTokens int }"
			b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, outputUsage))
			b.WriteString(fmt.Sprintf("%s\tPromptTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s\tTotalTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%s// openai.Embed\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := map[string]any{\"model\": %s, \"input\": %s}\n", pad, payloadVar, model, input))
		if dimensions > 0 {
			b.WriteString(fmt.Sprintf("%s%s[\"dimensions\"] = %d\n", pad, payloadVar, dimensions))
		}
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, bodyVar, payloadVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, 60*time.Second)\n", pad, ctxVar, cancelVar))
		b.WriteString(fmt.Sprintf("%sdefer %s()\n", pad, cancelVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, \"POST\", helpers.OpenAIURL(\"/embeddings\"), bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, reqErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Embed: %%w\", %s)", reqErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"OPENAI_API_KEY\"))\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Content-Type\", \"application/json\")\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, resVar, resErrVar, reqVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, resErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Embed: %%w\", %s)", resErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, resVar))
		b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, rawVar, resVar))
		b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, parsedVar))
		b.WriteString(fmt.Sprintf("%s\tData []struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tEmbedding []float64 `json:\"embedding\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t} `json:\"data\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tUsage struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tPromptTokens int `json:\"prompt_tokens\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tTotalTokens int `json:\"total_tokens\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t} `json:\"usage\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tError *struct{ Message string `json:\"message\"` } `json:\"error\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, parsedVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"openai.Embed parse: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s.Error != nil {\n", pad, parsedVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Embed: API error: %%s\", %s.Error.Message)", parsedVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif len(%s.Data) == 0 {\n", pad, parsedVar))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("openai.Embed: no embeddings in response")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s.Data[0].Embedding\n", pad, output, assign, parsedVar))
		if outputUsage != "" {
			b.WriteString(fmt.Sprintf("%s%s.PromptTokens = %s.Usage.PromptTokens\n", pad, outputUsage, parsedVar))
			b.WriteString(fmt.Sprintf("%s%s.TotalTokens = %s.Usage.TotalTokens\n", pad, outputUsage, parsedVar))
		}
		return b.String(), true

	case "openai.Stream":
		typed, decodeErr := typedActionAs[flowir.OpenAIStream](step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, decodeErr.Error()), true
		}
		if !st.isStreaming {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s// openai.Stream requires streaming method signature\n", pad))
			b.WriteString(errReturn(st, pad, `fmt.Errorf("openai.Stream requires operation stream: true")`))
			return b.String(), true
		}

		system, systemContext, userMessage, history, model := normalizeFlowExpr(typed.System.Source), normalizeFlowExpr(typed.SystemContext.Source), normalizeFlowExpr(typed.UserMessage.Source), normalizeFlowExpr(typed.History.Source), normalizeFlowExpr(typed.Model.Source)
		output, maxTokens := typed.Output, typed.MaxTokens
		var systemExpr string
		if system != "" && systemContext != "" {
			systemExpr = fmt.Sprintf(`%s + "\n\n== Context ==\n" + %s`, system, systemContext)
		} else if system != "" {
			systemExpr = system
		} else if systemContext != "" {
			systemExpr = systemContext
		} else {
			systemExpr = `""`
		}
		if userMessage == "" {
			return "", true
		}

		msgsVar := "_oaiMsgs" + sfx
		bodyVar := "_oaiReqBody" + sfx
		ctxVar := "_oaiCtx" + sfx
		cancelVar := "_oaiCancel" + sfx
		reqVar := "_oaiReq" + sfx
		reqErrVar := "_oaiReqErr" + sfx
		resVar := "_oaiRes" + sfx
		resErrVar := "_oaiResErr" + sfx
		statusErrVar := "_oaiStatusErr" + sfx
		scanErrVar := "_oaiScanErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// openai.Stream\n", pad))
		if output != "" {
			outputAssign := ":="
			if st.declared[output] {
				outputAssign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "string"
			b.WriteString(fmt.Sprintf("%s%s %s \"\"\n", pad, output, outputAssign))
		}
		b.WriteString(fmt.Sprintf("%s%s := make([]map[string]string, 0)\n", pad, msgsVar))
		b.WriteString(fmt.Sprintf("%s_oaiSystem%s := %s\n", pad, sfx, systemExpr))
		b.WriteString(fmt.Sprintf("%sif _oaiSystem%s != \"\" {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]string{\"role\": \"system\", \"content\": _oaiSystem%s})\n", pad, msgsVar, msgsVar, sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if history != "" {
			b.WriteString(fmt.Sprintf("%sfor _, _hMsg%s := range %s {\n", pad, sfx, history))
			b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]string{\"role\": _hMsg%s.Role, \"content\": _hMsg%s.Content})\n", pad, msgsVar, msgsVar, sfx, sfx))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		payloadVar := "_oaiPayload" + sfx
		b.WriteString(fmt.Sprintf("%s%s = append(%s, map[string]string{\"role\": \"user\", \"content\": %s})\n", pad, msgsVar, msgsVar, userMessage))
		b.WriteString(fmt.Sprintf("%s%s := map[string]any{\"model\": %s, \"messages\": %s, \"stream\": true}\n", pad, payloadVar, model, msgsVar))
		b.WriteString(fmt.Sprintf("%sif strings.HasPrefix(strings.TrimSpace(fmt.Sprint(%s)), \"gpt-5\") {\n", pad, model))
		b.WriteString(fmt.Sprintf("%s\t%s[\"max_completion_tokens\"] = %d\n", pad, payloadVar, maxTokens))
		b.WriteString(fmt.Sprintf("%s\t%s[\"reasoning_effort\"] = \"minimal\"\n", pad, payloadVar))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s[\"max_tokens\"] = %d\n", pad, payloadVar, maxTokens))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, bodyVar, payloadVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, 90*time.Second)\n", pad, ctxVar, cancelVar))
		b.WriteString(fmt.Sprintf("%sdefer %s()\n", pad, cancelVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, \"POST\", helpers.OpenAIURL(\"/chat/completions\"), bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, reqErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Stream: %%w\", %s)", reqErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"OPENAI_API_KEY\"))\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Content-Type\", \"application/json\")\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, resVar, resErrVar, reqVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, resErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Stream: %%w\", %s)", resErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, resVar))
		b.WriteString(fmt.Sprintf("%sif %s.StatusCode >= 300 {\n", pad, resVar))
		b.WriteString(fmt.Sprintf("%s\t%s := fmt.Errorf(\"openai.Stream: status %%d\", %s.StatusCode)\n", pad, statusErrVar, resVar))
		b.WriteString(errReturn(st, pad+"\t", statusErrVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s_scanner%s := bufio.NewScanner(%s.Body)\n", pad, sfx, resVar))
		b.WriteString(fmt.Sprintf("%sfor _scanner%s.Scan() {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t_line%s := _scanner%s.Text()\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\tif !strings.HasPrefix(_line%s, \"data: \") {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tcontinue\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_data%s := strings.TrimPrefix(_line%s, \"data: \")\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\tif _data%s == \"[DONE]\" {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar _delta%s struct {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tChoices []struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tDelta struct{ Content string `json:\"content\"` } `json:\"delta\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t} `json:\"choices\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif json.Unmarshal([]byte(_data%s), &_delta%s) == nil && len(_delta%s.Choices) > 0 {\n", pad, sfx, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\t\t_chunk%s := _delta%s.Choices[0].Delta.Content\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tif _chunk%s != \"\" {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\t\tselect {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tcase <-ctx.Done():\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t\t\t", "ctx.Err()"))
		b.WriteString(fmt.Sprintf("%s\t\t\tcase chunks <- _chunk%s:\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		if output != "" {
			b.WriteString(fmt.Sprintf("%s\t\t\t%s += _chunk%s\n", pad, output, sfx))
		}
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := _scanner%s.Err()\n", pad, scanErrVar, sfx))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, scanErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Stream: scan: %%w\", %s)", scanErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}
