package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepSecretConfig(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "secret.Get", "config.Get":
		key := arg("key")
		output := arg("output")
		defVal := arg("default")
		if key == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Action, step.Action+" requires key and output"), true
		}

		// Both secret.Get and config.Get use os.Getenv for now.
		// This avoids the complexity of injecting config structs into every service
		// and aligns with 12-factor app principles where config is in env.

		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

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
	case "model.Resolve":
		name := arg("name")
		output := arg("output")
		defVal := arg("default")
		if name == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "model.Resolve", "model.Resolve requires name and output"), true
		}

		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		pad := strings.Repeat("\t", indent)
		var b strings.Builder

		models := normalizer.InfraModels(st.infraValues)
		if models != nil {
			if alias := strings.Trim(name, "\""); alias != "" {
				if resolved := strings.TrimSpace(models.Aliases[alias]); resolved != "" {
					b.WriteString(fmt.Sprintf("%svar %s string = %q\n", pad, output, resolved))
					return b.String(), true
				}
			}
		}
		if defVal != "" {
			b.WriteString(fmt.Sprintf("%svar %s string = %s\n", pad, output, defVal))
			return b.String(), true
		}
		b.WriteString(fmt.Sprintf("%sreturn fmt.Errorf(\"model.Resolve: unknown model alias %%s\", %s)\n", pad, name))
		return b.String(), true
	}
	return "", false
}
