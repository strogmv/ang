package gamepoc

// The view gets art bindings, not ownership, authored histories or future jobs.
// Runtime perception still determines which cells may actually be rendered.
func clientSpatial(c SpatialCatalog) map[string]any {
	heights := make(map[string]int, len(c.Modules))
	for _, m := range c.Modules {
		heights[m.ID] = m.HeightMM
	}
	levels := make([]map[string]any, 0, len(c.Levels))
	for _, l := range c.Levels {
		zones := make([]map[string]any, 0, len(l.Zones))
		for _, z := range l.Zones {
			zones = append(zones, map[string]any{"id": z.ID, "kind": z.Kind, "rect": z.Rect, "style_id": z.StyleID, "resolved_variant": z.ResolvedVariant})
		}
		props := make([]map[string]any, 0, len(l.Props))
		for _, p := range l.Props {
			props = append(props, map[string]any{"id": p.ID, "module_id": p.ModuleID, "height_mm": heights[p.ModuleID], "level": p.Level, "rotation": p.Rotation, "rect": p.Rect, "glyph": p.Glyph, "style_id": p.StyleID, "resolved_variant": p.ResolvedVariant})
		}
		levels = append(levels, map[string]any{"id": l.ID, "width": l.Width, "height": l.Height, "cell_meters": l.CellMeters, "zones": zones, "props": props, "lights": l.Lights, "decorations": l.Decorations, "exits": l.Exits})
	}
	return map[string]any{"seed": c.Seed, "styles": c.Styles, "levels": levels}
}
