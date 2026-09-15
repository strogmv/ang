package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// emitBareErrorDiagnostics warns when a logic.Call function builds errors with
// fmt.Errorf in an operation served over HTTP: such an error has no status, so
// the client gets a 500 even for its own mistakes. Event subscribers and
// worker-only operations have no HTTP status to get wrong, so they are left
// alone — on dealingi-back all four warnings that had a position were of that
// kind. Nested steps are checked like top-level ones, and every warning carries
// the step position.
func emitBareErrorDiagnostics(services []normalizer.Service, endpoints []normalizer.Endpoint, opts PipelineOptions) {
	served := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		if strings.TrimSpace(ep.RPC) == "" {
			continue
		}
		served[rawBodyOpKey(ep.ServiceName, ep.RPC)] = struct{}{}
	}
	for _, svc := range services {
		for _, method := range svc.Methods {
			if _, ok := served[rawBodyOpKey(svc.Name, method.Name)]; !ok {
				continue
			}
			walkFlowSteps(method.Flow, func(step normalizer.FlowStep) {
				if step.Action != "logic.Call" {
					return
				}
				function, _ := step.Args["func"].(string)
				if !strings.Contains(function, "fmt.Errorf(") {
					return
				}
				// Steps assembled from definitions can lack a position; the
				// operation's own position is the next best place to look.
				file, line, column := step.File, step.Line, step.Column
				if strings.TrimSpace(file) == "" {
					file, line = parseSourcePos(method.Source)
					column = 0
				}
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "flow",
					Code:     "LAMBDA_BARE_ERROR",
					Severity: "warn",
					Message:  fmt.Sprintf("%s.%s is served over HTTP and its logic.Call uses fmt.Errorf, which reaches the client as 500", svc.Name, method.Name),
					Hint:     `Return errors.New(http.StatusXxx, "Code", "Message") for errors the client causes; use http.StatusInternalServerError explicitly for internal failures.`,
					File:     file,
					Line:     line,
					Column:   column,
					CUEPath:  step.CUEPath,
				}, opts)
			})
		}
	}
}
