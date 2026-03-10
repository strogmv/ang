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

func TestLookupLogos_SessionAndQuotaContract(t *testing.T) {
	t.Parallel()

	session, ok := LookupLogos("session.Get")
	if !ok {
		t.Fatal("expected session.Get logos entry")
	}
	if session.Effect != EffectSession {
		t.Fatalf("session.Get effect=%q want %q", session.Effect, EffectSession)
	}
	if session.ProducesVar != "output" {
		t.Fatalf("session.Get producesVar=%q want output", session.ProducesVar)
	}
	if len(session.ProducesTags) != 1 || session.ProducesTags[0] != ProduceSessionPresent {
		t.Fatalf("session.Get producesTags=%v want [%q]", session.ProducesTags, ProduceSessionPresent)
	}

	quota, ok := LookupLogos("quota.Check")
	if !ok {
		t.Fatal("expected quota.Check logos entry")
	}
	if quota.Effect != EffectState {
		t.Fatalf("quota.Check effect=%q want %q", quota.Effect, EffectState)
	}
	if len(quota.RequiresTags) != 1 || quota.RequiresTags[0] != RequireSessionPresent {
		t.Fatalf("quota.Check requiresTags=%v want [%q]", quota.RequiresTags, RequireSessionPresent)
	}
	if len(quota.ProducesTags) != 1 || quota.ProducesTags[0] != ProduceQuotaChecked {
		t.Fatalf("quota.Check producesTags=%v want [%q]", quota.ProducesTags, ProduceQuotaChecked)
	}
}

func TestLookupLogos_TxAndPureContracts(t *testing.T) {
	t.Parallel()

	txBlock, ok := LookupLogos("tx.Block")
	if !ok {
		t.Fatal("expected tx.Block logos entry")
	}
	if len(txBlock.ChildTags) != 1 || txBlock.ChildTags[0] != ProduceTxOpen {
		t.Fatalf("tx.Block childTags=%v want [%q]", txBlock.ChildTags, ProduceTxOpen)
	}

	dbLock, ok := LookupLogos("db.Lock")
	if !ok {
		t.Fatal("expected db.Lock logos entry")
	}
	if !dbLock.RequiresTx {
		t.Fatal("db.Lock must require tx")
	}
	if len(dbLock.RequiresTags) != 1 || dbLock.RequiresTags[0] != RequireTxOpen {
		t.Fatalf("db.Lock requiresTags=%v want [%q]", dbLock.RequiresTags, RequireTxOpen)
	}

	assign, ok := LookupLogos("mapping.Assign")
	if !ok {
		t.Fatal("expected mapping.Assign logos entry")
	}
	if assign.Effect != EffectPure {
		t.Fatalf("mapping.Assign effect=%q want pure", assign.Effect)
	}
	if !assign.TxCompatible {
		t.Fatal("mapping.Assign must be tx-compatible")
	}
}

func TestLookupLogos_CoversEveryFlowsemAction(t *testing.T) {
	t.Parallel()

	for _, entry := range ActionCatalog() {
		entry := entry
		t.Run(entry.Name, func(t *testing.T) {
			t.Parallel()
			if _, ok := LookupLogos(entry.Name); !ok {
				t.Fatalf("missing logos for action %q", entry.Name)
			}
		})
	}
}
