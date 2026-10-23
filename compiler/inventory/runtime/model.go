// Package inventory owns physical custody and account transfers. It has no client, AI or database dependency.
package inventory

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

type Cell struct{ X, Y int }
type Location struct {
	Kind  string `json:"kind"`
	Store string `json:"store,omitempty"`
	Level string `json:"level,omitempty"`
	At    Cell   `json:"at,omitempty"`
	Slot  string `json:"slot,omitempty"`
}
type Anchor struct {
	Kind  string `json:"kind"`
	Actor string `json:"actor,omitempty"`
	Item  string `json:"item,omitempty"`
	Level string `json:"level,omitempty"`
	At    Cell   `json:"at,omitempty"`
}
type Capacity struct {
	Mass   int64 `json:"mass_g"`
	Volume int64 `json:"volume_ml"`
	Stacks int   `json:"stacks"`
}
type Store struct {
	Name     string   `json:"name"`
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Anchor   Anchor   `json:"anchor"`
	Capacity Capacity `json:"capacity"`
	Owner    string   `json:"owner,omitempty"`
	Locked   bool     `json:"locked"`
}
type Definition struct {
	Weapon           bool   `json:"weapon"`
	EquipSlot        string `json:"equip_slot"`
	UseEffect        string `json:"use_effect"`
	UseReach         int    `json:"use_reach"`
	WearPerUse       int64  `json:"wear_per_use_ppm"`
	PriceUnit        string `json:"price_unit"`
	PriceNumerator   int64  `json:"price_numerator"`
	PriceDenominator int64  `json:"price_denominator"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	Carrier          string `json:"carrier"`
	Mass             int64  `json:"mass_g"`
	Volume           int64  `json:"volume_ml"`
	StackLimit       int64  `json:"stack_limit"`
	ContainerVolume  int64  `json:"container_volume_ml"`
	Currency         string `json:"currency,omitempty"`
	Denomination     int64  `json:"denomination"`
}
type Currency struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Carrier string `json:"carrier"`
	Scale   int64  `json:"minor_per_unit"`
	Item    string `json:"item,omitempty"`
}
type Account struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Currency string `json:"currency"`
	Balance  int64  `json:"balance"`
	Revision uint64 `json:"revision"`
}
type Entitlement struct {
	Actor      string `json:"actor"`
	Definition string `json:"definition"`
	Revoked    bool   `json:"revoked"`
}
type Item struct {
	ID         string   `json:"id"`
	Definition string   `json:"definition"`
	Quantity   int64    `json:"quantity"`
	Condition  int64    `json:"condition_ppm"`
	Owner      string   `json:"owner,omitempty"`
	Location   Location `json:"location"`
	Origin     string   `json:"origin"`
	Hidden     bool     `json:"hidden"`
	ForSale    bool     `json:"for_sale"`
	Revision   uint64   `json:"revision"`
	Batch      string   `json:"batch,omitempty"`
}
type Loot struct {
	Definition                 string `json:"definition"`
	Weight                     int64  `json:"weight"`
	Min, Max                   int64
	ConditionMin, ConditionMax int64
}
type Source struct {
	ID            string   `json:"id"`
	Definition    string   `json:"definition"`
	Quantity      int64    `json:"quantity"`
	Condition     int64    `json:"condition_ppm"`
	Owner         string   `json:"owner,omitempty"`
	Location      Location `json:"location"`
	Hidden        bool     `json:"hidden"`
	ForSale       bool     `json:"for_sale"`
	Rolls         int      `json:"rolls"`
	NothingWeight int64    `json:"nothing_weight"`
	Loot          []Loot   `json:"loot,omitempty"`
}
type Market struct {
	PriceUnit string   `json:"price_unit"`
	Accepted  []string `json:"accepted"`
	Barter    bool     `json:"barter"`
}
type PreparationPolicy struct {
	RallyDistanceWeight int    `json:"approach_rally_distance_weight"`
	AssemblyRadius      int    `json:"assembly_radius_cells"`
	MinContactMembers   int    `json:"min_contact_members"`
	SupportDistance     int    `json:"support_distance_cells"`
	Effect              string `json:"effect"`
	Slot                string `json:"slot"`
	LeaseTicks          int64  `json:"lease_ticks"`
	UnavailableTicks    int64  `json:"unavailable_after_ticks"`
}
type IssuePoint struct {
	ID, Name, Store string
	KnownProfiles   []string `json:"known_to_profiles"`
	AccessProfiles  []string `json:"access_profiles"`
}
type ActionPolicy struct {
	ReachCells   int   `json:"reach_cells"`
	FocusTicks   int64 `json:"focus_ticks"`
	ConsentTicks int64 `json:"transfer_consent_ticks"`
}
type Model struct {
	Actions         ActionPolicy           `json:"actions"`
	Preparation     PreparationPolicy      `json:"preparation"`
	IssuePoints     map[string]IssuePoint  `json:"issue_points"`
	Reservation     ReservationPolicy      `json:"reservation"`
	Markets         map[string]Market      `json:"markets"`
	LevelSettlement map[string]string      `json:"level_settlement"`
	Version         int                    `json:"version"`
	MaxDepth        int                    `json:"max_depth"`
	MaxItems        int                    `json:"max_instances"`
	Definitions     map[string]Definition  `json:"definitions"`
	Currencies      map[string]Currency    `json:"currencies"`
	Actors          map[string]bool        `json:"actors"`
	Owners          map[string]bool        `json:"owners"`
	Levels          map[string]Cell        `json:"levels"`
	Stores          map[string]Store       `json:"stores"`
	Accounts        map[string]Account     `json:"accounts"`
	Entitlements    map[string]Entitlement `json:"entitlements"`
	Sources         []Source               `json:"sources"`
}
type Result struct {
	Revision uint64
	Created  []string
	Replayed bool
}
type Cursor struct {
	RequestTag string
	Sequence   uint64
	Digest     [32]byte
	Result     Result
}
type State struct {
	Clock        int64
	Reservations map[string]Reservation
	Version      int
	Revision     uint64
	Items        map[string]Item
	Stores       map[string]Store
	Accounts     map[string]Account
	Entitlements map[string]Entitlement
	Cursors      map[string]Cursor
}

const moneyLimit int64 = 9000000000000000

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func random(seed int64, key string, index int, stream string) uint64 {
	b, _ := json.Marshal([]any{seed, key, index, stream})
	h := sha256.Sum256(b)
	return binary.LittleEndian.Uint64(h[:8])
}
func InstanceID(source string, roll, part int) string {
	return fmt.Sprintf("instance:%s:%d:%d", source, roll, part)
}
func ItemStore(id string) string  { return "item/" + id }
func ActorStore(id string) string { return "actor/" + id }
func New(m Model, seed int64) (State, error) {
	s := State{Reservations: map[string]Reservation{}, Version: m.Version, Items: map[string]Item{}, Stores: map[string]Store{}, Accounts: map[string]Account{}, Entitlements: map[string]Entitlement{}, Cursors: map[string]Cursor{}}
	for id, v := range m.Stores {
		s.Stores[id] = v
	}
	for id, v := range m.Accounts {
		s.Accounts[id] = v
	}
	for id, v := range m.Entitlements {
		s.Entitlements[id] = v
	}
	if m.Version != 1 || m.MaxDepth < 1 || m.MaxDepth > 64 || m.MaxItems < 1 {
		return s, errors.New("invalid inventory model limits")
	}
	seen := map[string]bool{}
	for _, src := range m.Sources {
		if src.ID == "" || seen[src.ID] {
			return s, fmt.Errorf("duplicate source %q", src.ID)
		}
		seen[src.ID] = true
		rolls := 1
		if len(src.Loot) > 0 {
			rolls = src.Rolls
		}
		if rolls < 1 || rolls > 1000 {
			return s, errors.New("invalid loot rolls")
		}
		for roll := 0; roll < rolls; roll++ {
			def, qty, condition := src.Definition, src.Quantity, src.Condition
			if len(src.Loot) > 0 {
				total := src.NothingWeight
				if total < 0 {
					return s, errors.New("negative loot weight")
				}
				for _, e := range src.Loot {
					if d, ok := m.Definitions[e.Definition]; !ok || d.Carrier != "physical" {
						return s, fmt.Errorf("loot source %s contains an invalid physical definition", src.ID)
					}
					if e.Weight < 1 || e.Min < 1 || e.Max < e.Min || e.Max > 100000 || e.ConditionMin < 0 || e.ConditionMax < e.ConditionMin || e.ConditionMax > 1000000 || total > math.MaxInt64-e.Weight {
						return s, errors.New("invalid loot entry")
					}
					total += e.Weight
				}
				if total < 1 {
					return s, errors.New("empty loot distribution")
				}
				pick := int64(random(seed, src.ID, roll, "choice") % uint64(total))
				if pick < src.NothingWeight {
					continue
				}
				pick -= src.NothingWeight
				for _, e := range src.Loot {
					if pick < e.Weight {
						def = e.Definition
						qty = e.Min + int64(random(seed, src.ID, roll, "quantity")%uint64(e.Max-e.Min+1))
						condition = e.ConditionMin + int64(random(seed, src.ID, roll, "condition")%uint64(e.ConditionMax-e.ConditionMin+1))
						break
					}
					pick -= e.Weight
				}
			}
			d, ok := m.Definitions[def]
			if !ok || d.Carrier != "physical" || d.StackLimit < 1 || qty < 1 || qty > 100000 {
				return s, fmt.Errorf("source %s requires a physical item and bounded quantity", src.ID)
			}
			for part := 0; qty > 0; part++ {
				if len(s.Items) >= m.MaxItems {
					return s, errors.New("inventory instance limit")
				}
				n := qty
				if n > d.StackLimit {
					n = d.StackLimit
				}
				if d.ContainerVolume > 0 {
					n = 1
				}
				qty -= n
				id := InstanceID(src.ID, roll, part)
				s.Items[id] = Item{ID: id, Definition: def, Quantity: n, Condition: condition, Owner: src.Owner, Location: src.Location, Origin: src.ID, Hidden: src.Hidden, ForSale: src.ForSale}
				if d.ContainerVolume > 0 {
					s.Stores[ItemStore(id)] = Store{ID: ItemStore(id), Name: d.Name, Kind: "container", Anchor: Anchor{Kind: "item", Item: id}, Capacity: Capacity{Mass: moneyLimit, Volume: d.ContainerVolume, Stacks: m.MaxItems}, Owner: src.Owner}
				}
			}
		}
	}
	if err := s.Validate(m); err != nil {
		return s, err
	}
	for _, src := range m.Sources {
		if _, err := s.Root(m, src.Location); err != nil {
			return s, fmt.Errorf("source %s: %w", src.ID, err)
		}
	}
	return s, nil
}
func (s State) Clone() State {
	n := s
	n.Reservations = make(map[string]Reservation, len(s.Reservations))
	for k, v := range s.Reservations {
		n.Reservations[k] = v
	}
	n.Items = make(map[string]Item, len(s.Items))
	for k, v := range s.Items {
		n.Items[k] = v
	}
	n.Stores = make(map[string]Store, len(s.Stores))
	for k, v := range s.Stores {
		n.Stores[k] = v
	}
	n.Accounts = make(map[string]Account, len(s.Accounts))
	for k, v := range s.Accounts {
		n.Accounts[k] = v
	}
	n.Entitlements = make(map[string]Entitlement, len(s.Entitlements))
	for k, v := range s.Entitlements {
		n.Entitlements[k] = v
	}
	n.Cursors = make(map[string]Cursor, len(s.Cursors))
	for k, v := range s.Cursors {
		v.Result.Created = append([]string(nil), v.Result.Created...)
		n.Cursors[k] = v
	}
	return n
}
func (m Model) validOwner(id string) bool { return id == "" || m.Owners[id] || m.Actors[id] }
func (m Model) validGround(l Location) bool {
	size, ok := m.Levels[l.Level]
	return ok && l.At.X >= 0 && l.At.Y >= 0 && l.At.X < size.X && l.At.Y < size.Y && l.Store == "" && l.Slot == ""
}
func plus(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > moneyLimit-b {
		return 0, errors.New("quantity overflow")
	}
	return a + b, nil
}
func times(a, b int64) (int64, error) {
	if a < 0 || b < 0 || (b > 0 && a > moneyLimit/b) {
		return 0, errors.New("quantity overflow")
	}
	return a * b, nil
}

// Quote keeps fractional source prices exact. A physical denomination cannot be
// rounded into existence: buy a compatible quantity or make a barter offer.
func Quote(d Definition, quantity int64) (int64, error) {
	if quantity < 1 || d.PriceDenominator < 1 {
		return 0, errors.New("no exact price")
	}
	n, e := times(d.PriceNumerator, quantity)
	if e != nil {
		return 0, e
	}
	if n%d.PriceDenominator != 0 {
		return 0, errors.New("price requires smaller denomination or a different quantity")
	}
	return n / d.PriceDenominator, nil
}
func FormatMoney(amount, scale int64) string {
	if scale < 1 {
		return "?"
	}
	digits := 0
	factor := scale
	for factor > 1 && factor%10 == 0 {
		digits++
		factor /= 10
	}
	if factor != 1 {
		return fmt.Sprintf("%d/%d", amount, scale)
	}
	if digits == 0 {
		return fmt.Sprintf("%d", amount)
	}
	return fmt.Sprintf("%d.%0*d", amount/scale, digits, amount%scale)
}
func (s State) Accessible(m Model, l Location) error { return s.accessible(m, l) }
