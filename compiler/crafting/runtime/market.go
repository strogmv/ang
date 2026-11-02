package crafting

import "fmt"

// Stage 2: the market lives by the same law. The price of a job is law.price_market's — every part at its price times
// the scarcity of its class in the hub (the law's curve at stock / the stock of the start), the labour times the scarcest
// class, all times the risk of the job's legality. A master who thinks little of the client may swap a system / prewar
// part for a handmade one and charge for the better; an order can be cancelled by the refund rules; masters restock,
// residents order repairs of their worn things.

type Economy struct {
	Scarcity     []Point          `json:"scarcity"`
	RiskPermille map[string]int64 `json:"risk_permille"`
	Par          map[string]int64 `json:"par,omitempty"` // units of each class in the hub at the start (filled by ANG)
}
type Honesty struct {
	Master            string `json:"master"`
	SwapBelowPermille int64  `json:"swap_below_permille"`
	SwapPermille      int64  `json:"swap_permille"`
}
type Caught struct {
	RelationDeltaPermille int64 `json:"relation_delta_permille"`
	RefundPermille        int64 `json:"refund_permille"`
}
type Refund struct {
	WaitingPermille int64 `json:"waiting_permille"`
	WorkingPermille int64 `json:"working_permille"`
	DonePermille    int64 `json:"done_permille"`
}
type Restock struct {
	Master string   `json:"master"`
	Class  string   `json:"class"`
	Below  int64    `json:"below"`
	Buy    int64    `json:"buy"`
	From   []string `json:"from"`
}
type ResidentThing struct {
	Actor        string `json:"actor"`
	Item         string `json:"item"`
	ConditionPPM int64  `json:"condition_ppm"`
	UsesPerDay   int64  `json:"uses_per_day"`
}
type ResidentRule struct {
	OrderBelowPPM int64 `json:"order_below_ppm"`
	FromHour      int64 `json:"from_hour"`
	ToHour        int64 `json:"to_hour"`
}
type Market struct {
	Honesty                  []Honesty       `json:"honesty"`
	Caught                   Caught          `json:"caught"`
	RelationPerOrderPermille int64           `json:"relation_per_order_permille"`
	Refund                   Refund          `json:"refund"`
	Restock                  []Restock       `json:"restock"`
	Residents                []ResidentThing `json:"residents"`
	ResidentRule             ResidentRule    `json:"resident_rule"`
}

// Swap is a caught or hidden substitution of a part by a master.
type Swap struct {
	Master string `json:"master"`
	Client string `json:"client,omitempty"`
	Recipe string `json:"recipe,omitempty"`
	Class  string `json:"class"`
	From   string `json:"from"`
	To     string `json:"to"`
	Paid   int64  `json:"paid,omitempty"`
	Second int64  `json:"second"`
	Seen   bool   `json:"seen,omitempty"`
}

// ScarcityPermille: the multiplier of the law's curve for a class with `stock` units in the hub now.
func (m Model) ScarcityPermille(class string, stock int64) int64 {
	par := m.Economy.Par[class]
	if par <= 0 || len(m.Economy.Scarcity) == 0 {
		return 1000
	}
	return Curve(m.Economy.Scarcity, stock*1000/par)
}

// PriceOrder: law.price_market applied to a job — parts × scarcity of their class, labour × the scarcest class,
// × legality risk. stock counts the units of each class in the hub now; charged are the uses the client pays for.
func (m Model) PriceOrder(s Master, p Plan, charged []Use, stock map[string]int64) int64 {
	r, _ := m.Recipe(p.Recipe)
	var parts int64
	worst := int64(1000)
	for _, u := range charged {
		k := int64(1000)
		if c, _, ok := m.PartOf(u.Definition); ok {
			k = m.ScarcityPermille(c.ID, stock[c.ID])
		}
		worst = max(worst, k)
		parts += u.Quantity * m.Prices[u.Definition] * k / 1000
	}
	labour := (s.LaborMinorPerHour*p.Seconds + 3599) / 3600 * worst / 1000
	risk := m.Economy.RiskPermille[r.Legal]
	if risk <= 0 {
		risk = 1000
	}
	return ((parts+labour)*risk + 999) / 1000
}

func (m Model) Honesty(master string) (Honesty, bool) {
	for _, h := range m.Market.Honesty {
		if h.Master == master {
			return h, true
		}
	}
	return Honesty{}, false
}

// SwapPlan: below the master's honesty threshold (his relation to the client, permille) and by the seed, the best-grade
// system / prewar part is replaced by a handmade unit of the same class from the master's stock. It returns the plan to
// work (re-planned quality) and the swap ("" class when none).
func (m Model) SwapPlan(w Worker, q Request, master string, relation int64, p Plan, stacks []Stack, target *Stack, bench Bench) (Plan, Swap) {
	h, ok := m.Honesty(master)
	if !ok || relation >= h.SwapBelowPermille || int64(roll(q.Seed, q.JobID, "swap", 0)%1000) >= h.SwapPermille {
		return p, Swap{}
	}
	for _, u := range p.Uses {
		if u.Origin == "handmade" {
			continue
		}
		class, _, isPart := m.PartOf(u.Definition)
		if !isPart {
			continue
		}
		// hide every better unit of the class; the engine then takes the handmade ones if the stock has them
		hidden := []Stack{}
		for _, s := range stacks {
			if c, _, ok := m.PartOf(s.Definition); ok && c.ID == class.ID {
				if _, origin := m.grade(s); origin != "handmade" {
					continue
				}
			}
			hidden = append(hidden, s)
		}
		alt, err := m.Plan(w, q, hidden, target, bench)
		if err != nil {
			continue
		}
		swapped := false
		for _, a := range alt.Uses {
			if c, _, ok := m.PartOf(a.Definition); ok && c.ID == class.ID && a.Origin == "handmade" {
				swapped = true
			}
		}
		if swapped {
			return alt, Swap{Master: master, Class: class.ID, From: u.Origin, To: "handmade"}
		}
	}
	return p, Swap{}
}

// RefundPermille: the share of the price returned on a cancel (waiting in the queue / being worked / done).
func (m Model) RefundPermille(progress int64, done bool) int64 {
	switch {
	case done:
		return m.Market.Refund.DonePermille
	case progress > 0:
		return m.Market.Refund.WorkingPermille
	}
	return m.Market.Refund.WaitingPermille
}

func (m Model) validateMarket() error {
	for _, h := range m.Market.Honesty {
		if _, ok := m.Master(h.Master); !ok || h.SwapPermille < 0 || h.SwapPermille > 1000 {
			return fmt.Errorf("crafting honesty %s: invalid", h.Master)
		}
	}
	for _, r := range m.Market.Restock {
		_, ok := m.Master(r.Master)
		_, part := m.Part(r.Class)
		if !ok || !part || r.Below < 1 || r.Buy < 1 || len(r.From) == 0 {
			return fmt.Errorf("crafting restock %s/%s: invalid", r.Master, r.Class)
		}
		for _, f := range r.From {
			if f != "own" && f != "market" && f != "vending" && f != "salvage" {
				return fmt.Errorf("crafting restock %s: unknown source %s", r.Master, f)
			}
		}
	}
	for _, t := range m.Market.Residents {
		repairable := false
		for _, ms := range m.Masters {
			for _, o := range ms.Offers {
				r, _ := m.Recipe(o)
				repairable = repairable || (r.Kind == "repair" || r.Kind == "maintain") && r.Target == t.Item
			}
		}
		if t.Actor == "" || t.ConditionPPM < 1 || t.ConditionPPM > Million || t.UsesPerDay < 1 || !repairable {
			return fmt.Errorf("crafting resident %s/%s: no master repairs it", t.Actor, t.Item)
		}
	}
	rr := m.Market.ResidentRule
	if len(m.Market.Residents) > 0 && (rr.OrderBelowPPM < 1 || rr.OrderBelowPPM >= Million || rr.FromHour < 0 || rr.ToHour <= rr.FromHour || rr.ToHour > 24) {
		return fmt.Errorf("crafting resident rule: invalid")
	}
	for _, k := range []int64{m.Market.Refund.WaitingPermille, m.Market.Refund.WorkingPermille, m.Market.Refund.DonePermille, m.Market.Caught.RefundPermille} {
		if k < 0 || k > 1000 {
			return fmt.Errorf("crafting refund shares must be 0..1000")
		}
	}
	return nil
}
