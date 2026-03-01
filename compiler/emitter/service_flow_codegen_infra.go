package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepInfra(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "cache.Get":
		key := arg("key")
		output := arg("output")
		if key == "" || output == "" {
			return ""
		}
		optional := true
		if v, ok := step.Args["optional"].(bool); ok {
			optional = v
		}
		var b strings.Builder
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		cacheRawV, cacheErrV := "_cacheRaw"+sfx, "_cacheErr"+sfx
		if optional {
			if assign == ":=" {
				b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, output))
			}
			b.WriteString(fmt.Sprintf("%s%s, %s := s.cache.Get(ctx, %s).Result()\n", pad, cacheRawV, cacheErrV, key))
			b.WriteString(fmt.Sprintf("%sif %s != nil && !errors.Is(%s, redis.Nil) {\n", pad, cacheErrV, cacheErrV))
			b.WriteString(errReturn(st, pad+"\t", cacheErrV))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sif %s == nil { %s = %s }\n", pad, cacheErrV, output, cacheRawV))
		} else {
			if assign == ":=" {
				b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, output))
			}
			b.WriteString(fmt.Sprintf("%s%s, %s := s.cache.Get(ctx, %s).Result()\n", pad, cacheRawV, cacheErrV, key))
			b.WriteString(fmt.Sprintf("%sif %s != nil && !errors.Is(%s, redis.Nil) {\n", pad, cacheErrV, cacheErrV))
			b.WriteString(errReturn(st, pad+"\t", cacheErrV))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n%s\t%s = %s\n%s}\n", pad, cacheErrV, pad, output, cacheRawV, pad))
		}
		return b.String()

	case "cache.Set":
		key := arg("key")
		value := arg("value")
		ttl := arg("ttl")
		if key == "" || value == "" {
			return ""
		}
		if ttl == "" {
			ttl = "0"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _cErr := s.cache.Set(ctx, %s, %s, %s).Err(); _cErr != nil {\n", pad, key, value, ttl))
		b.WriteString(errReturn(st, pad+"\t", "_cErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "cache.Del":
		key := arg("key")
		if key == "" {
			return ""
		}
		return fmt.Sprintf("%s_ = s.cache.Del(ctx, %s).Err()\n", pad, key)

	case "mail.Send":
		to := arg("to")
		subject := arg("subject")
		body := arg("body")
		if to == "" || subject == "" || body == "" {
			return ""
		}
		html := arg("html")
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _mErr := s.mailer.Send(ctx, port.EmailMessage{To: %s, Subject: %s, Text: %s", pad, to, subject, body))
		if html != "" {
			b.WriteString(fmt.Sprintf(", HTML: %s", html))
		}
		b.WriteString(fmt.Sprintf("}); _mErr != nil {\n"))
		b.WriteString(errReturn(st, pad+"\t", "_mErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "storage.Upload":
		key := arg("key")
		data := arg("data")
		output := arg("output")
		contentType := arg("contentType")
		if key == "" || data == "" {
			return ""
		}
		if contentType == "" {
			contentType = `"application/octet-stream"`
		}
		// data may be []byte or string; normalize to []byte first, then bytes.NewReader
		readerExpr := "_sUpReader" + sfx
		dataBytesVar := "_sUpDataBytes" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s []byte\n", pad, dataBytesVar))
		b.WriteString(fmt.Sprintf("%sswitch _v := any(%s).(type) {\n", pad, data))
		b.WriteString(fmt.Sprintf("%scase []byte:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = _v\n", pad, dataBytesVar))
		b.WriteString(fmt.Sprintf("%scase string:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = []byte(_v)\n", pad, dataBytesVar))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"storage.Upload: data must be string or []byte\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := bytes.NewReader(%s)\n", pad, readerExpr, dataBytesVar))
		if output == "" {
			sErrV := "_sErr" + sfx
			b.WriteString(fmt.Sprintf("%sif _, %s := s.storage.Upload(ctx, %s, %s, %s); %s != nil {\n", pad, sErrV, key, readerExpr, contentType, sErrV))
			b.WriteString(errReturn(st, pad+"\t", sErrV))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String()
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		b.WriteString(fmt.Sprintf("%s%s %s \"\"\n", pad, output, assign))
		sUpURLV, sUpErrV := "_sUpURL"+sfx, "_sUpErr"+sfx
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.Upload(ctx, %s, %s, %s)\n", pad, sUpURLV, sUpErrV, key, readerExpr, contentType))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sUpErrV))
		b.WriteString(errReturn(st, pad+"\t", sUpErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, output, sUpURLV))
		return b.String()

	case "storage.Download":
		key := arg("key")
		output := arg("output")
		if key == "" || output == "" {
			return ""
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "io.ReadCloser"
		var b strings.Builder
		sDlRCV, sDlErrV, sDlBytesV := "_sDlRC"+sfx, "_sDlErr"+sfx, "_sDlBytes"+sfx
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.Download(ctx, %s)\n", pad, sDlRCV, sDlErrV, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sDlErrV))
		b.WriteString(errReturn(st, pad+"\t", sDlErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Close()\n", pad, sDlRCV))
		b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s)\n", pad, sDlBytesV, sDlRCV))
		if assign == ":=" {
			b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, output, sDlBytesV))
		} else {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, output, sDlBytesV))
		}
		st.types[output] = "[]byte"
		return b.String()

	case "storage.GetURL":
		key := arg("key")
		output := arg("output")
		if key == "" || output == "" {
			return ""
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		var b strings.Builder
		sURLV, sErrV := "_sURL"+sfx, "_sErr"+sfx
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.GetURL(ctx, %s)\n", pad, sURLV, sErrV, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sErrV))
		b.WriteString(errReturn(st, pad+"\t", sErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, sURLV))
		return b.String()

	// -------------------------------------------------------------------------
	// STAGE 3: New capabilities
	// -------------------------------------------------------------------------

	case "http.Call":
		if out, ok := renderFlowHTTPCallAST(st, step, indent, sfx, arg); ok {
			return out
		}
		method := arg("method")
		url := arg("url")
		body := arg("body")
		output := arg("output")
		statusVar := arg("statusVar")
		if method == "" || url == "" {
			return ""
		}
		failOnError := true
		if v, ok := step.Args["failOnError"].(bool); ok {
			failOnError = v
		}
		httpReqV, httpReqErrV := "_httpReq"+sfx, "_httpReqErr"+sfx
		httpResV, httpErrV := "_httpRes"+sfx, "_hErr"+sfx
		httpBodyV := "_httpBody" + sfx
		var b strings.Builder
		bodyExpr := "nil"
		if body != "" {
			bodyExpr = fmt.Sprintf("strings.NewReader(%s)", body)
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(ctx, %q, %s, %s)\n", pad, httpReqV, httpReqErrV, method, url, bodyExpr))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, httpReqErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http: request: %w\", "+httpReqErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		// headers
		if hdrs, ok := step.Args["headers"].(map[string]string); ok {
			for hk, hv := range hdrs {
				b.WriteString(fmt.Sprintf("%s%s.Header.Set(%q, %s)\n", pad, httpReqV, hk, hv))
			}
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, httpResV, httpErrV, httpReqV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, httpErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http: %w\", "+httpErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, httpResV))
		b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, httpBodyV, httpResV))
		if statusVar != "" {
			assign := ":="
			if st.declared[statusVar] {
				assign = "="
			}
			st.declared[statusVar] = true
			st.pointers[statusVar] = false
			b.WriteString(fmt.Sprintf("%s%s %s %s.StatusCode\n", pad, statusVar, assign, httpResV))
		}
		if failOnError {
			b.WriteString(fmt.Sprintf("%sif %s.StatusCode >= 400 {\n", pad, httpResV))
			b.WriteString(errReturn(st, pad+"\t", "errors.New("+httpResV+".StatusCode, \"HTTP_ERROR\", string("+httpBodyV+"))"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, httpBodyV))
		}
		return b.String()

	case "rand.Code":
		output := arg("output")
		if output == "" {
			return ""
		}
		length := 6
		if v, ok := step.Args["length"]; ok {
			switch n := v.(type) {
			case int:
				length = n
			case int64:
				length = int(n)
			case float64:
				length = int(n)
			}
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		modBase := "_codeBase" + sfx
		codeNVar := "_codeN" + sfx
		codeBufVar := "_codeBuf" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := 1\n", pad, modBase))
		b.WriteString(fmt.Sprintf("%sfor _i := 0; _i < %d; _i++ { %s *= 10 }\n", pad, length, modBase))
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, 8)\n", pad, codeBufVar))
		b.WriteString(fmt.Sprintf("%sif _, _cErr := cryptorand.Read(%s); _cErr != nil {\n", pad, codeBufVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"rand.Code: %w\", _cErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := int(binary.BigEndian.Uint64(%s) %% uint64(%s))\n", pad, codeNVar, codeBufVar, modBase))
		b.WriteString(fmt.Sprintf("%s%s %s fmt.Sprintf(\"%%0%dd\", %s)\n", pad, output, assign, length, codeNVar))
		return b.String()

	case "rand.Token":
		output := arg("output")
		if output == "" {
			return ""
		}
		nbytes := 32
		if v, ok := step.Args["bytes"]; ok {
			switch n := v.(type) {
			case int:
				nbytes = n
			case int64:
				nbytes = int(n)
			case float64:
				nbytes = int(n)
			}
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		rbv, rerrv := "_rb"+sfx, "_rbErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, %d)\n", pad, rbv, nbytes))
		b.WriteString(fmt.Sprintf("%s_, %s := cryptorand.Read(%s)\n", pad, rerrv, rbv))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rerrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"rand.Token: %w\", "+rerrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s hex.EncodeToString(%s)\n", pad, output, assign, rbv))
		return b.String()

	case "str.Format":
		tmpl := arg("template")
		output := arg("output")
		if tmpl == "" || output == "" {
			return ""
		}
		var fmtArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				fmtArgs = x
			case string:
				if x != "" {
					fmtArgs = []string{x}
				}
			}
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		callArgs := tmpl
		if len(fmtArgs) > 0 {
			callArgs += ", " + strings.Join(fmtArgs, ", ")
		}
		return fmt.Sprintf("%s%s %s fmt.Sprintf(%s)\n", pad, output, assign, callArgs)

	case "json.Parse":
		input := arg("input")
		into := arg("into")
		output := arg("output")
		if input == "" || into == "" || output == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, into))
		b.WriteString(fmt.Sprintf("%sif _jErr := json.Unmarshal([]byte(%s), &%s); _jErr != nil {\n", pad, input, output))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"json: %w\", _jErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		st.declared[output] = true
		st.pointers[output] = false
		return b.String()

	case "json.Marshal":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return ""
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		jbv, jerrv := "_jb"+sfx, "_jErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := json.Marshal(%s)\n", pad, jbv, jerrv, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, jerrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"json: %w\", "+jerrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, jbv))
		return b.String()

	case "parallel.Run":
		branches, _ := step.Args["_branches"].(map[string][]normalizer.FlowStep)
		if len(branches) == 0 {
			return ""
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

		// Snapshot of currently declared vars (before parallel block)
		outerDeclared := make(map[string]bool, len(st.declared))
		for k, v := range st.declared {
			outerDeclared[k] = v
		}

		// Dry-run each branch to collect newly declared variables + their types
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

		// Pre-declare new vars in outer state and emit declarations
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
		b.WriteString(fmt.Sprintf("%sif _pErr != nil {\n%s\treturn resp, _pErr\n%s}\n", pad, pad, pad))
		return b.String()

	case "pdf.Render":
		tmplName := arg("template")
		data := arg("data")
		output := arg("output")
		if tmplName == "" || data == "" || output == "" {
			return ""
		}
		_ = tmplName // template name is validated but the generator uses data directly
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]byte"
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s_pdfBytes, _pdfErr := s.reportPDF.GenerateTenderReport(%s)\n", pad, data))
		b.WriteString(fmt.Sprintf("%sif _pdfErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "_pdfErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s _pdfBytes\n", pad, output, assign))
		return b.String()

	case "webhook.Send":
		url := arg("url")
		payload := arg("payload")
		if url == "" || payload == "" {
			return ""
		}
		event := arg("event") // optional X-Event-Type header
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
		return b.String()

	case "queue.Enqueue":
		subject := arg("subject")
		payload := arg("payload")
		if subject == "" || payload == "" {
			return ""
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
		return b.String()

	default:
		return ""
	}
}
