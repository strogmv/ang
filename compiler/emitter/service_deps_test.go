package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestValidateServiceDependencies(t *testing.T) {
	t.Run("unknown dependency", func(t *testing.T) {
		services := []normalizer.Service{
			{Name: "tender", Uses: []string{"chat"}},
		}
		err := ValidateServiceDependencies(services)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unknown service dependencies") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cycle dependency", func(t *testing.T) {
		services := []normalizer.Service{
			{Name: "tender", Uses: []string{"chat"}},
			{Name: "chat", Uses: []string{"tender"}},
		}
		err := ValidateServiceDependencies(services)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cycle detected") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid dependencies", func(t *testing.T) {
		services := []normalizer.Service{
			{Name: "tender", Uses: []string{"chat"}},
			{Name: "chat"},
		}
		if err := ValidateServiceDependencies(services); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}
