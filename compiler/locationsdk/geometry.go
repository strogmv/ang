package locationsdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

var stableID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,127}$`)

type preparedCatalog struct {
	geometries map[string]Geometry
	hashes     map[string]string
	modules    map[string]Module
	times      []TimeState
	events     []EventState
	decor      map[string]Decoration
}

func issue(code, path, message string, refs ...string) *ValidationError {
	return &ValidationError{Diagnostics: []Diagnostic{{Code: code, Path: path, Message: message, References: refs}}}
}

func lessCell(a, b Cell) bool { return a.Y < b.Y || a.Y == b.Y && a.X < b.X }

func canonicalCells(in []Cell, path string, valid func(Cell) bool) ([]Cell, *ValidationError) {
	out := append([]Cell{}, in...)
	sort.Slice(out, func(i, j int) bool { return lessCell(out[i], out[j]) })
	for i, at := range out {
		if !valid(at) {
			return nil, issue("cell_out_of_bounds", path, fmt.Sprintf("cell (%d,%d) is outside the supported local bounds", at.X, at.Y))
		}
		if i > 0 && out[i-1] == at {
			return nil, issue("duplicate_cell", path, fmt.Sprintf("cell (%d,%d) is repeated", at.X, at.Y))
		}
	}
	return out, nil
}

func cellSet(cells []Cell) map[Cell]bool {
	out := make(map[Cell]bool, len(cells))
	for _, c := range cells {
		out[c] = true
	}
	return out
}

func geometrySignature(g Geometry) string {
	g.ID = ""
	b, _ := json.Marshal(g)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func normalizeGeometry(g Geometry, path string) (Geometry, *ValidationError) {
	if !stableID.MatchString(g.ID) {
		return g, issue("invalid_id", path+".id", "geometry needs a stable identifier")
	}
	if g.Width < 1 || g.Depth < 1 || g.Width > 512 || g.Depth > 512 || g.Width*g.Depth > 65536 || g.Height < 1 || g.Height > 2048 {
		return g, issue("geometry_dimensions", path, "supported geometry bounds are 1..512 horizontal units, at most 65536 cells, and 1..2048 height units")
	}
	if len(g.Footprint) > 65536 || len(g.Blocked) > 65536 || len(g.UseClearance) > 65536 || len(g.MaintenanceClearance) > 65536 || len(g.Ports) > 256 || len(g.Sockets) > 256 {
		return g, issue("geometry_budget", path, "geometry exceeds cell, port or socket budgets")
	}
	inside := func(c Cell) bool { return c.X >= 0 && c.Y >= 0 && c.X < g.Width && c.Y < g.Depth }
	near := func(c Cell) bool { return c.X >= -2048 && c.Y >= -2048 && c.X <= g.Width+2048 && c.Y <= g.Depth+2048 }
	var e *ValidationError
	if g.Footprint, e = canonicalCells(g.Footprint, path+".footprint", inside); e != nil {
		return g, e
	}
	if g.Blocked, e = canonicalCells(g.Blocked, path+".blocked", inside); e != nil {
		return g, e
	}
	if g.UseClearance, e = canonicalCells(g.UseClearance, path+".use_clearance", near); e != nil {
		return g, e
	}
	if g.MaintenanceClearance, e = canonicalCells(g.MaintenanceClearance, path+".maintenance_clearance", near); e != nil {
		return g, e
	}
	physical := cellSet(g.Footprint)
	for _, at := range g.Blocked {
		if !physical[at] {
			return g, issue("blocked_without_footprint", path+".blocked", "blocked cells must belong to the physical footprint")
		}
	}
	for _, cells := range [][]Cell{g.UseClearance, g.MaintenanceClearance} {
		for _, at := range cells {
			if physical[at] {
				return g, issue("clearance_inside_material", path, "use and maintenance approaches cannot lie inside this module's physical material")
			}
		}
	}
	g.Ports = append([]Port{}, g.Ports...)
	portIDs := map[string]bool{}
	for i, p := range g.Ports {
		pp := fmt.Sprintf("%s.ports[%d]", path, i)
		if !stableID.MatchString(p.ID) || portIDs[p.ID] {
			return g, issue("duplicate_or_invalid_port", pp+".id", "port IDs must be stable and unique inside a geometry")
		}
		portIDs[p.ID] = true
		if p.Kind != "door" && p.Kind != "connection" && p.Kind != "use" {
			return g, issue("port_kind", pp+".kind", "supported port kinds are door, connection, use")
		}
		if _, ok := sideDelta(p.Side); !ok || !near(p.At) || physical[p.At] {
			return g, issue("port_geometry", pp, "port needs a cardinal side and a free local cell")
		}
		if len(p.Opening) == 0 {
			p.Opening = []Cell{p.At}
		}
		p.Opening, e = canonicalCells(p.Opening, pp+".opening", near)
		if e != nil {
			return g, e
		}
		if len(p.Opening) > 512 {
			return g, issue("port_opening_budget", pp+".opening", "opening exceeds 512 authored edge cells")
		}
		containsAt := false
		for j, at := range p.Opening {
			containsAt = containsAt || at == p.At
			if physical[at] {
				return g, issue("port_opening_inside_material", pp+".opening", "door/connection opening cannot include physical material")
			}
			if (p.Side == North || p.Side == South) && at.Y != p.At.Y || (p.Side == East || p.Side == West) && at.X != p.At.X {
				return g, issue("port_opening_geometry", pp+".opening", "opening cells must form an edge perpendicular to its side")
			}
			if j > 0 && abs(at.X-p.Opening[j-1].X)+abs(at.Y-p.Opening[j-1].Y) != 1 {
				return g, issue("port_opening_geometry", pp+".opening", "opening edge cells must be contiguous")
			}
		}
		if !containsAt {
			return g, issue("port_opening_geometry", pp+".opening", "the stable port anchor must belong to its opening")
		}
		if len(p.Approach) == 0 {
			return g, issue("port_without_approach", pp+".approach", "author at least one front approach cell")
		}
		p.Approach, e = canonicalCells(p.Approach, pp+".approach", near)
		if e != nil {
			return g, e
		}
		for _, at := range p.Approach {
			if physical[at] {
				return g, issue("port_approach_inside_material", pp+".approach", "a port approach cannot be occupied by this module")
			}
		}
		front, _ := sideDelta(p.Side)
		nearPort := false
		for _, at := range p.Approach {
			dx, dy := at.X-p.At.X, at.Y-p.At.Y
			if dx*front.X+dy*front.Y < 0 {
				return g, issue("port_approach_not_front", pp+".approach", "approach must be at or in front of the port's authored side")
			}
			for _, opening := range p.Opening {
				if abs(at.X-opening.X)+abs(at.Y-opening.Y) <= 1 {
					nearPort = true
				}
			}
		}
		if !nearPort {
			return g, issue("port_approach_too_far", pp+".approach", "at least one approach cell must touch its port")
		}
		g.Ports[i] = p
	}
	sort.Slice(g.Ports, func(i, j int) bool { return g.Ports[i].ID < g.Ports[j].ID })
	g.Sockets = append([]Socket{}, g.Sockets...)
	socketIDs := map[string]bool{}
	for i, s := range g.Sockets {
		sp := fmt.Sprintf("%s.sockets[%d]", path, i)
		if !stableID.MatchString(s.ID) || socketIDs[s.ID] || s.Blocking || !inside(s.At) || s.Width < 1 || s.Depth < 1 || s.Height < 1 || s.Width > g.Width-s.At.X || s.Depth > g.Depth-s.At.Y || s.Z < 0 || s.Z > g.Height || s.Height > g.Height-s.Z {
			return g, issue("socket_geometry", sp, "a socket must be uniquely named, nonblocking, and its entire box must fit the module bounds")
		}
		socketIDs[s.ID] = true
	}
	sort.Slice(g.Sockets, func(i, j int) bool { return g.Sockets[i].ID < g.Sockets[j].ID })
	return g, nil
}

func prepare(c Catalog) (preparedCatalog, *ValidationError) {
	p := preparedCatalog{geometries: map[string]Geometry{}, hashes: map[string]string{}, modules: map[string]Module{}, decor: map[string]Decoration{}}
	if c.UnitMM != UnitMM {
		return p, issue("unsupported_units", "catalog.unit_mm", "this compiler supports 500 mm authoring units")
	}
	if len(c.Geometries) < 1 || len(c.Geometries) > 4096 || len(c.Modules) < 1 || len(c.Modules) > 4096 || len(c.Decorations) > 4096 {
		return p, issue("catalog_budget", "catalog", "expected 1..4096 geometries/modules and at most 4096 decoration definitions")
	}
	totalCells := 0
	for i, g := range c.Geometries {
		totalCells += len(g.Footprint) + len(g.Blocked) + len(g.UseClearance) + len(g.MaintenanceClearance)
		if totalCells > 1048576 {
			return p, issue("catalog_budget", "catalog.geometries", "catalog exceeds 1048576 authored structural cells")
		}
		path := fmt.Sprintf("catalog.geometries[%d]", i)
		if _, ok := p.geometries[g.ID]; ok {
			return p, issue("duplicate_id", path+".id", "geometry ID is repeated", g.ID)
		}
		g, e := normalizeGeometry(g, path)
		if e != nil {
			return p, e
		}
		p.geometries[g.ID], p.hashes[g.ID] = g, geometrySignature(g)
	}
	for i, m := range c.Modules {
		path := fmt.Sprintf("catalog.modules[%d]", i)
		if !stableID.MatchString(m.ID) {
			return p, issue("invalid_id", path+".id", "module needs a stable identifier")
		}
		if _, ok := p.modules[m.ID]; ok {
			return p, issue("duplicate_id", path+".id", "module ID is repeated", m.ID)
		}
		if _, ok := p.geometries[m.GeometryID]; !ok {
			return p, issue("unknown_geometry", path+".geometry_id", "module references an unknown geometry", m.GeometryID)
		}
		if len(m.Variants) < 1 || len(m.Variants) > 256 {
			return p, issue("variant_budget", path+".variants", "each module needs 1..256 interchangeable variants")
		}
		m.Variants = append([]Variant{}, m.Variants...)
		seen := map[string]bool{}
		for j, v := range m.Variants {
			vp := fmt.Sprintf("%s.variants[%d]", path, j)
			if !stableID.MatchString(v.ID) || seen[v.ID] {
				return p, issue("duplicate_or_invalid_variant", vp+".id", "variant IDs must be stable and unique inside a module")
			}
			seen[v.ID] = true
			if v.ImageRef != "" {
				return p, issue("unsupported_unverified_asset", vp+".image_ref", "raster binding needs a future validated projection/pivot/scale calibration contract; this compiler currently accepts geometry-only variants")
			}
			if _, ok := p.geometries[v.GeometryID]; !ok {
				return p, issue("unknown_geometry", vp+".geometry_id", "variant references an unknown geometry", v.GeometryID)
			}
			if p.hashes[v.GeometryID] != p.hashes[m.GeometryID] {
				return p, issue("variant_incompatible", vp+".geometry_id", "replacement changes dimensions, physical footprint, blocked cells, ports, access or sockets", m.ID, v.ID)
			}
		}
		sort.Slice(m.Variants, func(i, j int) bool { return m.Variants[i].ID < m.Variants[j].ID })
		p.modules[m.ID] = m
	}
	if len(c.Times) < 1 || len(c.Events) < 1 || len(c.Times) > 32 || len(c.Events) > 32 || len(c.Times)*len(c.Events) > 256 {
		return p, issue("presentation_budget", "catalog", "provide 1..32 states per independent axis and at most 256 combinations")
	}
	p.times, p.events = append([]TimeState{}, c.Times...), append([]EventState{}, c.Events...)
	seen := map[string]bool{}
	for i, t := range p.times {
		if !stableID.MatchString(t.ID) || seen[t.ID] || math.IsNaN(t.Brightness) || math.IsInf(t.Brightness, 0) || t.Brightness <= 0 || t.Brightness > 1 {
			return p, issue("time_state", fmt.Sprintf("catalog.times[%d]", i), "time state needs a unique ID and finite brightness in (0,1]")
		}
		seen[t.ID] = true
		t.ActivityTags = uniqueStrings(t.ActivityTags)
		p.times[i] = t
	}
	seen = map[string]bool{}
	for i, e := range p.events {
		if !stableID.MatchString(e.ID) || seen[e.ID] {
			return p, issue("event_state", fmt.Sprintf("catalog.events[%d]", i), "event state needs a unique stable ID")
		}
		seen[e.ID] = true
		e.ActivityTags = uniqueStrings(e.ActivityTags)
		p.events[i] = e
	}
	sort.Slice(p.times, func(i, j int) bool { return p.times[i].ID < p.times[j].ID })
	sort.Slice(p.events, func(i, j int) bool { return p.events[i].ID < p.events[j].ID })
	for i, d := range c.Decorations {
		if d.ImageRef != "" {
			return p, issue("unsupported_unverified_asset", fmt.Sprintf("catalog.decorations[%d].image_ref", i), "decoration raster binding has no validated visual calibration contract")
		}
		_, duplicate := p.decor[d.ID]
		if !stableID.MatchString(d.ID) || duplicate || d.Width < 1 || d.Depth < 1 || d.Height < 1 || d.Width > 512 || d.Depth > 512 || d.Height > 2048 || d.Blocking {
			return p, issue("decoration_policy", fmt.Sprintf("catalog.decorations[%d]", i), "event decoration must have a unique ID, bounded dimensions and no physical blocking")
		}
		p.decor[d.ID] = d
	}
	return p, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func uniqueStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	n := 0
	for _, s := range out {
		if strings.TrimSpace(s) != "" && (n == 0 || out[n-1] != s) {
			out[n] = s
			n++
		}
	}
	return out[:n]
}

func sideDelta(s Side) (Cell, bool) {
	switch s {
	case North:
		return Cell{0, -1}, true
	case East:
		return Cell{1, 0}, true
	case South:
		return Cell{0, 1}, true
	case West:
		return Cell{-1, 0}, true
	}
	return Cell{}, false
}

func rotateCell(at Cell, w, d, rotation int) Cell {
	switch rotation {
	case 90:
		return Cell{d - 1 - at.Y, at.X}
	case 180:
		return Cell{w - 1 - at.X, d - 1 - at.Y}
	case 270:
		return Cell{at.Y, w - 1 - at.X}
	default:
		return at
	}
}

func rotateSide(side Side, rotation int) Side {
	sides := [...]Side{North, East, South, West}
	for i, s := range sides {
		if s == side {
			return sides[(i+rotation/90)%4]
		}
	}
	return side
}

func worldCells(in []Cell, g Geometry, instance Instance) []Cell {
	out := make([]Cell, len(in))
	for i, c := range in {
		c = rotateCell(c, g.Width, g.Depth, instance.Rotation)
		out[i] = Cell{c.X + instance.At.X, c.Y + instance.At.Y}
	}
	sort.Slice(out, func(i, j int) bool { return lessCell(out[i], out[j]) })
	return out
}
