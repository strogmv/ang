package locationsdk

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Build validates a complete authored layout before returning a usable level.
// It owns all returned slices: neither sorting inputs nor editing outputs can
// mutate a caller's catalog/layout. Seed selection depends on stable instance
// identity, never on the order of authoring records or a global random stream.
func Build(catalog Catalog, layout Layout) (CompiledLevel, error) {
	p, e := prepare(catalog)
	if e != nil {
		return CompiledLevel{Diagnostics: e.Diagnostics}, e
	}
	fail := func(code, path, message string, refs ...string) (CompiledLevel, error) {
		err := issue(code, path, message, refs...)
		return CompiledLevel{Diagnostics: err.Diagnostics}, err
	}
	if !stableID.MatchString(layout.ID) {
		return fail("invalid_id", "layout.id", "layout needs a stable identifier")
	}
	if layout.UnitMM != UnitMM {
		return fail("unsupported_units", "layout.unit_mm", "layout must use the catalog's 500 mm units")
	}
	if len(layout.Decks) != 0 {
		return fail("unsupported_decks", "layout.decks", "compile one planar deck at a time; inter-deck links are not implemented")
	}
	if layout.Width < 1 || layout.Depth < 1 || layout.Width > 2048 || layout.Depth > 2048 || layout.Width*layout.Depth > 1048576 || layout.Elevation < -1000000 || layout.Elevation > 1000000 {
		return fail("layout_bounds", "layout", "supported layout bounds are 1..2048 units per side, at most 1048576 cells, with bounded elevation")
	}
	if len(layout.Floor) < 1 || len(layout.Floor) > 4096 || len(layout.Instances) > 10000 || len(layout.ReservedPaths) > 4096 || len(layout.Drops) > 16384 {
		return fail("layout_budget", "layout", "layout exceeds the supported authoring record budget or has no floor")
	}
	inside := func(c Cell) bool { return c.X >= 0 && c.Y >= 0 && c.X < layout.Width && c.Y < layout.Depth }
	index := func(c Cell) int { return c.Y*layout.Width + c.X }
	count := layout.Width * layout.Depth
	support := make([]bool, count)
	floorSource := make([]int, count)
	floors := append([]FloorArea{}, layout.Floor...)
	floorIDs := map[string]bool{}
	for i, floor := range floors {
		path := fmt.Sprintf("layout.floor[%d]", i)
		if !stableID.MatchString(floor.ID) || floorIDs[floor.ID] || strings.TrimSpace(floor.Reason) == "" {
			return fail("floor_identity", path, "each floor area needs a unique ID and an authored reason")
		}
		floorIDs[floor.ID] = true
		r := floor.Rect
		if r.X < 0 || r.Y < 0 || r.W < 1 || r.H < 1 || r.X >= layout.Width || r.Y >= layout.Depth || r.W > layout.Width-r.X || r.H > layout.Depth-r.Y {
			return fail("floor_out_of_bounds", path+".rect", "floor rectangle must lie within layout bounds")
		}
	}
	sort.Slice(floors, func(i, j int) bool { return floors[i].ID < floors[j].ID })
	for i, floor := range floors {
		for y := floor.Rect.Y; y < floor.Rect.Y+floor.Rect.H; y++ {
			for x := floor.Rect.X; x < floor.Rect.X+floor.Rect.W; x++ {
				n := y*layout.Width + x
				if !support[n] {
					support[n], floorSource[n] = true, i
				}
			}
		}
	}
	out := CompiledLevel{ID: layout.ID, UnitMM: UnitMM, Width: layout.Width, Depth: layout.Depth, Elevation: layout.Elevation, Grounded: layout.Grounded,
		Instances: []CompiledInstance{}, Ports: []CompiledPort{}, Clearances: []CompiledClearance{}, ReservedPaths: []CompiledPath{}, Barriers: []Barrier{}, Drops: []DropAnnotation{}, Views: []PresentationView{}, Diagnostics: []Diagnostic{},
		Preview: PreviewData{UnitMM: UnitMM, Width: layout.Width, Depth: layout.Depth, Floor: []PreviewFloor{}}}
	material := make(map[Cell]string)
	blocked := make([]bool, count)
	instanceIDs := map[string]bool{}
	emittedCells := 0
	decorationIDs := map[string]bool{}
	eventDecorations := map[string][]CompiledDecoration{}
	for i, instance := range layout.Instances {
		path := fmt.Sprintf("layout.instances[%d]", i)
		if !stableID.MatchString(instance.ID) || instanceIDs[instance.ID] {
			return fail("duplicate_or_invalid_instance", path+".id", "instance IDs must be stable and unique")
		}
		instanceIDs[instance.ID] = true
		if instance.Rotation != 0 && instance.Rotation != 90 && instance.Rotation != 180 && instance.Rotation != 270 {
			return fail("unsupported_rotation", path+".rotation", "supported rotations are 0, 90, 180, 270")
		}
		if strings.TrimSpace(instance.Reason) == "" {
			return fail("missing_reason", path+".reason", "every generated instance must retain its authored purpose")
		}
		if instance.OwnerRef == "" {
			instance.OwnerRef = "unknown"
		}
		if !stableID.MatchString(instance.OwnerRef) {
			return fail("owner_metadata", path+".owner_ref", "owner must be a stable reference or explicit unknown")
		}
		if instance.Fate.Policy == "" {
			instance.Fate.Policy = "unknown"
		}
		if instance.Fate.Policy != "keep" && instance.Fate.Policy != "unknown" && instance.Fate.Policy != "until_event" || instance.Fate.Policy == "until_event" && !stableID.MatchString(instance.Fate.EventRef) || instance.Fate.Policy != "until_event" && instance.Fate.EventRef != "" {
			return fail("fate_metadata", path+".fate", "fate is keep, unknown, or until_event with a stable event reference; no runtime cleanup is implied")
		}
		module, ok := p.modules[instance.ModuleID]
		if !ok {
			return fail("unknown_module", path+".module_id", "instance references an unknown module", instance.ModuleID)
		}
		geometry := p.geometries[module.GeometryID]
		emittedCells += len(geometry.Footprint) + len(geometry.Blocked) + len(geometry.UseClearance) + len(geometry.MaintenanceClearance)
		if emittedCells > 4194304 {
			return fail("layout_budget", path, "expanded instances exceed the supported 4194304 structural-cell budget")
		}
		variant := module.Variants[0]
		if instance.VariantID == "" {
			h := sha256.Sum256([]byte(strconv.FormatInt(layout.Seed, 10) + "\x00" + instance.ID + "\x00" + module.ID))
			variant = module.Variants[binary.LittleEndian.Uint64(h[:8])%uint64(len(module.Variants))]
		} else {
			found := false
			for _, v := range module.Variants {
				if v.ID == instance.VariantID {
					variant, found = v, true
					break
				}
			}
			if !found {
				return fail("unknown_variant", path+".variant_id", "instance references an unknown visual variant", module.ID, instance.VariantID)
			}
		}
		w, d := geometry.Width, geometry.Depth
		if instance.Rotation == 90 || instance.Rotation == 270 {
			w, d = d, w
		}
		if instance.At.X < 0 || instance.At.Y < 0 || instance.At.X >= layout.Width || instance.At.Y >= layout.Depth || w > layout.Width-instance.At.X || d > layout.Depth-instance.At.Y {
			return fail("instance_out_of_bounds", path+".at", "rotated module bounds must fit the level", instance.ID)
		}
		physical := worldCells(geometry.Footprint, geometry, instance)
		for _, at := range physical {
			if !inside(at) || !support[index(at)] {
				return fail("unsupported_instance", path+".at", fmt.Sprintf("physical material at (%d,%d) has no authored floor support", at.X, at.Y), instance.ID)
			}
			if previous, exists := material[at]; exists {
				return fail("physical_overlap", path+".at", fmt.Sprintf("physical footprints overlap at (%d,%d)", at.X, at.Y), previous, instance.ID)
			}
			material[at] = instance.ID
		}
		obstacles := worldCells(geometry.Blocked, geometry, instance)
		for _, at := range obstacles {
			blocked[index(at)] = true
		}
		out.Instances = append(out.Instances, CompiledInstance{ID: instance.ID, ModuleID: module.ID, VariantID: variant.ID, ImageRef: variant.ImageRef, SurfaceID: variant.SurfaceID, At: instance.At, Rotation: instance.Rotation, Width: w, Depth: d, Height: geometry.Height, GeometryHash: p.hashes[module.GeometryID], Footprint: physical, Blocked: obstacles, Reason: instance.Reason, OwnerRef: instance.OwnerRef, Fate: instance.Fate})
		for kind, cells := range map[string][]Cell{"use": geometry.UseClearance, "maintenance": geometry.MaintenanceClearance} {
			if len(cells) > 0 {
				out.Clearances = append(out.Clearances, CompiledClearance{InstanceID: instance.ID, Kind: kind, Cells: worldCells(cells, geometry, instance), Reason: kind + " access required by module " + module.ID})
			}
		}
		for _, port := range geometry.Ports {
			at := worldCells([]Cell{port.At}, geometry, instance)[0]
			out.Ports = append(out.Ports, CompiledPort{ID: port.ID, InstanceID: instance.ID, Kind: port.Kind, At: at, Opening: worldCells(port.Opening, geometry, instance), Side: rotateSide(port.Side, instance.Rotation), Approach: worldCells(port.Approach, geometry, instance)})
		}
		occupied := map[string]bool{}
		for j, binding := range instance.Decorations {
			bp := fmt.Sprintf("%s.decorations[%d]", path, j)
			if !stableID.MatchString(binding.ID) || decorationIDs[binding.ID] || strings.TrimSpace(binding.Reason) == "" {
				return fail("decoration_identity", bp, "event attachments need unique stable IDs and an authored reason")
			}
			decorationIDs[binding.ID] = true
			decoration, ok := p.decor[binding.DecorationID]
			if !ok {
				return fail("unknown_decoration", bp+".decoration_id", "unknown presentation decoration", binding.DecorationID)
			}
			eventFound := false
			for _, event := range p.events {
				eventFound = eventFound || event.ID == binding.EventID
			}
			if !eventFound {
				return fail("unknown_event", bp+".event_id", "decoration references an unknown event", binding.EventID)
			}
			var socket Socket
			socketFound := false
			for _, s := range geometry.Sockets {
				if s.ID == binding.SocketID {
					socket, socketFound = s, true
				}
			}
			if !socketFound {
				return fail("unknown_socket", bp+".socket_id", "module has no matching attachment socket", instance.ID, binding.SocketID)
			}
			if decoration.Width > socket.Width || decoration.Depth > socket.Depth || decoration.Height > socket.Height {
				return fail("decoration_does_not_fit", bp, "decoration exceeds its socket's authored box", binding.ID)
			}
			key := binding.EventID + "\x00" + binding.SocketID
			if occupied[key] {
				return fail("socket_occupied", bp+".socket_id", "two event attachments occupy the same socket", instance.ID, binding.SocketID)
			}
			occupied[key] = true
			at := worldCells([]Cell{socket.At}, geometry, instance)[0]
			eventDecorations[binding.EventID] = append(eventDecorations[binding.EventID], CompiledDecoration{ID: binding.ID, InstanceID: instance.ID, SocketID: socket.ID, DecorationID: decoration.ID, ImageRef: decoration.ImageRef, At: at, Z: socket.Z + layout.Elevation, Reason: binding.Reason})
		}
	}
	// Canonical output ordering also makes diagnostic and JSON consumers stable.
	sort.Slice(out.Instances, func(i, j int) bool { return out.Instances[i].ID < out.Instances[j].ID })
	sort.Slice(out.Ports, func(i, j int) bool {
		a, b := out.Ports[i], out.Ports[j]
		return a.InstanceID < b.InstanceID || a.InstanceID == b.InstanceID && a.ID < b.ID
	})
	sort.Slice(out.Clearances, func(i, j int) bool {
		a, b := out.Clearances[i], out.Clearances[j]
		return a.InstanceID < b.InstanceID || a.InstanceID == b.InstanceID && a.Kind < b.Kind
	})
	walkable := func(at Cell) bool { return inside(at) && support[index(at)] && !blocked[index(at)] }
	for i, clearance := range out.Clearances {
		for _, at := range clearance.Cells {
			if !walkable(at) || material[at] != "" {
				return fail("clearance_obstructed", fmt.Sprintf("compiled.clearances[%d]", i), fmt.Sprintf("%s access at (%d,%d) is not physically clear", clearance.Kind, at.X, at.Y), clearance.InstanceID, material[at])
			}
		}
	}
	for i, port := range out.Ports {
		if !walkable(port.At) {
			return fail("port_obstructed", fmt.Sprintf("compiled.ports[%d].at", i), "module port is not on a walkable floor cell", port.InstanceID, port.ID)
		}
		for _, at := range port.Opening {
			if !walkable(at) || material[at] != "" {
				return fail("port_obstructed", fmt.Sprintf("compiled.ports[%d].opening", i), "every opening edge cell must be clear supported floor", port.InstanceID, port.ID)
			}
		}
		for _, at := range port.Approach {
			if !walkable(at) || material[at] != "" {
				return fail("port_approach_obstructed", fmt.Sprintf("compiled.ports[%d].approach", i), "front approach is outside floor or occupied", port.InstanceID, port.ID)
			}
		}
	}
	pathIDs := map[string]bool{}
	reservedCells := 0
	for i, path := range layout.ReservedPaths {
		pp := fmt.Sprintf("layout.reserved_paths[%d]", i)
		if !stableID.MatchString(path.ID) || pathIDs[path.ID] || strings.TrimSpace(path.Reason) == "" || path.Width < 1 || path.Width > 64 || len(path.Points) < 1 || len(path.Points) > 4096 {
			return fail("reserved_path_definition", pp, "path needs a unique ID, reason, 1..4096 points and a width in 1..64 units")
		}
		pathIDs[path.ID] = true
		cells, err := expandPath(path, layout.Width, layout.Depth)
		if err != nil {
			return fail("reserved_path_geometry", pp+".points", err.Error())
		}
		reservedCells += len(cells)
		if reservedCells > 4194304 {
			return fail("reserved_path_budget", pp, "reserved routes exceed 4194304 expanded cells")
		}
		for _, at := range cells {
			if !walkable(at) || material[at] != "" {
				return fail("reserved_path_obstructed", pp, fmt.Sprintf("reserved route at (%d,%d) is not clear floor", at.X, at.Y), path.ID, material[at])
			}
		}
		out.ReservedPaths = append(out.ReservedPaths, CompiledPath{ID: path.ID, Cells: cells, Reason: path.Reason})
	}
	sort.Slice(out.ReservedPaths, func(i, j int) bool { return out.ReservedPaths[i].ID < out.ReservedPaths[j].ID })
	if len(layout.Entries) < 1 || len(layout.Entries) > 1024 {
		return fail("entry_required", "layout.entries", "provide at least one and at most 1024 real floor entry cells")
	}
	for i, entry := range layout.Entries {
		if !walkable(entry) {
			return fail("entry_obstructed", fmt.Sprintf("layout.entries[%d]", i), "entry must be a free supported floor cell")
		}
	}
	reached := make([]bool, count)
	queue := []Cell{layout.Entries[0]}
	reached[index(queue[0])] = true
	for head := 0; head < len(queue); head++ {
		at := queue[head]
		for _, step := range [...]Cell{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			next := Cell{at.X + step.X, at.Y + step.Y}
			if walkable(next) && !reached[index(next)] {
				reached[index(next)] = true
				queue = append(queue, next)
			}
		}
	}
	for n, supported := range support {
		if supported && !blocked[n] && !reached[n] {
			return fail("floor_disconnected", "layout.floor", fmt.Sprintf("walkable floor (%d,%d) cannot be reached from the entry", n%layout.Width, n/layout.Width))
		}
	}
	type edge struct {
		at   Cell
		side Side
	}
	dropIDs, edges := map[string]bool{}, map[edge]bool{}
	for i, drop := range layout.Drops {
		dp := fmt.Sprintf("layout.drops[%d]", i)
		step, validSide := sideDelta(drop.Side)
		if !stableID.MatchString(drop.ID) || dropIDs[drop.ID] || !validSide || strings.TrimSpace(drop.Reason) == "" || drop.LowerElevation >= layout.Elevation || drop.LowerElevation < -1000000 || !walkable(drop.At) {
			return fail("drop_annotation", dp, "a drop needs a unique ID, free floor origin, cardinal direction, lower elevation and authored reason")
		}
		dropIDs[drop.ID] = true
		neighbor := Cell{drop.At.X + step.X, drop.At.Y + step.Y}
		if inside(neighbor) && (support[index(neighbor)] || blocked[index(neighbor)]) {
			return fail("drop_not_at_floor_edge", dp, "drop must coincide with a computed floor/void boundary; an interior wall is not a drop")
		}
		key := edge{drop.At, drop.Side}
		if edges[key] {
			return fail("duplicate_drop_edge", dp, "this floor edge already has an annotation")
		}
		edges[key] = true
		out.Drops = append(out.Drops, drop)
		if !drop.Barrier {
			continue
		}
		for _, port := range out.Ports {
			if port.Kind == "door" && port.Side == drop.Side {
				for _, at := range port.Opening {
					if at == drop.At {
						return fail("barrier_across_door", dp, "an automatic barrier cannot cross any cell of an authored doorway", port.InstanceID, port.ID)
					}
				}
			}
		}
		out.Barriers = append(out.Barriers, Barrier{ID: "barrier/" + drop.ID, SourceID: drop.ID, At: drop.At, Side: drop.Side, LowerElevation: drop.LowerElevation, Reason: "Boundary barrier requested by authored drop " + drop.ID + ": " + drop.Reason})
	}
	sort.Slice(out.Barriers, func(i, j int) bool { return out.Barriers[i].ID < out.Barriers[j].ID })
	sort.Slice(out.Drops, func(i, j int) bool { return out.Drops[i].ID < out.Drops[j].ID })
	for y := 0; y < layout.Depth; y++ {
		row := make([]byte, layout.Width)
		for x := 0; x < layout.Width; x++ {
			n := y*layout.Width + x
			row[x] = ' '
			if support[n] {
				row[x] = '.'
				floor := floors[floorSource[n]]
				out.Preview.Floor = append(out.Preview.Floor, PreviewFloor{At: Cell{x, y}, SourceID: floor.ID, Reason: floor.Reason})
			}
			if blocked[n] {
				row[x] = '#'
			}
		}
		out.Terrain = append(out.Terrain, string(row))
	}
	out.StructuralHash = structuralHash(out)
	for _, time := range p.times {
		for _, event := range p.events {
			decorations := append([]CompiledDecoration{}, eventDecorations[event.ID]...)
			sort.Slice(decorations, func(i, j int) bool { return decorations[i].ID < decorations[j].ID })
			tags := append(append([]string{}, time.ActivityTags...), event.ActivityTags...)
			out.Views = append(out.Views, PresentationView{TimeID: time.ID, EventID: event.ID, StructuralHash: out.StructuralHash, Brightness: time.Brightness, AccentColor: event.AccentColor, ActivityTags: uniqueStrings(tags), Decorations: decorations})
		}
	}
	return out, nil
}

func expandPath(path ReservedPath, width, depth int) ([]Cell, error) {
	cells := map[Cell]bool{}
	work := 0
	low, high := (path.Width-1)/2, path.Width/2
	stamp := func(at Cell) error {
		work += path.Width * path.Width
		if work > 4194304 {
			return fmt.Errorf("path expansion exceeds the bounded work budget")
		}
		if at.X-low < 0 || at.Y-low < 0 || at.X+high >= width || at.Y+high >= depth {
			return fmt.Errorf("path width leaves the level near (%d,%d)", at.X, at.Y)
		}
		for y := at.Y - low; y <= at.Y+high; y++ {
			for x := at.X - low; x <= at.X+high; x++ {
				cells[Cell{x, y}] = true
			}
		}
		return nil
	}
	for i, at := range path.Points {
		if at.X < 0 || at.Y < 0 || at.X >= width || at.Y >= depth {
			return nil, fmt.Errorf("path point (%d,%d) is outside level bounds", at.X, at.Y)
		}
		if i == 0 {
			if err := stamp(at); err != nil {
				return nil, err
			}
			continue
		}
		previous := path.Points[i-1]
		if previous.X != at.X && previous.Y != at.Y {
			return nil, fmt.Errorf("segment %d is diagonal; reserved routes must be orthogonal", i-1)
		}
		step := Cell{}
		if at.X > previous.X {
			step.X = 1
		} else if at.X < previous.X {
			step.X = -1
		}
		if at.Y > previous.Y {
			step.Y = 1
		} else if at.Y < previous.Y {
			step.Y = -1
		}
		for previous != at {
			previous = Cell{previous.X + step.X, previous.Y + step.Y}
			if err := stamp(previous); err != nil {
				return nil, err
			}
		}
	}
	out := make([]Cell, 0, len(cells))
	for at := range cells {
		out = append(out, at)
	}
	sort.Slice(out, func(i, j int) bool { return lessCell(out[i], out[j]) })
	return out, nil
}

func structuralHash(level CompiledLevel) string {
	// Appearance, purpose prose and seed are deliberately outside structural
	// identity. A visual replacement or presentation state cannot change it.
	type instance struct {
		ID           string
		At           Cell
		Rotation     int
		GeometryHash string
	}
	type barrier struct {
		At             Cell
		Side           Side
		LowerElevation int
	}
	value := struct {
		ID                              string
		UnitMM, Width, Depth, Elevation int
		Grounded                        bool
		Terrain                         []string
		Instances                       []instance
		Ports                           []CompiledPort
		Clearances                      []CompiledClearance
		Paths                           []CompiledPath
		Barriers                        []barrier
		Drops                           []DropAnnotation
	}{ID: level.ID, UnitMM: level.UnitMM, Width: level.Width, Depth: level.Depth, Elevation: level.Elevation, Grounded: level.Grounded, Terrain: level.Terrain, Instances: []instance{}, Ports: level.Ports, Clearances: append([]CompiledClearance{}, level.Clearances...), Paths: append([]CompiledPath{}, level.ReservedPaths...), Barriers: []barrier{}, Drops: append([]DropAnnotation{}, level.Drops...)}
	for _, item := range level.Instances {
		value.Instances = append(value.Instances, instance{item.ID, item.At, item.Rotation, item.GeometryHash})
	}
	for i := range value.Clearances {
		value.Clearances[i].Reason = ""
	}
	for i := range value.Paths {
		value.Paths[i].Reason = ""
	}
	for _, item := range level.Barriers {
		value.Barriers = append(value.Barriers, barrier{item.At, item.Side, item.LowerElevation})
	}
	for i := range value.Drops {
		value.Drops[i].Reason = ""
	}
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
