package gamepoc

import (
	"cuelang.org/go/cue/cuecontext"
	"testing"
)

func TestMarketFinishResolvesLayoutInstances(t *testing.T) {
	c, _ := fixturesStand(t, nil)
	ctx := cuecontext.New()
	check := func(instance string, valid bool) {
		t.Helper()
		v := ctx.CompileString("market_finish: {layout: \"layout.hall\", rules: [{instances: [\"" + instance + "\"], mesh: \"wall_head\", role: \"hide\", reason: \"cutaway\"}]}")
		finish, err := readMarketFinish(v, t.TempDir(), *c)
		if (err == nil) != valid || valid && (finish == nil || len(finish.Rules) != 1) {
			t.Errorf("instance %q: %+v, %v", instance, finish, err)
		}
	}
	check("loop.1", true)
	check("removed.1", false)
}
