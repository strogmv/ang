package normalizer

import (
	"fmt"
	goparser "go/parser"
	"strings"

	"cuelang.org/go/cue"
)

// Method metadata keys of the runtime declarations of an operation.
const (
	// MetaSubscribeWhen is subscribe: {when: "<Go bool expression>"}: the
	// operation's event subscriptions are made only when it holds at start
	// (cfg is the loaded *config.Config).
	MetaSubscribeWhen = "subscribe_when"
	// MetaHookOn is "OnStart" or "OnShutdown" (hook:).
	MetaHookOn = "hook_on"
	// MetaHookRole is the process role a hook runs in: "all" (default),
	// "api" or "worker".
	MetaHookRole = "hook_role"
	// MetaHookWhen is a Go bool expression; the hook runs only when it holds.
	MetaHookWhen = "hook_when"
)

// Process roles of the generated server (SERVER_ROLE).
const (
	RoleAll    = "all"
	RoleAPI    = "api"
	RoleWorker = "worker"
)

// parseOperationRuntime reads the operation keys that shape what the
// generated main runs around the operation:
//
//	subscribe: {when: "cfg.JobsEnabled"}
//	hook: "OnStart"   // or {on: "OnShutdown", role: "worker", when: "cfg.JobsEnabled"}
func parseOperationRuntime(value cue.Value, opName string, method *Method) error {
	pos := func(v cue.Value) string {
		if p := formatPos(v); p != "" {
			return p + ": "
		}
		return ""
	}
	setMeta := func(key string, v any) {
		if method.Metadata == nil {
			method.Metadata = map[string]any{}
		}
		method.Metadata[key] = v
	}

	if sub := value.LookupPath(cue.ParsePath("subscribe")); sub.Exists() {
		whenVal := sub.LookupPath(cue.ParsePath("when"))
		when, err := whenVal.String()
		if err != nil {
			return fmt.Errorf("%soperation %s: subscribe.when must be a Go bool expression such as \"cfg.JobsEnabled\": %v", pos(sub), opName, err)
		}
		if err := checkRuntimeCondition(when); err != nil {
			return fmt.Errorf("%soperation %s: subscribe.when %q: %v", pos(whenVal), opName, when, err)
		}
		setMeta(MetaSubscribeWhen, strings.TrimSpace(when))
	}

	hookVal := value.LookupPath(cue.ParsePath("hook"))
	if !hookVal.Exists() {
		return nil
	}
	on, role, when := "", RoleAll, ""
	if s, err := hookVal.String(); err == nil {
		on = s
	} else if hookVal.IncompleteKind() == cue.StructKind {
		on = getString(hookVal, "on")
		if r := strings.TrimSpace(getString(hookVal, "role")); r != "" {
			role = r
		}
		when = strings.TrimSpace(getString(hookVal, "when"))
	} else {
		return fmt.Errorf("%soperation %s: hook must be \"OnStart\", \"OnShutdown\" or {on, role?, when?}", pos(hookVal), opName)
	}
	switch strings.TrimSpace(on) {
	case "OnStart", "OnShutdown":
		on = strings.TrimSpace(on)
	default:
		return fmt.Errorf("%soperation %s: hook.on is %q; want \"OnStart\" or \"OnShutdown\"", pos(hookVal), opName, on)
	}
	switch role {
	case RoleAll, RoleAPI, RoleWorker:
	default:
		return fmt.Errorf("%soperation %s: hook.role is %q; want \"all\", \"api\" or \"worker\"", pos(hookVal), opName, role)
	}
	if when != "" {
		if err := checkRuntimeCondition(when); err != nil {
			return fmt.Errorf("%soperation %s: hook.when %q: %v", pos(hookVal), opName, when, err)
		}
	}
	if len(method.Input.Fields) > 0 {
		return fmt.Errorf("%soperation %s: a hook is called with an empty request, so its input must be empty (has %s)", pos(hookVal), opName, method.Input.Fields[0].Name)
	}
	setMeta(MetaHookOn, on)
	setMeta(MetaHookRole, role)
	if when != "" {
		setMeta(MetaHookWhen, when)
	}
	return nil
}

// checkRuntimeCondition accepts a Go expression; config field names are
// checked against the config later, when the IR is complete.
func checkRuntimeCondition(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return fmt.Errorf("is empty")
	}
	if _, err := goparser.ParseExpr(expr); err != nil {
		return fmt.Errorf("is not a Go expression: %v", err)
	}
	return nil
}

// MethodString reads a string metadata key of a method.
func MethodString(meta map[string]any, key string) string {
	s, _ := meta[key].(string)
	return s
}
