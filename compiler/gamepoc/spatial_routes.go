package gamepoc

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type SpatialEndpoint struct {
	Ref string `json:"ref,omitempty"`
	At  *Point `json:"at,omitempty"`
}
type SpatialRoute struct {
	ID      string          `json:"id"`
	From    SpatialEndpoint `json:"from"`
	To      SpatialEndpoint `json:"to"`
	StyleID string          `json:"style_id"`
	Symbol  string          `json:"symbol"`
}

// Route planning belongs to generation, not server ticks or Unity. Painting
// uses the completed passable grid, so authored props and service clearances
// are already reflected in geometry. Source endpoint records are not a reveal API.
func (l *SpatialLevel) compileRoutes(floor []bool, unique func(string, string) error) error {
	if len(l.Routes) > 64 {
		return fmt.Errorf("wayfinding: at most64 routes per level")
	}
	refs := map[string]Point{}
	for _, v := range l.Exits {
		refs[v.ID] = v.Landing
	}
	for _, v := range l.InteractionSpots {
		if _, ok := refs[v.ID]; ok {
			return fmt.Errorf("wayfinding: ambiguous endpoint %s", v.ID)
		}
		refs[v.ID] = v.At
	}
	endpoint := func(e SpatialEndpoint) (Point, error) {
		if (e.Ref == "") == (e.At == nil) {
			return Point{}, fmt.Errorf("endpoint needs exactly one of ref or at")
		}
		var p Point
		if e.At != nil {
			p = *e.At
		} else {
			var ok bool
			p, ok = refs[e.Ref]
			if !ok {
				return Point{}, fmt.Errorf("unknown endpoint %q", e.Ref)
			}
		}
		if !l.contains(p) || !floor[p.Y*l.Width+p.X] {
			return Point{}, fmt.Errorf("endpoint %d,%d is not walkable", p.X, p.Y)
		}
		return p, nil
	}
	// Only derived route marks are replaced on recompilation; manual marks persist.
	manual := make([]SpatialDecoration, 0, len(l.Decorations))
	for _, d := range l.Decorations {
		if d.RouteID == "" {
			manual = append(manual, d)
		}
	}
	if len(l.Routes) == 0 {
		if len(manual) != len(l.Decorations) {
			l.Decorations = manual
		}
		return nil
	}
	routes := append([]SpatialRoute(nil), l.Routes...)
	sort.Slice(routes, func(i, j int) bool { return routes[i].ID < routes[j].ID })
	generated := []SpatialDecoration{}
	budget := 8000000
	for _, r := range routes {
		if e := unique("route", r.ID); e != nil {
			return e
		}
		if strings.TrimSpace(r.Symbol) == "" || utf8.RuneCountInString(r.Symbol) > 4 || strings.ContainsAny(r.Symbol, "<>") || strings.IndexFunc(r.Symbol, unicode.IsControl) >= 0 {
			return fmt.Errorf("route %s: symbol must be1..4 plain characters", r.ID)
		}
		from, e := endpoint(r.From)
		if e != nil {
			return fmt.Errorf("route %s source: %w", r.ID, e)
		}
		to, e := endpoint(r.To)
		if e != nil {
			return fmt.Errorf("route %s target: %w", r.ID, e)
		}
		path, e := l.routePath(floor, from, to, &budget)
		if e != nil {
			return fmt.Errorf("route %s: %w", r.ID, e)
		}
		if len(path) < 2 {
			return fmt.Errorf("route %s: source and target must differ", r.ID)
		}
		for i, p := range path {
			axes := 0
			for _, j := range []int{i - 1, i + 1} {
				if j >= 0 && j < len(path) {
					if path[j].X != p.X {
						axes |= 1
					} else {
						axes |= 2
					}
				}
			}
			for bit := 1; bit <= 2; bit++ {
				if axes&bit == 0 {
					continue
				}
				shape := "line_x"
				if bit == 2 {
					shape = "line_y"
				}
				generated = append(generated, SpatialDecoration{ID: fmt.Sprintf("route.%s.%d.%d.%s", r.ID, p.X, p.Y, shape), At: p, StyleID: r.StyleID, Variant: -1, Shape: shape, Surface: "floor", RouteID: r.ID})
			}
			// Repeated symbols and endpoint labels make route meaning independent of color.
			if i == 0 || i == len(path)-1 || i%8 == 0 {
				generated = append(generated, SpatialDecoration{ID: fmt.Sprintf("route.%s.%d.%d.symbol", r.ID, p.X, p.Y), At: p, StyleID: r.StyleID, Variant: -1, Shape: "mark", Surface: "floor", RouteID: r.ID, Symbol: r.Symbol})
			}
			if len(generated) > 65536 {
				return fmt.Errorf("wayfinding: generated mark budget exceeded")
			}
		}
	}
	l.Decorations = append(manual, generated...)
	return nil
}

func (l *SpatialLevel) routePath(floor []bool, from, to Point, budget *int) ([]Point, error) {
	start, goal := from.Y*l.Width+from.X, to.Y*l.Width+to.X
	parent := make([]int, len(floor))
	for i := range parent {
		parent[i] = -1
	}
	parent[start] = start
	queue := []int{start}
	for head := 0; head < len(queue) && parent[goal] < 0; head++ {
		*budget--
		if *budget < 0 {
			return nil, fmt.Errorf("wayfinding search budget exceeded")
		}
		cell := queue[head]
		p := Point{X: cell % l.Width, Y: cell / l.Width}
		// Stable N,E,S,W order, independent of maps and route input order.
		for _, n := range [...]Point{{X: p.X, Y: p.Y - 1}, {X: p.X + 1, Y: p.Y}, {X: p.X, Y: p.Y + 1}, {X: p.X - 1, Y: p.Y}} {
			if !l.contains(n) {
				continue
			}
			index := n.Y*l.Width + n.X
			if floor[index] && parent[index] < 0 {
				parent[index] = cell
				queue = append(queue, index)
			}
		}
	}
	if parent[goal] < 0 {
		return nil, fmt.Errorf("destination unreachable")
	}
	result := []Point{}
	for cell := goal; ; cell = parent[cell] {
		result = append(result, Point{X: cell % l.Width, Y: cell / l.Width})
		if cell == start {
			break
		}
	}
	for a, b := 0, len(result)-1; a < b; a, b = a+1, b-1 {
		result[a], result[b] = result[b], result[a]
	}
	return result, nil
}
