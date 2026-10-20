package compiler

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// Diagnostic codes of the route rights check (target.route_rights, auth.right).
const (
	// CodeAuthRightMissing: a route gated by one of target.route_rights.roles
	// does not say which right it needs.
	CodeAuthRightMissing = "AUTH_RIGHT_MISSING"
	// CodeAuthRightUnknown: auth.right is not one of target.route_rights.rights.
	CodeAuthRightUnknown = "AUTH_RIGHT_UNKNOWN"
	// CodeAuthRightUnchecked: routes declare auth.right but no CheckRoleAccess
	// operation takes a right input, so the right would never be checked.
	CodeAuthRightUnchecked = "AUTH_RIGHT_UNCHECKED"
)

// CheckRouteRights refuses a build in which a route's right would go
// unchecked or unsaid: under target.route_rights every HTTP route whose
// auth.roles names one of its roles must declare auth.right (one of its
// rights, when it lists them), and any auth.right needs a CheckRoleAccess
// operation with a right input (the gate hands the right to it). WebSocket
// routes take no right (their gate runs before the upgrade, on the role).
func CheckRouteRights(schema *ir.Schema, def *normalizer.RouteRightsDef) error {
	if schema == nil {
		return nil
	}
	var missing, unknown, declared []string
	for _, ep := range schema.Endpoints {
		if ep.Auth == nil || strings.EqualFold(ep.Method, "WS") {
			continue
		}
		label := endpointLabel(ep)
		if ep.Source != "" {
			label = ep.Source + ": " + label
		}
		if ep.Auth.Right != "" {
			declared = append(declared, label)
		}
		if def == nil {
			continue
		}
		gated := false
		for _, role := range ep.Auth.Roles {
			if slices.Contains(def.Roles, role) {
				gated = true
			}
		}
		switch {
		case gated && ep.Auth.Right == "":
			missing = append(missing, label)
		case ep.Auth.Right != "" && len(def.Rights) > 0 && !slices.Contains(def.Rights, ep.Auth.Right):
			unknown = append(unknown, fmt.Sprintf("%s: right %q", label, ep.Auth.Right))
		}
	}
	var problems []string
	if len(missing) > 0 {
		sort.Strings(missing)
		problems = append(problems, fmt.Sprintf("%s: %d route(s) gated by %s declare no right (auth: {roles: [...], right: \"...\"}):\n  %s",
			CodeAuthRightMissing, len(missing), strings.Join(def.Roles, ", "), strings.Join(missing, "\n  ")))
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		problems = append(problems, fmt.Sprintf("%s: %d route(s) declare a right target.route_rights.rights does not list (%s):\n  %s",
			CodeAuthRightUnknown, len(unknown), strings.Join(def.Rights, ", "), strings.Join(unknown, "\n  ")))
	}
	if len(declared) > 0 && !roleAccessTakesRight(schema) {
		problems = append(problems, fmt.Sprintf("%s: %d route(s) declare auth.right, but no CheckRoleAccess operation has a right input, so nothing would check it (first: %s)",
			CodeAuthRightUnchecked, len(declared), declared[0]))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
}

func roleAccessTakesRight(schema *ir.Schema) bool {
	for _, svc := range schema.Services {
		for _, m := range svc.Methods {
			if m.Name != "CheckRoleAccess" || m.Input == nil {
				continue
			}
			for _, f := range m.Input.Fields {
				if f.Name == "right" {
					return true
				}
			}
		}
	}
	return false
}
