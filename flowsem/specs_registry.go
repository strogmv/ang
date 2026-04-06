package flowsem

// specs is built in init() so specs_infra.init() can register oauth/locale (and similar)
// before mergeSpecs runs.
var specs map[string]Spec

func init() {
	specs = mergeSpecs(specsCoreBase, specsCoreFlow, specsDomainOps, specsInfra)
}

func mergeSpecs(chunks ...map[string]Spec) map[string]Spec {
	out := make(map[string]Spec)
	for _, chunk := range chunks {
		for action, spec := range chunk {
			out[action] = spec
		}
	}
	return out
}
