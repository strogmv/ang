package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestRenderFlow_OpenAIChat_ToolLoop(t *testing.T) {
	t.Parallel()

	services := []normalizer.Service{
		{
			Name: "Assistant",
			Uses: []string{"Blog"},
			Methods: []normalizer.Method{
				{
					Name:        "LookupPostForAssistant",
					Description: "Lookup one post by slug for assistant tool use",
					Input: normalizer.Entity{
						Name: "LookupPostForAssistantRequest",
						Fields: []normalizer.Field{
							{Name: "Slug", Type: "string"},
						},
					},
					Output: normalizer.Entity{Name: "LookupPostForAssistantResponse"},
				},
			},
		},
		{
			Name: "Blog",
			Methods: []normalizer.Method{
				{
					Name:        "GetPost",
					Description: "Get one post by slug",
					Input: normalizer.Entity{
						Name: "GetPostRequest",
						Fields: []normalizer.Field{
							{Name: "Slug", Type: "string"},
						},
					},
					Output: normalizer.Entity{Name: "GetPostResponse"},
				},
			},
		},
	}

	steps := []normalizer.FlowStep{
		{
			Action: "openai.Chat",
			Args: map[string]any{
				"user_message":      "req.Message",
				"history":           "req.Messages",
				"model":             `"gpt-4o-mini"`,
				"tools":             []string{"LookupPostForAssistant", "Blog.GetPost"},
				"tool_choice":       `"Blog.GetPost"`,
				"max_rounds":        4,
				"output":            "reply",
				"output_usage":      "usage",
				"output_tool_calls": "toolCalls",
			},
		},
	}

	got := renderFlowForServiceWithSchemaAndSinkModeWithInfra(
		"Assistant",
		"ChatWithAssistant",
		false,
		steps,
		nil,
		nil,
		nil,
		map[string]any{
			flowInfraKeyServicesCatalog: services,
		},
	)

	mustContain := []string{
		`"tools": _oaiTools`,
		`"tool_choice"] = "Blog__GetPost"`,
		`for _oaiRound`,
		`LookupPostForAssistant`,
		`Blog__GetPost`,
		`json.RawMessage("{\"additionalProperties\":false,\"properties\":{\"Slug\":{\"type\":\"string\"}},\"required\":[\"Slug\"],\"type\":\"object\"}")`,
		`s.LookupPostForAssistant(ctx, _toolReq`,
		`s.BlogService.GetPost(ctx, _toolReq`,
		`reply.Content = _oaiParsed`,
		`reply.ToolCalls += len(_oaiParsed`,
		`toolCalls += len(_oaiParsed`,
		`usage.TotalTokens += _oaiParsed`,
		`openai.Chat exceeded max_rounds=4`,
	}
	for _, needle := range mustContain {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected generated tool chat to contain %q, got:\n%s", needle, got)
		}
	}
}

func TestRenderFlow_OpenAIChat_MissingToolFails(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{
			Action: "openai.Chat",
			Args: map[string]any{
				"user_message": "req.Message",
				"tools":        []string{"MissingTool"},
			},
		},
	}

	got := renderFlowForServiceWithSchemaAndSinkModeWithInfra(
		"Assistant",
		"ChatWithAssistant",
		false,
		steps,
		nil,
		nil,
		nil,
		map[string]any{
			flowInfraKeyServicesCatalog: []normalizer.Service{{Name: "Assistant"}},
		},
	)

	if !strings.Contains(got, `openai.Chat tool \"MissingTool\" not found in service \"Assistant\"`) {
		t.Fatalf("expected missing-tool error in generated flow, got:\n%s", got)
	}
}

func TestRenderFlow_OpenAIChat_StructuredOutput(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{
			Action: "openai.Chat",
			Args: map[string]any{
				"user_message":         "req.Message",
				"model":                `"gpt-4o-mini"`,
				"output":               "reply",
				"output_json":          "replyJSON",
				"response_json_name":   `"assistant_reply"`,
				"response_json_schema": `"{\"type\":\"object\",\"additionalProperties\":false,\"properties\":{\"content\":{\"type\":\"string\"}},\"required\":[\"content\"]}"`,
				"response_json_strict": true,
			},
		},
	}

	got := renderFlowForServiceWithSchemaAndSinkModeWithInfra(
		"Assistant",
		"ChatWithAssistant",
		false,
		steps,
		nil,
		nil,
		nil,
		nil,
	)

	mustContain := []string{
		`var replyJSON map[string]any`,
		`"response_format"] = map[string]any{"type": "json_schema"`,
		`"name": "assistant_reply"`,
		`"strict": true`,
		`json.RawMessage("{\"type\":\"object\",\"additionalProperties\":false,\"properties\":{\"content\":{\"type\":\"string\"}},\"required\":[\"content\"]}")`,
		`json.Unmarshal([]byte(`,
		`&replyJSON`,
		`decode structured output`,
	}
	for _, needle := range mustContain {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected generated structured chat to contain %q, got:\n%s", needle, got)
		}
	}
}
