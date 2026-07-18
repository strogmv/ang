package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepCache(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "cache.Get":
		typed, err := typedActionAs[flowir.CacheGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		output := typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		cacheRawV, cacheErrV := "_cacheRaw"+sfx, "_cacheErr"+sfx
		var b strings.Builder
		if assign == ":=" {
			b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, output))
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := s.cache.Get(ctx, %s).Result()\n", pad, cacheRawV, cacheErrV, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil && !errors.Is(%s, redis.Nil) {\n", pad, cacheErrV, cacheErrV))
		b.WriteString(errReturn(st, pad+"\t", cacheErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if typed.Optional {
			b.WriteString(fmt.Sprintf("%sif %s == nil { %s = %s }\n", pad, cacheErrV, output, cacheRawV))
		} else {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n%s\t%s = %s\n%s}\n", pad, cacheErrV, pad, output, cacheRawV, pad))
		}
		return b.String(), true

	case "cache.Set":
		typed, err := typedActionAs[flowir.CacheSet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		value := normalizeFlowExpr(typed.Value.Source)
		ttl := normalizeFlowExpr(typed.TTL.Source)
		if ttl == "" {
			ttl = "0"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _cErr := s.cache.Set(ctx, %s, %s, %s).Err(); _cErr != nil {\n", pad, key, value, ttl))
		b.WriteString(errReturn(st, pad+"\t", "_cErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "cache.Del":
		typed, err := typedActionAs[flowir.CacheDelete](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _cErr := s.cache.Del(ctx, %s).Err(); _cErr != nil {\n", pad, key))
		b.WriteString(errReturn(st, pad+"\t", "_cErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepStorageSimple(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "storage.Delete":
		typed, err := typedActionAs[flowir.StorageDelete](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		errVar := "_sDelErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s := s.storage.Delete(ctx, %s); %s != nil {\n", pad, errVar, key, errVar))
		b.WriteString(errReturn(st, pad+"\t", errVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "storage.List":
		typed, err := typedActionAs[flowir.StorageList](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		prefix := normalizeFlowExpr(typed.Prefix.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "[]string"
		listVar, errVar := "_sList"+sfx, "_sErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.List(ctx, %s)\n", pad, listVar, errVar, prefix))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", errVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, typed.Output, assign, listVar))
		return b.String(), true

	case "storage.GetURL":
		typed, err := typedActionAs[flowir.StorageGetURL](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		urlVar, errVar := "_sURL"+sfx, "_sErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.GetURL(ctx, %s)\n", pad, urlVar, errVar, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", errVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, typed.Output, assign, urlVar))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepStorageData(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "storage.Upload":
		typed, err := typedActionAs[flowir.StorageUpload](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		data := normalizeFlowExpr(typed.Data.Source)
		contentType := normalizeFlowExpr(typed.ContentType.Source)
		if contentType == "" {
			contentType = `"application/octet-stream"`
		}
		readerVar, bytesVar := "_sUpReader"+sfx, "_sUpDataBytes"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s []byte\n", pad, bytesVar))
		b.WriteString(fmt.Sprintf("%sswitch _v := any(%s).(type) {\n", pad, data))
		b.WriteString(fmt.Sprintf("%scase []byte:\n%s\t%s = _v\n", pad, pad, bytesVar))
		b.WriteString(fmt.Sprintf("%scase string:\n%s\t%s = []byte(_v)\n", pad, pad, bytesVar))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("storage.Upload: data must be string or []byte")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := bytes.NewReader(%s)\n", pad, readerVar, bytesVar))
		if typed.Output == "" {
			errVar := "_sErr" + sfx
			b.WriteString(fmt.Sprintf("%sif _, %s := s.storage.Upload(ctx, %s, %s, %s); %s != nil {\n", pad, errVar, key, readerVar, contentType, errVar))
			b.WriteString(errReturn(st, pad+"\t", errVar))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		b.WriteString(fmt.Sprintf("%s%s %s \"\"\n", pad, typed.Output, assign))
		urlVar, errVar := "_sUpURL"+sfx, "_sUpErr"+sfx
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.Upload(ctx, %s, %s, %s)\n", pad, urlVar, errVar, key, readerVar, contentType))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", errVar))
		b.WriteString(fmt.Sprintf("%s}\n%s%s = %s\n", pad, pad, typed.Output, urlVar))
		return b.String(), true

	case "storage.Download":
		typed, err := typedActionAs[flowir.StorageDownload](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		key := normalizeFlowExpr(typed.Key.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "[]byte"
		readerVar, downloadErr, bytesVar := "_sDlRC"+sfx, "_sDlErr"+sfx, "_sDlBytes"+sfx
		readErr := "_sDlReadErr" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.Download(ctx, %s)\n", pad, readerVar, downloadErr, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, downloadErr))
		b.WriteString(errReturn(st, pad+"\t", downloadErr))
		b.WriteString(fmt.Sprintf("%s}\n%sdefer %s.Close()\n", pad, pad, readerVar))
		b.WriteString(fmt.Sprintf("%s%s, %s := io.ReadAll(%s)\n", pad, bytesVar, readErr, readerVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, readErr))
		b.WriteString(errReturn(st, pad+"\t", readErr))
		b.WriteString(fmt.Sprintf("%s}\n%s%s %s %s\n", pad, pad, typed.Output, assign, bytesVar))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepMail(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	if step.Name != "mail.Send" {
		return "", false
	}
	pad := strings.Repeat("\t", indent)
	typed, err := typedActionAs[flowir.MailSend](step)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}
	to := normalizeFlowExpr(typed.To.Source)
	subject := normalizeFlowExpr(typed.Subject.Source)
	body := normalizeFlowExpr(typed.Body.Source)
	html := normalizeFlowExpr(typed.HTML.Source)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif _mErr := s.mailer.Send(ctx, port.EmailMessage{To: %s, Subject: %s, Text: %s", pad, to, subject, body))
	if html != "" {
		b.WriteString(fmt.Sprintf(", HTML: %s", html))
	}
	b.WriteString("}); _mErr != nil {\n")
	b.WriteString(errReturn(st, pad+"\t", "_mErr"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String(), true
}
