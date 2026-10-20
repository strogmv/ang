package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// openAIChatNames are the variables one openai.Chat step declares; sfx keeps
// them apart from those of another step in the same method.
type openAIChatNames struct {
	msgs, body, raw, callErr, parsed, call, usage, budget, payload, settleErr, reserveErr string
}

func newOpenAIChatNames(sfx string) openAIChatNames {
	return openAIChatNames{
		msgs:       "_oaiMsgs" + sfx,
		body:       "_oaiReqBody" + sfx,
		raw:        "_oaiRaw" + sfx,
		callErr:    "_oaiCallErr" + sfx,
		parsed:     "_oaiParsed" + sfx,
		call:       "_oaiCall" + sfx,
		usage:      "_oaiRoundUsage" + sfx,
		budget:     "_oaiBudget" + sfx,
		payload:    "_oaiPayload" + sfx,
		settleErr:  "_oaiSettleErr" + sfx,
		reserveErr: "_oaiReserveErr" + sfx,
	}
}

// renderOpenAIChat renders openai.Chat: one Chat Completions request, or a
// tool loop of up to max_rounds requests when tools are listed.
//
// Every request is one round of the same shape (writeOpenAIRound): the budget
// hook reserves before it and settles after it, its usage — cached input and
// reasoning tokens included — is added to the step's outputs, and an error
// stops the step only after the reservation is settled.
//
// The messages are built so that what never changes comes first: the system
// prompt, then the history; with system_context_position "before_user" the
// per-call context (the time, the locale, an id) goes after them, right before
// the user's message. The tool definitions and the payload's map keys are
// serialised in a fixed order. Two calls therefore share a byte-identical
// prefix, which is what the provider's prompt cache matches on.
func renderOpenAIChat(st *flowRenderState, typed flowir.OpenAIChat, pad, sfx string) string {
	system, systemContext, userMessage, history, model := normalizeFlowExpr(typed.System.Source), normalizeFlowExpr(typed.SystemContext.Source), normalizeFlowExpr(typed.UserMessage.Source), normalizeFlowExpr(typed.History.Source), normalizeFlowExpr(typed.Model.Source)
	output, outputUsage, outputToolCalls, outputJSON, outputRounds := typed.Output, typed.OutputUsage, typed.OutputToolCalls, typed.OutputJSON, typed.OutputRounds
	maxTokens, maxRounds := typed.MaxTokens, typed.MaxRounds
	responseJSONSchema, responseJSONName, responseJSONStrict := normalizeFlowExpr(typed.ResponseJSONSchema.Source), normalizeFlowExpr(typed.ResponseJSONName.Source), typed.ResponseJSONStrict
	toolChoice, toolNames := normalizeFlowExpr(typed.ToolChoice.Source), typed.Tools
	budget, cacheKey := normalizeFlowExpr(typed.Budget.Source), normalizeFlowExpr(typed.PromptCacheKey.Source)
	contextBeforeUser := typed.SystemContextPosition == "before_user"
	toolSpecs, toolErr := resolveOpenAITools(st, st.serviceName, toolNames)
	if toolErr != nil {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// openai.Chat tool resolution failed\n", pad))
		b.WriteString(errReturn(st, pad, fmt.Sprintf("fmt.Errorf(%q)", toolErr.Error())))
		return b.String()
	}
	toolChoice = normalizeOpenAIToolChoiceExpr(toolChoice, toolSpecs)
	if output == "" {
		output = "openaiReply"
	}
	if userMessage == "" {
		return ""
	}
	systemExpr := `""`
	switch {
	case system != "" && systemContext != "" && !contextBeforeUser:
		systemExpr = fmt.Sprintf(`%s + "\n\n== Context ==\n" + %s`, system, systemContext)
	case system != "":
		systemExpr = system
	case systemContext != "" && !contextBeforeUser:
		systemExpr = systemContext
	}
	n := newOpenAIChatNames(sfx)
	withTools := len(toolSpecs) > 0

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s// openai.Chat\n", pad))
	declare := func(name, typ string, fields ...string) {
		if name == "" || st.declared[name] {
			return
		}
		st.declared[name] = true
		st.pointers[name] = false
		if len(fields) == 0 {
			st.types[name] = typ
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, name, typ))
			return
		}
		st.types[name] = "struct{ " + strings.Join(fields, "; ") + " }"
		b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, name))
		for _, f := range fields {
			b.WriteString(fmt.Sprintf("%s\t%s\n", pad, f))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	usageFields := []string{"PromptTokens int", "CompletionTokens int", "TotalTokens int", "CachedTokens int", "ReasoningTokens int"}
	if withTools {
		declare(output, "", append([]string{"Content string", "FinishReason string", "ToolCalls int"}, usageFields...)...)
	} else {
		outputAssign := ":="
		if st.declared[output] {
			outputAssign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		if outputAssign == ":=" {
			b.WriteString(fmt.Sprintf("%svar %s string\n", pad, output))
		}
	}
	declare(outputUsage, "", usageFields...)
	declare(outputToolCalls, "int")
	declare(outputJSON, "map[string]any")
	declare(outputRounds, "[]helpers.LLMRoundUsage")
	if budget != "" {
		b.WriteString(fmt.Sprintf("%svar %s helpers.LLMBudget = %s\n", pad, n.budget, budget))
	}

	if withTools {
		toolsVar := "_oaiTools" + sfx
		b.WriteString(fmt.Sprintf("%s%s := make([]map[string]any, 0)\n", pad, n.msgs))
		b.WriteString(fmt.Sprintf("%s%s := []map[string]any{\n", pad, toolsVar))
		for _, spec := range toolSpecs {
			desc := strings.TrimSpace(spec.Method.Description)
			if desc == "" {
				desc = spec.Method.Name
			}
			b.WriteString(fmt.Sprintf("%s\t{\"type\": \"function\", \"function\": map[string]any{\"name\": %q, \"description\": %q, \"parameters\": json.RawMessage(%q)}},\n", pad, spec.ToolName, desc, spec.SchemaJSON))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	} else {
		b.WriteString(fmt.Sprintf("%s%s := make([]map[string]any, 0)\n", pad, n.msgs))
	}
	b.WriteString(fmt.Sprintf("%s_oaiSystem%s := %s\n", pad, sfx, systemExpr))
	b.WriteString(fmt.Sprintf("%sif _oaiSystem%s != \"\" {\n", pad, sfx))
	b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]any{\"role\": \"system\", \"content\": _oaiSystem%s})\n", pad, n.msgs, n.msgs, sfx))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	if history != "" {
		b.WriteString(fmt.Sprintf("%sfor _, _hMsg%s := range %s {\n", pad, sfx, history))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]any{\"role\": _hMsg%s.Role, \"content\": _hMsg%s.Content})\n", pad, n.msgs, n.msgs, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	if contextBeforeUser && systemContext != "" {
		b.WriteString(fmt.Sprintf("%sif _oaiContext%s := %s; _oaiContext%s != \"\" {\n", pad, sfx, systemContext, sfx))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]any{\"role\": \"system\", \"content\": \"== Context ==\\n\" + _oaiContext%s})\n", pad, n.msgs, n.msgs, sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%s%s = append(%s, map[string]any{\"role\": \"user\", \"content\": %s})\n", pad, n.msgs, n.msgs, userMessage))

	// The payload: the same keys in every round; json.Marshal sorts them.
	writePayload := func(p string) {
		if withTools {
			b.WriteString(fmt.Sprintf("%s%s := map[string]any{\"model\": %s, \"messages\": %s, \"tools\": _oaiTools%s}\n", p, n.payload, model, n.msgs, sfx))
			if toolChoice != "" {
				b.WriteString(fmt.Sprintf("%s%s[\"tool_choice\"] = %s\n", p, n.payload, toolChoice))
			}
		} else {
			b.WriteString(fmt.Sprintf("%s%s := map[string]any{\"model\": %s, \"messages\": %s}\n", p, n.payload, model, n.msgs))
		}
		b.WriteString(fmt.Sprintf("%sif strings.HasPrefix(strings.TrimSpace(fmt.Sprint(%s)), \"gpt-5\") {\n", p, model))
		b.WriteString(fmt.Sprintf("%s\t%s[\"max_completion_tokens\"] = %d\n", p, n.payload, maxTokens))
		b.WriteString(fmt.Sprintf("%s\t%s[\"reasoning_effort\"] = %q\n", p, n.payload, reasoningEffort(typed.ReasoningEffort)))
		b.WriteString(fmt.Sprintf("%s} else {\n", p))
		b.WriteString(fmt.Sprintf("%s\t%s[\"max_tokens\"] = %d\n", p, n.payload, maxTokens))
		b.WriteString(fmt.Sprintf("%s}\n", p))
		if responseJSONSchema != "" {
			b.WriteString(fmt.Sprintf("%s%s[\"response_format\"] = map[string]any{\"type\": \"json_schema\", \"json_schema\": map[string]any{\"name\": %s, \"strict\": %t, \"schema\": json.RawMessage(%s)}}\n", p, n.payload, responseJSONName, responseJSONStrict, responseJSONSchema))
		}
		if cacheKey != "" {
			b.WriteString(fmt.Sprintf("%sif _oaiCacheKey%s := strings.TrimSpace(fmt.Sprint(%s)); _oaiCacheKey%s != \"\" {\n", p, sfx, cacheKey, sfx))
			b.WriteString(fmt.Sprintf("%s\t%s[\"prompt_cache_key\"] = _oaiCacheKey%s\n", p, n.payload, sfx))
			b.WriteString(fmt.Sprintf("%s}\n", p))
		}
	}
	addUsage := func(p, target string) {
		if target == "" {
			return
		}
		b.WriteString(fmt.Sprintf("%s%s.PromptTokens += %s.Usage.PromptTokens\n", p, target, n.parsed))
		b.WriteString(fmt.Sprintf("%s%s.CompletionTokens += %s.Usage.CompletionTokens\n", p, target, n.parsed))
		b.WriteString(fmt.Sprintf("%s%s.TotalTokens += %s.Usage.TotalTokens\n", p, target, n.parsed))
		b.WriteString(fmt.Sprintf("%s%s.CachedTokens += %s.Usage.PromptTokensDetails.CachedTokens\n", p, target, n.parsed))
		b.WriteString(fmt.Sprintf("%s%s.ReasoningTokens += %s.Usage.CompletionTokensDetails.ReasoningTokens\n", p, target, n.parsed))
	}
	decodeJSON := func(p string) {
		if outputJSON == "" {
			return
		}
		b.WriteString(fmt.Sprintf("%sif strings.TrimSpace(%s.Choices[0].Message.Content) == \"\" {\n", p, n.parsed))
		b.WriteString(errReturn(st, p+"\t", `fmt.Errorf("openai.Chat: structured output content is empty")`))
		b.WriteString(fmt.Sprintf("%s}\n", p))
		b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal([]byte(%s.Choices[0].Message.Content), &%s); err != nil {\n", p, n.parsed, outputJSON))
		b.WriteString(errReturn(st, p+"\t", `fmt.Errorf("openai.Chat: decode structured output: %w", err)`))
		b.WriteString(fmt.Sprintf("%s}\n", p))
	}

	if !withTools {
		writePayload(pad)
		writeOpenAIRound(&b, st, n, pad, model, "0", maxTokens, budget != "", outputRounds, false)
		b.WriteString(fmt.Sprintf("%s%s = %s.Choices[0].Message.Content\n", pad, output, n.parsed))
		decodeJSON(pad)
		addUsage(pad, outputUsage)
		if outputToolCalls != "" {
			b.WriteString(fmt.Sprintf("%s%s = 0\n", pad, outputToolCalls))
		}
		return b.String()
	}

	roundVar := "_oaiRound" + sfx
	handledVar := "_oaiHandled" + sfx
	finishVar := "_oaiFinish" + sfx
	in := pad + "\t"
	b.WriteString(fmt.Sprintf("%sfor %s := 0; %s < %d; %s++ {\n", pad, roundVar, roundVar, maxRounds, roundVar))
	writePayload(in)
	writeOpenAIRound(&b, st, n, in, model, roundVar, maxTokens, budget != "", outputRounds, true)
	b.WriteString(fmt.Sprintf("%s%s := %s.Choices[0].FinishReason\n", in, finishVar, n.parsed))
	b.WriteString(fmt.Sprintf("%sif len(%s.Choices[0].Message.ToolCalls) == 0 {\n", in, n.parsed))
	b.WriteString(fmt.Sprintf("%s\t%s.Content = %s.Choices[0].Message.Content\n", in, output, n.parsed))
	decodeJSON(in + "\t")
	b.WriteString(fmt.Sprintf("%s\t%s.FinishReason = %s\n", in, output, finishVar))
	addUsage(in+"\t", output)
	addUsage(in+"\t", outputUsage)
	b.WriteString(fmt.Sprintf("%s\tbreak\n", in))
	b.WriteString(fmt.Sprintf("%s}\n", in))
	b.WriteString(fmt.Sprintf("%s_assistantMsg%s := map[string]any{\"role\": \"assistant\", \"content\": %s.Choices[0].Message.Content}\n", in, sfx, n.parsed))
	b.WriteString(fmt.Sprintf("%s_assistantCalls%s := make([]map[string]any, 0, len(%s.Choices[0].Message.ToolCalls))\n", in, sfx, n.parsed))
	b.WriteString(fmt.Sprintf("%sfor _, _tc%s := range %s.Choices[0].Message.ToolCalls {\n", in, sfx, n.parsed))
	b.WriteString(fmt.Sprintf("%s\t_assistantCalls%s = append(_assistantCalls%s, map[string]any{\"id\": _tc%s.ID, \"type\": _tc%s.Type, \"function\": map[string]any{\"name\": _tc%s.Function.Name, \"arguments\": _tc%s.Function.Arguments}})\n", in, sfx, sfx, sfx, sfx, sfx, sfx))
	b.WriteString(fmt.Sprintf("%s}\n", in))
	b.WriteString(fmt.Sprintf("%s_assistantMsg%s[\"tool_calls\"] = _assistantCalls%s\n", in, sfx, sfx))
	b.WriteString(fmt.Sprintf("%s%s = append(%s, _assistantMsg%s)\n", in, n.msgs, n.msgs, sfx))
	b.WriteString(fmt.Sprintf("%s%s := false\n", in, handledVar))
	b.WriteString(fmt.Sprintf("%sfor _, _tc%s := range %s.Choices[0].Message.ToolCalls {\n", in, sfx, n.parsed))
	b.WriteString(fmt.Sprintf("%s\tswitch _tc%s.Function.Name {\n", in, sfx))
	for _, spec := range toolSpecs {
		reqVarName := "_toolReq" + sfx + ExportName(spec.Method.Name)
		respVarName := "_toolResp" + sfx + ExportName(spec.Method.Name)
		rawRespVar := "_toolRespRaw" + sfx + ExportName(spec.Method.Name)
		// A tool that fails answers the model, not the caller. The model
		// asked for something the tool refused — a missing field, a date in
		// the past, a limit reached — and it is the one that can put it
		// right or say so in the user's words. Aborting the whole turn
		// instead leaves the user with silence after "one moment".
		toolErrVar := "_toolErr" + sfx + ExportName(spec.Method.Name)
		toolContentVar := "_toolContent" + sfx + ExportName(spec.Method.Name)
		p := in + "\t\t"
		b.WriteString(fmt.Sprintf("%scase %q:\n", p, spec.ToolName))
		b.WriteString(fmt.Sprintf("%s\tvar %s %s\n", p, reqVarName, spec.RequestType))
		b.WriteString(fmt.Sprintf("%s\tvar %s error\n", p, toolErrVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", p, toolContentVar))
		b.WriteString(fmt.Sprintf("%s\tif strings.TrimSpace(_tc%s.Function.Arguments) != \"\" {\n", p, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tif err := json.Unmarshal([]byte(_tc%s.Function.Arguments), &%s); err != nil {\n", p, sfx, reqVarName))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = fmt.Errorf(\"the arguments do not match the tool's parameters: %%w\", err)\n", p, toolErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", p))
		b.WriteString(fmt.Sprintf("%s\t}\n", p))
		b.WriteString(fmt.Sprintf("%s\tif %s == nil {\n", p, toolErrVar))
		if strings.EqualFold(strings.TrimSpace(spec.ServiceName), strings.TrimSpace(st.serviceName)) {
			b.WriteString(fmt.Sprintf("%s\t\t%s, err := s.%s(ctx, %s)\n", p, respVarName, ExportName(spec.Method.Name), reqVarName))
		} else {
			b.WriteString(fmt.Sprintf("%s\t\t%s, err := s.%sService.%s(ctx, %s)\n", p, respVarName, ExportName(spec.ServiceName), ExportName(spec.Method.Name), reqVarName))
		}
		b.WriteString(fmt.Sprintf("%s\t\tif err != nil {\n", p))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = err\n", p, toolErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t} else {\n", p))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s, _ := json.Marshal(%s)\n", p, rawRespVar, respVarName))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = string(%s)\n", p, toolContentVar, rawRespVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", p))
		b.WriteString(fmt.Sprintf("%s\t}\n", p))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", p, toolErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t_toolErrRaw, _ := json.Marshal(map[string]string{\"error\": %s.Error()})\n", p, toolErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s = string(_toolErrRaw)\n", p, toolContentVar))
		b.WriteString(fmt.Sprintf("%s\t}\n", p))
		b.WriteString(fmt.Sprintf("%s\t%s = append(%s, map[string]any{\"role\": \"tool\", \"tool_call_id\": _tc%s.ID, \"content\": %s})\n", p, n.msgs, n.msgs, sfx, toolContentVar))
		b.WriteString(fmt.Sprintf("%s\t%s = true\n", p, handledVar))
	}
	// A tool the model invented gets the same answer: it is told the name
	// does not exist, and the turn goes on.
	b.WriteString(fmt.Sprintf("%s\t\tdefault:\n", in))
	b.WriteString(fmt.Sprintf("%s\t\t\t_toolUnknownRaw, _ := json.Marshal(map[string]string{\"error\": \"there is no tool named \" + _tc%s.Function.Name})\n", in, sfx))
	b.WriteString(fmt.Sprintf("%s\t\t\t%s = append(%s, map[string]any{\"role\": \"tool\", \"tool_call_id\": _tc%s.ID, \"content\": string(_toolUnknownRaw)})\n", in, n.msgs, n.msgs, sfx))
	b.WriteString(fmt.Sprintf("%s\t\t\t%s = true\n", in, handledVar))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", in))
	b.WriteString(fmt.Sprintf("%s\t}\n", in))
	b.WriteString(fmt.Sprintf("%sif !%s {\n", in, handledVar))
	b.WriteString(errReturn(st, in+"\t", "fmt.Errorf(\"openai.Chat: tool calls were returned but none were executed\")"))
	b.WriteString(fmt.Sprintf("%s}\n", in))
	b.WriteString(fmt.Sprintf("%s%s.ToolCalls += len(%s.Choices[0].Message.ToolCalls)\n", in, output, n.parsed))
	if outputToolCalls != "" {
		b.WriteString(fmt.Sprintf("%s%s += len(%s.Choices[0].Message.ToolCalls)\n", in, outputToolCalls, n.parsed))
	}
	addUsage(in, output)
	addUsage(in, outputUsage)
	b.WriteString(fmt.Sprintf("%s%s.FinishReason = %s\n", in, output, finishVar))
	b.WriteString(fmt.Sprintf("%sif %s+1 == %d {\n", in, roundVar, maxRounds))
	b.WriteString(errReturn(st, in+"\t", fmt.Sprintf("fmt.Errorf(\"openai.Chat exceeded max_rounds=%d\")", maxRounds)))
	b.WriteString(fmt.Sprintf("%s}\n", in))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

// writeOpenAIRound renders one request of openai.Chat: marshal the payload,
// reserve (budget), send, parse, settle (budget) — the settle runs whatever
// the outcome, so a reservation is never left behind — and only then fail on
// an error. Afterwards <parsed> holds a response with at least one choice.
func writeOpenAIRound(b *strings.Builder, st *flowRenderState, n openAIChatNames, pad, model, round string, maxTokens int, budget bool, outputRounds string, withTools bool) {
	b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, n.body, n.payload))
	b.WriteString(fmt.Sprintf("%s%s := helpers.LLMRound{Provider: \"openai\", Model: strings.TrimSpace(fmt.Sprint(%s)), Round: %s, MaxOutputTokens: %d, RequestBytes: len(%s)}\n", pad, n.call, model, round, maxTokens, n.body))
	if budget {
		// Reserve before the call: a refusal is the step's error, and no
		// request is sent.
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, n.budget))
		b.WriteString(fmt.Sprintf("%s\tif %s := %s.Reserve(ctx, %s); %s != nil {\n", pad, n.reserveErr, n.budget, n.call, n.reserveErr))
		b.WriteString(errReturn(st, pad+"\t\t", n.reserveErr))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%s%s, %s := func() ([]byte, error) {\n", pad, n.raw, n.callErr))
	b.WriteString(fmt.Sprintf("%s\t_oaiCtx, _oaiCancel := context.WithTimeout(ctx, 60*time.Second)\n", pad))
	b.WriteString(fmt.Sprintf("%s\tdefer _oaiCancel()\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_oaiReq, _oaiReqErr := http.NewRequestWithContext(_oaiCtx, \"POST\", helpers.OpenAIURL(\"/chat/completions\"), bytes.NewReader(%s))\n", pad, n.body))
	b.WriteString(fmt.Sprintf("%s\tif _oaiReqErr != nil {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil, _oaiReqErr\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_oaiReq.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"OPENAI_API_KEY\"))\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_oaiReq.Header.Set(\"Content-Type\", \"application/json\")\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_oaiRes, _oaiResErr := http.DefaultClient.Do(_oaiReq)\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif _oaiResErr != nil {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil, _oaiResErr\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tdefer _oaiRes.Body.Close()\n", pad))
	b.WriteString(fmt.Sprintf("%s\treturn io.ReadAll(_oaiRes.Body)\n", pad))
	b.WriteString(fmt.Sprintf("%s}()\n", pad))
	b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, n.parsed))
	b.WriteString(fmt.Sprintf("%s\tChoices []struct {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tFinishReason string `json:\"finish_reason\"`\n", pad))
	if withTools {
		b.WriteString(fmt.Sprintf("%s\t\tMessage struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tContent string `json:\"content\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tToolCalls []struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tID string `json:\"id\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tType string `json:\"type\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tFunction struct {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\tName string `json:\"name\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\tArguments string `json:\"arguments\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t} `json:\"function\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t} `json:\"tool_calls\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t} `json:\"message\"`\n", pad))
	} else {
		b.WriteString(fmt.Sprintf("%s\t\tMessage struct{ Content string `json:\"content\"` } `json:\"message\"`\n", pad))
	}
	b.WriteString(fmt.Sprintf("%s\t} `json:\"choices\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tUsage struct {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tPromptTokens int `json:\"prompt_tokens\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tCompletionTokens int `json:\"completion_tokens\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tTotalTokens int `json:\"total_tokens\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tPromptTokensDetails struct{ CachedTokens int `json:\"cached_tokens\"` } `json:\"prompt_tokens_details\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tCompletionTokensDetails struct{ ReasoningTokens int `json:\"reasoning_tokens\"` } `json:\"completion_tokens_details\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\t} `json:\"usage\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tError *struct{ Message string `json:\"message\"` } `json:\"error\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, n.callErr))
	b.WriteString(fmt.Sprintf("%s\t%s = fmt.Errorf(\"openai.Chat: %%w\", %s)\n", pad, n.callErr, n.callErr))
	b.WriteString(fmt.Sprintf("%s} else if err := json.Unmarshal(%s, &%s); err != nil {\n", pad, n.raw, n.parsed))
	b.WriteString(fmt.Sprintf("%s\t%s = fmt.Errorf(\"openai.Chat: parse error: %%w\", err)\n", pad, n.callErr))
	b.WriteString(fmt.Sprintf("%s} else if %s.Error != nil {\n", pad, n.parsed))
	b.WriteString(fmt.Sprintf("%s\t%s = fmt.Errorf(\"openai.Chat: API error: %%s\", %s.Error.Message)\n", pad, n.callErr, n.parsed))
	b.WriteString(fmt.Sprintf("%s} else if len(%s.Choices) == 0 {\n", pad, n.parsed))
	b.WriteString(fmt.Sprintf("%s\t%s = fmt.Errorf(\"openai.Chat: no choices in response\")\n", pad, n.callErr))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := helpers.LLMUsage{InputTokens: %s.Usage.PromptTokens, CachedInputTokens: %s.Usage.PromptTokensDetails.CachedTokens, OutputTokens: %s.Usage.CompletionTokens, ReasoningTokens: %s.Usage.CompletionTokensDetails.ReasoningTokens, TotalTokens: %s.Usage.TotalTokens}\n", pad, n.usage, n.parsed, n.parsed, n.parsed, n.parsed, n.parsed))
	if budget {
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, n.budget))
		b.WriteString(fmt.Sprintf("%s\tif %s := %s.Settle(ctx, %s, %s, %s); %s != nil && %s == nil {\n", pad, n.settleErr, n.budget, n.call, n.usage, n.callErr, n.settleErr, n.callErr))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, n.callErr, n.settleErr))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	if outputRounds != "" {
		b.WriteString(fmt.Sprintf("%s%s = append(%s, helpers.LLMRoundUsage{Round: %s, Usage: %s})\n", pad, outputRounds, outputRounds, n.call, n.usage))
	} else {
		b.WriteString(fmt.Sprintf("%s_ = %s\n", pad, n.usage))
		if !budget {
			b.WriteString(fmt.Sprintf("%s_ = %s\n", pad, n.call))
		}
	}
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, n.callErr))
	b.WriteString(errReturn(st, pad+"\t", n.callErr))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
}
