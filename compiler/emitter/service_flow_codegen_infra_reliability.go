package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
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
		output := arg("output")
		if output == "" {
			output = "idemKey"
		}
		prefix := arg("prefix")
		if prefix == "" {
			prefix = `"idem:"`
		}

		fromList := []string{}
		switch raw := step.Args["from"].(type) {
		case []string:
			fromList = raw
		case []any:
			for _, v := range raw {
				if s, ok := v.(string); ok {
					fromList = append(fromList, s)
				}
			}
		}
		if len(fromList) == 0 {
			fromList = []string{`""`}
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
		key := arg("key")
		if key == "" {
			return "", true
		}
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
		key := arg("key")
		if key == "" {
			return "", true
		}
		ttl := arg("ttl")
		if ttl == "" {
			ttl = "24 * time.Hour"
		}

		dataVar := "_idemData" + sfx
		errVar := "_idemSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// idem.SaveResult\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(resp)\n", pad, dataVar))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, %s)\n", pad, errVar, key, dataVar, ttl))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"idem.SaveResult: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── dedupe.Once ───────────────────────────────────────────────────────────
	// Checks if key exists → skip the child steps. After child steps, marks key as done.
	// Args: key (expr), ttl? (duration string), do: [child steps]
	case "dedupe.Once":
		key := arg("key")
		if key == "" {
			return "", true
		}
		ttl := arg("ttl")
		if ttl == "" {
			ttl = "24 * time.Hour"
		}
		doSteps := child("_do")

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
		key := arg("key")
		rps := flowIntArg(step.Args, "rps", 0)
		if key == "" || rps <= 0 {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "rate limit exceeded"
		}

		rlKeyVar := "_rlKey" + sfx
		rlRawVar := "_rlRaw" + sfx
		rlErrVar := "_rlErr" + sfx
		rlCountVar := "_rlCount" + sfx
		rlDataVar := "_rlData" + sfx

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
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, rlDataVar, rlCountVar))
		b.WriteString(fmt.Sprintf("%ss.stateStore.Set(ctx, %s, %s, 2*time.Second) //nolint:errcheck\n", pad, rlKeyVar, rlDataVar))
		return b.String(), true

	// ── quota.Check ───────────────────────────────────────────────────────────
	// Fixed window key quota (hour/day/month). Throws 429 when limit exceeded.
	case "quota.Check":
		key := arg("key")
		limit := flowIntArg(step.Args, "limit", 0)
		if key == "" || limit <= 0 {
			return "", true
		}
		// window is a static enum — decoded without quotes by normalizer; handle at codegen time
		windowRaw, _ := step.Args["window"].(string)
		if windowRaw == "" {
			windowRaw = "day"
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "quota exceeded"
		}

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
		key := arg("key")
		limit := flowIntArg(step.Args, "limit", 0)
		if key == "" || limit <= 0 {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "Budget exhausted"
		}

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
		key := arg("key")
		tokens := arg("tokens")
		if key == "" || tokens == "" {
			return "", true
		}
		ttl := arg("ttl")
		if ttl == "" {
			ttl = "0"
		}

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
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return "", true
		}
		maxBytes := flowIntArg(step.Args, "max_bytes", 8000)
		if maxBytes <= 0 {
			maxBytes = 8000
		}
		strategy := arg("strategy")
		if strategy == "" {
			strategy = `"lines"`
		}

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
		key := arg("key")
		tier := arg("tier")
		if key == "" || tier == "" {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "Upgrade required"
		}

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
		key := arg("key")
		maxVal := flowIntArg(step.Args, "max", 0)
		if key == "" || maxVal <= 0 {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "concurrency limit exceeded"
		}

		clKeyVar := "_clKey" + sfx
		clRawVar := "_clRaw" + sfx
		clErrVar := "_clErr" + sfx
		clCountVar := "_clCount" + sfx
		clDataVar := "_clData" + sfx
		clSetErrVar := "_clSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// concurrency.Limit\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"cl:\" + %s\n", pad, clKeyVar, key))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, clRawVar, clErrVar, clKeyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, clErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"concurrency.Limit: %%w\", %s)", clErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s int\n", pad, clCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, clRawVar))
		b.WriteString(fmt.Sprintf("%s\tjson.Unmarshal(%s, &%s) //nolint:errcheck\n", pad, clRawVar, clCountVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, clCountVar, maxVal))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusServiceUnavailable, \"Service Unavailable\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s++\n", pad, clCountVar))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, clDataVar, clCountVar))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, 5*time.Minute)\n", pad, clSetErrVar, clKeyVar, clDataVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, clSetErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"concurrency.Limit: %%w\", %s)", clSetErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		// defer decrement
		b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_clDecRaw%s, _ := s.stateStore.Get(ctx, %s)\n", pad, sfx, clKeyVar))
		b.WriteString(fmt.Sprintf("%s\tvar _clDecCount%s int\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\tif _clDecRaw%s != nil {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tjson.Unmarshal(_clDecRaw%s, &_clDecCount%s) //nolint:errcheck\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _clDecCount%s > 0 {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\t_clDecCount%s--\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_clDecData%s, _ := json.Marshal(_clDecCount%s)\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\ts.stateStore.Set(ctx, %s, _clDecData%s, 5*time.Minute) //nolint:errcheck\n", pad, clKeyVar, sfx))
		b.WriteString(fmt.Sprintf("%s}()\n", pad))
		return b.String(), true

	// ── concurrency.Run ───────────────────────────────────────────────────────
	// Composite wrapper: acquire concurrency slot then execute child steps.
	case "concurrency.Run":
		limitStep := normalizer.FlowStep{
			Action: "concurrency.Limit",
			Args:   step.Args,
		}
		limitArg := func(name string) string {
			if v, ok := limitStep.Args[name].(string); ok {
				return normalizeFlowExpr(strings.TrimSpace(v))
			}
			return ""
		}
		limitChild := func(name string) []normalizer.FlowStep { return nil }
		limitCode, ok := renderFlowStepInfraReliability(st, limitStep, indent, sfx+"_run", limitArg, limitChild)
		if !ok {
			return "", false
		}
		var b strings.Builder
		b.WriteString(limitCode)
		if doSteps := child("_do"); len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── circuit.Check ─────────────────────────────────────────────────────────
	// Reads the circuit-open flag; returns 503 if circuit is open.
	// Args: name (string literal for circuit name), throw? (string message)
	case "circuit.Check":
		name := arg("name")
		if name == "" {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "circuit breaker open: " + strings.Trim(name, "\"")
		}

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
		name := arg("name")
		if name == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// circuit.RecordSuccess\n", pad))
		b.WriteString(fmt.Sprintf("%ss.stateStore.Delete(ctx, \"circuit:count:\"+%s) //nolint:errcheck\n", pad, name))
		b.WriteString(fmt.Sprintf("%ss.stateStore.Delete(ctx, \"circuit:open:\"+%s) //nolint:errcheck\n", pad, name))
		return b.String(), true

		// ── circuit.RecordFailure ─────────────────────────────────────────────────
		// Increments failure counter; opens circuit when threshold is reached.
		// Args: name (string literal), threshold? (int, default 5), openTTL? (duration, default 60s)
	case "circuit.RecordFailure":
		name := arg("name")
		if name == "" {
			return "", true
		}
		threshold := flowIntArg(step.Args, "threshold", 5)
		openTTL := arg("openTTL")
		if openTTL == "" {
			openTTL = "60 * time.Second"
		}

		cfKeyVar := "_cfKey" + sfx
		cfRawVar := "_cfRaw" + sfx
		cfCountVar := "_cfCount" + sfx
		cfDataVar := "_cfData" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// circuit.RecordFailure\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"circuit:count:\" + %s\n", pad, cfKeyVar, name))
		b.WriteString(fmt.Sprintf("%s%s, _ := s.stateStore.Get(ctx, %s)\n", pad, cfRawVar, cfKeyVar))
		b.WriteString(fmt.Sprintf("%svar %s int\n", pad, cfCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, cfRawVar))
		b.WriteString(fmt.Sprintf("%s\tjson.Unmarshal(%s, &%s) //nolint:errcheck\n", pad, cfRawVar, cfCountVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s++\n", pad, cfCountVar))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, cfDataVar, cfCountVar))
		b.WriteString(fmt.Sprintf("%ss.stateStore.Set(ctx, %s, %s, 5*time.Minute) //nolint:errcheck\n", pad, cfKeyVar, cfDataVar))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, cfCountVar, threshold))
		b.WriteString(fmt.Sprintf("%s\ts.stateStore.Set(ctx, \"circuit:open:\"+%s, []byte(\"1\"), %s) //nolint:errcheck\n", pad, name, openTTL))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	// ── circuit.Breaker ───────────────────────────────────────────────────────
	// Composite wrapper: check-open gate + auto record success/failure around child block.
	case "circuit.Breaker":
		name := arg("name")
		if name == "" {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "circuit breaker open: " + strings.Trim(name, "\"")
		}
		threshold := flowIntArg(step.Args, "threshold", 5)
		openTTL := arg("openTTL")
		if openTTL == "" {
			openTTL = "60 * time.Second"
		}

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

		if doSteps := child("_do"); len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── bulkhead.Acquire ──────────────────────────────────────────────────────
	// Named resource pool. Increments counter; defers release. Throws 503 when pool full.
	// Args: name (string literal), max (int), throw? (string message)
	case "bulkhead.Acquire":
		name := arg("name")
		maxBH := flowIntArg(step.Args, "max", 0)
		if name == "" || maxBH <= 0 {
			return "", true
		}
		throwMsg := arg("throw")
		if throwMsg == "" {
			throwMsg = "bulkhead full: " + strings.Trim(name, "\"")
		}

		bhKeyVar := "_bhKey" + sfx
		bhRawVar := "_bhRaw" + sfx
		bhErrVar := "_bhErr" + sfx
		bhCountVar := "_bhCount" + sfx
		bhDataVar := "_bhData" + sfx
		bhSetErrVar := "_bhSetErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// bulkhead.Acquire\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := \"bulkhead:\" + %s\n", pad, bhKeyVar, name))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, bhRawVar, bhErrVar, bhKeyVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, bhErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"bulkhead.Acquire: %%w\", %s)", bhErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s int\n", pad, bhCountVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, bhRawVar))
		b.WriteString(fmt.Sprintf("%s\tjson.Unmarshal(%s, &%s) //nolint:errcheck\n", pad, bhRawVar, bhCountVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s >= %d {\n", pad, bhCountVar, maxBH))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusServiceUnavailable, \"Service Unavailable\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s++\n", pad, bhCountVar))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, bhDataVar, bhCountVar))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, 5*time.Minute)\n", pad, bhSetErrVar, bhKeyVar, bhDataVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, bhSetErrVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"bulkhead.Acquire: %%w\", %s)", bhSetErrVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		// defer release
		b.WriteString(fmt.Sprintf("%sdefer func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_bhDecRaw%s, _ := s.stateStore.Get(ctx, %s)\n", pad, sfx, bhKeyVar))
		b.WriteString(fmt.Sprintf("%s\tvar _bhDecCount%s int\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\tif _bhDecRaw%s != nil {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\tjson.Unmarshal(_bhDecRaw%s, &_bhDecCount%s) //nolint:errcheck\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _bhDecCount%s > 0 {\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t\t_bhDecCount%s--\n", pad, sfx))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_bhDecData%s, _ := json.Marshal(_bhDecCount%s)\n", pad, sfx, sfx))
		b.WriteString(fmt.Sprintf("%s\ts.stateStore.Set(ctx, %s, _bhDecData%s, 5*time.Minute) //nolint:errcheck\n", pad, bhKeyVar, sfx))
		b.WriteString(fmt.Sprintf("%s}()\n", pad))
		return b.String(), true

	// ── bulkhead.Run ──────────────────────────────────────────────────────────
	// Composite wrapper: acquire bulkhead slot then execute child steps.
	case "bulkhead.Run":
		acquireStep := normalizer.FlowStep{
			Action: "bulkhead.Acquire",
			Args:   step.Args,
		}
		acquireArg := func(name string) string {
			if v, ok := acquireStep.Args[name].(string); ok {
				return normalizeFlowExpr(strings.TrimSpace(v))
			}
			return ""
		}
		acquireChild := func(name string) []normalizer.FlowStep { return nil }
		acquireCode, ok := renderFlowStepInfraReliability(st, acquireStep, indent, sfx+"_run", acquireArg, acquireChild)
		if !ok {
			return "", false
		}
		var b strings.Builder
		b.WriteString(acquireCode)
		if doSteps := child("_do"); len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── log.Emit ──────────────────────────────────────────────────────────────
	case "log.Emit":
		message := arg("message")
		if message == "" {
			message = `"flow log"`
		}
		level := strings.Trim(strings.ToLower(strings.TrimSpace(arg("level"))), "\"")
		if level == "" {
			level = "info"
		}
		fields := flowMapStringArg(step.Args["fields"])

		args := []string{message}
		if len(fields) > 0 {
			keys := flowSortedKeys(fields)
			for _, k := range keys {
				args = append(args, fmt.Sprintf("%q, %s", k, fields[k]))
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
		name := arg("name")
		if name == "" {
			return "", true
		}
		kind := arg("kind")
		if kind == "" {
			kind = `"counter"`
		}
		value := arg("value")
		if value == "" {
			value = "1"
		}
		labels := flowMapStringArg(step.Args["labels"])
		labelsExpr := "map[string]any{}"
		if len(labels) > 0 {
			keys := flowSortedKeys(labels)
			pairs := make([]string, 0, len(keys))
			for _, k := range keys {
				pairs = append(pairs, fmt.Sprintf("%q: %s", k, labels[k]))
			}
			labelsExpr = "map[string]any{" + strings.Join(pairs, ", ") + "}"
		}
		return fmt.Sprintf("%sslog.Info(\"metric.emit\", \"name\", %s, \"kind\", %s, \"value\", %s, \"labels\", %s)\n", pad, name, kind, value, labelsExpr), true

	// ── trace.Span ────────────────────────────────────────────────────────────
	// Starts a span for the child block and attaches optional attributes.
	case "trace.Span":
		name := arg("name")
		if name == "" {
			return "", true
		}
		spanCtxVar := "_traceCtx" + sfx
		spanVar := "_traceSpan" + sfx
		attrs := flowMapStringArg(step.Args["attrs"])

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := otel.Tracer(\"ang.flow\").Start(ctx, %s)\n", pad, spanCtxVar, spanVar, name))
		b.WriteString(fmt.Sprintf("%s_ = %s\n", pad, spanCtxVar))
		if len(attrs) > 0 {
			keys := flowSortedKeys(attrs)
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("%s%s.SetAttributes(attribute.String(%q, fmt.Sprint(%s)))\n", pad, spanVar, k, attrs[k]))
			}
		}
		b.WriteString(fmt.Sprintf("%sdefer %s.End()\n", pad, spanVar))
		if doSteps := child("_do"); len(doSteps) > 0 {
			b.WriteString(renderFlowSteps(st, doSteps, indent))
		}
		return b.String(), true

	// ── slo.Budget ────────────────────────────────────────────────────────────
	// Applies a latency budget context to child steps and logs budget overrun.
	case "slo.Budget":
		duration := arg("duration")
		if duration == "" {
			return "", true
		}
		name := arg("name")
		if name == "" {
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
		if doSteps := child("_do"); len(doSteps) > 0 {
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
