package gamepoc

import (
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const psycheFixture = `
import "list"
#Need: "safety" | "order"
#ActionTag: "flee" | "work"
_table: {
	skin: {need_weights: {safety: 1, order: 1.2}, emotion_gain: {fear: 1.1}, stress_response: "flight", affinity: ["work"], aversion: []}
	anal: {need_weights: {safety: 1, order: 1.7}, emotion_gain: {fear: 1}, stress_response: "freeze", affinity: ["work"], aversion: ["travel"]}
}
#VectorPsyche: {
	vectors: [...string]
	caste_bias: [string]: number
	need_bias_adjust: [string]: number
	need_bias: {for n in ["safety", "order"] {"\(n)": (_table[vectors[0]].need_weights[n] + _table[vectors[1]].need_weights[n]) / 2 * [for k, a in caste_bias if k == n {a}, 1][0]}}
	emotion_gain: {fear: (_table[vectors[0]].emotion_gain.fear + _table[vectors[1]].emotion_gain.fear) / 2}
	stress_response: _table[vectors[0]].stress_response
	affinity: list.Concat([_table[vectors[0]].affinity, _table[vectors[1]].affinity])
	aversion: list.Concat([_table[vectors[0]].aversion, _table[vectors[1]].aversion])
}
world: {
	laws: "law.psyche_vector_basic": psyche: {affinity_mul: 1.25, aversion_mul: 0.8, stress: {threshold: 0.7, response_mul: 1.5}}
	archetypes: "arch.a": {vectors: ["skin", "anal"], caste_bias: {order: 1.3}, chip_suppress: {fear: 0.1}, passionality: 1, moral: [{kind: "forbid", action: "flee", unless: {op: "gt", a: {ref: "actor.emotion.fear"}, b: {const: 0.9}}}]}
	archetypes: "arch.bad": {vectors: ["skin", "anal"], moral: [{kind: "forbid", action: "flee", unless: {op: "and", args: [{const: 1}, {const: 1}]}}]}
	ai: goals: "goal.scene.flee": {id: "goal.scene.flee", considerations: [{input: "signal.x", curve: "curve.linear"}]}
}
`

func fixtureRoot(t *testing.T) cue.Value {
	t.Helper()
	v := cuecontext.New().CompileString(psycheFixture, cue.Filename("psyche_fixture.cue"))
	if err := v.Err(); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestPsycheIsDerivedByCUEAndMoralUnlessIsCompiledOrRejected(t *testing.T) {
	root := fixtureRoot(t)
	c := psycheCompiler{root: root, world: root.LookupPath(cue.ParsePath("world"))}
	law, err := c.law()
	if err != nil || law.AffinityMul != 1.25 || law.Stress.Threshold != 0.7 {
		t.Fatalf("law not read from CUE: %+v %v", law, err)
	}
	p, err := c.actorPsyche("arch.a", cue.Value{}, nil, []string{"flee"})
	if err != nil {
		t.Fatal(err)
	}
	if p.NeedBias["order"] != (1.2+1.7)/2*1.3 || p.EmotionGain["fear"] != 1.05 || p.StressResponse != "flight" || p.ChipSuppress["fear"] != 0.1 || p.Passionality != 1 || len(p.Affinity) != 2 {
		t.Fatalf("derived fields wrong: %+v", p)
	}
	if len(p.Forbid) != 1 || p.Forbid[0].Action != "flee" || !p.Forbid[0].HasUnless || p.Forbid[0].UnlessEmotion != "fear" || p.Forbid[0].UnlessAbove != 0.9 || p.ProfileForbid[0] != "flee" {
		t.Fatalf("moral not compiled: %+v", p.Forbid)
	}
	demo, err := c.actorPsyche("", cue.Value{}, []string{"anal", "skin"}, nil)
	if err != nil || demo.StressResponse != "freeze" || demo.NeedBias["order"] != (1.2+1.7)/2 || len(demo.Forbid) != 0 {
		t.Fatalf("vector-only psyche wrong: %+v %v", demo, err)
	}
	if _, err := c.actorPsyche("arch.bad", cue.Value{}, nil, nil); err == nil || !strings.Contains(err.Error(), "unsupported `unless`") {
		t.Fatalf("unsupported moral expression was not rejected: %v", err)
	}
	if _, err := c.actorPsyche("arch.missing", cue.Value{}, nil, nil); err == nil {
		t.Fatal("unknown archetype accepted")
	}
	vocab := c.vocabulary()
	if strings.Join(vocab.Needs, ",") != "order,safety" || strings.Join(vocab.Actions, ",") != "flee,work" {
		t.Fatalf("vocabulary not enumerated from CUE disjunctions: %+v", vocab)
	}
}

func TestCatalogErrorsCarrySourcePositions(t *testing.T) {
	root := fixtureRoot(t)
	ai := root.LookupPath(cue.ParsePath("world.ai"))
	pos := catalogPosition(ai, `goals."goal.scene.flee".considerations[0].input`)
	if !strings.HasPrefix(pos, "psyche_fixture.cue:") {
		t.Fatalf("no file:line for a consideration: %q", pos)
	}
	parent := catalogPosition(ai, `goals."goal.scene.flee".considerations[7].curve`)
	if !strings.HasPrefix(parent, "psyche_fixture.cue:") {
		t.Fatalf("missing element did not fall back to its parent position: %q", parent)
	}
	if catalogPosition(ai, `nothing.here`) != "" {
		t.Fatal("invented a position")
	}
}
