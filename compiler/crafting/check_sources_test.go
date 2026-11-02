package crafting

import (
	"strings"
	"testing"

	runtime "github.com/strogmv/ang/compiler/crafting/runtime"
)

// Stage 2 content checks: a carrier the player can meet, a source with an owner and a place, a bypass with a law.
func TestSourceChecksCatchUnreachableCarriersAndALawlessBypass(t *testing.T) {
	m, w := fixture(), world()
	m.Sources.Carriers = []runtime.Carrier{{Item: "item.schematic.lamp", Kind: "copy", Teaches: "craft.lamp", StudyMinutes: 30, Place: runtime.CarrierPlace{Kind: "shop", Shop: "shop.m", N: 1}}}
	m.Recipes[1].Learn = []runtime.Learn{{Kind: "copy", From: "item.schematic.lamp"}}
	w.Items["item.schematic.lamp"] = sourcesItem(300, true)
	if err := Check(m, w); err != nil {
		t.Fatal(err)
	}
	unsold := w
	unsold.Items = map[string]WorldItem{}
	for k, v := range w.Items {
		unsold.Items[k] = v
	}
	unsold.Items["item.schematic.lamp"] = sourcesItem(300, false)
	if err := Check(m, unsold); err == nil || !strings.Contains(err.Error(), "neither sold nor present") {
		t.Fatalf("an unreachable carrier passed: %v", err)
	}
	m.Recipes = append(m.Recipes, runtime.Recipe{ID: "bypass.radio", Kind: "bypass", Name: "r.b", Hint: "h", Discipline: "electrics", Tier: 2, Target: "item.radio", Minutes: 30, Base: 3, RiskPermille: 200, Legal: "forbidden", Basis: "proposal", Learn: []runtime.Learn{{Kind: "start"}}, Sources: []runtime.SourceRef{{Path: "docs/x.md", Note: "proposal"}}})
	if err := Check(m, w); err == nil || !strings.Contains(err.Error(), "crime of the justice ladder") {
		t.Fatalf("a bypass without a law passed: %v", err)
	}
}

func sourcesItem(price int64, sold bool) WorldItem {
	return WorldItem{PriceMinor: price, StackLimit: 1, Sold: sold}
}
