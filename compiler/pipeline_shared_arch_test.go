package compiler

import (
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func collectWarnings() (*[]normalizer.Warning, PipelineOptions) {
	var got []normalizer.Warning
	return &got, PipelineOptions{WarningSink: func(w normalizer.Warning) { got = append(got, w) }}
}

func hasCode(warnings []normalizer.Warning, code string) bool {
	for _, w := range warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func companyEntity(reason string) normalizer.Entity {
	metadata := map[string]any{"shared_arch": true}
	if reason != "" {
		metadata["shared_arch_reason"] = reason
	}
	return normalizer.Entity{Name: "Company", Owner: "company", Source: "cue/domain/company.cue:10", Metadata: metadata}
}

func findCompany(service string, step normalizer.FlowStep) normalizer.Service {
	return normalizer.Service{Name: service, Methods: []normalizer.Method{{Name: "GetCompany", Flow: []normalizer.FlowStep{step}}}}
}

var repoFindCompany = normalizer.FlowStep{Action: "repo.Find", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company"}}

func TestEmitSharedArchDiagnostics_RequiresReason(t *testing.T) {
	t.Parallel()
	got, opts := collectWarnings()
	emitSharedArchDiagnostics([]normalizer.Entity{companyEntity("")}, []normalizer.Service{findCompany("Tender", repoFindCompany)}, opts)
	if !hasCode(*got, "SHARED_ARCH_REASON_REQUIRED") {
		t.Fatalf("expected SHARED_ARCH_REASON_REQUIRED, got: %#v", *got)
	}
}

// Every access from the entity's own context: nothing crosses a boundary.
func TestEmitSharedArchDiagnostics_OwnContextOnlyIsUnderused(t *testing.T) {
	t.Parallel()
	got, opts := collectWarnings()
	emitSharedArchDiagnostics([]normalizer.Entity{companyEntity("legacy")}, []normalizer.Service{
		findCompany("Company", repoFindCompany),
		findCompany("Admin", repoFindCompany),
	}, opts)
	if !hasCode(*got, "SHARED_ARCH_UNDERUSED") {
		t.Fatalf("expected SHARED_ARCH_UNDERUSED, got: %#v", *got)
	}
}

// One foreign context is enough: removing the mark would fail the boundary
// check. The old "fewer than two contexts" rule reported this as underused.
func TestEmitSharedArchDiagnostics_SingleForeignFlowUserNeedsTheMark(t *testing.T) {
	t.Parallel()
	got, opts := collectWarnings()
	emitSharedArchDiagnostics([]normalizer.Entity{companyEntity("tender reads companies")}, []normalizer.Service{findCompany("Tender", repoFindCompany)}, opts)
	if hasCode(*got, "SHARED_ARCH_UNDERUSED") {
		t.Fatalf("a foreign flow user needs the mark, got: %#v", *got)
	}
}

// A repository call in Go written in CUE crosses the boundary as well.
func TestEmitSharedArchDiagnostics_ForeignGoBlockUserCounts(t *testing.T) {
	t.Parallel()
	got, opts := collectWarnings()
	logicCall := normalizer.FlowStep{Action: "logic.Call", Args: map[string]any{
		"func": "(func(ctx context.Context, id string) error {\n\t_, err := s.CompanyRepo.FindByID(ctx, id)\n\treturn err\n})",
	}}
	emitSharedArchDiagnostics([]normalizer.Entity{companyEntity("search indexes companies")}, []normalizer.Service{
		findCompany("Company", repoFindCompany),
		findCompany("Search", logicCall),
	}, opts)
	if hasCode(*got, "SHARED_ARCH_UNDERUSED") {
		t.Fatalf("a Go-block user from another context needs the mark, got: %#v", *got)
	}
	register := SharedArchRegister([]normalizer.Entity{companyEntity("x")}, []normalizer.Service{findCompany("Search", logicCall)})
	if len(register) != 1 || len(register[0].ForeignGoUsers) != 1 || register[0].ForeignGoUsers[0] != "search" || len(register[0].ForeignFlowUsers) != 0 {
		t.Fatalf("register = %#v", register)
	}
}

// The register is architecture debt for ang vet, not a warning on every build.
func TestEmitSharedArchDiagnostics_NoAuditWarningInBuild(t *testing.T) {
	t.Parallel()
	got, opts := collectWarnings()
	emitSharedArchDiagnostics([]normalizer.Entity{companyEntity("tender reads companies")}, []normalizer.Service{findCompany("Tender", repoFindCompany)}, opts)
	if hasCode(*got, "SHARED_ARCH_AUDIT") {
		t.Fatalf("SHARED_ARCH_AUDIT must not be emitted by the build, got: %#v", *got)
	}
}
