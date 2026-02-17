package plan

import "fmt"

func ValidatePlan(p *BuildPlan) error {
	if p == nil {
		return fmt.Errorf("plan is nil")
	}
	if p.SchemaVersion == "" {
		return fmt.Errorf("schemaVersion is required")
	}
	if p.SchemaVersion != "1" {
		return fmt.Errorf("unsupported schemaVersion: %s", p.SchemaVersion)
	}
	if p.PlanVersion == "" {
		return fmt.Errorf("planVersion is required")
	}
	if p.WorkspaceRoot == "" {
		return fmt.Errorf("workspaceRoot is required")
	}
	if p.Status == "" {
		return fmt.Errorf("status is required")
	}
	return nil
}
