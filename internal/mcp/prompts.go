package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("ang/add-entity",
		mcp.WithPromptDescription("Guided flow to add a new domain entity in CUE and regenerate artifacts."),
		mcp.WithArgument("entity_name", mcp.ArgumentDescription("Entity name, e.g. User"), mcp.RequiredArgument()),
		mcp.WithArgument("fields", mcp.ArgumentDescription("Comma-separated fields, e.g. id:string,email:string,name:string"), mcp.RequiredArgument()),
		mcp.WithArgument("owner", mcp.ArgumentDescription("Owning service name, optional")),
	), func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name := strings.TrimSpace(request.Params.Arguments["entity_name"])
		fields := strings.TrimSpace(request.Params.Arguments["fields"])
		owner := strings.TrimSpace(request.Params.Arguments["owner"])
		if name == "" || fields == "" {
			return nil, fmt.Errorf("entity_name and fields are required")
		}
		text := fmt.Sprintf(
			"You are adding a new entity to ANG intent.\n"+
				"Goal:\n- entity: %s\n- fields: %s\n- owner: %s\n\n"+
				"Execution plan (use MCP tools in order):\n"+
				"1) Call ang_plan with goal: \"add entity %s with fields %s\".\n"+
				"2) Prefer cue_set_field for each new field when target entity already exists.\n"+
				"3) If structural edits are needed (new blocks/files), apply cue_apply_patch entries from plan.\n"+
				"4) Run run_preset('build').\n"+
				"5) If build fails, run ang_doctor and apply safe suggested patch(es), then run run_preset('build') again.\n"+
				"6) Return concise summary: changed cue files, build status, and generated artifact impact.\n\n"+
				"Field-level policy:\n- Use cue_set_field for single field add/update (predictable).\n- Use cue_apply_patch only for non-field structural changes.\n\n"+
				"Rules:\n- Edit only cue/* for intent changes.\n- Do not hand-edit generated internal/* files.\n- Keep output deterministic and minimal.\n",
			name, fields, owner, name, fields,
		)
		return mcp.NewGetPromptResult(
			"ANG guided prompt: add entity",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
			},
		), nil
	})

	s.AddPrompt(mcp.NewPrompt("ang/add-endpoint",
		mcp.WithPromptDescription("Guided flow to add HTTP endpoint in CUE and validate generation."),
		mcp.WithArgument("method", mcp.ArgumentDescription("HTTP method, e.g. POST"), mcp.RequiredArgument()),
		mcp.WithArgument("path", mcp.ArgumentDescription("HTTP path, e.g. /orders"), mcp.RequiredArgument()),
		mcp.WithArgument("service", mcp.ArgumentDescription("Service name, e.g. Orders"), mcp.RequiredArgument()),
		mcp.WithArgument("rpc", mcp.ArgumentDescription("RPC/method name, e.g. CreateOrder"), mcp.RequiredArgument()),
	), func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		method := strings.ToUpper(strings.TrimSpace(request.Params.Arguments["method"]))
		path := strings.TrimSpace(request.Params.Arguments["path"])
		service := strings.TrimSpace(request.Params.Arguments["service"])
		rpc := strings.TrimSpace(request.Params.Arguments["rpc"])
		if method == "" || path == "" || service == "" || rpc == "" {
			return nil, fmt.Errorf("method, path, service, rpc are required")
		}
		text := fmt.Sprintf(
			"Add an endpoint to ANG intent.\n"+
				"Target: %s %s -> %s.%s\n\n"+
				"Execution plan:\n"+
				"1) Call ang_schema to capture current entities/services/endpoints.\n"+
				"2) Call ang_plan with goal: \"add endpoint %s %s mapped to %s.%s\".\n"+
				"3) Apply cue patches from plan (cue_apply_patch).\n"+
				"4) Run run_preset('build').\n"+
				"5) If failed: ang_doctor -> apply safe fix -> build again.\n"+
				"6) Report endpoint presence in generated OpenAPI and any warnings.\n\n"+
				"Constraints:\n- Intent changes only in cue/api and cue/architecture as needed.\n- No direct edits in generated output folders.\n",
			method, path, service, rpc, method, path, service, rpc,
		)
		return mcp.NewGetPromptResult(
			"ANG guided prompt: add endpoint",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
			},
		), nil
	})

	s.AddPrompt(mcp.NewPrompt("ang/fix-bug",
		mcp.WithPromptDescription("Guided bug-fix flow based on build log and doctor suggestions."),
		mcp.WithArgument("goal", mcp.ArgumentDescription("Short bug goal, optional")),
	), func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		goal := strings.TrimSpace(request.Params.Arguments["goal"])
		if goal == "" {
			goal = "fix current build/runtime issue"
		}
		text := fmt.Sprintf(
			"Fix bug workflow for ANG project.\nGoal: %s\n\n"+
				"Execution plan:\n"+
				"1) Call ang_schema to confirm current entities/services/endpoints baseline.\n"+
				"2) Read build log resource: resource://ang/logs/build.\n"+
				"3) Run ang_doctor to extract structured error codes and suggestions.\n"+
				"4) For field-level fixes, use cue_set_field. For structural fixes, use cue_apply_patch.\n"+
				"5) Run run_preset('build').\n"+
				"6) If patch made things worse, inspect cue_history and use cue_undo.\n"+
				"7) If still failing, iterate doctor -> patch -> build up to 3 times.\n"+
				"8) Return final status with errors_fixed/errors_remaining and changed files.\n\n"+
				"Safety:\n- Prefer smallest valid patch.\n- Keep edits in cue/* only.\n- Preserve unrelated user changes.\n",
			goal,
		)
		return mcp.NewGetPromptResult(
			"ANG guided prompt: fix bug",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewEmbeddedResource(mcp.TextResourceContents{
					URI:      "resource://ang/logs/build",
					MIMEType: "text/plain",
					Text:     "Use this resource if available as primary diagnostic context.",
				})),
			},
		), nil
	})
}
