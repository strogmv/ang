package compiler

import (
	"path/filepath"
	"reflect"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang-ir/parser"
)

// InfraBundle contains project-level infrastructure intent extracted from
// cue/infra and cue/effects.
type InfraBundle struct {
	Has              bool
	Values           map[string]any
	ContextPatch     normalizer.InfraContextPatch
	Config           *normalizer.ConfigDef
	Auth             *normalizer.AuthDef
	Models           *normalizer.ModelsDef
	Session          *normalizer.SessionDef
	Templates        []normalizer.TemplateDef
	EmailTemplates   []normalizer.EmailTemplateDef
	InfraValueExists bool
	EffectsExists    bool
}

// LoadInfraBundle loads infrastructure intent from cue/infra and cue/effects,
// merging them into one extracted bundle. cue/effects is intended for effect
// taxonomy bindings (Handlers/TestHandlers/Middleware) and overlays cue/infra.
func LoadInfraBundle(projectPath string) (InfraBundle, error) {
	return LoadInfraBundleWithRoot(projectPath, DefaultCueRoot)
}

// LoadInfraBundleWithRoot is like LoadInfraBundle but uses a custom CUE root directory.
func LoadInfraBundleWithRoot(projectPath, cueRoot string) (InfraBundle, error) {
	if cueRoot == "" {
		cueRoot = DefaultCueRoot
	}
	var out InfraBundle

	p := parser.New()
	n := normalizer.New()
	reg := normalizer.NewInfraRegistry()

	valInfra, okInfra, err := LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "infra"))
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUEInfraLoad, "load "+cueRoot+"/infra", err)
	}
	valEffects, okEffects, err := LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "effects"))
	if err != nil {
		return out, WrapContractError(StageCUE, ErrCodeCUEInfraLoad, "load "+cueRoot+"/effects", err)
	}

	out.InfraValueExists = okInfra
	out.EffectsExists = okEffects
	out.Has = okInfra || okEffects
	if !out.Has {
		return out, nil
	}

	if okInfra {
		values, err := reg.ExtractAll(n, valInfra)
		if err != nil {
			if infraErr, ok := err.(*normalizer.InfraExtractError); ok {
				code, op, unwrapErr := infraErr.FailParams()
				return out, WrapContractError(StageCUE, code, op, unwrapErr)
			}
			return out, WrapContractError(StageCUE, ErrCodeCUEInfraConfigParse, "extract infrastructure definitions", err)
		}
		out.Values = mergeInfraValues(out.Values, values)

		out.Session, err = n.ExtractSession(valInfra)
		if err != nil {
			return out, WrapContractError(StageCUE, ErrCodeCUEInfraConfigParse, "extract session definition", err)
		}
		out.Templates, err = n.ExtractTemplates(valInfra)
		if err != nil {
			return out, WrapContractError(StageCUE, ErrCodeCUEInfraConfigParse, "extract templates", err)
		}
		out.EmailTemplates, err = n.ExtractEmailTemplates(valInfra)
		if err != nil {
			return out, WrapContractError(StageCUE, ErrCodeCUEInfraConfigParse, "extract email templates", err)
		}
	}

	if okEffects {
		values, err := reg.ExtractAll(n, valEffects)
		if err != nil {
			if infraErr, ok := err.(*normalizer.InfraExtractError); ok {
				code, op, unwrapErr := infraErr.FailParams()
				return out, WrapContractError(StageCUE, code, op, unwrapErr)
			}
			return out, WrapContractError(StageCUE, ErrCodeCUEInfraConfigParse, "extract effect definitions", err)
		}
		out.Values = mergeInfraValues(out.Values, values)
	}

	out.Config = normalizer.InfraConfig(out.Values)
	out.Auth = normalizer.InfraAuth(out.Values)
	out.Models = normalizer.InfraModels(out.Values)
	out.ContextPatch = reg.BuildContextPatch(out.Values)
	return out, nil
}

func mergeInfraValues(base map[string]any, overlay map[string]any) map[string]any {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if isNilInfraValue(v) {
			if _, exists := out[k]; exists {
				continue
			}
			out[k] = nil
			continue
		}
		switch k {
		case normalizer.InfraKeyEffectHandlers, normalizer.InfraKeyEffectTestHandlers:
			merged := &normalizer.EffectHandlersDef{Bindings: map[string]normalizer.EffectHandlerBinding{}}
			if existing, ok := out[k].(*normalizer.EffectHandlersDef); ok && existing != nil {
				for kind, binding := range existing.Bindings {
					merged.Bindings[kind] = binding
				}
			}
			if incoming, ok := v.(*normalizer.EffectHandlersDef); ok && incoming != nil {
				for kind, binding := range incoming.Bindings {
					merged.Bindings[kind] = binding
				}
			}
			out[k] = merged
		case normalizer.InfraKeyEffectMiddleware:
			merged := &normalizer.EffectMiddlewareCatalogDef{Chains: map[string][]normalizer.EffectMiddlewareDef{}}
			if existing, ok := out[k].(*normalizer.EffectMiddlewareCatalogDef); ok && existing != nil {
				for kind, chain := range existing.Chains {
					cp := append([]normalizer.EffectMiddlewareDef(nil), chain...)
					merged.Chains[kind] = cp
				}
			}
			if incoming, ok := v.(*normalizer.EffectMiddlewareCatalogDef); ok && incoming != nil {
				for kind, chain := range incoming.Chains {
					cp := append([]normalizer.EffectMiddlewareDef(nil), chain...)
					merged.Chains[kind] = cp
				}
			}
			out[k] = merged
		default:
			out[k] = v
		}
	}
	return out
}

func isNilInfraValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}
