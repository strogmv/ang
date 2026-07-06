package compiler

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/strogmv/ang-ir/normalizer"
)

func TestMergeArchitectureServiceMetadata_MergesDependsIntoServices(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	arch := ctx.CompileString(`
package architecture

#Services: {
	assistant: {
		name: "Assistant"
		depends: ["Blog", "Auth"]
		publishes: ["AssistantAsked"]
		subscribes: {
			PostPublished: "OnPostPublished"
			PostDeleted: {op: "OnPostDeleted", delivery: "broadcast"}
		}
	}
}
`)
	if err := arch.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	services := []normalizer.Service{
		{Name: "Assistant"},
	}
	got := mergeArchitectureServiceMetadata(services, arch)
	if len(got) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got))
	}
	if len(got[0].Uses) != 2 || got[0].Uses[0] != "Blog" || got[0].Uses[1] != "Auth" {
		t.Fatalf("unexpected merged uses: %#v", got[0].Uses)
	}
	if len(got[0].Publishes) != 1 || got[0].Publishes[0] != "AssistantAsked" {
		t.Fatalf("unexpected merged publishes: %#v", got[0].Publishes)
	}
	if got[0].Subscribes["PostPublished"] != "OnPostPublished" {
		t.Fatalf("unexpected merged subscribes: %#v", got[0].Subscribes)
	}
	if got[0].Subscribes["PostDeleted"] != "OnPostDeleted" {
		t.Fatalf("unexpected object subscription: %#v", got[0].Subscribes)
	}
	delivery, ok := got[0].Metadata["subscriptionDelivery"].(map[string]any)
	if !ok || delivery["PostPublished"] != "queue" || delivery["PostDeleted"] != "broadcast" {
		t.Fatalf("unexpected subscription delivery metadata: %#v", got[0].Metadata)
	}
}
