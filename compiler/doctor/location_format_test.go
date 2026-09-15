package doctor

import "testing"

func TestParseFSMLocationReadsEveryLogFormat(t *testing.T) {
	message := "Entity 'Order' FSM transition 'paid→shipped' references undefined state 'paid'"
	for name, log := range map[string]string{
		"warning block": "⚠️  ERROR [E_FSM_UNDEFINED_STATE]: " + message + "\n   at cue/domain/order.cue:3:0\n",
		"error line":    "❌ cue/domain/order.cue:3:1: ERROR [E_FSM_UNDEFINED_STATE]: " + message + "\n",
		"json event":    `{"stage":"diagnostic","status":"error","message":"` + message + `","code":"E_FSM_UNDEFINED_STATE","cueFile":"cue/domain/order.cue","line":3,"column":1}`,
	} {
		path, line, entity, state := parseFSMLocation(log)
		if path != "cue/domain/order.cue" || line != 3 || entity != "Order" || state != "paid" {
			t.Errorf("%s: path=%q line=%d entity=%q state=%q", name, path, line, entity, state)
		}
	}
}
