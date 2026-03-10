package contracts

import "github.com/strogmv/ang/compiler/normalizer"

func InfraStepsForValuesGo(values map[string]any) []InfraResolved {
	return normalizer.NewInfraRegistry().StepsForValues(normalizer.InfraLanguageGo, values)
}

func InfraStepsForValuesPython(values map[string]any) []InfraResolved {
	return normalizer.NewInfraRegistry().StepsForValues(normalizer.InfraLanguagePython, values)
}

func InfraEffectHandlers(values map[string]any) *normalizer.EffectHandlersDef {
	return normalizer.InfraEffectHandlers(values)
}

func InfraModels(values map[string]any) *normalizer.ModelsDef {
	return normalizer.InfraModels(values)
}

func InfraEffectTestHandlers(values map[string]any) *normalizer.EffectHandlersDef {
	return normalizer.InfraEffectTestHandlers(values)
}

func InfraEffectMiddleware(values map[string]any) *normalizer.EffectMiddlewareCatalogDef {
	return normalizer.InfraEffectMiddleware(values)
}

func InfraNotificationMuting(values map[string]any) *NotificationMute {
	return normalizer.InfraNotificationMuting(values)
}
