package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// CodeConfigLoadInOperation: an operation's Go calls config.Load() while the
// target gives every service its configuration as s.Cfg
// (target.service_config). config.Load reads the environment and the .env
// file again on every call.
const CodeConfigLoadInOperation = "CONFIG_LOAD_IN_OPERATION"

// CheckConfigLoads reports each operation whose Go (impls: go: or a flow
// step) calls config.Load(). Call it for targets with service_config on.
func CheckConfigLoads(schema *ir.Schema) []normalizer.Warning {
	if schema == nil {
		return nil
	}
	var out []normalizer.Warning
	for _, svc := range schema.Services {
		for _, m := range svc.Methods {
			n := 0
			if m.Impl != nil {
				n += strings.Count(m.Impl.Code, "config.Load()")
			}
			n += countInFlow(m.Flow, "config.Load()")
			if n == 0 {
				continue
			}
			out = append(out, normalizer.Warning{
				Kind:     "config",
				Code:     CodeConfigLoadInOperation,
				Severity: "warn",
				Op:       m.Name,
				Message:  fmt.Sprintf("%s.%s calls config.Load() %d time(s); the service already holds the loaded configuration as s.Cfg", svc.Name, m.Name, n),
				Hint:     "replace `cfg, err := config.Load()` with `cfg := s.Cfg`",
				Path:     m.Source,
			})
		}
	}
	return out
}

func countInFlow(steps []ir.FlowStep, needle string) int {
	n := 0
	for _, st := range steps {
		for _, v := range st.Args {
			if s, ok := v.(string); ok {
				n += strings.Count(s, needle)
			}
		}
		for _, nested := range [][]ir.FlowStep{st.Steps, st.Then, st.Else, st.Default, st.IfNew, st.IfExists} {
			n += countInFlow(nested, needle)
		}
		for _, branch := range st.Cases {
			n += countInFlow(branch, needle)
		}
	}
	return n
}
