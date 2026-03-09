package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepInfraConcurrencyAndDelivery(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "parallel.Run":
		branches, _ := step.Args["_branches"].(map[string][]normalizer.FlowStep)
		if len(branches) == 0 {
			return "", true
		}
		maxConcurrency := flowIntArg(step.Args, "maxConcurrency", 0)
		if maxConcurrency <= 0 {
			maxConcurrency = flowIntArg(step.Args, "maxParallel", 0)
		}
		if maxConcurrency <= 0 {
			maxConcurrency = 8
		}
		keys := make([]string, 0, len(branches))
		for k := range branches {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		outerDeclared := make(map[string]bool, len(st.declared))
		for k, v := range st.declared {
			outerDeclared[k] = v
		}

		type newVar struct {
			typ   string
			isPtr bool
		}
		newVars := make(map[string]newVar)
		for _, k := range keys {
			probeState := cloneFlowState(st)
			_ = renderFlowSteps(probeState, branches[k], indent+1)
			for varName := range probeState.declared {
				if outerDeclared[varName] {
					continue
				}
				goType := probeState.types[varName]
				if goType == "" {
					goType = "any"
				}
				newVars[varName] = newVar{goType, probeState.pointers[varName]}
			}
		}

		var b strings.Builder
		newVarNames := make([]string, 0, len(newVars))
		for n := range newVars {
			newVarNames = append(newVarNames, n)
		}
		sort.Strings(newVarNames)
		for _, varName := range newVarNames {
			v := newVars[varName]
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, varName, v.typ))
			st.declared[varName] = true
			st.pointers[varName] = v.isPtr
			st.types[varName] = v.typ
		}

		b.WriteString(fmt.Sprintf("%s_pCtx, _pCancel := context.WithCancel(ctx)\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer _pCancel()\n", pad))
		b.WriteString(fmt.Sprintf("%svar _wg sync.WaitGroup\n", pad))
		b.WriteString(fmt.Sprintf("%svar _mu sync.Mutex\n", pad))
		b.WriteString(fmt.Sprintf("%svar _pErr error\n", pad))
		b.WriteString(fmt.Sprintf("%s_pSem := make(chan struct{}, %d)\n", pad, maxConcurrency))
		b.WriteString(fmt.Sprintf("%s_ = _mu\n", pad))

		for _, k := range keys {
			branchSteps := branches[k]
			branchState := cloneFlowState(st)
			branchState.goroutineMode = true
			b.WriteString(fmt.Sprintf("%s_wg.Add(1)\n", pad))
			b.WriteString(fmt.Sprintf("%sgo func() {\n", pad))
			b.WriteString(fmt.Sprintf("%s\tdefer _wg.Done()\n", pad))
			b.WriteString(fmt.Sprintf("%s\t_pSem <- struct{}{}\n", pad))
			b.WriteString(fmt.Sprintf("%s\tdefer func() { <-_pSem }()\n", pad))
			b.WriteString(fmt.Sprintf("%s\tctx := _pCtx // shadow outer ctx; cancelled if sibling fails\n", pad))
			b.WriteString(fmt.Sprintf("%s\t_ = ctx\n", pad))
			b.WriteString(renderFlowSteps(branchState, branchSteps, indent+1))
			b.WriteString(fmt.Sprintf("%s}() // branch: %s\n", pad, k))
		}

		b.WriteString(fmt.Sprintf("%s_wg.Wait()\n", pad))
		b.WriteString(fmt.Sprintf("%sif _pErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "_pErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "pdf.Render":
		tmplName := arg("template")
		data := arg("data")
		output := arg("output")
		if tmplName == "" || data == "" || output == "" {
			return "", true
		}
		_ = tmplName
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]byte"
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s_pdfBytes, _pdfErr := s.reportPDF.GenerateReport(%s)\n", pad, data))
		b.WriteString(fmt.Sprintf("%sif _pdfErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "_pdfErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s _pdfBytes\n", pad, output, assign))
		return b.String(), true

	case "webhook.Send":
		url := arg("url")
		payload := arg("payload")
		if url == "" || payload == "" {
			return "", true
		}
		event := arg("event")
		retries := 3
		if v, ok := step.Args["retries"]; ok {
			switch n := v.(type) {
			case int:
				retries = n
			case int64:
				retries = int(n)
			case float64:
				retries = int(n)
			}
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whBytes, _whMarshalErr := json.Marshal(%s)\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\tif _whMarshalErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"webhook.Send: marshal: %w\", _whMarshalErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whDelay := time.Second\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar _whLastErr error\n", pad))
		b.WriteString(fmt.Sprintf("%s\tfor _retry := 0; _retry <= %d; _retry++ {\n", pad, retries))
		b.WriteString(fmt.Sprintf("%s\t\t_whReq, _whReqErr := http.NewRequestWithContext(ctx, \"POST\", %s, bytes.NewReader(_whBytes))\n", pad, url))
		b.WriteString(fmt.Sprintf("%s\t\tif _whReqErr != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_whLastErr = _whReqErr\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_whReq.Header.Set(\"Content-Type\", \"application/json\")\n", pad))
		if event != "" {
			b.WriteString(fmt.Sprintf("%s\t\t_whReq.Header.Set(\"X-Event-Type\", %s)\n", pad, event))
		}
		b.WriteString(fmt.Sprintf("%s\t\t_whRes, _whCallErr := http.DefaultClient.Do(_whReq)\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _whCallErr == nil && _whRes.StatusCode < 500 {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_whRes.Body.Close()\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_whLastErr = nil\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _whRes != nil { _whRes.Body.Close() }\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _whCallErr != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_whLastErr = _whCallErr\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t} else {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_whLastErr = fmt.Errorf(\"webhook: status %%d\", _whRes.StatusCode)\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif _retry < %d {\n", pad, retries))
		b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(_whDelay)\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t_whDelay *= 2\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _whLastErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"webhook.Send failed: %w\", _whLastErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "webhook.VerifySignature":
		payload := arg("payload")
		signature := arg("signature")
		if payload == "" || signature == "" {
			return "", true
		}
		algorithm := arg("algorithm")
		if algorithm == "" {
			algorithm = `"sha256"`
		}
		secretExpr := arg("secret")
		if secretExpr == "" {
			secretExpr = `os.Getenv("WEBHOOK_SECRET")`
		}
		throwExpr := arg("throw")
		if throwExpr == "" {
			throwExpr = `"invalid webhook signature"`
		}
		output := arg("output")
		strict := true
		if v, ok := step.Args["strict"].(bool); ok {
			strict = v
		}

		assign := ":="
		if output != "" {
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "bool"
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whAlg := strings.ToLower(strings.TrimSpace(%s))\n", pad, algorithm))
		b.WriteString(fmt.Sprintf("%s\tif _whAlg == \"\" { _whAlg = \"sha256\" }\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _whAlg != \"sha256\" {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"webhook.VerifySignature: unsupported algorithm %q\", _whAlg)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whSecret := %s\n", pad, secretExpr))
		b.WriteString(fmt.Sprintf("%s\tif strings.TrimSpace(_whSecret) == \"\" {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "WEBHOOK_SECRET_MISSING", "webhook.VerifySignature requires secret or WEBHOOK_SECRET env")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar _whBody []byte\n", pad))
		b.WriteString(fmt.Sprintf("%s\tswitch _v := any(%s).(type) {\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\tcase []byte:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_whBody = _v\n", pad))
		b.WriteString(fmt.Sprintf("%s\tcase string:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_whBody = []byte(_v)\n", pad))
		b.WriteString(fmt.Sprintf("%s\tdefault:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_whBodyJSON, _whBodyErr := json.Marshal(%s)\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\t\tif _whBodyErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t\t", "fmt.Errorf(\"webhook.VerifySignature: marshal payload: %w\", _whBodyErr)"))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_whBody = _whBodyJSON\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whProvided := strings.TrimSpace(%s)\n", pad, signature))
		b.WriteString(fmt.Sprintf("%s\t_whProvided = strings.TrimPrefix(_whProvided, _whAlg+\"=\")\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whMac := hmac.New(sha256.New, []byte(_whSecret))\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_, _ = _whMac.Write(_whBody)\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whExpected := hex.EncodeToString(_whMac.Sum(nil))\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_whValid := hmac.Equal([]byte(strings.ToLower(_whExpected)), []byte(strings.ToLower(_whProvided)))\n", pad))
		if output != "" {
			b.WriteString(fmt.Sprintf("%s\t%s %s _whValid\n", pad, output, assign))
		}
		if strict {
			b.WriteString(fmt.Sprintf("%s\tif !_whValid {\n", pad))
			b.WriteString(errReturn(st, pad+"\t\t", "errors.New(http.StatusUnauthorized, \"INVALID_WEBHOOK_SIGNATURE\", fmt.Sprint("+throwExpr+"))"))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "webhook.Ack":
		status := flowIntArg(step.Args, "status", 200)
		body := arg("body")
		if body == "" {
			body = `"ok"`
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// webhook.Ack marker: transport should acknowledge with status=%d body=%s\n", pad, status, body))
		return b.String(), true

	case "queue.Enqueue":
		subject := arg("subject")
		payload := arg("payload")
		if subject == "" || payload == "" {
			return "", true
		}
		timeout := arg("timeout")
		if timeout == "" {
			if timeoutMS := flowIntArg(step.Args, "timeoutMs", 0); timeoutMS > 0 {
				timeout = fmt.Sprintf("time.Duration(%d) * time.Millisecond", timeoutMS)
			} else {
				timeout = "3*time.Second"
			}
		}
		queueCtxVar := "_qCtx" + sfx
		queueCancelVar := "_qCancel" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := context.WithTimeout(ctx, %s)\n", pad, queueCtxVar, queueCancelVar, timeout))
		b.WriteString(fmt.Sprintf("%s\tdefer %s()\n", pad, queueCancelVar))
		b.WriteString(fmt.Sprintf("%s\t_qPayload, _qMarshalErr := json.Marshal(%s)\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\tif _qMarshalErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"queue.Enqueue: marshal: %w\", _qMarshalErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _qErr := s.queuePublisher.Enqueue(%s, %s, _qPayload); _qErr != nil {\n", pad, queueCtxVar, subject))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"queue.Enqueue: %w\", _qErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "queue.Dequeue":
		subject := arg("subject")
		output := arg("output")
		if subject == "" || output == "" {
			return "", true
		}
		timeout := arg("timeout")
		if timeout == "" {
			if timeoutMS := flowIntArg(step.Args, "timeoutMs", 0); timeoutMS > 0 {
				timeout = fmt.Sprintf("time.Duration(%d) * time.Millisecond", timeoutMS)
			} else {
				timeout = "3*time.Second"
			}
		}
		ackToken := arg("ackToken")
		attempts := flowIntArg(step.Args, "attempts", 0)
		if attempts <= 0 {
			if retries := flowIntArg(step.Args, "retries", -1); retries >= 0 {
				attempts = retries + 1
			} else {
				attempts = 2
			}
		}
		backoffMS := flowIntArg(step.Args, "backoffMs", 150)
		if backoffMS < 0 {
			backoffMS = 0
		}
		jitterMS := flowIntArg(step.Args, "jitterMs", 50)
		if jitterMS < 0 {
			jitterMS = 0
		}

		outAssign := ":="
		if st.declared[output] {
			outAssign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]byte"

		ackAssign := ":="
		if ackToken != "" {
			if st.declared[ackToken] {
				ackAssign = "="
			}
			st.declared[ackToken] = true
			st.pointers[ackToken] = false
			st.types[ackToken] = "string"
		}

		queueCtxVar := "_qdCtx" + sfx
		queueCancelVar := "_qdCancel" + sfx
		msgIDVar := "_qdMsgID" + sfx
		msgVar := "_qdMsg" + sfx
		errVar := "_qdErr" + sfx
		lastErrVar := "_qdLastErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_qdBackoff := time.Duration(%d) * time.Millisecond\n", pad, backoffMS))
		b.WriteString(fmt.Sprintf("%s\t_qdJitter := time.Duration(%d) * time.Millisecond\n", pad, jitterMS))
		b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, lastErrVar))
		b.WriteString(fmt.Sprintf("%s\tfor _qdTry := 0; _qdTry < %d; _qdTry++ {\n", pad, attempts))
		b.WriteString(fmt.Sprintf("%s\t\t%s, %s := context.WithTimeout(ctx, %s)\n", pad, queueCtxVar, queueCancelVar, timeout))
		b.WriteString(fmt.Sprintf("%s\t\t%s, %s, %s := s.queuePublisher.Dequeue(%s, %s)\n", pad, msgIDVar, msgVar, errVar, queueCtxVar, subject))
		b.WriteString(fmt.Sprintf("%s\t\t%s()\n", pad, queueCancelVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, errVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s %s %s\n", pad, output, outAssign, msgVar))
		if ackToken != "" {
			b.WriteString(fmt.Sprintf("%s\t\t\t%s %s %s\n", pad, ackToken, ackAssign, msgIDVar))
		}
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = nil\n", pad, lastErrVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, lastErrVar, errVar))
		b.WriteString(fmt.Sprintf("%s\t\tif _qdTry < %d-1 {\n", pad, attempts))
		b.WriteString(fmt.Sprintf("%s\t\t\t_qdSleep := _qdBackoff\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tif _qdJitter > 0 {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t_qdJitterN, _qdJitterErr := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(_qdJitter)))\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tif _qdJitterErr == nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\t_qdSleep += time.Duration(_qdJitterN.Int64())\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(_qdSleep)\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, lastErrVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"queue.Dequeue: %%w\", %s)", lastErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "queue.Ack":
		subject := arg("subject")
		messageID := arg("messageID")
		if subject == "" || messageID == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _qAckErr := s.queuePublisher.Ack(ctx, %s, %s); _qAckErr != nil {\n", pad, subject, messageID))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"queue.Ack: %w\", _qAckErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "queue.Nack":
		subject := arg("subject")
		messageID := arg("messageID")
		if subject == "" || messageID == "" {
			return "", true
		}
		reason := arg("reason")
		if reason == "" {
			reason = `"nack"`
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _qNackErr := s.queuePublisher.Nack(ctx, %s, %s, fmt.Sprint(%s)); _qNackErr != nil {\n", pad, subject, messageID, reason))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"queue.Nack: %w\", _qNackErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "dlq.Publish":
		subject := arg("subject")
		payload := arg("payload")
		if subject == "" || payload == "" {
			return "", true
		}
		reason := arg("reason")
		if reason == "" {
			reason = `"unspecified"`
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_dlqPayload, _dlqMarshalErr := json.Marshal(%s)\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\tif _dlqMarshalErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"dlq.Publish: marshal: %w\", _dlqMarshalErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _dlqErr := s.queuePublisher.PublishDLQ(ctx, %s, _dlqPayload, fmt.Sprint(%s)); _dlqErr != nil {\n", pad, subject, reason))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"dlq.Publish: %w\", _dlqErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}
