package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepInfraCacheMailStorage(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "cache.Get":
		key := arg("key")
		output := arg("output")
		if key == "" || output == "" {
			return "", true
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
		return b.String(), true

	case "cache.Set":
		key := arg("key")
		value := arg("value")
		ttl := arg("ttl")
		if key == "" || value == "" {
			return "", true
		}
		if ttl == "" {
			ttl = "0"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _cErr := s.cache.Set(ctx, %s, %s, %s).Err(); _cErr != nil {\n", pad, key, value, ttl))
		b.WriteString(errReturn(st, pad+"\t", "_cErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "cache.Del":
		key := arg("key")
		if key == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _cErr := s.cache.Del(ctx, %s).Err(); _cErr != nil {\n", pad, key))
		b.WriteString(errReturn(st, pad+"\t", "_cErr"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "mail.Send":
		to := arg("to")
		subject := arg("subject")
		body := arg("body")
		if to == "" || subject == "" || body == "" {
			return "", true
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
		return b.String(), true

	case "storage.Upload":
		key := arg("key")
		data := arg("data")
		output := arg("output")
		contentType := arg("contentType")
		if key == "" || data == "" {
			return "", true
		}
		if contentType == "" {
			contentType = `"application/octet-stream"`
		}
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
			return b.String(), true
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
		return b.String(), true

	case "storage.Download":
		key := arg("key")
		output := arg("output")
		if key == "" || output == "" {
			return "", true
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
		sDlReadErrV := "_sDlReadErr" + sfx
		b.WriteString(fmt.Sprintf("%s%s, %s := io.ReadAll(%s)\n", pad, sDlBytesV, sDlReadErrV, sDlRCV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sDlReadErrV))
		b.WriteString(errReturn(st, pad+"\t", sDlReadErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if assign == ":=" {
			b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, output, sDlBytesV))
		} else {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, output, sDlBytesV))
		}
		st.types[output] = "[]byte"
		return b.String(), true

	case "storage.Delete":
		key := arg("key")
		if key == "" {
			return "", true
		}
		var b strings.Builder
		sDelErrV := "_sDelErr" + sfx
		b.WriteString(fmt.Sprintf("%sif %s := s.storage.Delete(ctx, %s); %s != nil {\n", pad, sDelErrV, key, sDelErrV))
		b.WriteString(errReturn(st, pad+"\t", sDelErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "storage.List":
		prefix := arg("prefix")
		output := arg("output")
		if prefix == "" || output == "" {
			return "", true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "[]string"
		var b strings.Builder
		sListV, sErrV := "_sList"+sfx, "_sErr"+sfx
		b.WriteString(fmt.Sprintf("%s%s, %s := s.storage.List(ctx, %s)\n", pad, sListV, sErrV, prefix))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sErrV))
		b.WriteString(errReturn(st, pad+"\t", sErrV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, sListV))
		return b.String(), true

	case "storage.GetURL":
		key := arg("key")
		output := arg("output")
		if key == "" || output == "" {
			return "", true
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
		return b.String(), true
	}

	return "", false
}
