package compiler

import (
	"fmt"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/pkg/names"
)

// validateGoRepoReferences rejects Go in an operation (impl code, a logic.Call
// func, any flow argument) that reaches s.<X>Repo when no entity X has a
// repository. A repository of an entity is always reachable: the service
// implementation gets every one its operations' Go reaches, sources or not
// (emitter.serviceImplRepoEntities). Anything else would only fail later as
// "s.XRepo undefined" in the generated Go.
func validateGoRepoReferences(services []normalizer.Service, entities []normalizer.Entity, helpers []normalizer.Helper) error {
	withRepo := map[string]string{}
	dto := map[string]string{}
	for _, ent := range entities {
		goName := names.ToGoName(ent.Name)
		if isDTO, _ := ent.Metadata["dto"].(bool); isDTO {
			dto[goName] = ent.Name
			continue
		}
		withRepo[goName] = goName
	}
	check := func(svc normalizer.Service, method normalizer.Method, code, where string) error {
		for _, goName := range normalizer.GoRepoReferences(code) {
			if _, ok := withRepo[goName]; ok {
				continue
			}
			reason := fmt.Sprintf("no entity %s has a repository", goName)
			if name, ok := dto[goName]; ok {
				reason = fmt.Sprintf("%s is a DTO and has no repository", name)
			} else if near := nearestField(goName, withRepo); near != "" && levenshteinDistance(goName, near) <= 3 {
				reason += fmt.Sprintf("; did you mean s.%sRepo?", near)
			}
			return fmt.Errorf("%s: operation %s.%s uses s.%sRepo, but %s", where, svc.Name, method.Name, goName, reason)
		}
		return nil
	}
	// A method helper reaches the repositories of every service that calls it.
	for _, h := range helpers {
		if h.Kind != normalizer.HelperMethod {
			continue
		}
		for _, goName := range normalizer.GoRepoReferences(h.Code) {
			if _, ok := withRepo[goName]; ok {
				continue
			}
			reason := fmt.Sprintf("no entity %s has a repository", goName)
			if name, ok := dto[goName]; ok {
				reason = fmt.Sprintf("%s is a DTO and has no repository", name)
			} else if near := nearestField(goName, withRepo); near != "" && levenshteinDistance(goName, near) <= 3 {
				reason += fmt.Sprintf("; did you mean s.%sRepo?", near)
			}
			return fmt.Errorf("%s: helper %s uses s.%sRepo, but %s", h.Source, h.Name, goName, reason)
		}
	}
	for _, svc := range services {
		for _, method := range svc.Methods {
			var stepErr error
			walkFlowSteps(method.Flow, func(step normalizer.FlowStep) {
				if stepErr != nil {
					return
				}
				where := method.Source
				if step.File != "" {
					where = fmt.Sprintf("%s:%d", step.File, step.Line)
				}
				stepErr = check(svc, method, normalizer.FlowStepText(step), where)
			})
			if stepErr != nil {
				return stepErr
			}
			if method.Impl != nil {
				if err := check(svc, method, method.Impl.Code, method.Source); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
