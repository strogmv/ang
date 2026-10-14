package emitter

import (
	"slices"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// The manifest sorts its own copy: emitters that run after it must see the
// IR as it was.
func TestEmitManifestLeavesTheIRUnchanged(t *testing.T) {
	schema := &ir.Schema{Services: []ir.Service{{Name: "Orders", Publishes: []string{"OrderShipped", "OrderCreated"}}}}
	em := New(t.TempDir(), t.TempDir(), "templates")
	if err := em.EmitManifest(schema); err != nil {
		t.Fatal(err)
	}
	if got := schema.Services[0].Publishes; !slices.Equal(got, []string{"OrderShipped", "OrderCreated"}) {
		t.Fatalf("EmitManifest reordered the IR: %v", got)
	}
}
