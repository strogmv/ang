package flowsem

var specs = mergeSpecs(specsCoreBase, specsCoreFlow, specsDomainOps, specsInfra)

func mergeSpecs(chunks ...map[string]Spec) map[string]Spec {
	out := make(map[string]Spec)
	for _, chunk := range chunks {
		for action, spec := range chunk {
			out[action] = spec
		}
	}
	return out
}
