package flowfn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandFragments(t *testing.T) {
	prog, err := Parse(`
fragment guard {
  session.Get(output: session)
  quota.Check(key: session, limit: 10, window: "day")
}
use guard
openai.Chat(user_message: req.message, output: reply)
`)
	require.NoError(t, err)

	expanded, err := ExpandFragments(prog)
	require.NoError(t, err)
	require.Len(t, expanded.Nodes, 3)

	first, ok := expanded.Nodes[0].(*CallNode)
	require.True(t, ok)
	require.Equal(t, "session.Get", first.Action)
}
