package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func registerPlanTools(addTool toolAdder) {
	addTool("ang_plan", mcp.NewTool("ang_plan",
		mcp.WithDescription("Create a structured architecture plan from a natural-language goal and current CUE intent."),
		mcp.WithString("goal", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := strings.TrimSpace(mcp.ParseString(request, "goal", ""))
		if goal == "" {
			return mcp.NewToolResultText(`{"status":"invalid","message":"goal is required"}`), nil
		}
		plan, err := buildGoalPlan(goal)
		if err != nil {
			return mcp.NewToolResultText((&ANGReport{
				Status:      "Failed",
				Summary:     []string{"Unable to build plan from current intent."},
				NextActions: []string{"Fix CUE validation errors and retry ang_plan"},
				Artifacts:   map[string]string{"error": err.Error()},
			}).ToJSON()), nil
		}
		b, _ := json.MarshalIndent(plan, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})

	addTool("ang_doctor", mcp.NewTool("ang_doctor",
		mcp.WithDescription("Analyze build logs and suggest fixes."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logData, _ := os.ReadFile("ang-build.log")
		log := string(logData)
		resp := buildDoctorResponse(log)
		b, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}
