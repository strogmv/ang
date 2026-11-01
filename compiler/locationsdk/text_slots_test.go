package locationsdk

import "testing"

// demo-hub v11-text: an in-world inscription is a slot on a surface + a key or a price list (owner rule 31.10).
func TestTextSlotsValidateHostAreaLetteringAndText(t *testing.T) {
	base := Layout{ID: "t", Width: 20, Depth: 10, Instances: []Instance{{ID: "stall", ModuleID: "m", At: Cell{2, 2}}}}
	strips := &TextStrips{Count: 16, PitchMM: 305, WidthMM: 296}
	good := TextSlot{ID: "stall.strips", Instance: "stall", Anchor: "textslot.strips", CenterMM: PointMM{3446, 2455, 2075}, Face: South, WidthMM: 4871, HeightMM: 290, MarginMM: 30,
		Style: "stencil", Surface: "pvc", Ink: "#211A16", Wear: 380, MinGlyphMM: 150, MaxLines: 1, Layout: "strips_auto", Strips: strips, Key: "sign.hub.strips", Reason: "noren"}
	price := TextSlot{ID: "stall.price", Instance: "stall", CenterMM: PointMM{580, 2447, 1425}, Face: South, WidthMM: 720, HeightMM: 550, MarginMM: 40, Style: "chalk", Surface: "board", Ink: "#E8E3D3",
		MinGlyphMM: 75, MaxLines: 4, Layout: "rows", PriceList: &TextPriceList{Store: "store/a", LineKey: "sign.hub.line", FooterKey: "sign.hub.footer", Currency: "CRD", Rows: []TextPriceRow{{Item: "item.a", NameKey: "sign.hub.a"}}}, Reason: "board"}
	gate := TextSlot{ID: "gate.number", Instance: "stall", CenterMM: PointMM{0, 0, 9000}, Face: North, WidthMM: 600, HeightMM: 320, Style: "stencil", Surface: "steel", Ink: "#EFE6C8", MinGlyphMM: 200, MaxLines: 1, Layout: "rows", Key: "sign.hub.gate.number", Args: map[string]string{"number": "6"}, Reason: "gate"}
	ok := base
	ok.TextSlots = []TextSlot{good, price, gate}
	if err := validateTextSlots(ok); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*TextSlot){
		"unknown host":        func(s *TextSlot) { s.Instance = "nope" },
		"anchor not textslot": func(s *TextSlot) { s.Anchor = "lamp.1" },
		"anchor without host": func(s *TextSlot) { s.Instance = ""; s.Anchor = "textslot.x" },
		"face":                func(s *TextSlot) { s.Face = "up" },
		"margins fill area":   func(s *TextSlot) { s.MarginMM = 150 },
		"style":               func(s *TextSlot) { s.Style = "neon" },
		"surface":             func(s *TextSlot) { s.Surface = "wood" },
		"ink":                 func(s *TextSlot) { s.Ink = "red" },
		"min glyph over area": func(s *TextSlot) { s.MinGlyphMM = 300 },
		"strips missing":      func(s *TextSlot) { s.Strips = nil },
		"strips outside area": func(s *TextSlot) { s.Strips = &TextStrips{Count: 20, PitchMM: 305, WidthMM: 296} },
		"strip nodes count":   func(s *TextSlot) { s.Strips = &TextStrips{Count: 16, PitchMM: 305, WidthMM: 296, Nodes: []string{"a"}} },
		"no text":             func(s *TextSlot) { s.Key = "" },
		"key and price":       func(s *TextSlot) { s.PriceList = price.PriceList },
		"bad key":             func(s *TextSlot) { s.Key = "Sign" },
		"word argument":       func(s *TextSlot) { s.Args = map[string]string{"word": "exit"} },
		"reason":              func(s *TextSlot) { s.Reason = "" },
	} {
		bad := base
		s := good
		change(&s)
		bad.TextSlots = []TextSlot{s}
		if validateTextSlots(bad) == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	dup := base
	dup.TextSlots = []TextSlot{good, good}
	if validateTextSlots(dup) == nil {
		t.Fatal("duplicate slot accepted")
	}
	p := price
	p.PriceList = &TextPriceList{Store: "store/a", LineKey: "sign.hub.line", FooterKey: "sign.hub.footer", Currency: "CRD", Rows: []TextPriceRow{{Item: "item.a", NameKey: "sign.hub.a"}, {Item: "item.b", NameKey: "sign.hub.b"}, {Item: "item.c", NameKey: "sign.hub.c"}, {Item: "item.d", NameKey: "sign.hub.d"}}}
	over := base
	over.TextSlots = []TextSlot{p}
	if validateTextSlots(over) == nil {
		t.Fatal("price rows + footer over max_lines accepted")
	}
}
