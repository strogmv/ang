package generator

import "testing"

func FuzzStepRegistry_Register_NoPanic(f *testing.F) {
	f.Add("Service Impls", "go:service_impl", "Server Main", "go:server_main")
	f.Add("Service Impls", "go:service_impl", "Service Impls", "go:service_impl")
	f.Add("", "", "x", "y")

	f.Fuzz(func(t *testing.T, nameA, keyA, nameB, keyB string) {
		reg := NewStepRegistry()
		reg.Register(Step{Name: nameA, ArtifactKey: keyA, Run: func() error { return nil }})
		reg.Register(Step{Name: nameB, ArtifactKey: keyB, Run: func() error { return nil }})

		_ = reg.Err()
		_ = reg.Steps()
	})
}
