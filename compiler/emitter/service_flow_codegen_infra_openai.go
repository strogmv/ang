package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// renderFlowStepInfraOpenAI handles openai.Chat action.
func renderFlowStepInfraOpenAI(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "openai.Chat":
		system := arg("system")
		systemContext := arg("system_context")
		userMessage := arg("user_message")
		output := arg("output")
		history := arg("history")
		model := arg("model")
		if model == "" {
			model = `"gpt-4o-mini"`
		}
		maxTokens := flowIntArg(step.Args, "max_tokens", 4096)
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

		outputAssign := ":="
		if st.declared[output] {
			outputAssign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// openai.Chat\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := make([]map[string]string, 0)\n", pad, msgsVar))

		// system prompt
		b.WriteString(fmt.Sprintf("%s_oaiSystem%s := %s\n", pad, sfx, systemExpr))
		b.WriteString(fmt.Sprintf("%sif _oaiSystem%s != \"\" {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]string{\"role\": \"system\", \"content\": _oaiSystem%s})\n", pad, msgsVar, msgsVar, sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		// history
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
		b.WriteString(fmt.Sprintf("%s\t\tMessage struct{ Content string `json:\"content\"` } `json:\"message\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t} `json:\"choices\"`\n", pad))
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
		return b.String(), true

	case "openai.Stream":
		if !st.isStreaming {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s// openai.Stream requires streaming method signature\n", pad))
			b.WriteString(errReturn(st, pad, `fmt.Errorf("openai.Stream requires operation stream: true")`))
			return b.String(), true
		}

		system := arg("system")
		systemContext := arg("system_context")
		userMessage := arg("user_message")
		output := arg("output")
		history := arg("history")
		model := arg("model")
		if model == "" {
			model = `"gpt-4o"`
		}
		maxTokens := flowIntArg(step.Args, "max_tokens", 4096)
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
