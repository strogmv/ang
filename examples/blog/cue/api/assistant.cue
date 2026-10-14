package api

import "github.com/strogmv/ang/cue/schema"

LookupPostForAssistant: schema.#Operation & {
	service: "assistant"
	uses:    ["Blog"]
	description: "Lookup one post by slug for AI tool use"

	input: {
		slug: string @validate("required,min=2,max=120")
	}

	output: {
		id:      string
		title:   string
		status?: string
		excerpt?: string
	}

	flow: [
		{action: "flow.Call", op: "Blog.GetPost", args: {slug: "req.Slug"}, output: "post"},
		{action: "mapping.Assign", to: "resp.ID", value: "post.ID"},
		{action: "mapping.Assign", to: "resp.Title", value: "post.Title"},
		{action: "mapping.Assign", to: "resp.Status", value: "post.Status"},
		{action: "mapping.Assign", to: "resp.Excerpt", value: "post.Excerpt"},
	]
}

ChatWithAssistant: schema.#Operation & {
	service: "assistant"
	description: "Chat with the assistant using an explicit current-service tool allowlist"
	primary_operation_kind: "notify"
	capabilities: ["notify"]

	input: {
		userID:  string @validate("required,min=2,max=120")
		message: string @validate("required,min=2,max=4000")
		messages?: [...{
			role:    string
			content: string
		}]
	}

	output: {
		content:          string
		finishReason?:    string
		toolCalls?:       int
		promptTokens?:    int
		completionTokens?: int
		totalTokens?:     int
	}

	flow: [
		{action: "session.Get", output: "sessionID"},
		{action: "quota.Check", key: "sessionID", limit: 50, window: "\"day\"", throw: "Daily AI assistant limit reached"},
		{action: "budget.Check", key: "sessionID", limit: 100000, throw: "AI token budget exhausted"},
		{action: "openai.Chat",
			model: "\"gpt-4o-mini\"",
			system: "\"You are an internal assistant. Use tools only when needed and return concise answers.\"",
			user_message: "req.Message",
			history: "req.Messages",
			tools: ["LookupPostForAssistant", "Blog.GetPost"],
			tool_choice: "\"auto\"",
			max_rounds: 6,
			output: "reply",
		},
		{action: "mapping.Assign", to: "resp.Content", value: "reply.Content"},
		{action: "mapping.Assign", to: "resp.FinishReason", value: "reply.FinishReason"},
		{action: "mapping.Assign", to: "resp.ToolCalls", value: "reply.ToolCalls"},
		{action: "mapping.Assign", to: "resp.PromptTokens", value: "reply.PromptTokens"},
		{action: "mapping.Assign", to: "resp.CompletionTokens", value: "reply.CompletionTokens"},
		{action: "mapping.Assign", to: "resp.TotalTokens", value: "reply.TotalTokens"},
		{action: "budget.Consume", key: "sessionID", tokens: "reply.TotalTokens"},
	]
}
