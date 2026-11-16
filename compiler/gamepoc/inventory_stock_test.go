package gamepoc

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// 16.11: the commerce stock wrote `condition` and the inventory reads `condition_ppm`; the field was dropped silently and
// every item started at condition 0. Initial stock now refuses a field an inventory source has no place for.
func TestBusinessStockRefusesUnknownFields(t *testing.T) {
	ctx := cuecontext.New()
	var stock []inv.Source
	good := ctx.CompileString(`[{id: "a", definition: "item.x", quantity: 1, condition_ppm: 1000000, for_sale: true, location: {kind: "store", store: "s"}}]`)
	if err := decodeRowsStrict(good, &stock); err != nil || stock[0].Condition != 1000000 {
		t.Fatalf("a well-formed stock row: %v %v", err, stock)
	}
	bad := ctx.CompileString(`[{id: "a", definition: "item.x", quantity: 1, condition: 1000000, location: {kind: "store", store: "s"}}]`)
	if err := decodeRowsStrict(bad, &stock); err == nil || !strings.Contains(err.Error(), "condition") || !strings.Contains(err.Error(), `"a"`) {
		t.Fatalf("a misspelt field must be refused, got %v", err)
	}
}

// The stores of the commerce are read as strictly: a store row with a field a store has no place for is refused.
func TestBusinessStoresRefuseUnknownFields(t *testing.T) {
	ctx := cuecontext.New()
	var stores []inv.Store
	good := ctx.CompileString(`[{id: "s", name: "n", kind: "container", anchor: {kind: "world", level: "l", at: {X: 1, Y: 2}}, capacity: {mass_g: 1, volume_ml: 1, stacks: 1}, locked: true}]`)
	if err := decodeRowsStrict(good, &stores); err != nil || stores[0].ID != "s" {
		t.Fatalf("a well-formed store: %v %v", err, stores)
	}
	bad := ctx.CompileString(`[{id: "s", name: "n", kind: "container", capacity: {mass_g: 1, volume_ml: 1, stacks: 1}, lock: true}]`)
	if err := decodeRowsStrict(bad, &stores); err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("a misspelt store field must be refused, got %v", err)
	}
}
