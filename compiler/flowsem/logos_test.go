package flowsem

import "testing"

func TestLookupLogos_ExplicitRegistry(t *testing.T) {
	t.Parallel()
	logos, ok := LookupLogos("openai.Chat")
	if !ok {
		t.Fatalf("expected explicit logos entry")
	}
	if logos.Effect != EffectAI {
		t.Fatalf("effect=%q want %q", logos.Effect, EffectAI)
	}
	if logos.TxCompatible {
		t.Fatalf("openai.Chat must not be tx-compatible")
	}
}

func TestLookupLogos_PrefixFallback(t *testing.T) {
	t.Parallel()
	logos, ok := LookupLogos("state.Set")
	if !ok {
		t.Fatalf("expected prefix fallback")
	}
	if logos.Effect != EffectState {
		t.Fatalf("effect=%q want %q", logos.Effect, EffectState)
	}
}
