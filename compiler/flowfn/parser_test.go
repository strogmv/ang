package flowfn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseProgram(t *testing.T) {
	src := `
fragment guard {
  session.Get(output: session)
  quota.Check(key: session, limit: 50, window: "day")
  budget.Check(key: session, limit: 1000)
}

use guard
if req.enabled {
  openai.Chat(user_message: req.message, output: reply)
} else {
  mapping.Assign(to: status, value: "skipped")
}
for item in req.items {
  repo.Save(input: item)
}
try {
  tx.Block {
    db.Lock(output: row)
  }
} catch {
  event.Publish(name: "failed")
}
`
	prog, err := Parse(src)
	require.NoError(t, err)
	require.Len(t, prog.Nodes, 5)

	frag, ok := prog.Nodes[0].(*FragmentNode)
	require.True(t, ok)
	require.Equal(t, "guard", frag.Name)

	call, ok := frag.Body[0].(*CallNode)
	require.True(t, ok)
	require.Equal(t, "session.Get", call.Action)
	require.Equal(t, 3, call.Pos.Line)

	ifNode, ok := prog.Nodes[2].(*IfNode)
	require.True(t, ok)
	require.Equal(t, "req.enabled", ifNode.Condition)
	require.Len(t, ifNode.Then, 1)
	require.Len(t, ifNode.Else, 1)

	forNode, ok := prog.Nodes[3].(*ForNode)
	require.True(t, ok)
	require.Equal(t, "item", forNode.Alias)
	require.Equal(t, "req.items", forNode.Each)

	tryNode, ok := prog.Nodes[4].(*TryNode)
	require.True(t, ok)
	require.Len(t, tryNode.Do, 1)
	require.Len(t, tryNode.Catch, 1)
}
