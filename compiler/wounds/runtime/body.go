package wounds

import (
	"bytes"
	"hash/fnv"
)

// Bleed is one met vessel (or a broken femur) bleeding at its own rate.
type Bleed struct {
	Structure string `json:"structure"`
	Kind      string `json:"kind"`
	Internal  bool   `json:"internal,omitempty"`
	// microlitres per second at full pressure, after the mechanism and the difficulty
	RateUL    int64 `json:"rate_ul_s"`
	ClotHalfS int64 `json:"clot_half_s,omitempty"`
	Since     int64 `json:"since"`
	// stopped by its own clot (or for good by a capsule / a medkit); a hand or a dressing only reduces
	Stopped bool `json:"stopped,omitempty"`
	// perfusion: unconscious from this body second while this vessel still runs (carotid, heart); 0 — none
	KnockoutAt int64 `json:"knockout_at,omitempty"`
}

// Wound is one channel: where, how deep, what it met, who made it and what was done about it.
type Wound struct {
	ID         string   `json:"id"`
	Zone       string   `json:"zone"`
	Depth      string   `json:"depth"`
	Mechanism  string   `json:"mechanism"`
	Weapon     string   `json:"weapon,omitempty"`
	Author     string   `json:"author,omitempty"`
	At         int64    `json:"at"`
	Structures []string `json:"structures"`
	Bleeds     []Bleed  `json:"bleeds,omitempty"`
	Effects    []string `json:"effects,omitempty"`
	// aid on it: who keeps pressure, the controls applied (tourniquet, packing, dressing, seal), since when the tourniquet
	Pressure     string   `json:"pressure,omitempty"`
	Controls     []string `json:"controls,omitempty"`
	TourniquetAt int64    `json:"tourniquet_at,omitempty"`
	Splinted     bool     `json:"splinted,omitempty"`
	Treated      bool     `json:"treated,omitempty"`
	LimbLost     bool     `json:"limb_lost,omitempty"`
	DoomAt       int64    `json:"doom_at,omitempty"` // death at this body second without care; 0 — none
	HealAt       int64    `json:"heal_at"`
	LostUL       int64    `json:"lost_ul"` // blood lost through it (who made it answers for it)
	Aiders       []string `json:"aiders,omitempty"`
}

// Condition is a body state that is not a wound: a poison, a disease, a burn, an infection.
type Condition struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Author  string   `json:"author,omitempty"`
	At      int64    `json:"at"`
	OnsetAt int64    `json:"onset_at"`
	DoomAt  int64    `json:"doom_at,omitempty"`
	HealAt  int64    `json:"heal_at,omitempty"`
	Effects []string `json:"effects,omitempty"`
}

// Care is a treatment in progress (the world does not stop): bleeding and doom are held until it ends.
type Care struct {
	Provider   string `json:"provider"`
	Mode       string `json:"mode"` // full | stabilize
	Start      int64  `json:"start"`
	End        int64  `json:"end"`
	PriceMinor int64  `json:"price_minor"`
	Seed       int64  `json:"seed"`
}

// Body is the persistent wound state of one person. Clock is the body's own second counter; the host ticks it with the
// Life clock (also during travel when it catches up).
type Body struct {
	ID         string      `json:"id"`
	Chip       string      `json:"chip"`
	Clock      int64       `json:"clock"`
	VolumeUL   int64       `json:"volume_ul"`
	LostUL     int64       `json:"lost_ul"`
	RegenAcc   int64       `json:"regen_acc,omitempty"`
	Wounds     []Wound     `json:"wounds,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
	// a concussion / a choke: unconscious until; a shock: on the ground (conscious) until
	KnockoutFrom  int64 `json:"knockout_from,omitempty"`
	KnockoutUntil int64 `json:"knockout_until,omitempty"`
	StunUntil     int64 `json:"stun_until,omitempty"`
	PainOffUntil  int64 `json:"pain_off_until,omitempty"`
	// unconscious from blood loss (with the wake hysteresis)
	Down    bool   `json:"down,omitempty"`
	Dead    bool   `json:"dead,omitempty"`
	DeadAt  int64  `json:"dead_at,omitempty"`
	Cause   string `json:"cause,omitempty"`   // the wound or condition id
	Culprit string `json:"culprit,omitempty"` // who answers for the death
	Alarmed bool   `json:"alarmed,omitempty"`
	// a protected body (platinum) stopped where death would come: a story node, not a roll
	Story bool  `json:"story,omitempty"`
	Care  *Care `json:"care,omitempty"`
	Seq   int   `json:"seq"`
	// the last state reported in an event (weak / confused / down / ok)
	Reported string `json:"reported,omitempty"`
}

// NewBody: a whole adult with the chip profile (none / bronze / ...).
func (m Model) NewBody(id, chip string) Body {
	return Body{ID: id, Chip: chip, VolumeUL: m.Blood.VolumeML * 1000, Reported: "ok"}
}

// Event is what changed: the host turns it into barks, justice, a chip alarm, the panel.
type Event struct {
	Type    string `json:"type"`
	Body    string `json:"body"`
	At      int64  `json:"at"`
	Wound   string `json:"wound,omitempty"`
	Culprit string `json:"culprit,omitempty"`
	Crime   string `json:"crime,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// roll is a deterministic 64-bit draw from the seed and the named stream (splitmix64 of the FNV-1a of the keys).
func roll(seed int64, keys ...string) uint64 {
	h := fnv.New64a()
	for _, k := range keys {
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte{0})
	}
	x := h.Sum64() ^ uint64(seed)*0x9E3779B97F4A7C15
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}
func chance(ppm int64, seed int64, keys ...string) bool {
	if ppm <= 0 {
		return false
	}
	if ppm >= Million {
		return true
	}
	return int64(roll(seed, keys...)%Million) < ppm
}
func inSpan(s Span, seed int64, keys ...string) int64 {
	if s.Max <= s.Min {
		return s.Min
	}
	return s.Min + int64(roll(seed, keys...)%uint64(s.Max-s.Min+1))
}

// pick draws a key from integer weights in sorted key order.
func pick(weights map[string]int64, seed int64, keys ...string) string {
	total := int64(0)
	names := sortedKeys(weights)
	for _, k := range names {
		if weights[k] > 0 {
			total += weights[k]
		}
	}
	if total <= 0 {
		return ""
	}
	r := int64(roll(seed, keys...) % uint64(total))
	for _, k := range names {
		if weights[k] <= 0 {
			continue
		}
		if r < weights[k] {
			return k
		}
		r -= weights[k]
	}
	return names[len(names)-1]
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
func addUnique(list []string, s ...string) []string {
	for _, x := range s {
		if !contains(list, x) {
			list = append(list, x)
		}
	}
	return list
}
func without(list []string, drop ...string) []string {
	out := list[:0:0]
	for _, x := range list {
		if !contains(drop, x) {
			out = append(out, x)
		}
	}
	return out
}

// Clone is a deep copy (forecasts and scenario runs never touch the live body).
func (b Body) Clone() Body {
	c := b
	c.Wounds = make([]Wound, len(b.Wounds))
	for i, w := range b.Wounds {
		w.Structures = append([]string(nil), w.Structures...)
		w.Bleeds = append([]Bleed(nil), w.Bleeds...)
		w.Effects = append([]string(nil), w.Effects...)
		w.Controls = append([]string(nil), w.Controls...)
		w.Aiders = append([]string(nil), w.Aiders...)
		c.Wounds[i] = w
	}
	c.Conditions = make([]Condition, len(b.Conditions))
	for i, x := range b.Conditions {
		x.Effects = append([]string(nil), x.Effects...)
		c.Conditions[i] = x
	}
	if b.Care != nil {
		cc := *b.Care
		c.Care = &cc
	}
	return c
}

func (b *Body) wound(id string) *Wound {
	for i := range b.Wounds {
		if b.Wounds[i].ID == id {
			return &b.Wounds[i]
		}
	}
	return nil
}

// LostPermille: the share of the blood volume lost.
func (b Body) LostPermille() int64 {
	if b.VolumeUL <= 0 {
		return 1000
	}
	return b.LostUL * 1000 / b.VolumeUL
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
