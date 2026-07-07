package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// renderFlowStepInfraReliability handles idempotency, deduplication, rate limiting,
// concurrency limiting, circuit breaker, and bulkhead actions.
func renderFlowStepInfraReliability(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {

	// ── idem.DeriveKey ────────────────────────────────────────────────────────
	// Computes a deterministic idempotency key from a list of expressions.
	// Args: from (list of expr strings), output (var name), prefix? (string prefix)
	case "idem.DeriveKey", "idempotency.DeriveKey":
		typed, err := flowir.DecodeAs[flowir.IdempotencyDeriveKey](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, prefix := typed.Output, normalizeFlowExpr(typed.Prefix.Source)
		fromList := make([]string, 0, len(typed.From))
		for _, v := range typed.From {
			fromList = append(fromList, normalizeFlowExpr(v.Source))
		}

		hasherVar := "_idemHasher" + sfx
		fmtArgs := make([]string, 0, len(fromList))
		fmtVerbs := make([]string, 0, len(fromList))
		for _, e := range fromList {
			fmtVerbs = append(fmtVerbs, "%v")
			fmtArgs = append(fmtArgs, e)
		}
		formatStr := strings.Join(fmtVerbs, "|")
		argsStr := strings.Join(fmtArgs, ", ")

		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// idem.DeriveKey\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := sha256.New()\n", pad, hasherVar))
		b.WriteString(fmt.Sprintf("%sfmt.Fprintf(%s, %q, %s)\n", pad, hasherVar, formatStr, argsStr))
		b.WriteString(fmt.Sprintf("%s%s := %s + hex.EncodeToString(%s.Sum(nil))[:16]\n", pad, output, prefix, hasherVar))
		return b.String(), true

		// ── idem.Check ────────────────────────────────────────────────────────────
		// Reads stateStore for given key; if found, unmarshals into resp and returns early.
		// Args: key (expr)
	case "idem.Check", "idempotency.Check":
		typed, err := flowir.DecodeAs[flowir.IdempotencyCheck](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		rawVar := "_idemRaw" + sfx
		errVar := "_idemErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// idem.Check\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"idem.Check: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\tif err := json.Unmarshal(%s, &resp); err != nil {\n", pad, rawVar))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"idem.Check unmarshal: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(returnSuccess(st, pad+"\t"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

		// ── idem.SaveResult ───────────────────────────────────────────────────────
		// Marshals current resp and stores it under the idempotency key.
		// Args: key (expr), ttl? (duration string like "24*time.Hour")
	case "idem.SaveResult", "idempotency.SaveResult":
		typed, err := flowir.DecodeAs[flowir.IdempotencySaveResult](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, ttl := normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.TTL.Source)

		dataVar := "_idemData" + sfx
		marshalErrVar := "_idemMarshalErr" + sfx
		errVar := "_idemSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// idem.SaveResult\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(resp)\n", pad, dataVar, marshalErrVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, marshalErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%s marshal: %%w\", %s)", step.Action, marshalErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, %s)\n", pad, errVar, key, dataVar, ttl))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%s: %%w\", %s)", step.Action, errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── dedupe.Once ───────────────────────────────────────────────────────────
	// Checks if key exists → skip the child steps. After child steps, marks key as done.
	// Args: key (expr), ttl? (duration string), do: [child steps]
	case "dedupe.Once":
		typed, err := flowir.DecodeAs[flowir.DedupeOnce](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, ttl, doSteps := normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.TTL.Source), typed.Steps

		rawVar := "_dedupeRaw" + sfx
		errVar := "_dedupeErr" + sfx
		markErrVar := "_dedupeMarkErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// dedupe.Once\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"dedupe.Once: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, rawVar))
		if len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(cloneFlowState(st), doSteps, indent+1))
		}
		b.WriteString(fmt.Sprintf("%s\t%s := s.stateStore.Set(ctx, %s, []byte(\"1\"), %s)\n", pad, markErrVar, key, ttl))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, markErrVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"dedupe.Once mark: %%w\", %s)", markErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

		// ── ratelimit.Check ───────────────────────────────────────────────────────
		// Per-key per-second token counter. Throws 429 if limit exceeded.
		// Args: key (expr), rps (int), throw? (string message)
	case "ratelimit.Check", "ratelimit.Limit":
		typed, err := flowir.DecodeAs[flowir.RateLimit](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, rps, throwMsg := normalizeFlowExpr(typed.Key.Source), typed.RPS, typed.Throw

		rlKeyVar := "_rlKey" + sfx
		rlRawVar := "_rlRaw" + sfx
		rlErrVar := "_rlErr" + sfx
		rlCountVar := "_rlCount" + sfx
		rlDataVar := "_rlData" + sfx
		rlMarshalErrVar := "_rlMarshalErr" + sfx
		rlSetErrVar := "_rlSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// ratelimit.Check\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"rl:%%v:%%d\", %s, time.Now().Unix())\n", pad, rlKeyVar, key))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rlRawVar, rlErrVar, rlKeyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rlErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"ratelimit.Check: %%w\", %s)", rlErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s int\n", pad, rlCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rlRawVar))
		b.WriteString(fmt.Sprintf("%s\tjson.Unmarshal(%s, &%s) //nolint:errcheck\n", pad, rlRawVar, rlCountVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, rlCountVar, rps))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusTooManyRequests, \"Too Many Requests\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s++\n", pad, rlCountVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(%s)\n", pad, rlDataVar, rlMarshalErrVar, rlCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rlMarshalErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%s marshal: %%w\", %s)", step.Action, rlMarshalErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, 2*time.Second)\n", pad, rlSetErrVar, rlKeyVar, rlDataVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rlSetErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%s: %%w\", %s)", step.Action, rlSetErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── quota.Check ───────────────────────────────────────────────────────────
	// Fixed window key quota (hour/day/month). Throws 429 when limit exceeded.
	case "quota.Check":
		typed, err := flowir.DecodeAs[flowir.QuotaCheck](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, limit, windowRaw, throwMsg := normalizeFlowExpr(typed.Key.Source), typed.Limit, typed.Window, typed.Throw

		var bucketFmt, ttlExpr string
		switch windowRaw {
		case "hour":
			bucketFmt = "2006-01-02T15"
			ttlExpr = "2 * time.Hour"
		case "month":
			bucketFmt = "2006-01"
			ttlExpr = "32 * 24 * time.Hour"
		default: // "day"
			bucketFmt = "2006-01-02"
			ttlExpr = "25 * time.Hour"
		}

		bucketVar := "_quotaBucket" + sfx
		ttlVar := "_quotaTTL" + sfx
		keyVar := "_quotaKey" + sfx
		rawVar := "_quotaRaw" + sfx
		errVar := "_quotaErr" + sfx
		countVar := "_quotaCount" + sfx
		setErrVar := "_quotaSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// quota.Check (window=%s)\n", pad, windowRaw))
		b.WriteString(fmt.Sprintf("%s%s := time.Now().UTC().Format(%q)\n", pad, bucketVar, bucketFmt))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, ttlVar, ttlExpr))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"quota:%%v:%%s\", %s, %s)\n", pad, keyVar, key, bucketVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, keyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"quota.Check: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := 0\n", pad, countVar))
		b.WriteString(fmt.Sprintf("%sif len(%s) > 0 {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\t%s, _ = strconv.Atoi(string(%s))\n", pad, countVar, rawVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, countVar, limit))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusTooManyRequests, \"Too Many Requests\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s++\n", pad, countVar))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, []byte(strconv.Itoa(%s)), %s)\n", pad, setErrVar, keyVar, countVar, ttlVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, setErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"quota.Check: %%w\", %s)", setErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── budget.Check ──────────────────────────────────────────────────────────
	// Cumulative token budget guard per key. Throws 402 when exhausted.
	case "budget.Check":
		typed, err := flowir.DecodeAs[flowir.BudgetCheck](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, limit, throwMsg := normalizeFlowExpr(typed.Key.Source), typed.Limit, typed.Throw

		keyVar := "_budgetKey" + sfx
		rawVar := "_budgetRaw" + sfx
		errVar := "_budgetErr" + sfx
		usedVar := "_budgetUsed" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// budget.Check\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"budget:%%v\", %s)\n", pad, keyVar, key))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, keyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"budget.Check: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := 0\n", pad, usedVar))
		b.WriteString(fmt.Sprintf("%sif len(%s) > 0 {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\t%s, _ = strconv.Atoi(string(%s))\n", pad, usedVar, rawVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, usedVar, limit))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusPaymentRequired, \"Payment Required\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── budget.Consume ────────────────────────────────────────────────────────
	// Increments cumulative token budget for key.
	case "budget.Consume":
		typed, err := flowir.DecodeAs[flowir.BudgetConsume](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, tokens, ttl := normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Tokens.Source), normalizeFlowExpr(typed.TTL.Source)

		keyVar := "_budgetKey" + sfx
		rawVar := "_budgetRaw" + sfx
		errVar := "_budgetErr" + sfx
		curVar := "_budgetCur" + sfx
		setErrVar := "_budgetSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// budget.Consume\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"budget:%%v\", %s)\n", pad, keyVar, key))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, keyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"budget.Consume: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := 0\n", pad, curVar))
		b.WriteString(fmt.Sprintf("%sif len(%s) > 0 {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\t%s, _ = strconv.Atoi(string(%s))\n", pad, curVar, rawVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, []byte(strconv.Itoa(%s + (%s))), %s)\n", pad, setErrVar, keyVar, curVar, tokens, ttl))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, setErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"budget.Consume: %%w\", %s)", setErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── context.Trim ──────────────────────────────────────────────────────────
	// Shrinks large string context before LLM calls.
	case "context.Trim":
		typed, err := flowir.DecodeAs[flowir.ContextTrim](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output, maxBytes, strategy := normalizeFlowExpr(typed.Input.Source), typed.Output, typed.MaxBytes, normalizeFlowExpr(typed.Strategy.Source)

		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		inputVar := "_ctxInput" + sfx
		maxVar := "_ctxMaxBytes" + sfx
		strategyVar := "_ctxStrategy" + sfx
		truncVar := "_ctxTrunc" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// context.Trim\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, inputVar, input))
		b.WriteString(fmt.Sprintf("%s%s := %d\n", pad, maxVar, maxBytes))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, strategyVar, strategy))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, inputVar))
		b.WriteString(fmt.Sprintf("%sif len(%s) > %s {\n", pad, inputVar, maxVar))
		b.WriteString(fmt.Sprintf("%s\t%s := %s[:%s]\n", pad, truncVar, inputVar, maxVar))
		b.WriteString(fmt.Sprintf("%s\tswitch %s {\n", pad, strategyVar))
		b.WriteString(fmt.Sprintf("%s\tcase \"chars\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t// keep hard cut\n", pad))
		b.WriteString(fmt.Sprintf("%s\tcase \"sentences\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _idx := strings.LastIndexAny(%s, \".!?\"); _idx > 0 {\n", pad, truncVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = %s[:_idx+1]\n", pad, truncVar, truncVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tdefault:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _idx := strings.LastIndex(%s, \"\\n\"); _idx > 0 {\n", pad, truncVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = %s[:_idx]\n", pad, truncVar, truncVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = %s + \"\\n... (truncated)\"\n", pad, output, truncVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── profile.Require ───────────────────────────────────────────────────────
	// Enforces minimum profile tier (free < ops < enterprise).
	case "profile.Require":
		typed, err := flowir.DecodeAs[flowir.ProfileRequire](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key, tier, throwMsg := normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Tier.Source), typed.Throw

		keyVar := "_profileKey" + sfx
		rawVar := "_profileRaw" + sfx
		errVar := "_profileErr" + sfx
		tierVar := "_profileTier" + sfx
		orderVar := "_profileOrder" + sfx
		reqTierVar := "_requiredTier" + sfx
		curRankVar := "_profileRank" + sfx
		reqRankVar := "_requiredRank" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// profile.Require\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := fmt.Sprintf(\"profile:%%v\", %s)\n", pad, keyVar, key))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, keyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"profile.Require: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"free\"\n", pad, tierVar))
		b.WriteString(fmt.Sprintf("%sif len(%s) > 0 {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\t%s = string(%s)\n", pad, tierVar, rawVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := map[string]int{\"free\": 0, \"ops\": 1, \"enterprise\": 2}\n", pad, orderVar))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, reqTierVar, tier))
		b.WriteString(fmt.Sprintf("%s%s := %s[%s]\n", pad, curRankVar, orderVar, tierVar))
		b.WriteString(fmt.Sprintf("%s%s := %s[\"enterprise\"]\n", pad, reqRankVar, orderVar))
		b.WriteString(fmt.Sprintf("%sif _v, _ok := %s[%s]; _ok {\n", pad, orderVar, reqTierVar))
		b.WriteString(fmt.Sprintf("%s\t%s = _v\n", pad, reqRankVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s < %s {\n", pad, curRankVar, reqRankVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusForbidden, \"Forbidden\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

		// ── concurrency.Limit ─────────────────────────────────────────────────────
		// State-store semaphore. Increments counter; defers decrement. Throws 503 when full.
		// Args: key (expr string), max (int), throw? (string message)
	case "concurrency.Limit":
		typed, err := flowir.DecodeAs[flowir.ConcurrencyLimit](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		return renderStateSemaphore(st, pad, sfx, "concurrency.Limit", "cl:", normalizeFlowExpr(typed.Key.Source), typed.Max, typed.Throw), true

	// ── concurrency.Run ───────────────────────────────────────────────────────
	// Composite wrapper: acquire concurrency slot then execute child steps.
	case "concurrency.Run":
		typed, err := flowir.DecodeAs[flowir.ConcurrencyRun](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		var b strings.Builder
		b.WriteString(renderStateSemaphore(st, pad, sfx+"_run", "concurrency.Run", "cl:", normalizeFlowExpr(typed.Key.Source), typed.Max, typed.Throw))
		if doSteps := typed.Steps; len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── mutex.With ────────────────────────────────────────────────────────────
	// Process-local mutex keyed by string. Optional bounded wait with polling.
	case "mutex.With":
		typed, err := flowir.DecodeAs[flowir.MutexWith](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		mutexHelper := "_FlowMutexForKey"
		if strings.TrimSpace(st.serviceName) != "" {
			mutexHelper = "_" + ExportName(st.serviceName) + "FlowMutexForKey"
		}
		waitExpr, pollExpr, throwMsg := normalizeFlowExpr(typed.Wait.Source), normalizeFlowExpr(typed.Poll.Source), typed.Throw
		mutexVar := "_mutex" + sfx
		startVar := "_mutexStart" + sfx
		waitVar := "_mutexWait" + sfx
		pollVar := "_mutexPoll" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// mutex.With\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := %s(fmt.Sprint(%s))\n", pad, mutexVar, mutexHelper, key))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, waitVar, waitExpr))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, pollVar, pollExpr))
		b.WriteString(fmt.Sprintf("%sif %s <= 0 {\n", pad, pollVar))
		b.WriteString(fmt.Sprintf("%s\t%s = 50 * time.Millisecond\n", pad, pollVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s <= 0 {\n", pad, waitVar))
		b.WriteString(fmt.Sprintf("%s\t%s.Lock()\n", pad, mutexVar))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := time.Now()\n", pad, startVar))
		b.WriteString(fmt.Sprintf("%s\tfor {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif %s.TryLock() {\n", pad, mutexVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif time.Since(%s) >= %s {\n", pad, startVar, waitVar))
		b.WriteString(errReturn(st, pad+"\t\t\t", fmt.Sprintf("errors.New(http.StatusServiceUnavailable, \"MUTEX_BUSY\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\ttime.Sleep(%s)\n", pad, pollVar))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Unlock()\n", pad, mutexVar))
		if doSteps := typed.Steps; len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── circuit.Check ─────────────────────────────────────────────────────────
	// Reads the circuit-open flag; returns 503 if circuit is open.
	// Args: name (string literal for circuit name), throw? (string message)
	case "circuit.Check":
		typed, err := flowir.DecodeAs[flowir.CircuitAction](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		name, throwMsg := normalizeFlowExpr(typed.Name.Source), typed.Throw

		circKeyVar := "_circKey" + sfx
		circRawVar := "_circRaw" + sfx
		circErrVar := "_circErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// circuit.Check\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"circuit:open:\" + %s\n", pad, circKeyVar, name))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, circRawVar, circErrVar, circKeyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, circErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.Check: %%w\", %s)", circErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, circRawVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusServiceUnavailable, \"Service Unavailable\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── circuit.RecordSuccess ─────────────────────────────────────────────────
	// Resets the failure counter and closes the circuit.
	// Args: name (string literal for circuit name)
	case "circuit.RecordSuccess":
		typed, err := flowir.DecodeAs[flowir.CircuitAction](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		name := normalizeFlowExpr(typed.Name.Source)
		delCountErrVar := "_circDelCountErr" + sfx
		delOpenErrVar := "_circDelOpenErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// circuit.RecordSuccess\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Delete(ctx, \"circuit:count:\"+%s)\n", pad, delCountErrVar, name))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, delCountErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.RecordSuccess: %%w\", %s)", delCountErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Delete(ctx, \"circuit:open:\"+%s)\n", pad, delOpenErrVar, name))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, delOpenErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.RecordSuccess: %%w\", %s)", delOpenErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

		// ── circuit.RecordFailure ─────────────────────────────────────────────────
		// Increments failure counter; opens circuit when threshold is reached.
		// Args: name (string literal), threshold? (int, default 5), openTTL? (duration, default 60s)
	case "circuit.RecordFailure":
		typed, err := flowir.DecodeAs[flowir.CircuitAction](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		name, threshold, openTTL := normalizeFlowExpr(typed.Name.Source), typed.Threshold, normalizeFlowExpr(typed.OpenTTL.Source)

		cfKeyVar := "_cfKey" + sfx
		cfRawVar := "_cfRaw" + sfx
		cfGetErrVar := "_cfGetErr" + sfx
		cfCountVar := "_cfCount" + sfx
		cfDataVar := "_cfData" + sfx
		cfMarshalErrVar := "_cfMarshalErr" + sfx
		cfSetErrVar := "_cfSetErr" + sfx
		cfOpenSetErrVar := "_cfOpenSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// circuit.RecordFailure\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"circuit:count:\" + %s\n", pad, cfKeyVar, name))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, cfRawVar, cfGetErrVar, cfKeyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cfGetErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.RecordFailure: %%w\", %s)", cfGetErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s int\n", pad, cfCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cfRawVar))
		b.WriteString(fmt.Sprintf("%s\tjson.Unmarshal(%s, &%s) //nolint:errcheck\n", pad, cfRawVar, cfCountVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s++\n", pad, cfCountVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(%s)\n", pad, cfDataVar, cfMarshalErrVar, cfCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cfMarshalErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.RecordFailure marshal: %%w\", %s)", cfMarshalErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, 5*time.Minute)\n", pad, cfSetErrVar, cfKeyVar, cfDataVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cfSetErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.RecordFailure: %%w\", %s)", cfSetErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, cfCountVar, threshold))
		b.WriteString(fmt.Sprintf("%s\t%s := s.stateStore.Set(ctx, \"circuit:open:\"+%s, []byte(\"1\"), %s)\n", pad, cfOpenSetErrVar, name, openTTL))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, cfOpenSetErrVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"circuit.RecordFailure open: %%w\", %s)", cfOpenSetErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── circuit.Breaker ───────────────────────────────────────────────────────
	// Composite wrapper: check-open gate + auto record success/failure around child block.
	case "circuit.Breaker":
		typed, err := flowir.DecodeAs[flowir.CircuitAction](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		name, throwMsg, threshold, openTTL := normalizeFlowExpr(typed.Name.Source), typed.Throw, typed.Threshold, normalizeFlowExpr(typed.OpenTTL.Source)

		cbOpenKeyVar := "_cbOpenKey" + sfx
		cbOpenRawVar := "_cbOpenRaw" + sfx
		cbOpenErrVar := "_cbOpenErr" + sfx
		cbFailKeyVar := "_cbFailKey" + sfx
		cbFailRawVar := "_cbFailRaw" + sfx
		cbFailCountVar := "_cbFailCount" + sfx
		cbFailDataVar := "_cbFailData" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// circuit.Breaker\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"circuit:open:\" + %s\n", pad, cbOpenKeyVar, name))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, cbOpenRawVar, cbOpenErrVar, cbOpenKeyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cbOpenErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"circuit.Breaker check: %%w\", %s)", cbOpenErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cbOpenRawVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusServiceUnavailable, \"Service Unavailable\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif err != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s := \"circuit:count:\" + %s\n", pad, cbFailKeyVar, name))
		b.WriteString(fmt.Sprintf("%s\t\t%s, _ := s.stateStore.Get(ctx, %s)\n", pad, cbFailRawVar, cbFailKeyVar))
		b.WriteString(fmt.Sprintf("%s\t\tvar %s int\n", pad, cbFailCountVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != nil {\n", pad, cbFailRawVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tjson.Unmarshal(%s, &%s) //nolint:errcheck\n", pad, cbFailRawVar, cbFailCountVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s++\n", pad, cbFailCountVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s, _ := json.Marshal(%s)\n", pad, cbFailDataVar, cbFailCountVar))
		b.WriteString(fmt.Sprintf("%s\t\ts.stateStore.Set(ctx, %s, %s, 5*time.Minute) //nolint:errcheck\n", pad, cbFailKeyVar, cbFailDataVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s >= %d {\n", pad, cbFailCountVar, threshold))
		b.WriteString(fmt.Sprintf("%s\t\t\ts.stateStore.Set(ctx, \"circuit:open:\"+%s, []byte(\"1\"), %s) //nolint:errcheck\n", pad, name, openTTL))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\treturn\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\ts.stateStore.Delete(ctx, \"circuit:count:\"+%s) //nolint:errcheck\n", pad, name))
		b.WriteString(fmt.Sprintf("%s\ts.stateStore.Delete(ctx, \"circuit:open:\"+%s) //nolint:errcheck\n", pad, name))
		b.WriteString(fmt.Sprintf("%s}()\n", pad))

		if doSteps := typed.Steps; len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── bulkhead.Acquire ──────────────────────────────────────────────────────
	// Named resource pool. Increments counter; defers release. Throws 503 when pool full.
	// Args: name (string literal), max (int), throw? (string message)
	case "bulkhead.Acquire":
		typedEarly, typedErr := flowir.DecodeAs[flowir.BulkheadAction](step)
		if typedErr != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, typedErr.Error()), true
		}
		return renderStateSemaphore(st, pad, sfx, "bulkhead.Acquire", "bulkhead:", normalizeFlowExpr(typedEarly.Name.Source), typedEarly.Max, typedEarly.Throw), true

	// ── bulkhead.Run ──────────────────────────────────────────────────────────
	// Composite wrapper: acquire bulkhead slot then execute child steps.
	case "bulkhead.Run":
		typed, err := flowir.DecodeAs[flowir.BulkheadAction](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		var b strings.Builder
		b.WriteString(renderStateSemaphore(st, pad, sfx+"_run", "bulkhead.Run", "bulkhead:", normalizeFlowExpr(typed.Name.Source), typed.Max, typed.Throw))
		if doSteps := typed.Steps; len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── log.Emit ──────────────────────────────────────────────────────────────
	case "log.Emit":
		typed, err := flowir.DecodeAs[flowir.LogEmit](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		message, level := normalizeFlowExpr(typed.Message.Source), strings.ToLower(typed.Level)
		fields := typed.Fields

		args := []string{message}
		if len(fields) > 0 {
			keys := make([]string, 0, len(fields))
			for k := range fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				args = append(args, fmt.Sprintf("%q, %s", k, normalizeFlowExpr(fields[k].Source)))
			}
		}
		fn := "Info"
		switch level {
		case "debug":
			fn = "Debug"
		case "warn", "warning":
			fn = "Warn"
		case "error":
			fn = "Error"
		}
		return fmt.Sprintf("%sslog.%s(%s)\n", pad, fn, strings.Join(args, ", ")), true

	// ── metric.Emit ───────────────────────────────────────────────────────────
	// Current implementation emits structured metric events via slog.
	case "metric.Emit":
		typed, err := flowir.DecodeAs[flowir.MetricEmit](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		name, kind, value, labels := normalizeFlowExpr(typed.Name.Source), fmt.Sprintf("%q", typed.Kind), normalizeFlowExpr(typed.Value.Source), typed.Labels
		labelsExpr := "map[string]any{}"
		if len(labels) > 0 {
			keys := make([]string, 0, len(labels))
			for k := range labels {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			pairs := make([]string, 0, len(keys))
			for _, k := range keys {
				pairs = append(pairs, fmt.Sprintf("%q: %s", k, normalizeFlowExpr(labels[k].Source)))
			}
			labelsExpr = "map[string]any{" + strings.Join(pairs, ", ") + "}"
		}
		return fmt.Sprintf("%sslog.Info(\"metric.emit\", \"name\", %s, \"kind\", %s, \"value\", %s, \"labels\", %s)\n", pad, name, kind, value, labelsExpr), true

	// ── trace.Span ────────────────────────────────────────────────────────────
	// Starts a span for the child block and attaches optional attributes.
	case "trace.Span":
		typed, err := flowir.DecodeAs[flowir.TraceSpan](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		name := normalizeFlowExpr(typed.Name.Source)
		spanCtxVar := "_traceCtx" + sfx
		spanVar := "_traceSpan" + sfx
		attrs := typed.Attributes

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := otel.Tracer(\"ang.flow\").Start(ctx, %s)\n", pad, spanCtxVar, spanVar, name))
		b.WriteString(fmt.Sprintf("%s_ = %s\n", pad, spanCtxVar))
		if len(attrs) > 0 {
			keys := make([]string, 0, len(attrs))
			for k := range attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("%s%s.SetAttributes(attribute.String(%q, fmt.Sprint(%s)))\n", pad, spanVar, k, normalizeFlowExpr(attrs[k].Source)))
			}
		}
		b.WriteString(fmt.Sprintf("%sdefer %s.End()\n", pad, spanVar))
		if doSteps := typed.Steps; len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── slo.Budget ────────────────────────────────────────────────────────────
	// Applies a latency budget context to child steps and logs budget overrun.
	case "slo.Budget":
		typed, err := flowir.DecodeAs[flowir.SLOBudget](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		duration, name := normalizeFlowExpr(typed.Duration.Source), fmt.Sprintf("%q", typed.Name)
		if typed.Name == "" {
			name = `"flow"`
		}
		startVar := "_sloStart" + sfx
		ctxVar := "_sloCtx" + sfx
		cancelVar := "_sloCancel" + sfx
		limitVar := "_sloLimit" + sfx
		prevCtxVar := "_sloPrevCtx" + sfx
		elapsedVar := "_sloElapsed" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := time.Now()\n", pad, startVar))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, limitVar, duration))
		b.WriteString(fmt.Sprintf("%s%s := ctx\n", pad, prevCtxVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, %s)\n", pad, ctxVar, cancelVar, limitVar))
		b.WriteString(fmt.Sprintf("%sctx = %s\n", pad, ctxVar))
		if doSteps := typed.Steps; len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		b.WriteString(fmt.Sprintf("%s%s()\n", pad, cancelVar))
		b.WriteString(fmt.Sprintf("%sctx = %s\n", pad, prevCtxVar))
		b.WriteString(fmt.Sprintf("%s%s := time.Since(%s)\n", pad, elapsedVar, startVar))
		b.WriteString(fmt.Sprintf("%sif %s > %s {\n", pad, elapsedVar, limitVar))
		b.WriteString(fmt.Sprintf("%s\tslog.Warn(\"slo.budget.exceeded\", \"name\", %s, \"elapsed_ms\", %s.Milliseconds(), \"budget_ms\", %s.Milliseconds())\n", pad, name, elapsedVar, limitVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}

func renderStateSemaphore(st *flowRenderState, pad, sfx, action, prefix, key string, max int, throwMessage string) string {
	keyVar := "_semKey" + sfx
	rawVar := "_semRaw" + sfx
	errVar := "_semErr" + sfx
	countVar := "_semCount" + sfx
	dataVar := "_semData" + sfx
	setErrVar := "_semSetErr" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s// %s\n", pad, action))
	b.WriteString(fmt.Sprintf("%s%s := %q + %s\n", pad, keyVar, prefix, key))
	b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, keyVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(%q, %s)", action+": %w", errVar)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%svar %s int\n", pad, countVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil { json.Unmarshal(%s, &%s) } //nolint:errcheck\n", pad, rawVar, rawVar, countVar))
	b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, countVar, max))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusServiceUnavailable, \"Service Unavailable\", %q)", throwMessage)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s++\n", pad, countVar))
	b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, dataVar, countVar))
	b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, 5*time.Minute)\n", pad, setErrVar, keyVar, dataVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, setErrVar))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(%q, %s)", action+": %w", setErrVar)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_raw, _ := s.stateStore.Get(ctx, %s)\n", pad, keyVar))
	b.WriteString(fmt.Sprintf("%s\tvar _count int\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif _raw != nil { json.Unmarshal(_raw, &_count) } //nolint:errcheck\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif _count > 0 { _count-- }\n", pad))
	b.WriteString(fmt.Sprintf("%s\t_data, _ := json.Marshal(_count)\n", pad))
	b.WriteString(fmt.Sprintf("%s\ts.stateStore.Set(ctx, %s, _data, 5*time.Minute) //nolint:errcheck\n", pad, keyVar))
	b.WriteString(fmt.Sprintf("%s}()\n", pad))
	return b.String()
}

func flowMapStringArg(raw any) map[string]string {
	out := map[string]string{}
	m, ok := raw.(map[string]string)
	if !ok {
		return out
	}
	for k, v := range m {
		key := strings.TrimSpace(k)
		val := normalizeFlowExpr(strings.TrimSpace(v))
		if key == "" || val == "" {
			continue
		}
		out[key] = val
	}
	return out
}

func flowSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
