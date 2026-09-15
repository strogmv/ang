package emitter

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"text/template"

	"github.com/strogmv/ang-ir/normalizer"
)

// serviceImplMethodJob is one generated service method file.
type serviceImplMethodJob struct {
	name       string
	ctx        TemplateContext
	unit       string // formatter unit, relative to the backend
	path       string // where the file is written
	sourcePath string // the file's place in the project, for //line paths
}

// renderFlowTemplateArgs is the RenderFlow template function:
// (service, [method], [isStreaming], steps).
func renderFlowTemplateArgs(args []any, infra map[string]any, services []normalizer.Service, entities []normalizer.Entity, events []normalizer.EventDef, sink func(normalizer.Warning)) string {
	if len(args) < 2 {
		return ""
	}
	serviceName, _ := args[0].(string)
	methodName := ""
	isStreaming := false
	var steps []normalizer.FlowStep
	switch len(args) {
	case 2:
		steps, _ = args[1].([]normalizer.FlowStep)
	case 3:
		methodName, _ = args[1].(string)
		steps, _ = args[2].([]normalizer.FlowStep)
	default:
		methodName, _ = args[1].(string)
		if b, ok := args[2].(bool); ok {
			isStreaming = b
			steps, _ = args[3].([]normalizer.FlowStep)
		} else {
			steps, _ = args[2].([]normalizer.FlowStep)
		}
	}
	infraValues := cloneInfraValues(infra)
	infraValues[flowInfraKeyServicesCatalog] = services
	return renderFlowForServiceWithSchemaAndSinkModeWithInfra(serviceName, methodName, isStreaming, steps, entities, events, sink, infraValues)
}

// runServiceImplMethodJobs renders, formats and writes method files on up to
// ServiceImplWorkers goroutines. Each worker executes its own clone of the
// method template whose RenderFlow collects warnings per job; warnings reach
// WarningSink afterwards in job order, so diagnostics come out exactly as a
// sequential run would give them, and the first failing job in that order is
// the error returned.
func (e *Emitter) runServiceImplMethodJobs(methodT *template.Template, renderFlowWith func(func(normalizer.Warning)) func(args ...any) string, jobs []serviceImplMethodJob) error {
	if len(jobs) == 0 {
		return nil
	}
	workers := e.ServiceImplWorkers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	type result struct {
		warnings []normalizer.Warning
		err      error
	}
	results := make([]result, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		t, err := methodT.Clone()
		if err != nil {
			close(next)
			wg.Wait()
			return err
		}
		var current *result
		t.Funcs(template.FuncMap{"RenderFlow": renderFlowWith(func(warning normalizer.Warning) {
			current.warnings = append(current.warnings, warning)
		})})
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				current = &results[i]
				current.err = writeServiceImplMethod(t, jobs[i])
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()

	for i, r := range results {
		if e.WarningSink != nil {
			for _, warning := range r.warnings {
				e.WarningSink(warning)
			}
		}
		if r.err != nil {
			return r.err
		}
		logGenerated("Generated Service Impl Method: %s\n", jobs[i].path)
	}
	return nil
}

func writeServiceImplMethod(t *template.Template, job serviceImplMethodJob) error {
	var buf bytes.Buffer
	if err := t.Execute(&buf, job.ctx); err != nil {
		return fmt.Errorf("execute method template for %s: %w", job.name, err)
	}
	formatted, err := formatGoStrict(buf.Bytes(), job.unit)
	if err != nil {
		return err
	}
	formatted = finalizeLineDirectives(formatted, job.sourcePath)
	return writeFileAtomic(job.path, formatted, 0644)
}
