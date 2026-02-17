package compiler

import (
	"fmt"
	"os"

	planpkg "github.com/strogmv/ang/compiler/plan"
)

func ApplyPlan(basePath string, p *planpkg.BuildPlan) error {
	if err := planpkg.ValidatePlan(p); err != nil {
		return err
	}
	currentInputHash, _ := ComputeProjectHash(basePath)
	if p.InputHash != "" && currentInputHash != "" && p.InputHash != currentInputHash {
		return fmt.Errorf("plan precondition failed: input hash mismatch")
	}
	currentWorkspaceHash, _ := planpkg.ComputeWorkspaceHash(basePath)
	if p.Preconditions.WorkspaceHash != "" && currentWorkspaceHash != "" && p.Preconditions.WorkspaceHash != currentWorkspaceHash {
		return fmt.Errorf("plan precondition failed: workspace hash mismatch")
	}

	// PR1: apply contract and preconditions only; file writes land in PR3.
	if os.Getenv("ANG_PLAN_APPLY_STRICT") == "1" {
		return fmt.Errorf("apply phase is not implemented yet (PR1 scaffold)")
	}
	return nil
}
