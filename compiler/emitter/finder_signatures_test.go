package emitter

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestComputeFinderSignature_ReturnTypeAndParams(t *testing.T) {
	t.Parallel()

	sig := ComputeFinderSignature("User", normalizer.RepositoryFinder{
		Name:    "FindByEmail",
		Returns: "one",
		Where: []normalizer.FinderWhere{
			{Param: "email", ParamType: "string"},
			{Param: "createdAfter", ParamType: "time"},
		},
	}, "")

	if sig.Name != "FindByEmail" {
		t.Fatalf("unexpected name %q", sig.Name)
	}
	if sig.ReturnType != "*domain.User" {
		t.Fatalf("unexpected return type %q", sig.ReturnType)
	}
	if sig.ParamsSig != "email string, createdAfter time.Time" {
		t.Fatalf("unexpected params %q", sig.ParamsSig)
	}
	if sig.ArgNames != "email, createdAfter" {
		t.Fatalf("unexpected args %q", sig.ArgNames)
	}
	if !sig.HasTime {
		t.Fatalf("expected HasTime=true")
	}
}

func TestComputeFinderSignature_FallbackReturnType(t *testing.T) {
	t.Parallel()

	sig := ComputeFinderSignature("Notification", normalizer.RepositoryFinder{
		Name: "FindAllCustom",
	}, "[]domain.Notification")

	if sig.ReturnType != "[]domain.Notification" {
		t.Fatalf("unexpected fallback return type %q", sig.ReturnType)
	}
	if !sig.ReturnSlice {
		t.Fatalf("expected ReturnSlice=true")
	}
}
