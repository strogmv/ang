package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowStepInfraDataTransform(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "regex.Match":
		typed, err := flowir.DecodeAs[flowir.RegexMatch](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, pattern, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Pattern.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "bool"
		reV, errV := "_re"+sfx, "_reErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := regexp.Compile(%s)\n", pad, reV, errV, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"regex.Match: %w\", "+errV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s.MatchString(%s)\n", pad, output, assign, reV, input))
		return b.String(), true

	case "regex.Replace":
		typed, err := flowir.DecodeAs[flowir.RegexReplace](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, pattern, repl, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Pattern.Source), normalizeFlowExpr(typed.Replacement.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		reV, errV := "_re"+sfx, "_reErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := regexp.Compile(%s)\n", pad, reV, errV, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"regex.Replace: %w\", "+errV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s.ReplaceAllString(%s, %s)\n", pad, output, assign, reV, input, repl))
		return b.String(), true

	case "base64.Encode":
		typed, err := flowir.DecodeAs[flowir.Base64Encode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		return fmt.Sprintf("%s%s %s base64.StdEncoding.EncodeToString([]byte(%s))\n", pad, output, assign, input), true

	case "base64.Decode":
		typed, err := flowir.DecodeAs[flowir.Base64Decode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		bytesV, errV := "_b64Bytes"+sfx, "_b64Err"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := base64.StdEncoding.DecodeString(%s)\n", pad, bytesV, errV, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"base64.Decode: %w\", "+errV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, bytesV))
		return b.String(), true

	case "url.Parse":
		typed, err := flowir.DecodeAs[flowir.URLParse](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "*url.URL"
		uV, errV := "_url"+sfx, "_urlErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := url.Parse(%s)\n", pad, uV, errV, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"url.Parse: %w\", "+errV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, uV))
		return b.String(), true

	case "path.Base":
		typed, err := flowir.DecodeAs[flowir.PathBase](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		normV := "_pathNorm" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ReplaceAll(%s, \"\\\\\", \"/\")\n", pad, normV, input))
		b.WriteString(renderFlowAssignTarget(st, pad, output, "path.Base("+normV+")", "string"))
		return b.String(), true

	case "url.Build":
		typed, err := flowir.DecodeAs[flowir.URLBuild](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		base, output, pathExpr := normalizeFlowExpr(typed.Base.Source), typed.Output, normalizeFlowExpr(typed.Path.Source)
		segments := make([]string, 0, len(typed.Segments))
		for _, v := range typed.Segments {
			segments = append(segments, normalizeFlowExpr(v.Source))
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		uV, errV := "_url"+sfx, "_urlErr"+sfx
		qV := "_urlQ" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := url.Parse(%s)\n", pad, uV, errV, base))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"url.Build: %w\", "+errV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if len(segments) > 0 {
			allParts := make([]string, 0, len(segments)+1)
			allParts = append(allParts, uV+".Path")
			allParts = append(allParts, segments...)
			b.WriteString(fmt.Sprintf("%s%s.Path = path.Join(%s)\n", pad, uV, strings.Join(allParts, ", ")))
		} else if pathExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s.Path = %s\n", pad, uV, pathExpr))
		}
		if qMap := typed.Query; len(qMap) > 0 {
			b.WriteString(fmt.Sprintf("%s%s := %s.Query()\n", pad, qV, uV))
			for k, v := range qMap {
				b.WriteString(fmt.Sprintf("%s%s.Set(%q, %s)\n", pad, qV, k, normalizeFlowExpr(v.Source)))
			}
			b.WriteString(fmt.Sprintf("%s%s.RawQuery = %s.Encode()\n", pad, uV, qV))
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s.String()\n", pad, output, assign, uV))
		return b.String(), true

	case "query.Encode":
		typed, err := flowir.DecodeAs[flowir.QueryEncode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		outV := "_qsOut" + sfx
		valV := "_qsVal" + sfx
		mapV := "_qsMap" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s string\n", pad, outV))
		b.WriteString(fmt.Sprintf("%sswitch %s := any(%s).(type) {\n", pad, valV, input))
		b.WriteString(fmt.Sprintf("%scase url.Values:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = %s.Encode()\n", pad, outV, valV))
		b.WriteString(fmt.Sprintf("%scase map[string]string:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := url.Values{}\n", pad, mapV))
		b.WriteString(fmt.Sprintf("%s\tfor _k, _v := range %s { %s.Set(_k, _v) }\n", pad, valV, mapV))
		b.WriteString(fmt.Sprintf("%s\t%s = %s.Encode()\n", pad, outV, mapV))
		b.WriteString(fmt.Sprintf("%scase map[string]any:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := url.Values{}\n", pad, mapV))
		b.WriteString(fmt.Sprintf("%s\tfor _k, _v := range %s { %s.Set(_k, fmt.Sprint(_v)) }\n", pad, valV, mapV))
		b.WriteString(fmt.Sprintf("%s\t%s = %s.Encode()\n", pad, outV, mapV))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"query.Encode: input must be url.Values or map[string]...\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, outV))
		return b.String(), true

	case "query.Decode":
		typed, err := flowir.DecodeAs[flowir.QueryDecode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "url.Values"
		valsV, errV := "_qsVals"+sfx, "_qsErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := url.ParseQuery(%s)\n", pad, valsV, errV, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"query.Decode: %w\", "+errV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, valsV))
		return b.String(), true

	case "hash.Sum":
		typed, err := flowir.DecodeAs[flowir.HashSum](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output, alg := normalizeFlowExpr(typed.Input.Source), typed.Output, normalizeFlowExpr(typed.Algorithm.Source)
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		algV := "_hashAlg" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, algV, alg))
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, algV))
		b.WriteString(fmt.Sprintf("%scase \"sha256\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_sum := sha256.Sum256([]byte(%s))\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s %s hex.EncodeToString(_sum[:])\n", pad, output, assign))
		b.WriteString(fmt.Sprintf("%scase \"sha1\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_sum := sha1.Sum([]byte(%s))\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s %s hex.EncodeToString(_sum[:])\n", pad, output, assign))
		b.WriteString(fmt.Sprintf("%scase \"md5\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_sum := md5.Sum([]byte(%s))\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s %s hex.EncodeToString(_sum[:])\n", pad, output, assign))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"hash.Sum: unsupported algorithm %q\", "+algV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "hash.HMAC":
		typed, err := flowir.DecodeAs[flowir.HashHMAC](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, key, output, alg := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), typed.Output, normalizeFlowExpr(typed.Algorithm.Source)
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		algV := "_hmacAlg" + sfx
		hV := "_hmac" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, algV, alg))
		b.WriteString(fmt.Sprintf("%svar %s hash.Hash\n", pad, hV))
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, algV))
		b.WriteString(fmt.Sprintf("%scase \"sha256\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = hmac.New(sha256.New, []byte(%s))\n", pad, hV, key))
		b.WriteString(fmt.Sprintf("%scase \"sha1\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = hmac.New(sha1.New, []byte(%s))\n", pad, hV, key))
		b.WriteString(fmt.Sprintf("%scase \"md5\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = hmac.New(md5.New, []byte(%s))\n", pad, hV, key))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"hash.HMAC: unsupported algorithm %q\", "+algV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s_, _ = %s.Write([]byte(%s))\n", pad, hV, input))
		b.WriteString(fmt.Sprintf("%s%s %s hex.EncodeToString(%s.Sum(nil))\n", pad, output, assign, hV))
		return b.String(), true

	case "uuid.New":
		typed, err := flowir.DecodeAs[flowir.UUIDNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output := typed.Output
		return renderFlowAssignTarget(st, pad, output, "uuid.NewString()", "string"), true

	case "ulid.New":
		typed, err := flowir.DecodeAs[flowir.ULIDNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output := typed.Output
		rawV := "_ulidRaw" + sfx
		msV := "_ulidMs" + sfx
		encV := "_ulidEnc" + sfx
		outV := "_ulidOut" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, 16)\n", pad, rawV))
		b.WriteString(fmt.Sprintf("%s%s := uint64(time.Now().UTC().UnixMilli())\n", pad, msV))
		b.WriteString(fmt.Sprintf("%s%s[0] = byte(%s >> 40)\n", pad, rawV, msV))
		b.WriteString(fmt.Sprintf("%s%s[1] = byte(%s >> 32)\n", pad, rawV, msV))
		b.WriteString(fmt.Sprintf("%s%s[2] = byte(%s >> 24)\n", pad, rawV, msV))
		b.WriteString(fmt.Sprintf("%s%s[3] = byte(%s >> 16)\n", pad, rawV, msV))
		b.WriteString(fmt.Sprintf("%s%s[4] = byte(%s >> 8)\n", pad, rawV, msV))
		b.WriteString(fmt.Sprintf("%s%s[5] = byte(%s)\n", pad, rawV, msV))
		b.WriteString(fmt.Sprintf("%sif _, _ulidErr := cryptorand.Read(%s[6:]); _ulidErr != nil {\n", pad, rawV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"ulid.New: %w\", _ulidErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := base32.NewEncoding(\"0123456789ABCDEFGHJKMNPQRSTVWXYZ\").WithPadding(base32.NoPadding)\n", pad, encV))
		b.WriteString(fmt.Sprintf("%s%s := %s.EncodeToString(%s)\n", pad, outV, encV, rawV))
		b.WriteString(renderFlowAssignTarget(st, pad, output, outV, "string"))
		return b.String(), true

	case "time.Now":
		typed, err := flowir.DecodeAs[flowir.TimeNow](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, format := typed.Output, normalizeFlowExpr(typed.Format)
		if format != "" {
			return renderFlowAssignTarget(st, pad, output, "time.Now().UTC().Format("+format+")", "string"), true
		}
		return renderFlowAssignTarget(st, pad, output, "time.Now().UTC()", "time.Time"), true

	case "time.Format":
		typed, err := flowir.DecodeAs[flowir.TimeFormat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output, format := normalizeFlowExpr(typed.Input.Source), typed.Output, normalizeFlowExpr(typed.Format)
		if format == "" {
			format = "time.RFC3339"
		}
		timezone := normalizeFlowExpr(typed.Timezone)
		if timezone != "" {
			// timezone-aware format: convert to location first
			locVar := "_tzLoc" + sfx
			tVar := "_tzTime" + sfx
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s, _ := time.LoadLocation(fmt.Sprint(%s))\n", pad, locVar, timezone))
			b.WriteString(fmt.Sprintf("%sif %s == nil { %s = time.UTC }\n", pad, locVar, locVar))
			b.WriteString(fmt.Sprintf("%s%s := %s.In(%s)\n", pad, tVar, input, locVar))
			rest := renderFlowAssignTarget(st, pad, output, tVar+".Format("+format+")", "string")
			b.WriteString(rest)
			return b.String(), true
		}
		return renderFlowAssignTarget(st, pad, output, input+".Format("+format+")", "string"), true

	case "time.InZone":
		// time.InZone: convert time.Time to given IANA timezone → output is time.Time in that zone
		typed, err := flowir.DecodeAs[flowir.TimeInZone](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output, timezone := normalizeFlowExpr(typed.Input.Source), typed.Output, normalizeFlowExpr(typed.Timezone)
		locVar := "_inZoneLoc" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, _ := time.LoadLocation(fmt.Sprint(%s))\n", pad, locVar, timezone))
		b.WriteString(fmt.Sprintf("%sif %s == nil { %s = time.UTC }\n", pad, locVar, locVar))
		rest := renderFlowAssignTarget(st, pad, output, input+".In("+locVar+")", "time.Time")
		b.WriteString(rest)
		return b.String(), true

	case "math.Op":
		typed, err := flowir.DecodeAs[flowir.MathOperation](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		op, output := normalizeFlowExpr(typed.Operation.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "float64"

		precision := typed.Precision
		a, bExpr, val, min, max := normalizeFlowExpr(typed.A.Source), normalizeFlowExpr(typed.B.Source), normalizeFlowExpr(typed.Value.Source), normalizeFlowExpr(typed.Min.Source), normalizeFlowExpr(typed.Max.Source)
		opV := "_mathOp" + sfx
		factorV := "_roundFactor" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, opV, op))
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, opV))
		if a != "" && bExpr != "" {
			b.WriteString(fmt.Sprintf("%scase \"min\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Min(float64(%s), float64(%s))\n", pad, output, assign, a, bExpr))
			b.WriteString(fmt.Sprintf("%scase \"max\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Max(float64(%s), float64(%s))\n", pad, output, assign, a, bExpr))
		}
		if val != "" && min != "" && max != "" {
			b.WriteString(fmt.Sprintf("%scase \"clamp\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Max(float64(%s), math.Min(float64(%s), float64(%s)))\n", pad, output, assign, min, max, val))
		}
		if val != "" {
			b.WriteString(fmt.Sprintf("%scase \"round\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s := math.Pow(10, float64(%d))\n", pad, factorV, precision))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Round(float64(%s)*%s) / %s\n", pad, output, assign, val, factorV, factorV))
		}
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"math.Op: unsupported op %q\", "+opV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "num.Add", "num.Sub", "num.Mul", "num.Div":
		typed, err := flowir.DecodeAs[flowir.NumberBinary](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		a, bExpr, output := normalizeFlowExpr(typed.A.Source), normalizeFlowExpr(typed.B.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "float64"
		switch step.Action {
		case "num.Add":
			return fmt.Sprintf("%s%s %s float64(%s) + float64(%s)\n", pad, output, assign, a, bExpr), true
		case "num.Sub":
			return fmt.Sprintf("%s%s %s float64(%s) - float64(%s)\n", pad, output, assign, a, bExpr), true
		case "num.Mul":
			return fmt.Sprintf("%s%s %s float64(%s) * float64(%s)\n", pad, output, assign, a, bExpr), true
		default: // num.Div
			den := "_numDivDen" + sfx
			var out strings.Builder
			out.WriteString(fmt.Sprintf("%s%s := float64(%s)\n", pad, den, bExpr))
			out.WriteString(fmt.Sprintf("%sif %s == 0 {\n", pad, den))
			out.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"num.Div: division by zero\")"))
			out.WriteString(fmt.Sprintf("%s}\n", pad))
			out.WriteString(fmt.Sprintf("%s%s %s float64(%s) / %s\n", pad, output, assign, a, den))
			return out.String(), true
		}

	case "jsonpath.Get":
		typed, err := flowir.DecodeAs[flowir.JSONPathGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, pathExpr, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Path.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "any"
		valV := "_jsonPathVal" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, _jsonPathErr%s := helpers.JSONPathGet(%s, %s)\n", pad, valV, sfx, input, pathExpr))
		b.WriteString(fmt.Sprintf("%sif _jsonPathErr%s != nil {\n", pad, sfx))
		b.WriteString(errReturn(st, pad+"\t", "_jsonPathErr"+sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, valV))
		return b.String(), true

	case "jsonpath.Set":
		typed, err := flowir.DecodeAs[flowir.JSONPathSet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, pathExpr, value, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Path.Source), normalizeFlowExpr(typed.Value.Source), typed.Output
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "map[string]any"
		pathV := "_jpSetPath" + sfx
		partsV := "_jpSetParts" + sfx
		rootV := "_jpRoot" + sfx
		curV := "_jpCurMap" + sfx
		partV := "_jpPart" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, pathV, pathExpr))
		b.WriteString(fmt.Sprintf("%s%s = strings.TrimPrefix(%s, \"$\")\n", pad, pathV, pathV))
		b.WriteString(fmt.Sprintf("%s%s = strings.TrimPrefix(%s, \".\")\n", pad, pathV, pathV))
		b.WriteString(fmt.Sprintf("%s%s, _ok := any(%s).(map[string]any)\n", pad, rootV, input))
		b.WriteString(fmt.Sprintf("%sif !_ok {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jsonpath.Set: input must be map[string]any\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, pathV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jsonpath.Set: path is empty\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := strings.Split(%s, \".\")\n", pad, partsV, pathV))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, curV, rootV))
		b.WriteString(fmt.Sprintf("%sfor _i, %s := range %s {\n", pad, partV, partsV))
		b.WriteString(fmt.Sprintf("%s\tif %s == \"\" { continue }\n", pad, partV))
		b.WriteString(fmt.Sprintf("%s\tif strings.Contains(%s, \"[\") {\n", pad, partV))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"jsonpath.Set: array notation is not supported in this version\")"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _i == len(%s)-1 {\n", pad, partsV))
		b.WriteString(fmt.Sprintf("%s\t\t%s[%s] = %s\n", pad, curV, partV, value))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_next, _ok := %s[%s].(map[string]any)\n", pad, curV, partV))
		b.WriteString(fmt.Sprintf("%s\tif !_ok || _next == nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_next = map[string]any{}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s[%s] = _next\n", pad, curV, partV))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = _next\n", pad, curV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, rootV))
		return b.String(), true
	}

	return "", false
}

func stepIntArgOrDefault(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return fallback
	}
}
