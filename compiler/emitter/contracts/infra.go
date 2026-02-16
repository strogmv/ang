package contracts

import "github.com/strogmv/ang/compiler/normalizer"

func InfraStepsForValuesGo(values map[string]any) []InfraResolved {
	return normalizer.NewInfraRegistry().StepsForValues(normalizer.InfraLanguageGo, values)
}

func InfraStepsForValuesPython(values map[string]any) []InfraResolved {
	return normalizer.NewInfraRegistry().StepsForValues(normalizer.InfraLanguagePython, values)
}

func InfraNotificationMuting(values map[string]any) *NotificationMute {
	return normalizer.InfraNotificationMuting(values)
}
