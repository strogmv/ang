package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// itemDisplayFixture: a client folder with one model, the renderer, item_look.gd, an icon and the chip glyph, and a catalog
// whose record matches them.
func itemDisplayFixture(t *testing.T) (*ItemDisplayCatalog, map[string]itemWorldItem, string) {
	t.Helper()
	client := t.TempDir()
	write := func(rel, text string) string {
		path := filepath.Join(client, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:])
	}
	modelSHA := write("kit/items/item.bottle.glb", "bottle")
	tool := write("tools/item_icons.gd", "renderer")
	look := write("item_look.gd", "look")
	write("assets/icons/chip.svg", "<svg/>")
	write("assets/icons/tools.svg", "<svg/>")
	full := "full"
	c := &ItemDisplayCatalog{
		Record: "art-source/items/v1/record.json", NearUpToFrameM: 6,
		Bodies: map[string]ItemBody{"body.bottle": {Model: "res://kit/items/item.bottle.glb", Budget: "small", Rest: "stand",
			Held: &ItemHeld{Hand: "right", GripM: [3]float64{0, 0.1, 0}}, Carried: ItemCarried{At: "none"},
			Icon: ItemIcon{YawDeg: 25, PitchDeg: 20, Zoom: 1}, States: []string{"fill"}, Source: "test", Note: "a bottle"}},
		Items: map[string]ItemLook{
			"item.water": {Body: "body.bottle", State: &ItemState{Fill: full}},
			"item.crd":   {Glyph: "chip"},
		},
		StandIn:  ItemStandIn{Default: "chip", Items: map[string]string{"item.headset": "head"}},
		required: map[string]string{"item.water": "a shelf"},
	}
	c.record.Schema = "items-display.record.v1"
	c.record.Renderer.Tool, c.record.Renderer.Look, c.record.Renderer.PX = tool, look, 256
	c.record.Models = map[string]itemRecordModel{"res://kit/items/item.bottle.glb": {SHA256: modelSHA, Content: "content",
		Files: map[string]string{"res://kit/items/item.bottle.glb": modelSHA}, Triangles: 800, SizeM: [3]float64{0.08, 0.22, 0.08},
		MinM: [3]float64{-0.04, 0, -0.04}, Nodes: []string{"bottle", "state_fill_full"}}}
	icon := write("kit/items/icons/item.water.png", "png")
	c.record.Icons = map[string]itemRecordIcon{"item.water": {PNG: "res://kit/items/icons/item.water.png", PX: 256, SHA256: icon,
		Key: itemIconKey("content", &ItemState{Fill: full}, ItemIcon{YawDeg: 25, PitchDeg: 20, Zoom: 1}, tool, look, 256)}}
	items := map[string]itemWorldItem{"item.water": {Carrier: "physical"}, "item.crd": {Carrier: "digital"},
		"item.spanner": {Carrier: "physical", Tags: []string{"tool"}}, "item.headset": {Carrier: "physical", Tags: []string{"tool"}},
		"item.coat": {Carrier: "physical", Tags: []string{"tool"}}}
	c.Contact.Strength, c.Contact.SoftM, c.Contact.Tint = 0.5, 0.04, [3]float64{0.3, 0.3, 0.4}
	c.wornSlots = map[string][]string{"item.coat": {"torso", "legs"}}
	write("assets/icons/head.svg", "<svg/>")
	write("assets/icons/torso.svg", "<svg/>")
	return c, items, client
}

func TestItemDisplayAcceptsAWholeChain(t *testing.T) {
	c, items, client := itemDisplayFixture(t)
	c.StandIn.Tags = append(c.StandIn.Tags, struct {
		Tag   string `json:"tag"`
		Glyph string `json:"glyph"`
	}{"tool", "tools"})
	if err := checkItemDisplay(c, items, client); err != nil {
		t.Fatal(err)
	}
	if c.standIns["item.spanner"] != "tools" || c.standIns["item.headset"] != "head" || c.standIns["item.coat"] != "torso" || len(c.standIns) != 3 {
		t.Fatalf("stand-ins: %v", c.standIns)
	}
	view := clientItemDisplay(c)
	if view["coverage"].(map[string]int)["own"] != 1 {
		t.Fatalf("coverage: %v", view["coverage"])
	}
}

func TestItemDisplayRefusesBrokenLooks(t *testing.T) {
	cases := map[string]func(c *ItemDisplayCatalog, items map[string]itemWorldItem, client string){
		"stale": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			l := c.Items["item.water"]
			l.State = &ItemState{Fill: "empty"}
			c.Items["item.water"] = l
		},
		"changed since": func(_ *ItemDisplayCatalog, _ map[string]itemWorldItem, client string) {
			_ = os.WriteFile(filepath.Join(client, "kit/items/item.bottle.glb"), []byte("another bottle"), 0o644)
		},
		"budget": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			m := c.record.Models["res://kit/items/item.bottle.glb"]
			m.Triangles = 1600
			c.record.Models["res://kit/items/item.bottle.glb"] = m
		},
		"off the model": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			b := c.Bodies["body.bottle"]
			b.Held = &ItemHeld{Hand: "right", GripM: [3]float64{0, 0.5, 0}}
			c.Bodies["body.bottle"] = b
		},
		"digital thing only": func(c *ItemDisplayCatalog, items map[string]itemWorldItem, _ string) {
			items["item.crd"] = itemWorldItem{Carrier: "physical"}
		},
		"no look of its own": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			c.required["item.spanner"] = "the hero's start"
		},
		"exactly the channels": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			c.Items["item.water"] = ItemLook{Body: "body.bottle"}
		},
		"no state_used_": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			b := c.Bodies["body.bottle"]
			b.States = []string{"fill", "used"}
			c.Bodies["body.bottle"] = b
		},
		"another renderer": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			c.record.Renderer.Look = "other"
		},
		"strength 0..1": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			c.Contact.Strength = 0
		},
		"look of its own": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			c.StandIn.Items["item.water"] = "head"
		},
		"not published by the server": func(c *ItemDisplayCatalog, _ map[string]itemWorldItem, _ string) {
			b := c.Bodies["body.bottle"]
			b.States = []string{"fill", "spoil"}
			c.Bodies["body.bottle"] = b
		},
	}
	for want, breakIt := range cases {
		t.Run(want, func(t *testing.T) {
			c, items, client := itemDisplayFixture(t)
			breakIt(c, items, client)
			err := checkItemDisplay(c, items, client)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want an error with %q, got %v", want, err)
			}
		})
	}
}

func TestItemDisplayPlacesUseTheLook(t *testing.T) {
	c, _, _ := itemDisplayFixture(t)
	b := c.Bodies["body.bottle"]
	b.Shop = &ItemShop{Torgomat: &ItemRows{Row: "shelf", Full: "res://kit/t/full.glb", Half: "res://kit/t/half.glb", Few: "res://kit/t/few.glb"}}
	c.Bodies["body.bottle"] = b
	v := &VendingCatalog{Models: map[string]VendingModel{"item.water": {Item: "res://kit/items/item.bottle.glb", Full: "res://kit/t/full.glb", Half: "res://kit/t/half.glb", Few: "res://kit/t/few.glb"}}}
	if err := checkItemDisplayPlaces(c, v, nil); err != nil {
		t.Fatal(err)
	}
	v.Models["item.water"] = VendingModel{Item: "res://kit/torgomat/vm.item.water_1l.glb"}
	if err := checkItemDisplayPlaces(c, v, nil); err == nil || !strings.Contains(err.Error(), "its look's body") {
		t.Fatalf("a torgomat unit that is not the look's body must be refused, got %v", err)
	}
}

// The icon key is the line scripts/item-icons.py hashes: a change of state, turn or renderer changes it.
func TestItemIconKeyFollowsItsInputs(t *testing.T) {
	icon := ItemIcon{YawDeg: 30, PitchDeg: 35, Zoom: 1}
	base := itemIconKey("m", nil, icon, "r", "l", 256)
	charge := 0.0
	for name, other := range map[string]string{
		"state":    itemIconKey("m", &ItemState{Charge: &charge}, icon, "r", "l", 256),
		"turn":     itemIconKey("m", nil, ItemIcon{YawDeg: 31, PitchDeg: 35, Zoom: 1}, "r", "l", 256),
		"renderer": itemIconKey("m", nil, icon, "r2", "l", 256),
		"look":     itemIconKey("m", nil, icon, "r", "l2", 256),
		"model":    itemIconKey("m2", nil, icon, "r", "l", 256),
	} {
		if other == base {
			t.Fatalf("the icon key ignores its %s", name)
		}
	}
	// the exact line of the game's script (scripts/item-icons.py icon_key)
	sum := sha256.Sum256([]byte("items-icon.v1|model=m|fill=-|charge=-|used=-|yaw=30.000|pitch=35.000|zoom=1.000|renderer=r|look=l|px=256"))
	if base != hex.EncodeToString(sum[:]) {
		t.Fatal("the icon key line differs from the game's script")
	}
}
