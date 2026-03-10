package flowfn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTranspileProgram(t *testing.T) {
	steps, err := ParseTranspile(`
fragment guard {
  session.Get(output: session)
  quota.Check(key: session, limit: 10, window: "day")
  budget.Check(key: session, limit: 1000)
}
use guard
if req.ok {
  tx.Block {
    db.Lock(output: row)
  }
} else {
  event.Publish(name: "skip")
}
try {
  openai.Chat(user_message: req.message, output: reply)
} catch {
  mapping.Assign(to: status, value: "fallback")
}
`)
	require.NoError(t, err)
	require.Len(t, steps, 5)
	require.Equal(t, "session.Get", steps[0].Action)
	require.Equal(t, "flow.If", steps[3].Action)
	require.Equal(t, "flow.Try", steps[4].Action)

	thenSteps, ok := steps[3].Args["_then"].([]Step)
	require.True(t, ok)
	require.Len(t, thenSteps, 1)
	require.Equal(t, "tx.Block", thenSteps[0].Action)
}
