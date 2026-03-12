package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func flowStringOrListExpr(v any) string {
	switch raw := v.(type) {
	case string:
		return normalizeFlowExpr(strings.TrimSpace(raw))
	case []string:
		if len(raw) == 0 {
			return ""
		}
		parts := make([]string, 0, len(raw))
		for _, item := range raw {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			parts = append(parts, flowQuoteOrExpr(item))
		}
		if len(parts) == 0 {
			return ""
		}
		return "[]string{" + strings.Join(parts, ", ") + "}"
	case []any:
		if len(raw) == 0 {
			return ""
		}
		parts := make([]string, 0, len(raw))
		for _, item := range raw {
			parts = append(parts, flowQuoteOrExpr(fmt.Sprint(item)))
		}
		return "[]string{" + strings.Join(parts, ", ") + "}"
	default:
		return ""
	}
}

func flowQuoteOrExpr(v string) string {
	s := normalizeFlowExpr(strings.TrimSpace(v))
	if s == "" {
		return `""`
	}
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) || (strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`")) {
		return s
	}
	if flowLooksLikeIdentifierPath(s) {
		return s
	}
	return fmt.Sprintf("%q", s)
}

func flowLooksLikeIdentifierPath(s string) bool {
	if s == "" {
		return false
	}
	partLen := 0
	for i, r := range s {
		if r == '.' {
			if partLen == 0 || i == len(s)-1 {
				return false
			}
			partLen = 0
			continue
		}
		if partLen == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
		partLen++
	}
	return partLen > 0
}

func renderFlowStepEventOrchestration(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "notify.Send", "notify.Email":
		channel := arg("channel")
		to := arg("to")
		templateExpr := arg("template")
		textExpr := arg("text")
		subjectExpr := arg("subject")
		htmlExpr := arg("html")
		dataExpr := arg("data")
		output := arg("output")
		if step.Action == "notify.Email" {
			channel = `"email"`
		}
		if channel == "" || to == "" {
			return "", true
		}
		if templateExpr == "" && textExpr == "" {
			return "", true
		}
		declareOutput := output != "" && !st.declared[output]
		if declareOutput {
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "string"
		}

		var b strings.Builder
		if output != "" {
			assign := "="
			if declareOutput {
				assign = ":="
			}
			b.WriteString(fmt.Sprintf("%s%s %s \"\"\n", pad, output, assign))
		}
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_notifyChannel%s := strings.ToLower(strings.TrimSpace(fmt.Sprint(%s)))\n", pad, sfx, channel))
		b.WriteString(fmt.Sprintf("%s\tif _notifyChannel%s == \"\" {\n", pad, sfx))
		if step.Action == "notify.Email" {
			b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusBadRequest, "INVALID_NOTIFY_CHANNEL", "notify.Email requires non-empty recipient")`))
		} else {
			b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusBadRequest, "INVALID_NOTIFY_CHANNEL", "notify.Send requires non-empty channel")`))
		}
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_notifyMeta%s := map[string]any{\"to\": fmt.Sprint(%s)}\n", pad, sfx, to))
		if textExpr != "" {
			b.WriteString(fmt.Sprintf("%s\t_notifyMeta%s[\"text\"] = fmt.Sprint(%s)\n", pad, sfx, textExpr))
		}
		if subjectExpr != "" {
			b.WriteString(fmt.Sprintf("%s\t_notifyMeta%s[\"subject\"] = fmt.Sprint(%s)\n", pad, sfx, subjectExpr))
		}
		if htmlExpr != "" {
			b.WriteString(fmt.Sprintf("%s\t_notifyMeta%s[\"html\"] = fmt.Sprint(%s)\n", pad, sfx, htmlExpr))
		}
		b.WriteString(fmt.Sprintf("%s\t_notifyMsg%s := port.NotificationMessage{Channels: []string{_notifyChannel%s}, Metadata: _notifyMeta%s", pad, sfx, sfx, sfx))
		if templateExpr != "" {
			b.WriteString(fmt.Sprintf(", Template: fmt.Sprint(%s)", templateExpr))
		}
		if dataExpr != "" {
			b.WriteString(fmt.Sprintf(", Payload: %s", dataExpr))
		}
		b.WriteString("}\n")
		b.WriteString(fmt.Sprintf("%s\tif s.dispatcher == nil {\n", pad))
		if step.Action == "notify.Email" {
			b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "NOTIFY_DISPATCHER_NOT_CONFIGURED", "notify.Email requires notification dispatcher wiring")`))
		} else {
			b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "NOTIFY_DISPATCHER_NOT_CONFIGURED", "notify.Send requires notification dispatcher wiring")`))
		}
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _notifyErr%s := s.dispatcher.Dispatch(ctx, _notifyMsg%s); _notifyErr%s != nil {\n", pad, sfx, sfx, sfx))
		if step.Action == "notify.Email" {
			b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"notify.Email: %%w\", _notifyErr%s)", sfx)))
		} else {
			b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"notify.Send: %%w\", _notifyErr%s)", sfx)))
		}
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		if output != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = uuid.NewString()\n", pad, output))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "approval.Request":
		approvalKey := arg("approvalKey")
		title := arg("title")
		requestedBy := arg("requestedBy")
		approvers := arg("approvers")
		if approvers == "" {
			approvers = flowStringOrListExpr(step.Args["approvers"])
		}
		policy := arg("policy")
		payload := arg("payload")
		description := arg("description")
		deadline := arg("deadline")
		ttl := arg("ttl")
		approvalIDOut := arg("approvalId")
		statusOut := arg("status")
		if approvalKey == "" || title == "" || requestedBy == "" || approvers == "" || policy == "" || payload == "" {
			return "", true
		}
		if approvalIDOut != "" && !st.declared[approvalIDOut] {
			st.declared[approvalIDOut] = true
			st.pointers[approvalIDOut] = false
			st.types[approvalIDOut] = "string"
		}
		if statusOut != "" && !st.declared[statusOut] {
			st.declared[statusOut] = true
			st.pointers[statusOut] = false
			st.types[statusOut] = "string"
		}

		keyVar := "_approvalKey" + sfx
		mapKeyVar := "_approvalMapKey" + sfx
		idVar := "_approvalID" + sfx
		statusVar := "_approvalStatus" + sfx
		ttlVar := "_approvalTTL" + sfx
		mapRawVar := "_approvalMapRaw" + sfx
		mapErrVar := "_approvalMapErr" + sfx
		rawVar := "_approvalRaw" + sfx
		getErrVar := "_approvalGetErr" + sfx
		recordVar := "_approvalRecord" + sfx
		marshalErrVar := "_approvalMarshalErr" + sfx
		blobVar := "_approvalBlob" + sfx
		setErrVar := "_approvalSetErr" + sfx
		mapSetErrVar := "_approvalMapSetErr" + sfx
		existingVar := "_approvalExisting" + sfx
		deadlineVar := "_approvalDeadline" + sfx
		deadlineErrVar := "_approvalDeadlineErr" + sfx
		durVar := "_approvalDur" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif s.stateStore == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "STATE_STORE_NOT_CONFIGURED", "approval.Request requires state store wiring")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := strings.TrimSpace(fmt.Sprint(%s))\n", pad, keyVar, approvalKey))
		b.WriteString(fmt.Sprintf("%s\tif %s == \"\" {\n", pad, keyVar))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusBadRequest, "INVALID_APPROVAL_KEY", "approval.Request requires non-empty approvalKey")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := \"approval:key:\" + %s\n", pad, mapKeyVar, keyVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", pad, idVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"pending\"\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t%s := 7 * 24 * time.Hour\n", pad, ttlVar))
		if ttl != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, ttlVar, ttl))
		}
		if deadline != "" {
			b.WriteString(fmt.Sprintf("%s\t%s, %s := time.Parse(time.RFC3339, strings.TrimSpace(fmt.Sprint(%s)))\n", pad, deadlineVar, deadlineErrVar, deadline))
			b.WriteString(fmt.Sprintf("%s\tif %s == nil {\n", pad, deadlineErrVar))
			b.WriteString(fmt.Sprintf("%s\t\t%s := time.Until(%s)\n", pad, durVar, deadlineVar))
			b.WriteString(fmt.Sprintf("%s\t\tif %s > 0 {\n", pad, durVar))
			b.WriteString(fmt.Sprintf("%s\t\t\t%s = %s\n", pad, ttlVar, durVar))
			b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%s\t%s, %s := s.stateStore.Get(ctx, %s)\n", pad, mapRawVar, mapErrVar, mapKeyVar))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, mapErrVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Request: lookup key: %%w\", %s)", mapErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, mapRawVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s = strings.TrimSpace(string(%s))\n", pad, idVar, mapRawVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != \"\" {\n", pad, idVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s, %s := s.stateStore.Get(ctx, \"approval:id:\"+%s)\n", pad, rawVar, getErrVar, idVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tif %s != nil {\n", pad, getErrVar))
		b.WriteString(errReturn(st, pad+"\t\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Request: load existing: %%w\", %s)", getErrVar)))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tif %s != nil {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tvar %s map[string]any\n", pad, existingVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, existingVar))
		b.WriteString(errReturn(st, pad+"\t\t\t\t\t", "fmt.Errorf(\"approval.Request: unmarshal existing: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tif _v, _ok := %s[\"status\"]; _ok {\n", pad, existingVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\t%s = strings.TrimSpace(fmt.Sprint(_v))\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s == \"\" {\n", pad, idVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s = uuid.NewString()\n", pad, idVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s := map[string]any{\n", pad, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"id\": %s,\n", pad, idVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"approvalKey\": %s,\n", pad, keyVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"title\": %s,\n", pad, title))
		if description != "" {
			b.WriteString(fmt.Sprintf("%s\t\t\t\"description\": %s,\n", pad, description))
		}
		b.WriteString(fmt.Sprintf("%s\t\t\t\"requestedBy\": %s,\n", pad, requestedBy))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"approvers\": %s,\n", pad, approvers))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"policy\": %s,\n", pad, policy))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"payload\": %s,\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"status\": \"pending\",\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\"createdAt\": time.Now().UTC().Format(time.RFC3339),\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s, %s := json.Marshal(%s)\n", pad, blobVar, marshalErrVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != nil {\n", pad, marshalErrVar))
		b.WriteString(errReturn(st, pad+"\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Request: marshal: %%w\", %s)", marshalErrVar)))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s := s.stateStore.Set(ctx, \"approval:id:\"+%s, %s, %s)\n", pad, setErrVar, idVar, blobVar, ttlVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != nil {\n", pad, setErrVar))
		b.WriteString(errReturn(st, pad+"\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Request: save record: %%w\", %s)", setErrVar)))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s := s.stateStore.Set(ctx, %s, []byte(%s), %s)\n", pad, mapSetErrVar, mapKeyVar, idVar, ttlVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != nil {\n", pad, mapSetErrVar))
		b.WriteString(errReturn(st, pad+"\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Request: save key map: %%w\", %s)", mapSetErrVar)))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		if approvalIDOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, approvalIDOut, idVar))
		}
		if statusOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, statusOut, statusVar))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "approval.Wait":
		approvalID := arg("approvalId")
		timeout := arg("timeout")
		timeoutMode := arg("onTimeout")
		timeoutSteps := child("_onTimeout")
		decisionOut := arg("decision")
		statusOut := arg("status")
		decidedByOut := arg("decidedBy")
		decidedAtOut := arg("decidedAt")
		reasonOut := arg("reason")
		if approvalID == "" {
			return "", true
		}
		if timeout == "" {
			timeout = "15 * time.Minute"
		}
		if timeoutMode == "" {
			if len(timeoutSteps) > 0 {
				timeoutMode = `"fallback"`
			} else {
				timeoutMode = `"reject"`
			}
		}

		declareOut := map[string]bool{}
		for _, out := range []string{decisionOut, statusOut, decidedByOut, decidedAtOut, reasonOut} {
			if out == "" || st.declared[out] {
				continue
			}
			declareOut[out] = true
			st.declared[out] = true
			st.pointers[out] = false
			st.types[out] = "string"
		}

		keyVar := "_approvalWaitKey" + sfx
		ctxVar := "_approvalWaitCtx" + sfx
		cancelVar := "_approvalWaitCancel" + sfx
		tickerVar := "_approvalWaitTicker" + sfx
		rawVar := "_approvalWaitRaw" + sfx
		errVar := "_approvalWaitErr" + sfx
		recordVar := "_approvalWaitRecord" + sfx
		modeVar := "_approvalWaitMode" + sfx
		decisionVar := "_approvalWaitDecision" + sfx
		statusVar := "_approvalWaitStatus" + sfx
		decidedByVar := "_approvalWaitDecidedBy" + sfx
		decidedAtVar := "_approvalWaitDecidedAt" + sfx
		reasonVar := "_approvalWaitReason" + sfx
		persistBlobVar := "_approvalWaitPersistBlob" + sfx
		persistMarshalErrVar := "_approvalWaitPersistMarshalErr" + sfx
		persistSetErrVar := "_approvalWaitPersistSetErr" + sfx
		doneLabel := "_approvalWaitDone" + sfx

		var b strings.Builder
		for _, out := range []string{decisionOut, statusOut, decidedByOut, decidedAtOut, reasonOut} {
			if declareOut[out] {
				b.WriteString(fmt.Sprintf("%svar %s string\n", pad, out))
			}
		}
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif s.stateStore == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "STATE_STORE_NOT_CONFIGURED", "approval.Wait requires state store wiring")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := \"approval:id:\" + strings.TrimSpace(fmt.Sprint(%s))\n", pad, keyVar, approvalID))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := context.WithTimeout(ctx, %s)\n", pad, ctxVar, cancelVar, timeout))
		b.WriteString(fmt.Sprintf("%s\tdefer %s()\n", pad, cancelVar))
		b.WriteString(fmt.Sprintf("%s\t%s := time.NewTicker(500 * time.Millisecond)\n", pad, tickerVar))
		b.WriteString(fmt.Sprintf("%s\tdefer %s.Stop()\n", pad, tickerVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"pending\"\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", pad, decidedByVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", pad, decidedAtVar))
		b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", pad, reasonVar))
		b.WriteString(fmt.Sprintf("%s\t%s:\n", pad, doneLabel))
		b.WriteString(fmt.Sprintf("%s\tfor {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s, %s := s.stateStore.Get(%s, %s)\n", pad, rawVar, errVar, ctxVar, keyVar))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Wait: load: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tif %s != nil {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tvar %s map[string]any\n", pad, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, recordVar))
		b.WriteString(errReturn(st, pad+"\t\t\t\t", "fmt.Errorf(\"approval.Wait: unmarshal: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = strings.ToLower(strings.TrimSpace(fmt.Sprint(%s[\"status\"])))\n", pad, statusVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = strings.ToLower(strings.TrimSpace(fmt.Sprint(%s[\"decision\"])))\n", pad, decisionVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tif %s == \"\" {\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = %s\n", pad, decisionVar, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = strings.TrimSpace(fmt.Sprint(%s[\"decidedBy\"]))\n", pad, decidedByVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = strings.TrimSpace(fmt.Sprint(%s[\"decidedAt\"]))\n", pad, decidedAtVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s = strings.TrimSpace(fmt.Sprint(%s[\"reason\"]))\n", pad, reasonVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tif %s == \"approved\" || %s == \"rejected\" || %s == \"timed_out\" || %s == \"approved\" || %s == \"rejected\" || %s == \"timed_out\" {\n", pad, statusVar, statusVar, statusVar, decisionVar, decisionVar, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\tbreak %s\n", pad, doneLabel))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tselect {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\tcase <-%s.Done():\n", pad, ctxVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s := strings.Trim(strings.ToLower(strings.TrimSpace(fmt.Sprint(%s))), \"\\\"\")\n", pad, modeVar, timeoutMode))
		b.WriteString(fmt.Sprintf("%s\t\t\tswitch %s {\n", pad, modeVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tcase \"auto-approve\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"approved\"\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"approved\"\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"system\"\n", pad, decidedByVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = time.Now().UTC().Format(time.RFC3339)\n", pad, decidedAtVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"approval auto-approved on timeout\"\n", pad, reasonVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tcase \"fallback\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"timed_out\"\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"timed_out\"\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"system\"\n", pad, decidedByVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = time.Now().UTC().Format(time.RFC3339)\n", pad, decidedAtVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"approval wait timeout\"\n", pad, reasonVar))
		if len(timeoutSteps) > 0 {
			b.WriteString(renderFlowSteps(cloneFlowState(st), timeoutSteps, indent+4))
		}
		b.WriteString(fmt.Sprintf("%s\t\t\tcase \"\", \"reject\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"rejected\"\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"rejected\"\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"system\"\n", pad, decidedByVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = time.Now().UTC().Format(time.RFC3339)\n", pad, decidedAtVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"approval wait timeout\"\n", pad, reasonVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tdefault:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"timed_out\"\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"timed_out\"\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"system\"\n", pad, decidedByVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = time.Now().UTC().Format(time.RFC3339)\n", pad, decidedAtVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t%s = \"approval wait timeout\"\n", pad, reasonVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s := map[string]any{\n", pad, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\"id\": strings.TrimPrefix(%s, \"approval:id:\"),\n", pad, keyVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\"status\": %s,\n", pad, statusVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\"decision\": %s,\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\"decidedBy\": %s,\n", pad, decidedByVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\"decidedAt\": %s,\n", pad, decidedAtVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t\t\"reason\": %s,\n", pad, reasonVar))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s, %s := json.Marshal(%s)\n", pad, persistBlobVar, persistMarshalErrVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tif %s != nil {\n", pad, persistMarshalErrVar))
		b.WriteString(errReturn(st, pad+"\t\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Wait: timeout marshal: %%w\", %s)", persistMarshalErrVar)))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\t%s := s.stateStore.Set(ctx, %s, %s, 7*24*time.Hour)\n", pad, persistSetErrVar, keyVar, persistBlobVar))
		b.WriteString(fmt.Sprintf("%s\t\t\tif %s != nil {\n", pad, persistSetErrVar))
		b.WriteString(errReturn(st, pad+"\t\t\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Wait: timeout persist: %%w\", %s)", persistSetErrVar)))
		b.WriteString(fmt.Sprintf("%s\t\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t\tbreak %s\n", pad, doneLabel))
		b.WriteString(fmt.Sprintf("%s\t\tcase <-%s.C:\n", pad, tickerVar))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		if decisionOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, decisionOut, decisionVar))
		}
		if statusOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, statusOut, statusVar))
		}
		if decidedByOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, decidedByOut, decidedByVar))
		}
		if decidedAtOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, decidedAtOut, decidedAtVar))
		}
		if reasonOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, reasonOut, reasonVar))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "approval.Decide":
		approvalID := arg("approvalId")
		decision := arg("decision")
		actor := arg("actor")
		reason := arg("reason")
		statusOut := arg("status")
		if approvalID == "" || decision == "" || actor == "" {
			return "", true
		}
		declareStatusOut := statusOut != "" && !st.declared[statusOut]
		if declareStatusOut {
			st.declared[statusOut] = true
			st.pointers[statusOut] = false
			st.types[statusOut] = "string"
		}

		keyVar := "_approvalDecideKey" + sfx
		rawVar := "_approvalDecideRaw" + sfx
		errVar := "_approvalDecideErr" + sfx
		recordVar := "_approvalDecideRecord" + sfx
		decisionVar := "_approvalDecideDecision" + sfx
		marshalVar := "_approvalDecideBlob" + sfx
		marshalErrVar := "_approvalDecideMarshalErr" + sfx
		setErrVar := "_approvalDecideSetErr" + sfx

		var b strings.Builder
		if declareStatusOut {
			b.WriteString(fmt.Sprintf("%svar %s string\n", pad, statusOut))
		}
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif s.stateStore == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "STATE_STORE_NOT_CONFIGURED", "approval.Decide requires state store wiring")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := \"approval:id:\" + strings.TrimSpace(fmt.Sprint(%s))\n", pad, keyVar, approvalID))
		b.WriteString(fmt.Sprintf("%s\t%s := strings.Trim(strings.ToLower(strings.TrimSpace(fmt.Sprint(%s))), \"\\\"\")\n", pad, decisionVar, decision))
		b.WriteString(fmt.Sprintf("%s\tswitch %s {\n", pad, decisionVar))
		b.WriteString(fmt.Sprintf("%s\tcase \"approved\", \"rejected\", \"timed_out\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\tdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusBadRequest, "INVALID_APPROVAL_DECISION", "approval.Decide decision must be approved|rejected|timed_out")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, keyVar))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Decide: load: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s == nil {\n", pad, rawVar))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusNotFound, "APPROVAL_NOT_FOUND", "approval not found")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tvar %s map[string]any\n", pad, recordVar))
		b.WriteString(fmt.Sprintf("%s\tif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, recordVar))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"approval.Decide: unmarshal: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif %s == nil { %s = map[string]any{} }\n", pad, recordVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\t%s[\"status\"] = %s\n", pad, recordVar, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t%s[\"decision\"] = %s\n", pad, recordVar, decisionVar))
		b.WriteString(fmt.Sprintf("%s\t%s[\"decidedBy\"] = strings.TrimSpace(fmt.Sprint(%s))\n", pad, recordVar, actor))
		b.WriteString(fmt.Sprintf("%s\t%s[\"decidedAt\"] = time.Now().UTC().Format(time.RFC3339)\n", pad, recordVar))
		if reason != "" {
			b.WriteString(fmt.Sprintf("%s\t%s[\"reason\"] = strings.TrimSpace(fmt.Sprint(%s))\n", pad, recordVar, reason))
		}
		b.WriteString(fmt.Sprintf("%s\t%s, %s := json.Marshal(%s)\n", pad, marshalVar, marshalErrVar, recordVar))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, marshalErrVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Decide: marshal: %%w\", %s)", marshalErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := s.stateStore.Set(ctx, %s, %s, 7*24*time.Hour)\n", pad, setErrVar, keyVar, marshalVar))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, setErrVar))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"approval.Decide: save: %%w\", %s)", setErrVar)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		if statusOut != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, statusOut, decisionVar))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Broadcast":
		name := arg("name")
		payload := renderEventPayloadExpr(st, step, name, arg)
		if name == "" || payload == "" {
			return "", true
		}
		return fmt.Sprintf("%sif s.publisher != nil {\n%s\t_ = s.publisher.Broadcast%s(ctx, %s)\n%s}\n",
			pad, pad, ExportName(name), payload, pad), true

	case "event.Outbox":
		name := arg("name")
		payload := renderEventPayloadExpr(st, step, name, arg)
		if name == "" || payload == "" {
			return "", true
		}
		idExpr := arg("id")
		if idExpr == "" {
			idExpr = "uuid.NewString()"
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif s.outbox == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "OUTBOX_NOT_CONFIGURED", "event.Outbox requires outbox repository wiring")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_obPayload, _obMarshalErr := json.Marshal(%s)\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\tif _obMarshalErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"event.Outbox: marshal: %w\", _obMarshalErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_obID := %s\n", pad, idExpr))
		b.WriteString(fmt.Sprintf("%s\tif _obErr := s.outbox.SaveEvent(ctx, _obID, %s, _obPayload); _obErr != nil {\n", pad, name))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"event.Outbox: %w\", _obErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Wait":
		name := arg("name")
		timeout := arg("timeout")
		match := arg("match")
		output := arg("output")
		if name == "" {
			return "", true
		}
		if timeout == "" {
			timeout = "5*time.Minute"
		}

		var b strings.Builder
		if output != "" && !st.declared[output] {
			b.WriteString(fmt.Sprintf("%svar %s any\n", pad, output))
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "any"
		}

		b.WriteString(fmt.Sprintf("%s// event.Wait: %s\n", pad, name))
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_waitCtx, _waitCancel := context.WithTimeout(ctx, %s)\n", pad, timeout))
		b.WriteString(fmt.Sprintf("%s\tdefer _waitCancel()\n", pad))

		waitCall := fmt.Sprintf("s.publisher.Wait(_waitCtx, %q, %q)", name, match)

		b.WriteString(fmt.Sprintf("%s\t_evt, _waitErr := %s\n", pad, waitCall))
		b.WriteString(fmt.Sprintf("%s\tif _waitErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"event.Wait(%%s): %%w\", %q, _waitErr)", name)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))

		if output != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = _evt\n", pad, output))
		}

		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Subscribe":
		name := arg("name")
		match := arg("match")
		doSteps := child("_do")
		if name == "" || len(doSteps) == 0 {
			return "", true
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// event.Subscribe: %s\n", pad, name))
		b.WriteString(fmt.Sprintf("%sif s.publisher != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\ts.publisher.Subscribe(ctx, %q, %q, func(ctx context.Context, evt any) {\n", pad, name, match))

		subState := cloneFlowState(st)
		subState.goroutineMode = true
		subState.declared["evt"] = true
		subState.types["evt"] = "any"

		b.WriteString(renderFlowSteps(subState, doSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t})\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Match":
		evtVar := arg("event")
		matchCriteria := arg("match")
		throwMsg := arg("throw")
		if evtVar == "" || matchCriteria == "" {
			return "", true
		}
		if throwMsg == "" {
			throwMsg = fmt.Sprintf("event match failed for %s", matchCriteria)
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !helpers.MatchEvent(%s, %q) {\n", pad, evtVar, matchCriteria))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"EVENT_MISMATCH\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}
