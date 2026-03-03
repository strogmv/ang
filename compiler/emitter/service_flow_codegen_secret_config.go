package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepSecretConfig(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	switch step.Action {
	case "secret.Get", "config.Get":
		key := arg("key")
		output := arg("output")
		defVal := arg("default")

		// Both secret.Get and config.Get use os.Getenv for now.
		// This avoids the complexity of injecting config structs into every service
		// and aligns with 12-factor app principles where config is in env.

		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		pad := strings.Repeat("\t", indent)
		var b strings.Builder

		// var output string = os.Getenv(key)
		b.WriteString(fmt.Sprintf("%svar %s string = os.Getenv(%s)\n", pad, output, key))

		// Apply default if provided
		if defVal != "" {
			b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, output))
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, output, defVal))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}

		return b.String(), true
	}
	return "", false
}
