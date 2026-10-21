package gamepoc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/behavior"
)

// moralForbid mirrors behavior.MoralForbid; only "actor.emotion.<e> > const" is compiled.
type moralForbid struct {
	Action        string  `json:"action"`
	UnlessEmotion string  `json:"unless_emotion"`
	UnlessAbove   float64 `json:"unless_above"`
	HasUnless     bool    `json:"has_unless"`
}

// psycheData mirrors behavior.Psyche: everything derived by CUE (#VectorPsyche), nothing typed by hand.
type psycheData struct {
	Vectors        []string           `json:"vectors"`
	NeedBias       map[string]float64 `json:"need_bias"`
	EmotionGain    map[string]float64 `json:"emotion_gain"`
	ChipSuppress   map[string]float64 `json:"chip_suppress"`
	StressResponse string             `json:"stress_response"`
	Affinity       []string           `json:"affinity"`
	Aversion       []string           `json:"aversion"`
	Forbid         []moralForbid      `json:"forbid"`
	ProfileForbid  []string           `json:"profile_forbid"`
	Passionality   int                `json:"passionality"`
	Relations      map[string]float64 `json:"relations"`
}
type psycheLaw struct {
	AffinityMul float64 `json:"affinity_mul"`
	AversionMul float64 `json:"aversion_mul"`
	Stress      struct {
		Threshold   float64 `json:"threshold"`
		ResponseMul float64 `json:"response_mul"`
	} `json:"stress"`
}

// PsycheTable is embedded into world.json: the psyche law and one derived psyche per scene actor.
type PsycheTable struct {
	Law    psycheLaw             `json:"law"`
	Actors map[string]psycheData `json:"actors"`
}

type psycheCompiler struct{ root, world cue.Value }

// derive fills #VectorPsyche with the effective vectors and the archetype's caste layer,
// so the formula stays in CUE and ANG only reads the derived fields.
func (c psycheCompiler) derive(vectors []string, casteBias, adjust cue.Value) (psycheData, error) {
	var out psycheData
	if len(vectors) == 0 {
		return out, fmt.Errorf("psyche requires at least one vector")
	}
	v := c.root.LookupPath(cue.MakePath(cue.Def("#VectorPsyche")))
	if !v.Exists() {
		return out, fmt.Errorf("content/world must define #VectorPsyche")
	}
	v = v.FillPath(cue.ParsePath("vectors"), vectors)
	if casteBias.Exists() {
		v = v.FillPath(cue.ParsePath("caste_bias"), casteBias)
	}
	if adjust.Exists() {
		v = v.FillPath(cue.ParsePath("need_bias_adjust"), adjust)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return out, fmt.Errorf("psyche for %v: %w", vectors, err)
	}
	if err := v.Decode(&out); err != nil {
		return out, err
	}
	out.Vectors = vectors
	if out.ChipSuppress == nil {
		out.ChipSuppress = map[string]float64{}
	}
	return out, nil
}

func (c psycheCompiler) law() (psycheLaw, error) {
	var out psycheLaw
	v := c.world.LookupPath(cue.MakePath(cue.Str("laws"), cue.Str("law.psyche_vector_basic"), cue.Str("psyche")))
	if !v.Exists() {
		return out, fmt.Errorf("law.psyche_vector_basic is required for utility weights")
	}
	if err := v.Decode(&out); err != nil {
		return out, err
	}
	if out.AffinityMul <= 0 || out.AversionMul <= 0 || out.Stress.ResponseMul <= 0 {
		return out, fmt.Errorf("law.psyche_vector_basic multipliers must be positive")
	}
	return out, nil
}

// compileMoral keeps forbids as hard vetoes. Unsupported `unless` shapes fail the build
// instead of silently losing a rule.
func compileMoral(v cue.Value, where string) ([]moralForbid, error) {
	if !v.Exists() {
		return nil, nil
	}
	var rules []struct {
		Kind   string          `json:"kind"`
		Action string          `json:"action"`
		Unless json.RawMessage `json:"unless"`
	}
	if err := v.Decode(&rules); err != nil {
		return nil, fmt.Errorf("%s moral: %w", where, err)
	}
	var out []moralForbid
	for i, r := range rules {
		if r.Kind != "forbid" {
			continue
		}
		f := moralForbid{Action: r.Action}
		if len(r.Unless) > 0 && string(r.Unless) != "null" {
			var u struct {
				Op string `json:"op"`
				A  struct {
					Ref string `json:"ref"`
				} `json:"a"`
				B struct {
					Const *float64 `json:"const"`
				} `json:"b"`
			}
			if err := json.Unmarshal(r.Unless, &u); err != nil || u.Op != "gt" || !strings.HasPrefix(u.A.Ref, "actor.emotion.") || u.B.Const == nil {
				return nil, fmt.Errorf("%s moral[%d]: unsupported `unless` expression; only {op: gt, a: {ref: actor.emotion.<e>}, b: {const}} is compiled", where, i)
			}
			f.HasUnless, f.UnlessEmotion, f.UnlessAbove = true, strings.TrimPrefix(u.A.Ref, "actor.emotion."), *u.B.Const
		}
		out = append(out, f)
	}
	return out, nil
}

// enumValues lists the literals of a CUE string disjunction such as #ActionTag.
func enumValues(v cue.Value) []string {
	if !v.Exists() {
		return nil
	}
	op, args := v.Expr()
	if op != cue.OrOp {
		if s, err := v.String(); err == nil {
			return []string{s}
		}
		return nil
	}
	var out []string
	for _, a := range args {
		out = append(out, enumValues(a)...)
	}
	sort.Strings(out)
	return out
}

func (c psycheCompiler) vocabulary() *behavior.Vocabulary {
	return &behavior.Vocabulary{
		Needs:    enumValues(c.root.LookupPath(cue.MakePath(cue.Def("#Need")))),
		Emotions: enumValues(c.root.LookupPath(cue.MakePath(cue.Def("#Emotion")))),
		Actions:  enumValues(c.root.LookupPath(cue.MakePath(cue.Def("#ActionTag")))),
	}
}

// actorPsyche derives one scene actor: a person via its archetype (caste layer, moral, chip,
// passionality, person overrides) or a demo agent that only declares vectors.
func (c psycheCompiler) actorPsyche(archetype string, personOverrides, personRelations cue.Value, vectors []string, profileForbid []string) (psycheData, error) {
	var casteBias, adjust, moral, chip cue.Value
	var out psycheData
	passionality := 0
	if archetype != "" {
		arch := c.world.LookupPath(cue.MakePath(cue.Str("archetypes"), cue.Str(archetype)))
		if !arch.Exists() {
			return psycheData{}, fmt.Errorf("unknown archetype %s", archetype)
		}
		if len(vectors) == 0 {
			if err := arch.LookupPath(cue.ParsePath("vectors")).Decode(&vectors); err != nil {
				return psycheData{}, err
			}
		}
		casteBias, adjust, moral, chip = arch.LookupPath(cue.ParsePath("caste_bias")), arch.LookupPath(cue.ParsePath("need_bias_adjust")), arch.LookupPath(cue.ParsePath("moral")), arch.LookupPath(cue.ParsePath("chip_suppress"))
		if p, err := arch.LookupPath(cue.ParsePath("passionality")).Int64(); err == nil {
			passionality = int(p)
		}
		if personOverrides.Exists() {
			if m := personOverrides.LookupPath(cue.ParsePath("moral")); m.Exists() {
				moral = m
			}
			if p, err := personOverrides.LookupPath(cue.ParsePath("passionality")).Int64(); err == nil {
				passionality = int(p)
			}
		}
		if personRelations.Exists() {
			var relations []struct {
				To       string  `json:"to"`
				Kind     string  `json:"kind"`
				Strength float64 `json:"strength"`
			}
			if err := personRelations.Decode(&relations); err != nil {
				return psycheData{}, err
			}
			for _, rel := range relations {
				if rel.Strength > 0 && (rel.Kind == "trust" || rel.Kind == "mentorship") {
					if out.Relations == nil {
						out.Relations = map[string]float64{}
					}
					out.Relations[rel.To] = rel.Strength
				}
			}
		}
	}
	derived, err := c.derive(vectors, casteBias, adjust)
	if err != nil {
		return derived, err
	}
	derived.Relations = out.Relations
	out = derived
	if chip.Exists() {
		if err := chip.Decode(&out.ChipSuppress); err != nil {
			return out, err
		}
	}
	if out.Forbid, err = compileMoral(moral, archetype); err != nil {
		return out, err
	}
	out.Passionality = passionality
	out.ProfileForbid = append([]string(nil), profileForbid...)
	return out, nil
}

// utilityFixtures resolves scenario cases (vectors, forbid_actions, moral) into the psyche the
// generated tests consume, keeping the derivation in CUE.
func (c psycheCompiler) utilityFixtures(scenarios cue.Value, law psycheLaw) ([]byte, error) {
	cases := scenarios.LookupPath(cue.ParsePath("utility_cases"))
	out := map[string]any{"law": law, "cases": []any{}}
	if !cases.Exists() {
		return json.Marshal(out)
	}
	iter, err := cases.List()
	if err != nil {
		return nil, err
	}
	var resolved []any
	for iter.Next() {
		item := iter.Value()
		var raw map[string]any
		if err := item.Decode(&raw); err != nil {
			return nil, err
		}
		var vectors, forbid []string
		if err := item.LookupPath(cue.ParsePath("vectors")).Decode(&vectors); err != nil {
			return nil, err
		}
		_ = item.LookupPath(cue.ParsePath("forbid_actions")).Decode(&forbid)
		psyche, err := c.derive(vectors, cue.Value{}, cue.Value{})
		if err != nil {
			return nil, fmt.Errorf("utility case %v: %w", raw["id"], err)
		}
		if psyche.Forbid, err = compileMoral(item.LookupPath(cue.ParsePath("moral")), fmt.Sprint(raw["id"])); err != nil {
			return nil, err
		}
		psyche.ProfileForbid = forbid
		raw["psyche"] = psyche
		resolved = append(resolved, raw)
	}
	out["cases"] = resolved
	return json.Marshal(out)
}

// catalogPosition maps a validator path such as goals."goal.x".considerations[0].input to the
// CUE source position of the nearest existing ancestor, for file:line diagnostics.
func catalogPosition(ai cue.Value, path string) string {
	var selectors []cue.Selector
	rest := path
	for rest != "" {
		switch {
		case rest[0] == '"':
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				return ""
			}
			selectors = append(selectors, cue.Str(rest[1:1+end]))
			rest = strings.TrimPrefix(rest[2+end:], ".")
		case rest[0] == '[':
			end := strings.Index(rest, "]")
			if end < 0 {
				return ""
			}
			index, err := strconv.Atoi(rest[1:end])
			if err != nil {
				return ""
			}
			selectors = append(selectors, cue.Index(index))
			rest = strings.TrimPrefix(rest[end+1:], ".")
		default:
			end := strings.IndexAny(rest, ".[")
			if end < 0 {
				selectors = append(selectors, cue.Str(rest))
				rest = ""
			} else {
				selectors = append(selectors, cue.Str(rest[:end]))
				rest = strings.TrimPrefix(rest[end:], ".")
			}
		}
	}
	for len(selectors) > 0 {
		v := ai.LookupPath(cue.MakePath(selectors...))
		if v.Exists() {
			if pos := v.Pos(); pos.IsValid() {
				return pos.String()
			}
		}
		selectors = selectors[:len(selectors)-1]
	}
	return ""
}
