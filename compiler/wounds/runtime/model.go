// Package wounds is the wound model instead of hit points (doc 97). A blow or a projectile makes a wound: a zone, a depth
// and the structures the channel met (artery, vein, organ, bone, nerve). Blood is a volume; every met vessel bleeds at its
// own rate, slower as the pressure falls, until a clot, a hand, a tourniquet or a capsule stops it. The loss thresholds
// (ATLS classes) make a body weak, confused, unconscious and dead. The engine is deterministic: integer arithmetic only, a
// fixed one-second step and rolls derived from the seed and the wound id, so a save replays exactly. It owns no custody
// and no money: the host spends the aid item and debits the CRD the engine quotes. Not connected to combat here: the
// fighting layer calls ApplyHit / Tick / ApplyAid (design/wounds/codex-handoff.md).
package wounds

import (
	"encoding/json"
	"fmt"
	"sort"
)

const Million = 1000000

// Span of seconds (or of the named unit); the engine rolls inside it.
type Span struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type Difficulty struct {
	ID            string `json:"id"`
	BleedPermille int64  `json:"bleed_permille"`
	DoomPermille  int64  `json:"doom_permille"`
	AidPermille   int64  `json:"aid_permille"`
	PricePermille int64  `json:"price_permille"`
}
type Blood struct {
	VolumeML          int64 `json:"volume_ml"`
	RegenMLH          int64 `json:"regen_ml_h"`
	StepS             int64 `json:"step_s"`
	StoppedBelowMLMin int64 `json:"stopped_below_ml_min"`
}
type Pressure struct {
	SlopePermille int64 `json:"slope_permille"`
	FloorPermille int64 `json:"floor_permille"`
}
type StateMod struct {
	AP           int   `json:"ap"`
	PacePermille int64 `json:"pace_permille"`
}
type States struct {
	WeakPermille      int64    `json:"weak_permille"`
	ConfusedPermille  int64    `json:"confused_permille"`
	DownPermille      int64    `json:"down_permille"`
	DeadPermille      int64    `json:"dead_permille"`
	WakeBelowPermille int64    `json:"wake_below_permille"`
	Weak              StateMod `json:"weak"`
	Confused          StateMod `json:"confused"`
}
type EffectDef struct {
	AP           int   `json:"ap"`
	PacePermille int64 `json:"pace_permille"`
	HoldWeapon   bool  `json:"hold_weapon"`
	Falls        bool  `json:"falls"`
}
type Walk struct {
	NormalMMS            int64 `json:"normal_mm_s"`
	SplintedPacePermille int64 `json:"splinted_pace_permille"`
}
type Mechanism struct {
	Name          string `json:"name"`
	BleedPermille int64  `json:"bleed_permille"`
	Incapacitate  *Span  `json:"incapacitate,omitempty"`
	Condition     string `json:"condition,omitempty"`
}
type Chip struct {
	Monitor           bool   `json:"monitor"`
	AlarmAt           string `json:"alarm_at"`
	ClotPermille      int64  `json:"clot_permille"`
	RegenPermille     int64  `json:"regen_permille"`
	DownBonusPermille int64  `json:"down_bonus_permille"`
	Protected         bool   `json:"protected"`
}
type Structure struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Kind        string           `json:"kind"`
	MinDepth    string           `json:"min_depth"`
	Exposure    map[string]int64 `json:"exposure"`
	BleedMLMin  int64            `json:"bleed_ml_min"`
	ClotHalfS   int64            `json:"clot_half_s"`
	Internal    bool             `json:"internal"`
	Effects     []string         `json:"effects"`
	Knockout    *Span            `json:"knockout,omitempty"`
	KnockoutFor *Span            `json:"knockout_for,omitempty"`
	Doom        *Span            `json:"doom,omitempty"`
	LethalPPM   int64            `json:"lethal_ppm"`
	Severity    int              `json:"severity"`
	HealDays    int64            `json:"heal_days"`
	CapsuleMin  int64            `json:"capsule_min"`
	Sign        string           `json:"sign"`
}
type Zone struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Part       string      `json:"part"`
	Tourniquet bool        `json:"tourniquet"`
	Junctional bool        `json:"junctional"`
	Cavity     bool        `json:"cavity"`
	Structures []Structure `json:"structures"`
}
type Mode struct {
	Mechanism string           `json:"mechanism"`
	Depth     map[string]int64 `json:"depth"`
}
type Weapon struct {
	Weapon         string          `json:"weapon"`
	Name           string          `json:"name"`
	Modes          map[string]Mode `json:"modes"`
	SkillDeepenPPM int64           `json:"skill_deepen_ppm"`
}
type Technique struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Mechanism    string           `json:"mechanism"`
	Zone         string           `json:"zone"`
	Angle        string           `json:"angle"`
	Depth        map[string]int64 `json:"depth"`
	Knockout     *Span            `json:"knockout,omitempty"`
	Wake         *Span            `json:"wake,omitempty"`
	SafeAfterKOS int64            `json:"safe_after_ko_s,omitempty"`
	DeathPPMPerS int64            `json:"death_ppm_per_s,omitempty"`
	ArrestPPM    int64            `json:"arrest_ppm,omitempty"`
}
type Armor struct {
	Item  string           `json:"item"`
	Zones []string         `json:"zones"`
	Stop  map[string]int64 `json:"stop"`
	Shift map[string]int64 `json:"shift"`
}
type AidKind struct {
	Name        string              `json:"name"`
	Seconds     int64               `json:"seconds"`
	Held        bool                `json:"held"`
	Zones       []string            `json:"zones"`
	Reduce      map[string]int64    `json:"reduce"`
	LimbInside  bool                `json:"limb_inside"`
	LimbSafeS   int64               `json:"limb_safe_s,omitempty"`
	LimbLostS   int64               `json:"limb_lost_s,omitempty"`
	LimbEffects map[string][]string `json:"limb_effects,omitempty"`
	LimbLostFx  map[string][]string `json:"limb_lost_effects,omitempty"`
	Ease        []string            `json:"ease"`
}
type AidItem struct {
	Item       string `json:"item"`
	Kind       string `json:"kind"`
	SuccessPPM int64  `json:"success_ppm"`
	Improvised bool   `json:"improvised"`
}
type Medkit struct {
	Item       string   `json:"item"`
	Level      int      `json:"level"`
	SuccessPPM int64    `json:"success_ppm"`
	MaxDepth   string   `json:"max_depth"`
	Treats     []string `json:"treats"`
	Tourniquet bool     `json:"tourniquet"`
	Packing    bool     `json:"packing"`
	Splint     bool     `json:"splint"`
	PainOff    bool     `json:"pain_off"`
	PainOffS   int64    `json:"pain_off_s"`
	InfuseML   int64    `json:"infuse_ml"`
}
type AidSkill struct {
	Skill       string `json:"skill"`
	PerPointPPM int64  `json:"per_point_ppm"`
	CapPPM      int64  `json:"cap_ppm"`
}
type Aid struct {
	Kinds   map[string]AidKind `json:"kinds"`
	Items   []AidItem          `json:"items"`
	Medkits []Medkit           `json:"medkits"`
	Skill   AidSkill           `json:"skill"`
}
type Price struct {
	BaseMinor        int64 `json:"base_minor"`
	PerSeverityMinor int64 `json:"per_severity_minor"`
	PerHourMinor     int64 `json:"per_hour_minor"`
	StabilizeMinor   int64 `json:"stabilize_minor"`
}
type Stabilize struct {
	Min               int64 `json:"min"`
	RestoreToPermille int64 `json:"restore_to_permille"`
}

// Provider treats in world time: the capsule (paid CRD, own chip), the medblok (system trust) and the grey healer.
type Provider struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Operator         string    `json:"operator"`
	Places           []string  `json:"places"`
	Chip             string    `json:"chip"`
	TrustMinPermille int64     `json:"trust_min_permille"`
	Price            Price     `json:"price"`
	TimePermille     int64     `json:"time_permille"`
	BloodMinPer100ML int64     `json:"blood_min_per_100ml"`
	Stabilize        Stabilize `json:"stabilize"`
	Cures            []string  `json:"cures"`
	RefusesKinds     []string  `json:"refuses_kinds"`
	Journal          []string  `json:"journal"`
	ComplicationPPM  int64     `json:"complication_ppm"`
	Complication     string    `json:"complication,omitempty"`
}
type ConditionDef struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Onset      Span     `json:"onset"`
	Effects    []string `json:"effects"`
	Doom       *Span    `json:"doom,omitempty"`
	Severity   int      `json:"severity"`
	CapsuleMin int64    `json:"capsule_min"`
	HealDays   int64    `json:"heal_days"`
	Sign       string   `json:"sign"`
}
type Resolve struct {
	SeriousSeverity int                         `json:"serious_severity"`
	Reactions       map[string]map[string]int64 `json:"reactions"`
	CourageBase     int64                       `json:"courage_base"`
}
type Justice struct {
	Death   string `json:"death"`
	Serious string `json:"serious"`
	Hit     string `json:"hit"`
	Hero    string `json:"hero"`
}
type Route struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	MM   int64  `json:"mm"`
}

// When is a window of seconds or «never» (scenario expectations).
type When struct {
	Never bool
	MinS  int64
	MaxS  int64
}

func (w *When) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		if s != "never" {
			return fmt.Errorf("when: %q", s)
		}
		*w = When{Never: true}
		return nil
	}
	var r struct {
		MinS int64 `json:"min_s"`
		MaxS int64 `json:"max_s"`
	}
	d := json.NewDecoder(bytesReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return err
	}
	*w = When{MinS: r.MinS, MaxS: r.MaxS}
	return nil
}
func (w When) MarshalJSON() ([]byte, error) {
	if w.Never {
		return json.Marshal("never")
	}
	return json.Marshal(map[string]int64{"min_s": w.MinS, "max_s": w.MaxS})
}

// Holds reports whether a moment (-1 = never) is inside the window.
func (w When) Holds(at int64) bool {
	if w.Never {
		return at < 0
	}
	return at >= w.MinS && at <= w.MaxS
}

type ScenarioHit struct {
	Weapon     string   `json:"weapon,omitempty"`
	Mechanism  string   `json:"mechanism,omitempty"`
	Mode       string   `json:"mode"`
	Zone       string   `json:"zone"`
	Depth      string   `json:"depth"`
	Structures []string `json:"structures"`
}
type ScenarioTechnique struct {
	ID     string `json:"id"`
	Weapon string `json:"weapon,omitempty"`
	HoldS  int64  `json:"hold_s,omitempty"`
}
type ScenarioAid struct {
	AtS  int64  `json:"at_s"`
	Kind string `json:"kind"`
	Item string `json:"item"`
}
type Expect struct {
	Down      When `json:"down"`
	Dead      When `json:"dead"`
	AidedDead When `json:"aided_dead"`
}
type Scenario struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Chip      string             `json:"chip"`
	Hit       *ScenarioHit       `json:"hit,omitempty"`
	Technique *ScenarioTechnique `json:"technique,omitempty"`
	Condition string             `json:"condition,omitempty"`
	Aid       []ScenarioAid      `json:"aid"`
	RiskRuns  int                `json:"risk_runs"`
	Expect    Expect             `json:"expect"`
}

// Model is the compiled catalog world.wounds.
type Model struct {
	Version    int                         `json:"version"`
	Difficulty Difficulty                  `json:"difficulty"`
	Blood      Blood                       `json:"blood"`
	Pressure   Pressure                    `json:"pressure"`
	States     States                      `json:"states"`
	Effects    map[string]EffectDef        `json:"effects"`
	Walk       Walk                        `json:"walk"`
	Mechanisms map[string]Mechanism        `json:"mechanisms"`
	Chips      map[string]Chip             `json:"chips"`
	Zones      []Zone                      `json:"zones"`
	Aim        map[string]map[string]int64 `json:"aim"`
	Angles     map[string]map[string]int64 `json:"angles"`
	Weapons    []Weapon                    `json:"weapons"`
	Techniques []Technique                 `json:"techniques"`
	Armor      []Armor                     `json:"armor"`
	Aid        Aid                         `json:"aid"`
	Providers  []Provider                  `json:"providers"`
	Conditions []ConditionDef              `json:"conditions"`
	Resolve    Resolve                     `json:"resolve"`
	Justice    Justice                     `json:"justice"`
	Routes     []Route                     `json:"routes"`
	Scenarios  []Scenario                  `json:"scenarios"`
}

var depthRank = map[string]int{"graze": 0, "shallow": 1, "deep": 2, "through": 3}
var depthNames = []string{"graze", "shallow", "deep", "through"}
var kinds = map[string]bool{"capillary": true, "vein": true, "deep_vein": true, "artery": true, "critical": true, "organ": true, "bone": true, "nerve": true, "brain": true}
var signs = map[string]bool{"none": true, "blood": true, "blood_flow": true, "blood_spurt": true, "limp": true, "clutch": true, "gasp": true, "still": true, "bruise": true}

// Zone returns the zone by id.
func (m Model) Zone(id string) (Zone, bool) {
	for _, z := range m.Zones {
		if z.ID == id {
			return z, true
		}
	}
	return Zone{}, false
}

// Structure returns a structure and its zone.
func (m Model) Structure(id string) (Structure, Zone, bool) {
	for _, z := range m.Zones {
		for _, s := range z.Structures {
			if s.ID == id {
				return s, z, true
			}
		}
	}
	return Structure{}, Zone{}, false
}
func (m Model) Weapon(id string) (Weapon, bool) {
	for _, w := range m.Weapons {
		if w.Weapon == id {
			return w, true
		}
	}
	return Weapon{}, false
}
func (m Model) Technique(id string) (Technique, bool) {
	for _, t := range m.Techniques {
		if t.ID == id {
			return t, true
		}
	}
	return Technique{}, false
}
func (m Model) Condition(id string) (ConditionDef, bool) {
	for _, c := range m.Conditions {
		if c.ID == id {
			return c, true
		}
	}
	return ConditionDef{}, false
}
func (m Model) Provider(id string) (Provider, bool) {
	for _, p := range m.Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}
func (m Model) Medkit(item string) (Medkit, bool) {
	for _, k := range m.Aid.Medkits {
		if k.Item == item {
			return k, true
		}
	}
	return Medkit{}, false
}
func (m Model) AidItem(item, kind string) (AidItem, bool) {
	for _, a := range m.Aid.Items {
		if a.Item == item && a.Kind == kind {
			return a, true
		}
	}
	return AidItem{}, false
}

// Validate is the structural gate of the catalog (content rules against the world: compiler/wounds.Check).
func (m Model) Validate() error {
	if m.Version != 1 {
		return fmt.Errorf("wounds: unsupported version %d", m.Version)
	}
	d := m.Difficulty
	if d.ID == "" || d.BleedPermille < 100 || d.BleedPermille > 1000 || d.DoomPermille < 1000 || d.AidPermille < 1000 || d.PricePermille < 0 {
		return fmt.Errorf("wounds: invalid difficulty")
	}
	if m.Blood.VolumeML < 1000 || m.Blood.StepS != 1 || m.Blood.RegenMLH < 0 || m.Blood.StoppedBelowMLMin < 1 {
		return fmt.Errorf("wounds: invalid blood (the step is one second)")
	}
	if m.Pressure.FloorPermille < 1 || m.Pressure.FloorPermille > 1000 || m.Pressure.SlopePermille < 0 {
		return fmt.Errorf("wounds: invalid pressure")
	}
	s := m.States
	if !(0 < s.WeakPermille && s.WeakPermille < s.ConfusedPermille && s.ConfusedPermille < s.DownPermille && s.DownPermille < s.DeadPermille && s.DeadPermille <= 1000) {
		return fmt.Errorf("wounds: loss thresholds must rise: weak < confused < down < dead")
	}
	if s.WakeBelowPermille <= s.ConfusedPermille || s.WakeBelowPermille > s.DownPermille {
		return fmt.Errorf("wounds: wake threshold must lie between confused and down")
	}
	for _, e := range []string{"pain", "drop_weapon", "limp", "no_walk", "breathless", "concussion"} {
		if _, ok := m.Effects[e]; !ok {
			return fmt.Errorf("wounds: effect %s undefined", e)
		}
	}
	if m.Walk.NormalMMS < 1 {
		return fmt.Errorf("wounds: invalid walk")
	}
	if _, ok := m.Chips["none"]; !ok {
		return fmt.Errorf("wounds: chip profile none (the unchipped) is required")
	}
	for id, c := range m.Chips {
		if c.ClotPermille < 100 || c.RegenPermille < 0 {
			return fmt.Errorf("wounds: chip %s invalid", id)
		}
	}
	seen := map[string]bool{}
	for _, z := range m.Zones {
		if z.Tourniquet && z.Junctional {
			return fmt.Errorf("wounds: zone %s is both a limb and a junction", z.ID)
		}
		if _, ok := m.Aim[z.Part]; !ok {
			return fmt.Errorf("wounds: zone %s part %s has no aim", z.ID, z.Part)
		}
		for _, st := range z.Structures {
			if seen[st.ID] {
				return fmt.Errorf("wounds: duplicate structure %s", st.ID)
			}
			seen[st.ID] = true
			if !kinds[st.Kind] || !signs[st.Sign] {
				return fmt.Errorf("wounds: structure %s kind/sign", st.ID)
			}
			if _, ok := depthRank[st.MinDepth]; !ok {
				return fmt.Errorf("wounds: structure %s depth", st.ID)
			}
			for mech, p := range st.Exposure {
				if _, ok := m.Mechanisms[mech]; !ok || p < 0 || p > Million {
					return fmt.Errorf("wounds: structure %s exposure %s", st.ID, mech)
				}
			}
			for _, e := range st.Effects {
				if _, ok := m.Effects[e]; !ok {
					return fmt.Errorf("wounds: structure %s effect %s", st.ID, e)
				}
			}
			if st.Severity < 1 || st.HealDays < 1 || st.CapsuleMin < 1 || st.BleedMLMin < 0 || st.ClotHalfS < 0 || st.LethalPPM < 0 || st.LethalPPM > Million {
				return fmt.Errorf("wounds: structure %s numbers", st.ID)
			}
			for _, sp := range []*Span{st.Knockout, st.KnockoutFor, st.Doom} {
				if sp != nil && (sp.Min < 0 || sp.Max < sp.Min) {
					return fmt.Errorf("wounds: structure %s span", st.ID)
				}
			}
		}
	}
	for part, zs := range m.Aim {
		total := int64(0)
		for z, w := range zs {
			if _, ok := m.Zone(z); !ok || w < 1 {
				return fmt.Errorf("wounds: aim %s zone %s", part, z)
			}
			total += w
		}
		if total < 1 {
			return fmt.Errorf("wounds: aim %s empty", part)
		}
	}
	for angle, zs := range m.Angles {
		for z := range zs {
			if _, ok := m.Zone(z); !ok {
				return fmt.Errorf("wounds: angle %s zone %s", angle, z)
			}
		}
	}
	for _, w := range m.Weapons {
		if _, ok := w.Modes["*"]; !ok {
			return fmt.Errorf("wounds: weapon %s has no default mode", w.Weapon)
		}
		for mode, md := range w.Modes {
			if err := m.validDepths(md.Mechanism, md.Depth); err != nil {
				return fmt.Errorf("wounds: weapon %s mode %s: %w", w.Weapon, mode, err)
			}
		}
	}
	for _, t := range m.Techniques {
		if _, ok := m.Zone(t.Zone); !ok {
			return fmt.Errorf("wounds: technique %s zone", t.ID)
		}
		if err := m.validDepths(t.Mechanism, t.Depth); err != nil {
			return fmt.Errorf("wounds: technique %s: %w", t.ID, err)
		}
		if t.Mechanism == "choke" && (t.Knockout == nil || t.Wake == nil) {
			return fmt.Errorf("wounds: technique %s needs knockout and wake", t.ID)
		}
	}
	for _, a := range m.Armor {
		for _, z := range a.Zones {
			if _, ok := m.Zone(z); !ok {
				return fmt.Errorf("wounds: armor %s zone %s", a.Item, z)
			}
		}
	}
	for _, k := range []string{"pressure", "tourniquet", "packing", "dressing", "seal", "medkit"} {
		a, ok := m.Aid.Kinds[k]
		if !ok || a.Seconds < 1 || len(a.Zones) == 0 {
			return fmt.Errorf("wounds: aid kind %s", k)
		}
		for _, z := range a.Zones {
			zone, ok := m.Zone(z)
			if !ok || k == "tourniquet" && !zone.Tourniquet {
				return fmt.Errorf("wounds: aid %s zone %s", k, z)
			}
		}
		for kind, r := range a.Reduce {
			if !kinds[kind] || r < 0 || r > 1000 {
				return fmt.Errorf("wounds: aid %s reduce %s", k, kind)
			}
		}
	}
	for _, it := range m.Aid.Items {
		if _, ok := m.Aid.Kinds[it.Kind]; !ok || it.Kind == "pressure" || it.Kind == "medkit" || it.SuccessPPM < 0 || it.SuccessPPM > Million {
			return fmt.Errorf("wounds: aid item %s", it.Item)
		}
	}
	for _, k := range m.Aid.Medkits {
		if _, ok := depthRank[k.MaxDepth]; !ok || k.Level < 1 || k.SuccessPPM > Million {
			return fmt.Errorf("wounds: medkit %s", k.Item)
		}
		for _, t := range k.Treats {
			if !kinds[t] || t == "organ" || t == "brain" || t == "critical" || t == "bone" {
				return fmt.Errorf("wounds: medkit %s cannot treat %s", k.Item, t)
			}
		}
	}
	conds := map[string]bool{}
	for _, c := range m.Conditions {
		conds[c.ID] = true
		if c.Severity < 1 || c.CapsuleMin < 1 || c.HealDays < 1 || c.Onset.Max < c.Onset.Min {
			return fmt.Errorf("wounds: condition %s", c.ID)
		}
	}
	for id, mech := range m.Mechanisms {
		if mech.Condition != "" && !conds[mech.Condition] {
			return fmt.Errorf("wounds: mechanism %s condition %s", id, mech.Condition)
		}
	}
	for _, p := range m.Providers {
		if p.TimePermille < 100 || p.Stabilize.Min < 1 || len(p.Cures) == 0 || p.Chip != "own" && p.Chip != "any" {
			return fmt.Errorf("wounds: provider %s", p.ID)
		}
		if p.Complication != "" && !conds[p.Complication] {
			return fmt.Errorf("wounds: provider %s complication", p.ID)
		}
	}
	if _, ok := m.Provider("capsule"); !ok {
		return fmt.Errorf("wounds: the capsule provider is required")
	}
	for resp, ws := range m.Resolve.Reactions {
		total := int64(0)
		for r, w := range ws {
			if r != "fight_on" && r != "flee" && r != "fall" && r != "surrender" || w < 0 {
				return fmt.Errorf("wounds: reaction %s/%s", resp, r)
			}
			total += w
		}
		if total < 1 {
			return fmt.Errorf("wounds: reactions of %s empty", resp)
		}
	}
	if m.Justice.Death == "" || m.Justice.Hero == "" {
		return fmt.Errorf("wounds: justice hooks")
	}
	ids := map[string]bool{}
	for _, sc := range m.Scenarios {
		if ids[sc.ID] {
			return fmt.Errorf("wounds: duplicate scenario %s", sc.ID)
		}
		ids[sc.ID] = true
		if _, ok := m.Chips[sc.Chip]; !ok {
			return fmt.Errorf("wounds: scenario %s chip %s", sc.ID, sc.Chip)
		}
		if sc.Hit == nil && sc.Technique == nil && sc.Condition == "" {
			return fmt.Errorf("wounds: scenario %s does nothing", sc.ID)
		}
		if sc.Condition != "" && !conds[sc.Condition] {
			return fmt.Errorf("wounds: scenario %s condition", sc.ID)
		}
		if sc.Hit != nil {
			z, ok := m.Zone(sc.Hit.Zone)
			if !ok {
				return fmt.Errorf("wounds: scenario %s zone", sc.ID)
			}
			if sc.Hit.Weapon == "" && sc.Hit.Mechanism == "" {
				return fmt.Errorf("wounds: scenario %s needs a weapon or a mechanism", sc.ID)
			}
			for _, id := range sc.Hit.Structures {
				st, sz, ok := m.Structure(id)
				if !ok || sz.ID != z.ID || depthRank[st.MinDepth] > depthRank[sc.Hit.Depth] {
					return fmt.Errorf("wounds: scenario %s structure %s", sc.ID, id)
				}
			}
		}
		for _, a := range sc.Aid {
			if _, ok := m.Aid.Kinds[a.Kind]; !ok {
				return fmt.Errorf("wounds: scenario %s aid %s", sc.ID, a.Kind)
			}
		}
	}
	return nil
}

func (m Model) validDepths(mech string, depth map[string]int64) error {
	if _, ok := m.Mechanisms[mech]; !ok {
		return fmt.Errorf("mechanism %s", mech)
	}
	total := int64(0)
	for d, w := range depth {
		if _, ok := depthRank[d]; !ok || w < 0 {
			return fmt.Errorf("depth %s", d)
		}
		total += w
	}
	if total < 1 {
		return fmt.Errorf("no depth weight")
	}
	return nil
}

// sortedKeys keeps every map walk deterministic.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
