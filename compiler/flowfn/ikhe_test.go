package flowfn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProgramMissingPrerequisites(t *testing.T) {
	prog, err := ParseAndExpand(`
openai.Chat(user_message: req.message, output: reply)
`)
	require.NoError(t, err)
	diags := ValidateProgram(prog)
	require.Len(t, diags, 2)
	require.Equal(t, "MISSING_EFFECT_PREREQUISITE", diags[0].Code)
}

func TestValidateProgramExternalEffectInTx(t *testing.T) {
	prog, err := ParseAndExpand(`
tx.Block {
  session.Get(output: session)
  quota.Check(key: session, limit: 10, window: "day")
  budget.Check(key: session, limit: 1000)
  openai.Chat(user_message: req.message, output: reply)
}
`)
	require.NoError(t, err)
	diags := ValidateProgram(prog)
	require.Len(t, diags, 1)
	require.Equal(t, "EXTERNAL_EFFECT_IN_TX", diags[0].Code)
}

func TestParseValidateTranspile(t *testing.T) {
	steps, diags, err := ParseValidateTranspile(`
session.Get(output: session)
quota.Check(key: session, limit: 10, window: "day")
budget.Check(key: session, limit: 1000)
openai.Chat(user_message: req.message, output: reply)
`)
	require.NoError(t, err)
	require.Empty(t, diags)
	require.Len(t, steps, 4)
}
