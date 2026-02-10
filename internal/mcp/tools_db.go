package mcp

import (
	"context"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func registerDBTools(addTool toolAdder) {
	addTool("ang_db_drift_detector", mcp.NewTool("ang_db_drift_detector",
		mcp.WithDescription("Detect discrepancies between CUE domain models and physical database schema"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "status")
		out, err := cmd.CombinedOutput()

		report := &ANGReport{
			Status:  "In Sync",
			Summary: []string{"Checking database drift"},
		}

		output := string(out)
		if err != nil || strings.Contains(output, "DRIFT DETECTED") || (len(output) > 0 && !strings.Contains(output, "in sync")) {
			report.Status = "Drift Detected"
			report.Summary = append(report.Summary, "Database schema is out of sync with CUE.")
			report.Artifacts = map[string]string{"sql_diff": output}
			report.NextActions = append(report.NextActions, "Run ang_db_sync to apply changes")
			report.Rationale = "Your CUE definitions changed, but the database still uses the old schema."
		} else {
			report.Summary = append(report.Summary, "Schema is healthy.")
		}

		return mcp.NewToolResultText(report.ToJSON()), nil
	})

	addTool("ang_db_sync", mcp.NewTool("ang_db_sync",
		mcp.WithDescription("Synchronize database schema with current CUE intent."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("./ang_bin", "db", "sync")
		out, err := cmd.CombinedOutput()
		status := "Success"
		if err != nil {
			status = "Failed"
		}
		report := &ANGReport{Status: status, Artifacts: map[string]string{"log": string(out)}}
		return mcp.NewToolResultText(report.ToJSON()), nil
	})
}
