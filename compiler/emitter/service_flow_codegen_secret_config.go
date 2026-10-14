package emitter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepSecretConfig(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	var key, defVal, output string
	switch step.Name {
	case "secret.Get":
		typed, err := typedActionAs[flowir.SecretGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key, defVal, output = normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Default.Source), typed.Output
	case "config.Get":
		typed, err := typedActionAs[flowir.ConfigGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key, defVal, output = normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Default.Source), typed.Output
	case "model.Resolve":
		typed, err := typedActionAs[flowir.ModelResolve](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		name := normalizeFlowExpr(typed.Name.Source)
		output = typed.Output
		defVal = normalizeFlowExpr(typed.Default.Source)
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		var b strings.Builder
		if models := normalizer.InfraModels(st.infraValues); models != nil {
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
	default:
		return "", false
	}

	st.declared[output] = true
	st.pointers[output] = false
	st.types[output] = "string"
	var b strings.Builder
	// target.service_config: a config.Get of a key the generated config
	// declares reads the loaded configuration, the one the server started
	// with (env, .env and CUE defaults), instead of the process environment.
	if field := flowServiceConfigField(st.infraValues, key); step.Name == "config.Get" && field != "" {
		b.WriteString(fmt.Sprintf("%svar %s string = s.Cfg.%s\n", pad, output, field))
		if defVal != "" {
			b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n%s\t%s = %s\n%s}\n", pad, output, pad, output, defVal, pad))
		}
		return b.String(), true
	}
	b.WriteString(fmt.Sprintf("%svar %s string = os.Getenv(%s)\n", pad, output, key))
	if defVal != "" {
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n%s\t%s = %s\n%s}\n", pad, output, pad, output, defVal, pad))
	}
	return b.String(), true
}

// flowInfraKeyServiceConfig holds, when target.service_config is on, the Go
// names of the string fields of the generated config by their env var.
const flowInfraKeyServiceConfig = "emitter.service_config_fields"

// flowServiceConfigField is the config field a literal config.Get key names,
// "" when service_config is off, the key is not a literal or no string field
// has that env var.
func flowServiceConfigField(infraValues map[string]any, key string) string {
	fields, _ := infraValues[flowInfraKeyServiceConfig].(map[string]string)
	if len(fields) == 0 {
		return ""
	}
	name, err := strconv.Unquote(strings.TrimSpace(key))
	if err != nil {
		return ""
	}
	return fields[name]
}
