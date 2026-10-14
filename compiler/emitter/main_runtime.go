package emitter

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"sort"
	"strings"
	"time"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// MainRuntimeContext is what the generated main runs besides HTTP routes and
// subscriptions: the process role, lifecycle hooks and queue workers. With
// Lifecycle false the main is generated exactly as before these existed.
type MainRuntimeContext struct {
	// Lifecycle: the main has a run context that ends on SIGTERM and drains
	// work within ShutdownTimeout.
	Lifecycle bool
	// Roles: SERVER_ROLE=api|worker|all selects what the process runs.
	Roles bool
	// ShutdownTimeout is a Go duration string; "" means 30s.
	ShutdownTimeout string
	StartHooks      []MainHook
	ShutdownHooks   []MainHook
	Queues          []MainQueue
}

// MainHook is an operation run at start (OnStart) or on shutdown (OnShutdown).
type MainHook struct {
	Service string
	Method  string
	// Cond is the Go condition the hook runs under ("" = always): its role
	// (serveAPI / runWorkers) and its when: expression.
	Cond string
}

// ShutdownTimeoutGo is the Go statement value of the drain timeout.
func (r MainRuntimeContext) ShutdownTimeoutGo() string {
	d := strings.TrimSpace(r.ShutdownTimeout)
	if d == "" {
		d = "30s"
	}
	return fmt.Sprintf("time.ParseDuration(%q)", d)
}

// buildMainRuntime collects hooks and queues from the IR and decides whether
// the main needs the lifecycle code at all.
func buildMainRuntime(ctx MainContext) MainRuntimeContext {
	rt := MainRuntimeContext{
		Roles:           ctx.ServerRoles,
		ShutdownTimeout: ctx.ShutdownTimeout,
		Queues:          mainQueues(ctx.EntitiesIR),
	}
	for _, svc := range ctx.ServicesIR {
		for _, m := range svc.Methods {
			on := normalizer.MethodString(m.Metadata, normalizer.MetaHookOn)
			if on == "" {
				continue
			}
			hook := MainHook{
				Service: svc.Name,
				Method:  m.Name,
				Cond:    runtimeCondition(normalizer.MethodString(m.Metadata, normalizer.MetaHookRole), normalizer.MethodString(m.Metadata, normalizer.MetaHookWhen)),
			}
			if on == "OnStart" {
				rt.StartHooks = append(rt.StartHooks, hook)
			} else {
				rt.ShutdownHooks = append(rt.ShutdownHooks, hook)
			}
		}
	}
	// Services are in dependency order: start them that way, stop them in
	// reverse.
	for i, j := 0, len(rt.ShutdownHooks)-1; i < j; i, j = i+1, j-1 {
		rt.ShutdownHooks[i], rt.ShutdownHooks[j] = rt.ShutdownHooks[j], rt.ShutdownHooks[i]
	}
	rt.Lifecycle = rt.Roles || rt.ShutdownTimeout != "" || len(rt.StartHooks) > 0 || len(rt.ShutdownHooks) > 0 || len(rt.Queues) > 0
	return rt
}

// runtimeCondition joins a role and a when: expression into one Go condition.
func runtimeCondition(role, when string) string {
	var parts []string
	switch role {
	case normalizer.RoleAPI:
		parts = append(parts, "serveAPI")
	case normalizer.RoleWorker:
		parts = append(parts, "runWorkers")
	}
	if w := strings.TrimSpace(when); w != "" {
		if len(parts) > 0 {
			w = "(" + w + ")"
		}
		parts = append(parts, w)
	}
	return strings.Join(parts, " && ")
}

// subscribeWhen is the subscribe.when: expression of the method handling a
// subscription, "" when it has none.
func subscribeWhen(svc ir.Service, method string) string {
	for _, m := range svc.Methods {
		if m.Name == method {
			return normalizer.MethodString(m.Metadata, normalizer.MetaSubscribeWhen)
		}
	}
	return ""
}

// validateMainRuntime checks what the runtime declarations put into the
// generated main before it is rendered: the drain timeout parses, and every
// cfg.<Field> of a when: expression names a config field.
func validateMainRuntime(ctx MainContext, rt MainRuntimeContext, config *ir.Config) error {
	if d := strings.TrimSpace(rt.ShutdownTimeout); d != "" {
		if v, err := time.ParseDuration(d); err != nil || v <= 0 {
			return fmt.Errorf("target.shutdown_timeout %q is not a positive duration such as \"30s\"", d)
		}
	}
	known := map[string]bool{}
	add := func(name string) {
		known[strings.ToLower(strings.ReplaceAll(name, "_", ""))] = true
	}
	for _, f := range ensureRuntimeConfigFields(nil).Fields {
		add(f.Name)
	}
	if config != nil {
		for _, f := range config.Fields {
			add(f.Name)
		}
	}
	type cond struct{ where, expr string }
	var conds []cond
	for _, svc := range ctx.ServicesIR {
		for _, m := range svc.Methods {
			for _, key := range []string{normalizer.MetaSubscribeWhen, normalizer.MetaHookWhen} {
				if expr := normalizer.MethodString(m.Metadata, key); expr != "" {
					where := svc.Name + "." + m.Name
					if m.Source != "" {
						where = m.Source + ": " + where
					}
					conds = append(conds, cond{where, expr})
				}
			}
		}
	}
	for _, q := range rt.Queues {
		if q.When != "" {
			conds = append(conds, cond{q.Source + ": queue " + q.Entity, q.When})
		}
	}
	var errs []string
	for _, c := range conds {
		expr, err := goparser.ParseExpr(c.expr)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: when %q is not a Go expression: %v", c.where, c.expr, err))
			continue
		}
		var unknown []string
		ast.Inspect(expr, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "cfg" && !known[strings.ToLower(sel.Sel.Name)] {
				unknown = append(unknown, sel.Sel.Name)
			}
			return true
		})
		if len(unknown) > 0 {
			sort.Strings(unknown)
			errs = append(errs, fmt.Sprintf("%s: when %q uses cfg.%s, which is not a config field (cue/infra #AppConfig)", c.where, c.expr, strings.Join(unknown, ", cfg.")))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// MainQueue is a queue entity's worker started by the main (EmitQueuesFromIR
// writes its store).
type MainQueue struct {
	Entity      string
	Type        string
	Service     string
	Handler     string
	Inputs      []MainQueueInput
	When        string
	Source      string
	Concurrency int
	Attempts    int
	Poll        string
	Heartbeat   string
	Stale       string
	Backoff     string
	MaxBackoff  string
}

// MainQueueInput fills the handler request field Request from job.Job.
type MainQueueInput struct {
	Request string
	Job     string
}

// Cond is the Go condition the worker starts under.
func (q MainQueue) Cond() string {
	return runtimeCondition(normalizer.RoleWorker, q.When)
}

// goDuration writes a duration string ("90s") as a Go expression
// ("90 * time.Second"); the normalizer has checked that it parses.
func goDuration(v string) string {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return "0"
	}
	for _, u := range []struct {
		unit time.Duration
		name string
	}{{time.Hour, "time.Hour"}, {time.Minute, "time.Minute"}, {time.Second, "time.Second"}, {time.Millisecond, "time.Millisecond"}} {
		if d%u.unit == 0 {
			if d == u.unit {
				return u.name
			}
			return fmt.Sprintf("%d * %s", d/u.unit, u.name)
		}
	}
	return fmt.Sprintf("time.Duration(%d)", int64(d))
}
