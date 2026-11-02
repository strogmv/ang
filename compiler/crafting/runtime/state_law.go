package crafting

import (
	"fmt"
	"strings"
)

// Record is a line of the system journal of the hero's chip: rebinding attempts and rebinds at a terminal, scans,
// confiscations. It is evidence the system holds forever (law.attribution_basic: chip_log).
type Record struct {
	Second int64  `json:"second"`
	Kind   string `json:"kind"`
	Item   string `json:"item,omitempty"`
	Unit   string `json:"unit,omitempty"`
}

// Hack is a thing whose binding the hero forged: uses are counted from its wear (the combat layer wears an equipped
// weapon per use), each use may misfire or wound by the seed.
type Hack struct {
	By        string `json:"by"`
	Second    int64  `json:"second"`
	Condition int64  `json:"condition"`
	Uses      int64  `json:"uses"`
	Jammed    bool   `json:"jammed,omitempty"`
}

// LawState: the hero before the law — the system trust, the cases, the ledger count, the sanctions in force.
type LawState struct {
	Trust     int64           `json:"trust"`
	Cases     []Case          `json:"cases,omitempty"`
	Serial    uint64          `json:"serial,omitempty"`
	Offences  int64           `json:"offences,omitempty"`
	Frozen    bool            `json:"frozen,omitempty"`
	Expulsion bool            `json:"expulsion,omitempty"`
	Wanted    bool            `json:"wanted,omitempty"`
	Journal   []Record        `json:"journal,omitempty"`
	Hacked    map[string]Hack `json:"hacked,omitempty"`
	RaidDay   int64           `json:"raid_day,omitempty"`
	Day       int64           `json:"day,omitempty"`
	Seen      map[string]bool `json:"seen,omitempty"` // detections already rolled (act:item:day), so a check fires once
}

// MarketState: hidden or caught swaps per thing, the masters' restock clock, the residents' wear clock and orders.
type MarketState struct {
	Swaps     map[string]Swap  `json:"swaps,omitempty"`
	Restocked map[string]int64 `json:"restocked,omitempty"`
	Ordered   map[string]int64 `json:"ordered,omitempty"`
	WearDay   int64            `json:"wear_day,omitempty"`
}

func (s LawState) clone() LawState {
	out := s
	out.Cases = nil
	for _, c := range s.Cases {
		c.Evidence = append([]Evidence(nil), c.Evidence...)
		c.Pending = append([]Evidence(nil), c.Pending...)
		out.Cases = append(out.Cases, c)
	}
	out.Journal = append([]Record(nil), s.Journal...)
	if s.Hacked != nil {
		out.Hacked = map[string]Hack{}
		for k, v := range s.Hacked {
			out.Hacked[k] = v
		}
	}
	if s.Seen != nil {
		out.Seen = map[string]bool{}
		for k, v := range s.Seen {
			out.Seen[k] = v
		}
	}
	return out
}
func (s MarketState) clone() MarketState {
	out := s
	if s.Swaps != nil {
		out.Swaps = map[string]Swap{}
		for k, v := range s.Swaps {
			out.Swaps[k] = v
		}
	}
	for _, p := range []*map[string]int64{&out.Restocked, &out.Ordered} {
		if *p != nil {
			m := map[string]int64{}
			for k, v := range *p {
				m[k] = v
			}
			*p = m
		}
	}
	return out
}

// Sanction applies the rung of a known case to the law state; the host moves money and things (fine, confiscation).
// It returns the fine of the ledger when the rung fines.
func (m Model) Sanction(s *LawState, a Act) int64 {
	var fine int64
	for _, k := range a.Sanctions {
		switch k {
		case "ration_fine":
			s.Offences++
			fine = m.Fine(s.Offences)
		case "journal_entry", "rating_drop":
			s.Trust = max(0, s.Trust-m.Law.Trust.LossJournalEntry)
		case "account_freeze":
			s.Frozen = true
		case "expulsion":
			s.Expulsion = true
		}
	}
	if a.Reaction == "call_wide" || a.Reaction == "turret_lethal" {
		s.Wanted = true
	}
	return fine
}

func (m Model) validateLawState(s State) error {
	l := s.Law
	if len(m.Law.Acts) == 0 {
		return nil
	}
	if l.Trust < 0 || l.Trust > 1000 || l.Offences < 0 {
		return fmt.Errorf("crafting state: trust or offences out of bounds")
	}
	ids := map[string]bool{}
	for _, c := range l.Cases {
		if _, ok := m.Act(c.Act); !ok || ids[c.ID] || c.ID == "" || c.Suspect == "" {
			return fmt.Errorf("crafting state: invalid case %s", c.ID)
		}
		for _, e := range append(append([]Evidence(nil), c.Evidence...), c.Pending...) {
			if e.Weight < 1 || e.Weight > 1000 || e.Kind == "" {
				return fmt.Errorf("crafting state: case %s has invalid evidence", c.ID)
			}
		}
		ids[c.ID] = true
	}
	for item, h := range l.Hacked {
		if item == "" || h.By == "" || h.Uses < 0 || h.Condition < 0 || h.Condition > Million {
			return fmt.Errorf("crafting state: invalid hacked thing %s", item)
		}
	}
	for item, sw := range s.Market.Swaps {
		if _, ok := m.Master(sw.Master); !ok || item == "" || !strings.HasPrefix(sw.To, "handmade") {
			return fmt.Errorf("crafting state: invalid swap on %s", item)
		}
	}
	return nil
}
