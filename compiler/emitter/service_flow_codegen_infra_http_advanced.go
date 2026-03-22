package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepInfraHTTPAdvanced(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "http.Request":
		return renderHTTPRequest(st, step, pad, sfx, arg)
	case "http.RetryPolicy":
		return renderHTTPRetryPolicy(st, step, pad, sfx, arg)
	case "http.Paginate":
		return renderHTTPPaginate(st, step, pad, sfx, arg)
	}

	return "", false
}

// writeHTTPAuth appends auth and header setup lines for an HTTP request variable.
func writeHTTPAuth(b *strings.Builder, pad, reqVar string, step normalizer.FlowStep, arg func(string) string) {
	// headers (sorted for deterministic output)
	if hdrs, ok := step.Args["headers"].(map[string]string); ok {
		keys := make([]string, 0, len(hdrs))
		for k := range hdrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, hk := range keys {
			b.WriteString(fmt.Sprintf("%s%s.Header.Set(%q, %s)\n", pad, reqVar, hk, hdrs[hk]))
		}
	}
	// auth: "bearer:TOKEN_EXPR" or "basic:USER_EXPR:PASS_EXPR"
	if authStr := arg("auth"); authStr != "" {
		if strings.HasPrefix(authStr, "bearer:") {
			token := authStr[len("bearer:"):]
			b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Authorization\", \"Bearer \"+%s)\n", pad, reqVar, token))
		} else if strings.HasPrefix(authStr, "basic:") {
			rest := authStr[len("basic:"):]
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) == 2 {
				b.WriteString(fmt.Sprintf("%s%s.SetBasicAuth(%s, %s)\n", pad, reqVar, parts[0], parts[1]))
			}
		}
	}
}

// writeHTTPQuery appends URL query param setup lines using a temporary Values variable.
func writeHTTPQuery(b *strings.Builder, pad, reqVar, qVar string, step normalizer.FlowStep) {
	qp, ok := step.Args["query"].(map[string]string)
	if !ok || len(qp) == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("%s%s := %s.URL.Query()\n", pad, qVar, reqVar))
	keys := make([]string, 0, len(qp))
	for k := range qp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s%s.Set(%q, %s)\n", pad, qVar, k, qp[k]))
	}
	b.WriteString(fmt.Sprintf("%s%s.URL.RawQuery = %s.Encode()\n", pad, reqVar, qVar))
}

// renderHTTPRequest handles http.Request — enriched HTTP call with auth, query params, typed JSON decode.
//
// CUE args:
//
//	method      string   — HTTP method ("GET", "POST", …)
//	url         string   — URL expression
//	body?       string   — body expression (passed to strings.NewReader)
//	headers?    map      — static/dynamic header key→value
//	auth?       string   — "bearer:TOKEN_EXPR" or "basic:USER_EXPR:PASS_EXPR"
//	query?      map      — URL query params key→value expression
//	timeout?    string   — duration expr (default "10*time.Second")
//	into?       string   — Go type name for JSON unmarshal; output var receives this type
//	output?     string   — variable name for result (string if no into, or typed if into set)
//	statusVar?  string   — variable name to store HTTP status code
//	failOnError? bool    — return error on 4xx/5xx (default true)
func renderHTTPRequest(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) (string, bool) {
	method := arg("method")
	url := arg("url")
	if method == "" || url == "" {
		return renderInvalidFlowStepConfig(st, pad, "http.Request", "http.Request requires method and url"), true
	}
	body := arg("body")
	output := arg("output")
	into := arg("into")
	statusVar := arg("statusVar")
	timeout := arg("timeout")
	if timeout == "" {
		timeout = "10*time.Second"
	}
	failOnError := true
	if v, ok := step.Args["failOnError"].(bool); ok {
		failOnError = v
	}

	reqV := "_httpReq" + sfx
	reqErrV := "_httpReqErr" + sfx
	resV := "_httpRes" + sfx
	hErrV := "_hErr" + sfx
	bodyV := "_httpBody" + sfx
	callCtxV := "_httpCallCtx" + sfx
	cancelV := "_httpCancel" + sfx
	qV := "_httpQ" + sfx
	jErrV := "_jErr" + sfx

	bodyExpr := "nil"
	if body != "" {
		bodyExpr = fmt.Sprintf("strings.NewReader(%s)", body)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s// http.Request: %s %s\n", pad, method, url))
	b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, %s)\n", pad, callCtxV, cancelV, timeout))
	b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, %q, %s, %s)\n", pad, reqV, reqErrV, callCtxV, method, url, bodyExpr))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, reqErrV))
	b.WriteString(fmt.Sprintf("%s\t%s()\n", pad, cancelV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http.Request: %w\", "+reqErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	writeHTTPAuth(&b, pad, reqV, step, arg)
	writeHTTPQuery(&b, pad, reqV, qV, step)

	b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, resV, hErrV, reqV))
	b.WriteString(fmt.Sprintf("%s%s()\n", pad, cancelV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, hErrV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http.Request: %w\", "+hErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, bodyV, resV))
	b.WriteString(fmt.Sprintf("%s%s.Body.Close()\n", pad, resV))

	if statusVar != "" {
		assign := ":="
		if st.declared[statusVar] {
			assign = "="
		}
		st.declared[statusVar] = true
		st.pointers[statusVar] = false
		b.WriteString(fmt.Sprintf("%s%s %s %s.StatusCode\n", pad, statusVar, assign, resV))
	}
	if failOnError {
		b.WriteString(fmt.Sprintf("%sif %s.StatusCode >= 400 {\n", pad, resV))
		b.WriteString(errReturn(st, pad+"\t", "errors.New("+resV+".StatusCode, \"HTTP_ERROR\", string("+bodyV+"))"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	if into != "" && output != "" {
		if !st.declared[output] {
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, into))
			st.declared[output] = true
			st.pointers[output] = false
		}
		b.WriteString(fmt.Sprintf("%sif %s := json.Unmarshal(%s, &%s); %s != nil {\n", pad, jErrV, bodyV, output, jErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http.Request: unmarshal: %w\", "+jErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	} else if output != "" {
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, bodyV))
	}
	return b.String(), true
}

// renderHTTPRetryPolicy handles http.RetryPolicy — HTTP call with status-code-aware retry.
//
// CUE args:
//
//	method      string     — HTTP method
//	url         string     — URL expression
//	body?       string     — body expression
//	headers?    map        — request headers
//	auth?       string     — "bearer:TOKEN" or "basic:USER:PASS"
//	timeout?    string     — per-attempt timeout (default "10*time.Second")
//	attempts?   int        — max attempts (default 3)
//	backoffMs?  int        — ms between retries (default 500)
//	retryOn?    []int      — status codes to retry (default [429, 503])
//	output?     string     — variable name for response body string
//	statusVar?  string     — variable name for status code
//	failOnError? bool      — return error on 4xx/5xx after all retries (default true)
func renderHTTPRetryPolicy(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) (string, bool) {
	method := arg("method")
	url := arg("url")
	if method == "" || url == "" {
		return renderInvalidFlowStepConfig(st, pad, "http.RetryPolicy", "http.RetryPolicy requires method and url"), true
	}
	body := arg("body")
	output := arg("output")
	statusVar := arg("statusVar")
	timeout := arg("timeout")
	if timeout == "" {
		timeout = "10*time.Second"
	}
	attempts := flowIntArg(step.Args, "attempts", 3)
	if attempts < 1 {
		attempts = 1
	}
	backoffMs := flowIntArg(step.Args, "backoffMs", 500)
	failOnError := true
	if v, ok := step.Args["failOnError"].(bool); ok {
		failOnError = v
	}

	// Parse retryOn: accepts []int, []int64, []float64, []any
	var retryOn []int
	if v, ok := step.Args["retryOn"]; ok {
		switch rv := v.(type) {
		case []int:
			retryOn = rv
		case []int64:
			for _, n := range rv {
				retryOn = append(retryOn, int(n))
			}
		case []float64:
			for _, n := range rv {
				retryOn = append(retryOn, int(n))
			}
		case []any:
			for _, n := range rv {
				switch nn := n.(type) {
				case int:
					retryOn = append(retryOn, nn)
				case int64:
					retryOn = append(retryOn, int(nn))
				case float64:
					retryOn = append(retryOn, int(nn))
				}
			}
		}
	}
	if len(retryOn) == 0 {
		retryOn = []int{429, 503}
	}

	reqV := "_httpReq" + sfx
	reqErrV := "_httpReqErr" + sfx
	resV := "_httpRes" + sfx
	hErrV := "_hErr" + sfx
	bodyV := "_httpBody" + sfx
	tryV := "_httpTry" + sfx
	callCtxV := "_httpCallCtx" + sfx
	cancelV := "_httpCancel" + sfx
	retryStV := "_retryStatus" + sfx
	qV := "_httpQ" + sfx

	bodyExpr := "nil"
	if body != "" {
		bodyExpr = fmt.Sprintf("strings.NewReader(%s)", body)
	}

	// Build "not retryable" condition: status != 429 && status != 503
	var notParts []string
	for _, code := range retryOn {
		notParts = append(notParts, fmt.Sprintf("%s != %d", retryStV, code))
	}
	notRetryable := strings.Join(notParts, " && ")

	inner := pad + "\t"
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s// http.RetryPolicy: %s %s (retry on %v, max %d attempts)\n", pad, method, url, retryOn, attempts))
	b.WriteString(fmt.Sprintf("%svar %s *http.Response\n", pad, resV))
	b.WriteString(fmt.Sprintf("%svar %s error\n", pad, hErrV))
	b.WriteString(fmt.Sprintf("%sfor %s := 0; %s < %d; %s++ {\n", pad, tryV, tryV, attempts, tryV))

	b.WriteString(fmt.Sprintf("%s%s, %s := context.WithTimeout(ctx, %s)\n", inner, callCtxV, cancelV, timeout))
	b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(%s, %q, %s, %s)\n", inner, reqV, reqErrV, callCtxV, method, url, bodyExpr))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", inner, reqErrV))
	b.WriteString(fmt.Sprintf("%s\t%s()\n", inner, cancelV))
	b.WriteString(errReturn(st, inner+"\t", "fmt.Errorf(\"http.RetryPolicy: request: %w\", "+reqErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", inner))

	writeHTTPAuth(&b, inner, reqV, step, arg)
	writeHTTPQuery(&b, inner, reqV, qV, step)

	b.WriteString(fmt.Sprintf("%s%s, %s = http.DefaultClient.Do(%s)\n", inner, resV, hErrV, reqV))
	b.WriteString(fmt.Sprintf("%s%s()\n", inner, cancelV))
	b.WriteString(fmt.Sprintf("%sif %s == nil && %s != nil {\n", inner, hErrV, resV))
	b.WriteString(fmt.Sprintf("%s\t%s := %s.StatusCode\n", inner, retryStV, resV))
	b.WriteString(fmt.Sprintf("%s\tif %s {\n", inner, notRetryable))
	b.WriteString(fmt.Sprintf("%s\t\tbreak\n", inner))
	b.WriteString(fmt.Sprintf("%s\t}\n", inner))
	b.WriteString(fmt.Sprintf("%s\t%s.Body.Close()\n", inner, resV))
	b.WriteString(fmt.Sprintf("%s}\n", inner))
	if backoffMs > 0 {
		b.WriteString(fmt.Sprintf("%sif %s+1 < %d {\n", inner, tryV, attempts))
		b.WriteString(fmt.Sprintf("%s\ttime.Sleep(%d * time.Millisecond)\n", inner, backoffMs))
		b.WriteString(fmt.Sprintf("%s}\n", inner))
	}
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	// Post-loop: check errors, read body, handle status
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, hErrV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"http.RetryPolicy: %w\", "+hErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, resV))
	b.WriteString(errReturn(st, pad+"\t", "errors.New(502, \"HTTP_ERROR\", \"empty response\")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", pad, bodyV, resV))
	b.WriteString(fmt.Sprintf("%s%s.Body.Close()\n", pad, resV))

	if statusVar != "" {
		assign := ":="
		if st.declared[statusVar] {
			assign = "="
		}
		st.declared[statusVar] = true
		st.pointers[statusVar] = false
		b.WriteString(fmt.Sprintf("%s%s %s %s.StatusCode\n", pad, statusVar, assign, resV))
	}
	if failOnError {
		b.WriteString(fmt.Sprintf("%sif %s.StatusCode >= 400 {\n", pad, resV))
		b.WriteString(errReturn(st, pad+"\t", "errors.New("+resV+".StatusCode, \"HTTP_ERROR\", string("+bodyV+"))"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	if output != "" {
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, bodyV))
	}
	return b.String(), true
}

// renderHTTPPaginate handles http.Paginate — cursor-based pagination loop over an external API.
//
// CUE args:
//
//	url          string   — base URL expression
//	method?      string   — HTTP method (default "GET")
//	into         string   — Go type for each page response (e.g. "PagedResponse")
//	as           string   — variable name for each page (in scope for expressions below)
//	cursor_expr  string   — Go expression to extract next cursor from page (empty = done)
//	items_expr?  string   — Go expression for items slice to accumulate (e.g. "page.Items")
//	cursor_param? string  — query param name for cursor (default "cursor")
//	output?      string   — variable name for accumulated items
//	output_type? string   — Go type for output slice (default "[]any")
//	max_pages?   int      — safety limit on iterations (default 100)
//	headers?     map      — request headers
//	auth?        string   — "bearer:TOKEN" or "basic:USER:PASS"
func renderHTTPPaginate(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) (string, bool) {
	url := arg("url")
	into := arg("into")
	asVar := arg("as")
	cursorExpr := arg("cursor_expr")
	if url == "" || into == "" || asVar == "" || cursorExpr == "" {
		return renderInvalidFlowStepConfig(st, pad, "http.Paginate", "http.Paginate requires url, into, as, and cursor_expr"), true
	}
	itemsExpr := arg("items_expr")
	cursorParam := arg("cursor_param")
	output := arg("output")
	outputType := arg("output_type")
	method := arg("method")
	if method == "" {
		method = "GET"
	}
	if cursorParam == "" {
		cursorParam = "cursor"
	}
	if outputType == "" {
		outputType = "[]any"
	}
	maxPages := flowIntArg(step.Args, "max_pages", 100)
	if maxPages < 1 {
		maxPages = 1
	}

	uV := "_u" + sfx
	uErrV := "_uErr" + sfx
	qV := "_pq" + sfx
	reqV := "_httpReq" + sfx
	reqErrV := "_httpReqErr" + sfx
	resV := "_httpRes" + sfx
	hErrV := "_hErr" + sfx
	bodyV := "_httpBody" + sfx
	jErrV := "_jErr" + sfx
	cursorV := "_cursor" + sfx
	pageNV := "_pageN" + sfx
	inner := pad + "\t"

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s// http.Paginate: %s %s\n", pad, method, url))
	if output != "" && !st.declared[output] {
		b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, outputType))
		st.declared[output] = true
		st.pointers[output] = false
	}
	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, cursorV))
	b.WriteString(fmt.Sprintf("%sfor %s := 0; %s < %d; %s++ {\n", pad, pageNV, pageNV, maxPages, pageNV))

	b.WriteString(fmt.Sprintf("%s%s, %s := url.Parse(%s)\n", inner, uV, uErrV, url))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", inner, uErrV))
	b.WriteString(errReturn(st, inner+"\t", "fmt.Errorf(\"http.Paginate: url: %w\", "+uErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", inner))
	b.WriteString(fmt.Sprintf("%s%s := %s.Query()\n", inner, qV, uV))
	b.WriteString(fmt.Sprintf("%sif %s != \"\" {\n", inner, cursorV))
	b.WriteString(fmt.Sprintf("%s\t%s.Set(%q, %s)\n", inner, qV, cursorParam, cursorV))
	b.WriteString(fmt.Sprintf("%s}\n", inner))
	b.WriteString(fmt.Sprintf("%s%s.RawQuery = %s.Encode()\n", inner, uV, qV))

	b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(ctx, %q, %s.String(), nil)\n", inner, reqV, reqErrV, method, uV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", inner, reqErrV))
	b.WriteString(errReturn(st, inner+"\t", "fmt.Errorf(\"http.Paginate: request: %w\", "+reqErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", inner))

	writeHTTPAuth(&b, inner, reqV, step, arg)

	b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", inner, resV, hErrV, reqV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", inner, hErrV))
	b.WriteString(errReturn(st, inner+"\t", "fmt.Errorf(\"http.Paginate: %w\", "+hErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", inner))
	b.WriteString(fmt.Sprintf("%s%s, _ := io.ReadAll(%s.Body)\n", inner, bodyV, resV))
	b.WriteString(fmt.Sprintf("%s%s.Body.Close()\n", inner, resV))
	b.WriteString(fmt.Sprintf("%sif %s.StatusCode >= 400 {\n", inner, resV))
	b.WriteString(errReturn(st, inner+"\t", "errors.New("+resV+".StatusCode, \"HTTP_ERROR\", string("+bodyV+"))"))
	b.WriteString(fmt.Sprintf("%s}\n", inner))

	b.WriteString(fmt.Sprintf("%svar %s %s\n", inner, asVar, into))
	b.WriteString(fmt.Sprintf("%sif %s := json.Unmarshal(%s, &%s); %s != nil {\n", inner, jErrV, bodyV, asVar, jErrV))
	b.WriteString(errReturn(st, inner+"\t", "fmt.Errorf(\"http.Paginate: unmarshal: %w\", "+jErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", inner))

	if itemsExpr != "" && output != "" {
		b.WriteString(fmt.Sprintf("%s%s = append(%s, %s...)\n", inner, output, output, itemsExpr))
	}

	b.WriteString(fmt.Sprintf("%s%s = %s\n", inner, cursorV, cursorExpr))
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", inner, cursorV))
	b.WriteString(fmt.Sprintf("%s\tbreak\n", inner))
	b.WriteString(fmt.Sprintf("%s}\n", inner))

	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String(), true
}
