package gamepoc

import "testing"

func TestCombatPaceV2Budgets(t *testing.T) {
	good := CombatPaceV2{NormalPhaseS: 5, FastPhaseS: 2.5, FastMoveMaxS: .2, InputBacklogS: .3, NormalTurnS: 1.2, FastTurnS: .6, MoveMPS: 3.8, MoveMaxS: .4, HoldMultiplier: 2.5, HoldAction: "combat_speed", CameraMargin: .08, StrikeImpactS: .45}
	p := paceOK()
	p.V2 = &good
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*CombatPaceV2){
		"phase deadline": func(v *CombatPaceV2) { v.FastPhaseS = 2.6 },
		"normal phase":   func(v *CombatPaceV2) { v.NormalTurnS = 1.3 }, "fast phase": func(v *CombatPaceV2) { v.FastTurnS = .7 },
		"fast movement cap": func(v *CombatPaceV2) { v.FastMoveMaxS = v.MoveMaxS + .01 },
		"movement":          func(v *CombatPaceV2) { v.MoveMPS = 0 }, "movement cap": func(v *CombatPaceV2) { v.MoveMaxS = 0 },
		"speed hold": func(v *CombatPaceV2) { v.HoldMultiplier = 4 }, "binding": func(v *CombatPaceV2) { v.HoldAction = "" },
		"camera": func(v *CombatPaceV2) { v.CameraMargin = .5 }, "impact": func(v *CombatPaceV2) { v.StrikeImpactS = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			v := good
			change(&v)
			p.V2 = &v
			if p.validate() == nil {
				t.Fatal("invalid pace accepted")
			}
		})
	}
}
