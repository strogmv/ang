package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepOpenAI handles OpenAI actions directly from TypedStep.
func renderTypedStepOpenAI(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Name {
	case "openai.Chat":
		typed, decodeErr := typedActionAs[flowir.OpenAIChat](step)
		if decodeErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, decodeErr.Error()), true
		}
		system, systemContext, userMessage, history, model := normalizeFlowExpr(typed.System.Source), normalizeFlowExpr(typed.SystemContext.Source), normalizeFlowExpr(typed.UserMessage.Source), normalizeFlowExpr(typed.History.Source), normalizeFlowExpr(typed.Model.Source)
		output, outputUsage, outputToolCalls, outputJSON := typed.Output, typed.OutputUsage, typed.OutputToolCalls, typed.OutputJSON
		maxTokens, maxRounds := typed.MaxTokens, typed.MaxRounds
		responseJSONSchema, responseJSONName, responseJSONStrict := normalizeFlowExpr(typed.ResponseJSONSchema.Source), normalizeFlowExpr(typed.ResponseJSONName.Source), typed.ResponseJSONStrict
		toolChoice, toolNames := normalizeFlowExpr(typed.ToolChoice.Source), typed.Tools
		toolSpecs, toolErr := resolveOpenAITools(st, st.serviceName, toolNames)
		if toolErr != nil {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s// openai.Chat tool resolution failed\n", pad))
			b.WriteString(errReturn(st, pad, fmt.Sprintf("fmt.Errorf(%q)", toolErr.Error())))
			return b.String(), true
		}
		toolChoice = normalizeOpenAIToolChoiceExpr(toolChoice, toolSpecs)
		if output == "" {
			output = "openaiReply"
		}
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
		rawVar := "_oaiRaw" + sfx
		parsedVar := "_oaiParsed" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// openai.Chat\n", pad))
		if len(toolSpecs) == 0 {
			outputAssign := ":="
			if st.declared[output] {
				outputAssign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "string"
			if outputUsage != "" && !st.declared[outputUsage] {
				st.declared[outputUsage] = true
				st.pointers[outputUsage] = false
				st.types[outputUsage] = "struct{ PromptTokens int; CompletionTokens int; TotalTokens int }"
				b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, outputUsage))
				b.WriteString(fmt.Sprintf("%s\tPromptTokens int\n", pad))
				b.WriteString(fmt.Sprintf("%s\tCompletionTokens int\n", pad))
				b.WriteString(fmt.Sprintf("%s\tTotalTokens int\n", pad))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
			}
			if outputToolCalls != "" && !st.declared[outputToolCalls] {
				st.declared[outputToolCalls] = true
				st.pointers[outputToolCalls] = false
				st.types[outputToolCalls] = "int"
				b.WriteString(fmt.Sprintf("%svar %s int\n", pad, outputToolCalls))
			}
			if outputJSON != "" && !st.declared[outputJSON] {
				st.declared[outputJSON] = true
				st.pointers[outputJSON] = false
				st.types[outputJSON] = "map[string]any"
				b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, outputJSON))
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
			b.WriteString(fmt.Sprintf("%s%s := map[string]any{\"model\": %s, \"messages\": %s}\n", pad, payloadVar, model, msgsVar))
			b.WriteString(fmt.Sprintf("%sif strings.HasPrefix(strings.TrimSpace(fmt.Sprint(%s)), \"gpt-5\") {\n", pad, model))
			b.WriteString(fmt.Sprintf("%s\t%s[\"max_completion_tokens\"] = %d\n", pad, payloadVar, maxTokens))
			b.WriteString(fmt.Sprintf("%s\t%s[\"reasoning_effort\"] = \"minimal\"\n", pad, payloadVar))
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s[\"max_tokens\"] = %d\n", pad, payloadVar, maxTokens))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			if responseJSONSchema != "" {
				b.WriteString(fmt.Sprintf("%s%s[\"response_format\"] = map[string]any{\"type\": \"json_schema\", \"json_schema\": map[string]any{\"name\": %s, \"strict\": %t, \"schema\": json.RawMessage(%s)}}\n", pad, payloadVar, responseJSONName, responseJSONStrict, responseJSONSchema))
			}
			b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, bodyVar, payloadVar))
			b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, 60*time.Second)\n", pad, ctxVar, cancelVar))
			b.WriteString(fmt.Sprintf("%sdefer %s()\n", pad, cancelVar))
			b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, \"POST\", \"https://api.openai.com/v1/chat/completions\", bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, reqErrVar))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat: %%w\", %s)", reqErrVar)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"OPENAI_API_KEY\"))\n", pad, reqVar))
			b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Content-Type\", \"application/json\")\n", pad, reqVar))
			b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, resVar, resErrVar, reqVar))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, resErrVar))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat: %%w\", %s)", resErrVar)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, resVar))
			b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, rawVar, resVar))
			b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, parsedVar))
			b.WriteString(fmt.Sprintf("%s\tChoices []struct {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tFinishReason string `json:\"finish_reason\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tMessage struct{ Content string `json:\"content\"` } `json:\"message\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\t} `json:\"choices\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\tUsage struct {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tPromptTokens int `json:\"prompt_tokens\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tCompletionTokens int `json:\"completion_tokens\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tTotalTokens int `json:\"total_tokens\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\t} `json:\"usage\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s\tError *struct{ Message string `json:\"message\"` } `json:\"error\"`\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, parsedVar))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"openai.Chat: parse error: %w\", err)"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sif %s.Error != nil {\n", pad, parsedVar))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat: API error: %%s\", %s.Error.Message)", parsedVar)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sif len(%s.Choices) == 0 {\n", pad, parsedVar))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"openai.Chat: no choices in response\")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s %s %s.Choices[0].Message.Content\n", pad, output, outputAssign, parsedVar))
			if outputJSON != "" {
				b.WriteString(fmt.Sprintf("%sif strings.TrimSpace(%s.Choices[0].Message.Content) == \"\" {\n", pad, parsedVar))
				b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("openai.Chat: structured output content is empty")`))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
				b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal([]byte(%s.Choices[0].Message.Content), &%s); err != nil {\n", pad, parsedVar, outputJSON))
				b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("openai.Chat: decode structured output: %w", err)`))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
			}
			if outputUsage != "" {
				b.WriteString(fmt.Sprintf("%s%s.PromptTokens = %s.Usage.PromptTokens\n", pad, outputUsage, parsedVar))
				b.WriteString(fmt.Sprintf("%s%s.CompletionTokens = %s.Usage.CompletionTokens\n", pad, outputUsage, parsedVar))
				b.WriteString(fmt.Sprintf("%s%s.TotalTokens = %s.Usage.TotalTokens\n", pad, outputUsage, parsedVar))
			}
			if outputToolCalls != "" {
				b.WriteString(fmt.Sprintf("%s%s = 0\n", pad, outputToolCalls))
			}
			return b.String(), true
		}

		outputType := "struct{ Content string; FinishReason string; ToolCalls int; PromptTokens int; CompletionTokens int; TotalTokens int }"
		if !st.declared[output] {
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = outputType
			b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, output))
			b.WriteString(fmt.Sprintf("%s\tContent string\n", pad))
			b.WriteString(fmt.Sprintf("%s\tFinishReason string\n", pad))
			b.WriteString(fmt.Sprintf("%s\tToolCalls int\n", pad))
			b.WriteString(fmt.Sprintf("%s\tPromptTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s\tCompletionTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s\tTotalTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if outputUsage != "" && !st.declared[outputUsage] {
			st.declared[outputUsage] = true
			st.pointers[outputUsage] = false
			st.types[outputUsage] = "struct{ PromptTokens int; CompletionTokens int; TotalTokens int }"
			b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, outputUsage))
			b.WriteString(fmt.Sprintf("%s\tPromptTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s\tCompletionTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s\tTotalTokens int\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if outputToolCalls != "" && !st.declared[outputToolCalls] {
			st.declared[outputToolCalls] = true
			st.pointers[outputToolCalls] = false
			st.types[outputToolCalls] = "int"
			b.WriteString(fmt.Sprintf("%svar %s int\n", pad, outputToolCalls))
		}
		if outputJSON != "" && !st.declared[outputJSON] {
			st.declared[outputJSON] = true
			st.pointers[outputJSON] = false
			st.types[outputJSON] = "map[string]any"
			b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, outputJSON))
		}
		toolsVar := "_oaiTools" + sfx
		payloadVar := "_oaiPayload" + sfx
		roundVar := "_oaiRound" + sfx
		handledVar := "_oaiHandled" + sfx
		finishVar := "_oaiFinish" + sfx
		b.WriteString(fmt.Sprintf("%s%s := make([]map[string]any, 0)\n", pad, msgsVar))
		b.WriteString(fmt.Sprintf("%s%s := []map[string]any{\n", pad, toolsVar))
		for _, spec := range toolSpecs {
			desc := strings.TrimSpace(spec.Method.Description)
			if desc == "" {
				desc = spec.Method.Name
			}
			b.WriteString(fmt.Sprintf("%s\t{\"type\": \"function\", \"function\": map[string]any{\"name\": %q, \"description\": %q, \"parameters\": json.RawMessage(%q)}},\n", pad, spec.ToolName, desc, spec.SchemaJSON))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s_oaiSystem%s := %s\n", pad, sfx, systemExpr))
		b.WriteString(fmt.Sprintf("%sif _oaiSystem%s != \"\" {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]any{\"role\": \"system\", \"content\": _oaiSystem%s})\n", pad, msgsVar, msgsVar, sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if history != "" {
			b.WriteString(fmt.Sprintf("%sfor _, _hMsg%s := range %s {\n", pad, sfx, history))
			b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]any{\"role\": _hMsg%s.Role, \"content\": _hMsg%s.Content})\n", pad, msgsVar, msgsVar, sfx, sfx))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%s%s = append(%s, map[string]any{\"role\": \"user\", \"content\": %s})\n", pad, msgsVar, msgsVar, userMessage))
		b.WriteString(fmt.Sprintf("%sfor %s := 0; %s < %d; %s++ {\n", pad, roundVar, roundVar, maxRounds, roundVar))
		b.WriteString(fmt.Sprintf("%s\t%s := map[string]any{\"model\": %s, \"messages\": %s, \"tools\": %s}\n", pad, payloadVar, model, msgsVar, toolsVar))
		if toolChoice != "" {
			b.WriteString(fmt.Sprintf("%s\t%s[\"tool_choice\"] = %s\n", pad, payloadVar, toolChoice))
		}
		b.WriteString(fmt.Sprintf("%s\tif strings.HasPrefix(strings.TrimSpace(fmt.Sprint(%s)), \"gpt-5\") {\n", pad, model))
		b.WriteString(fmt.Sprintf("%s\t\t%s[\"max_completion_tokens\"] = %d\n", pad, payloadVar, maxTokens))
		b.WriteString(fmt.Sprintf("%s\t\t%s[\"reasoning_effort\"] = \"minimal\"\n", pad, payloadVar))
		b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s[\"max_tokens\"] = %d\n", pad, payloadVar, maxTokens))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		if responseJSONSchema != "" {
			b.WriteString(fmt.Sprintf("%s\t%s[\"response_format\"] = map[string]any{\"type\": \"json_schema\", \"json_schema\": map[string]any{\"name\": %s, \"strict\": %t, \"schema\": json.RawMessage(%s)}}\n", pad, payloadVar, responseJSONName, responseJSONStrict, responseJSONSchema))
		}
		b.WriteString(fmt.Sprintf("%s\t%s, _ := json.Marshal(%s)\n", pad, bodyVar, payloadVar))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := context.WithTimeout(ctx, 60*time.Second)\n", pad, ctxVar, cancelVar))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := http.NewRequestWithContext(%s, \"POST\", \"https://api.openai.com/v1/chat/completions\", bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, reqErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s()\n", pad, cancelVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat: %%w\", %s)", reqErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"OPENAI_API_KEY\"))\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s\t%s.Header.Set(\"Content-Type\", \"application/json\")\n", pad, reqVar))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := http.DefaultClient.Do(%s)\n", pad, resVar, resErrVar, reqVar))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, resErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s()\n", pad, cancelVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat: %%w\", %s)", resErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s, _ := io.ReadAll(%s.Body)\n", pad, rawVar, resVar))
		b.WriteString(fmt.Sprintf("%s\t_ = %s.Body.Close()\n", pad, resVar))
		b.WriteString(fmt.Sprintf("%s\t%s()\n", pad, cancelVar))
		b.WriteString(fmt.Sprintf("%s\tvar %s struct {\n", pad, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t\tChoices []struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tFinishReason string `json:\"finish_reason\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tMessage struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tContent string `json:\"content\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tToolCalls []struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\tID string `json:\"id\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\tType string `json:\"type\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\tFunction struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\t\tName string `json:\"name\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\t\tArguments string `json:\"arguments\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\t} `json:\"function\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t} `json:\"tool_calls\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t} `json:\"message\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t} `json:\"choices\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tUsage struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tPromptTokens int `json:\"prompt_tokens\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tCompletionTokens int `json:\"completion_tokens\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tTotalTokens int `json:\"total_tokens\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t} `json:\"usage\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tError *struct{ Message string `json:\"message\"` } `json:\"error\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, parsedVar))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"openai.Chat: parse error: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s.Error != nil {\n", pad, parsedVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat: API error: %%s\", %s.Error.Message)", parsedVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif len(%s.Choices) == 0 {\n", pad, parsedVar))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"openai.Chat: no choices in response\")"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := %s.Choices[0].FinishReason\n", pad, finishVar, parsedVar))
		b.WriteString(fmt.Sprintf("%s\tif len(%s.Choices[0].Message.ToolCalls) == 0 {\n", pad, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s.Content = %s.Choices[0].Message.Content\n", pad, output, parsedVar))
		if outputJSON != "" {
			b.WriteString(fmt.Sprintf("%s\t\tif strings.TrimSpace(%s.Choices[0].Message.Content) == \"\" {\n", pad, parsedVar))
			b.WriteString(errReturn(st, pad+"\t\t\t", `fmt.Errorf("openai.Chat: structured output content is empty")`))
			b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\tif err := json.Unmarshal([]byte(%s.Choices[0].Message.Content), &%s); err != nil {\n", pad, parsedVar, outputJSON))
			b.WriteString(errReturn(st, pad+"\t\t\t", `fmt.Errorf("openai.Chat: decode structured output: %w", err)`))
			b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%s\t\t%s.FinishReason = %s\n", pad, output, finishVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s.PromptTokens += %s.Usage.PromptTokens\n", pad, output, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s.CompletionTokens += %s.Usage.CompletionTokens\n", pad, output, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s.TotalTokens += %s.Usage.TotalTokens\n", pad, output, parsedVar))
		if outputUsage != "" {
			b.WriteString(fmt.Sprintf("%s\t\t%s.PromptTokens += %s.Usage.PromptTokens\n", pad, outputUsage, parsedVar))
			b.WriteString(fmt.Sprintf("%s\t\t%s.CompletionTokens += %s.Usage.CompletionTokens\n", pad, outputUsage, parsedVar))
			b.WriteString(fmt.Sprintf("%s\t\t%s.TotalTokens += %s.Usage.TotalTokens\n", pad, outputUsage, parsedVar))
		}
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_assistantMsg%s := map[string]any{\"role\": \"assistant\", \"content\": %s.Choices[0].Message.Content}\n", pad, sfx, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t_assistantCalls%s := make([]map[string]any, 0, len(%s.Choices[0].Message.ToolCalls))\n", pad, sfx, parsedVar))
		b.WriteString(fmt.Sprintf("%s\tfor _, _tc%s := range %s.Choices[0].Message.ToolCalls {\n", pad, sfx, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t\t_assistantCalls%s = append(_assistantCalls%s, map[string]any{\"id\": _tc%s.ID, \"type\": _tc%s.Type, \"function\": map[string]any{\"name\": _tc%s.Function.Name, \"arguments\": _tc%s.Function.Arguments}})\n", pad, sfx, sfx, sfx, sfx, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_assistantMsg%s[\"tool_calls\"] = _assistantCalls%s\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, _assistantMsg%s)\n", pad, msgsVar, msgsVar, sfx))
		b.WriteString(fmt.Sprintf("%s\t%s := false\n", pad, handledVar))
		b.WriteString(fmt.Sprintf("%s\tfor _, _tc%s := range %s.Choices[0].Message.ToolCalls {\n", pad, sfx, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t\tswitch _tc%s.Function.Name {\n", pad, sfx))
		for _, spec := range toolSpecs {
			reqVarName := "_toolReq" + sfx + ExportName(spec.Method.Name)
			respVarName := "_toolResp" + sfx + ExportName(spec.Method.Name)
			rawRespVar := "_toolRespRaw" + sfx + ExportName(spec.Method.Name)
			b.WriteString(fmt.Sprintf("%s\t\t\tcase %q:\n", pad, spec.ToolName))
			b.WriteString(fmt.Sprintf("%s\t\t\t\tvar %s %s\n", pad, reqVarName, spec.RequestType))
			b.WriteString(fmt.Sprintf("%s\t\t\t\tif strings.TrimSpace(_tc%s.Function.Arguments) != \"\" {\n", pad, sfx))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t\tif err := json.Unmarshal([]byte(_tc%s.Function.Arguments), &%s); err != nil {\n", pad, sfx, reqVarName))
			b.WriteString(errReturn(st, pad+"\t\t\t\t\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat tool %s args: %%w\", err)", spec.Method.Name)))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t}\n", pad))
			if strings.EqualFold(strings.TrimSpace(spec.ServiceName), strings.TrimSpace(st.serviceName)) {
				b.WriteString(fmt.Sprintf("%s\t\t\t\t%s, err := s.%s(ctx, %s)\n", pad, respVarName, ExportName(spec.Method.Name), reqVarName))
			} else {
				b.WriteString(fmt.Sprintf("%s\t\t\t\t%s, err := s.%sService.%s(ctx, %s)\n", pad, respVarName, ExportName(spec.ServiceName), ExportName(spec.Method.Name), reqVarName))
			}
			b.WriteString(fmt.Sprintf("%s\t\t\t\tif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t\t\t\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat tool %s: %%w\", err)", spec.Method.Name)))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t%s, _ := json.Marshal(%s)\n", pad, rawRespVar, respVarName))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = append(%s, map[string]any{\"role\": \"tool\", \"tool_call_id\": _tc%s.ID, \"content\": string(%s)})\n", pad, msgsVar, msgsVar, sfx, rawRespVar))
			b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = true\n", pad, handledVar))
		}
		b.WriteString(fmt.Sprintf("%s\t\t\tdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat unknown tool: %%s\", _tc%s.Function.Name)", sfx)))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif !%s {\n", pad, handledVar))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"openai.Chat: tool calls were returned but none were executed\")"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s.ToolCalls += len(%s.Choices[0].Message.ToolCalls)\n", pad, output, parsedVar))
		if outputToolCalls != "" {
			b.WriteString(fmt.Sprintf("%s\t%s += len(%s.Choices[0].Message.ToolCalls)\n", pad, outputToolCalls, parsedVar))
		}
		b.WriteString(fmt.Sprintf("%s\t%s.PromptTokens += %s.Usage.PromptTokens\n", pad, output, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t%s.CompletionTokens += %s.Usage.CompletionTokens\n", pad, output, parsedVar))
		b.WriteString(fmt.Sprintf("%s\t%s.TotalTokens += %s.Usage.TotalTokens\n", pad, output, parsedVar))
		if outputUsage != "" {
			b.WriteString(fmt.Sprintf("%s\t%s.PromptTokens += %s.Usage.PromptTokens\n", pad, outputUsage, parsedVar))
			b.WriteString(fmt.Sprintf("%s\t%s.CompletionTokens += %s.Usage.CompletionTokens\n", pad, outputUsage, parsedVar))
			b.WriteString(fmt.Sprintf("%s\t%s.TotalTokens += %s.Usage.TotalTokens\n", pad, outputUsage, parsedVar))
		}
		b.WriteString(fmt.Sprintf("%s\t%s.FinishReason = %s\n", pad, output, finishVar))
		b.WriteString(fmt.Sprintf("%s\tif %s+1 == %d {\n", pad, roundVar, maxRounds))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat exceeded max_rounds=%d\")", maxRounds)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

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
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, \"POST\", \"https://api.openai.com/v1/embeddings\", bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
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
			systemExpr = fmt.Sprintf(`%s + "\n\n== Current project CUE content ==\n" + %s`, system, systemContext)
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
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, \"POST\", \"https://api.openai.com/v1/chat/completions\", bytes.NewReader(%s))\n", pad, reqVar, reqErrVar, ctxVar, bodyVar))
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
