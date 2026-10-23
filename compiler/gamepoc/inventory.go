package gamepoc

import (
	"cuelang.org/go/cue"
	"fmt"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"math"
	"math/big"
	"sort"
)

func inventoryModel(world, level cue.Value, levelID string) (inv.Model, error) {
	m := inv.Model{Version: 1, Definitions: map[string]inv.Definition{}, Currencies: map[string]inv.Currency{}, Actors: map[string]bool{}, Owners: map[string]bool{}, Levels: map[string]inv.Cell{}, Stores: map[string]inv.Store{}, Accounts: map[string]inv.Account{}, Entitlements: map[string]inv.Entitlement{}}
	readString := func(v cue.Value, p string) string { s, _ := v.LookupPath(cue.ParsePath(p)).String(); return s }
	readInt := func(v cue.Value, p string) int64 { n, _ := v.LookupPath(cue.ParsePath(p)).Int64(); return n }
	readFloat := func(v cue.Value, p string, otherwise float64) float64 {
		f, e := v.LookupPath(cue.ParsePath(p)).Float64()
		if e != nil {
			return otherwise
		}
		return f
	}
	objects := func(v cue.Value, visit func(string, cue.Value) error) error {
		it, err := v.Fields()
		if err != nil {
			return err
		}
		for it.Next() {
			if err := visit(it.Selector().Unquoted(), it.Value()); err != nil {
				return err
			}
		}
		return nil
	}
	m.LevelSettlement = map[string]string{levelID: readString(level, "settlement")}
	if err := world.LookupPath(cue.ParsePath("economy.markets")).Decode(&m.Markets); err != nil {
		return m, err
	}
	policy := world.LookupPath(cue.ParsePath("inventory"))
	if err := policy.LookupPath(cue.ParsePath("reservation")).Decode(&m.Reservation); err != nil {
		return m, err
	}
	if err := policy.LookupPath(cue.ParsePath("preparation")).Decode(&m.Preparation); err != nil {
		return m, err
	}
	m.IssuePoints = map[string]inv.IssuePoint{}
	if err := policy.LookupPath(cue.ParsePath("actions")).Decode(&m.Actions); err != nil {
		return m, err
	}
	m.MaxDepth = int(readInt(policy, "max_depth"))
	m.MaxItems = int(readInt(policy, "max_instances"))
	if err := world.LookupPath(cue.ParsePath("economy.currencies")).Decode(&m.Currencies); err != nil {
		return m, err
	}
	if err := policy.LookupPath(cue.ParsePath("initial_entitlements")).Decode(&m.Entitlements); err != nil {
		return m, err
	}
	if err := policy.LookupPath(cue.ParsePath("initial_accounts")).Decode(&m.Accounts); err != nil {
		return m, err
	}
	for id, a := range m.Accounts {
		a.ID = id
		m.Accounts[id] = a
	}
	var capacity inv.Capacity
	if err := policy.LookupPath(cue.ParsePath("actor_capacity")).Decode(&capacity); err != nil {
		return m, err
	}
	for _, kind := range []string{"persons", "factions", "settlements"} {
		if err := objects(world.LookupPath(cue.ParsePath(kind)), func(id string, v cue.Value) error {
			m.Owners[id] = true
			if kind == "persons" {
				m.Actors[id] = true
				sid := inv.ActorStore(id)
				m.Stores[sid] = inv.Store{ID: sid, Kind: "actor", Anchor: inv.Anchor{Kind: "actor", Actor: id}, Owner: id, Capacity: capacity}
			}
			return nil
		}); err != nil {
			return m, err
		}
	}
	if err := objects(world.LookupPath(cue.ParsePath("items")), func(id string, v cue.Value) error {
		definition := inv.Definition{ID: id, Name: readString(v, "name"), Carrier: readString(v, "carrier"), Mass: readInt(v, "physical.mass_g"), Volume: readInt(v, "physical.volume_ml"), StackLimit: readInt(v, "stack_limit"), ContainerVolume: readInt(v, "container.capacity_ml"), Currency: readString(v, "currency.unit"), Denomination: readInt(v, "currency.amount")}
		definition.Weapon = v.LookupPath(cue.ParsePath("weapon")).Exists() || v.LookupPath(cue.ParsePath("use")).Exists()
		definition.EquipSlot = readString(v, "equipment.slot")
		definition.UseEffect = readString(v, "use.effect")
		definition.UseReach = int(readInt(v, "use.reach"))
		definition.WearPerUse = int64(math.Round(readFloat(v, "use.wear_per_use", 0) * 1000000))
		definition.PriceUnit = readString(v, "base_price.unit")
		raw, e := v.LookupPath(cue.ParsePath("base_price.avg")).MarshalJSON()
		if e != nil {
			return e
		}
		price, ok := new(big.Rat).SetString(string(raw))
		if !ok {
			return fmt.Errorf("invalid price %s", id)
		}
		price.Mul(price, big.NewRat(m.Currencies[definition.PriceUnit].Scale, 1))
		if !price.Num().IsInt64() || !price.Denom().IsInt64() {
			return fmt.Errorf("price precision exceeds int64: %s", id)
		}
		definition.PriceNumerator = price.Num().Int64()
		definition.PriceDenominator = price.Denom().Int64()
		m.Definitions[id] = definition

		return nil
	}); err != nil {
		return m, err
	}
	size := inv.Cell{X: int(readInt(level, "size.w")), Y: int(readInt(level, "size.h"))}
	m.Levels[levelID] = size
	storageID := func(name string) string { return levelID + "/" + name }
	if err := objects(level.LookupPath(cue.ParsePath("storage")), func(name string, v cue.Value) error {
		var capacity inv.Capacity
		var at inv.Cell
		if err := v.LookupPath(cue.ParsePath("capacity")).Decode(&capacity); err != nil {
			return err
		}
		if err := v.LookupPath(cue.ParsePath("at")).Decode(&at); err != nil {
			return err
		}
		locked, _ := v.LookupPath(cue.ParsePath("locked")).Bool()
		id := storageID(name)
		m.Stores[id] = inv.Store{ID: id, Name: readString(v, "name"), Kind: readString(v, "kind"), Anchor: inv.Anchor{Kind: "world", Level: levelID, At: at}, Capacity: capacity, Owner: readString(v, "owner"), Locked: locked}
		return nil
	}); err != nil {
		return m, err
	}
	if err := objects(level.LookupPath(cue.ParsePath("equipment_issue")), func(name string, v cue.Value) error {
		var point inv.IssuePoint
		point.ID = levelID + "/" + name
		point.Name = readString(v, "name")
		point.Store = storageID(readString(v, "storage"))
		if err := v.LookupPath(cue.ParsePath("known_to_profiles")).Decode(&point.KnownProfiles); err != nil {
			return err
		}
		if err := v.LookupPath(cue.ParsePath("access_profiles")).Decode(&point.AccessProfiles); err != nil {
			return err
		}
		m.IssuePoints[point.ID] = point
		return nil
	}); err != nil {
		return m, err
	}
	props, err := level.LookupPath(cue.ParsePath("props")).List()
	if err != nil {
		return m, err
	}
	for props.Next() {
		p := props.Value()
		def := world.LookupPath(cue.MakePath(cue.Str("prop_defs"), cue.Str(readString(p, "def"))))
		volume := readInt(def, "container.capacity_ml")
		if volume < 1 {
			continue
		}
		var at inv.Cell
		if err := p.LookupPath(cue.ParsePath("at")).Decode(&at); err != nil {
			return m, err
		}
		id := storageID("prop/" + readString(p, "id"))
		m.Stores[id] = inv.Store{ID: id, Name: readString(def, "name"), Kind: "container", Anchor: inv.Anchor{Kind: "world", Level: levelID, At: at}, Capacity: inv.Capacity{Mass: 9000000000000000, Volume: volume, Stacks: m.MaxItems}, Owner: readString(p, "owner"), Locked: p.LookupPath(cue.ParsePath("lock")).Exists()}
	}
	placements, err := level.LookupPath(cue.ParsePath("placements")).List()
	if err != nil {
		return m, err
	}
	for placements.Next() {
		p := placements.Value()
		src := inv.Source{ID: levelID + "/" + readString(p, "id"), Definition: readString(p, "content.item"), Quantity: readInt(p, "content.qty"), Condition: int64(math.Round(readFloat(p, "content.condition", 1) * 1000000)), Owner: readString(p, "owner"), Hidden: readString(p, "visibility") == "hidden"}
		src.ForSale, _ = p.LookupPath(cue.ParsePath("for_sale")).Bool()
		where := p.LookupPath(cue.ParsePath("where"))
		switch {
		case where.LookupPath(cue.ParsePath("at")).Exists():
			src.Location = inv.Location{Kind: "ground", Level: levelID}
			if err := where.LookupPath(cue.ParsePath("at")).Decode(&src.Location.At); err != nil {
				return m, err
			}
		case readString(where, "prop") != "":
			src.Location = inv.Location{Kind: "store", Store: storageID("prop/" + readString(where, "prop"))}
		case readString(where, "storage") != "":
			src.Location = inv.Location{Kind: "store", Store: storageID(readString(where, "storage"))}
		case readString(where, "actor") != "":
			src.Location = inv.Location{Kind: "store", Store: inv.ActorStore(readString(where, "actor"))}
		case readString(where, "inside") != "":
			src.Location = inv.Location{Kind: "store", Store: inv.ItemStore(inv.InstanceID(levelID+"/"+readString(where, "inside"), 0, 0))}
		default:
			return m, fmt.Errorf("placement %s has no supported location", src.ID)
		}
		if readString(p, "content.kind") == "loot" {
			table := world.LookupPath(cue.MakePath(cue.Str("loot_tables"), cue.Str(readString(p, "content.table"))))
			src.Rolls = int(readInt(table, "rolls"))
			src.NothingWeight = int64(math.Round(readFloat(table, "nothing_weight", 0) * 1000000))
			entries, err := table.LookupPath(cue.ParsePath("entries")).List()
			if err != nil {
				return m, err
			}
			for entries.Next() {
				e := entries.Value()
				src.Loot = append(src.Loot, inv.Loot{Definition: readString(e, "item"), Weight: int64(math.Round(readFloat(e, "weight", 0) * 1000000)), Min: readInt(e, "qty.min"), Max: readInt(e, "qty.max"), ConditionMin: int64(math.Round(readFloat(e, "condition.min", 1) * 1000000)), ConditionMax: int64(math.Round(readFloat(e, "condition.max", 1) * 1000000))})
			}
			if len(src.Loot) == 0 {
				return m, fmt.Errorf("empty loot source %s", src.ID)
			}
		}
		m.Sources = append(m.Sources, src)
	}
	fills, _ := level.LookupPath(cue.ParsePath("fills")).List()
	for fills.Next() {
		kind := readString(fills.Value(), "kind")
		if kind == "stalls" || kind == "clutter" {
			return m, fmt.Errorf("inventory for fill %s must be materialized before loading this level", readString(fills.Value(), "id"))
		}
	}

	// Life equipment stores are physical inventory at their declared map coordinates.
	if life := world.LookupPath(cue.ParsePath("life")); life.Exists() {
		var data struct {
			Locations map[string]struct {
				Terrain []string `json:"terrain"`
			} `json:"locations"`
			Maintenance struct {
				Assets []struct {
					ID        string       `json:"id"`
					Name      string       `json:"name"`
					Level     string       `json:"level"`
					At        inv.Cell     `json:"at"`
					Part      string       `json:"part_definition"`
					Spares    int          `json:"initial_spares"`
					Condition int64        `json:"initial_condition_ppm"`
					Capacity  inv.Capacity `json:"storage_capacity"`
				}
			} `json:"maintenance"`
		}
		if err := life.Decode(&data); err != nil {
			return m, err
		}
		for id, l := range data.Locations {
			if len(l.Terrain) > 0 {
				m.Levels[id] = inv.Cell{X: len(l.Terrain[0]), Y: len(l.Terrain)}
			}
		}
		for _, a := range data.Maintenance.Assets {
			if _, ok := m.Definitions[a.Part]; !ok {
				return m, fmt.Errorf("unknown maintenance part %s", a.Part)
			}
			for _, kind := range []string{"spares", "installed", "retired"} {
				id := "maintenance/" + a.ID + "/" + kind
				if _, exists := m.Stores[id]; exists {
					return m, fmt.Errorf("duplicate maintenance store %s", id)
				}
				m.Stores[id] = inv.Store{ID: id, Name: a.Name + " · " + kind, Kind: "container", Anchor: inv.Anchor{Kind: "world", Level: a.Level, At: a.At}, Capacity: a.Capacity}
				quantity := int64(0)
				condition := int64(1000000)
				if kind == "spares" {
					quantity = int64(a.Spares)
				}
				if kind == "installed" {
					quantity = 1
					condition = a.Condition
				}
				if quantity > 0 {
					m.Sources = append(m.Sources, inv.Source{ID: id, Definition: a.Part, Quantity: quantity, Condition: condition, Location: inv.Location{Kind: "store", Store: id}})
				}
			}
		}
	}
	sort.Slice(m.Sources, func(i, j int) bool { return m.Sources[i].ID < m.Sources[j].ID })
	if _, err := inv.New(m, 425); err != nil {
		return m, fmt.Errorf("inventory initial state: %w", err)
	}
	return m, nil
}
