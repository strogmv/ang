package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepDataTransform(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "regex.Match":
		typed, err := typedActionAs[flowir.RegexMatch](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, pattern := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Pattern.Source)
		reVar, errVar := "_re"+sfx, "_reErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := regexp.Compile(%s)\n", pad, reVar, errVar, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"regex.Match: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, fmt.Sprintf("%s.MatchString(%s)", reVar, input), "bool"))
		return b.String(), true

	case "regex.Replace":
		typed, err := typedActionAs[flowir.RegexReplace](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, pattern, replacement := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Pattern.Source), normalizeFlowExpr(typed.Replacement.Source)
		reVar, errVar := "_re"+sfx, "_reErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := regexp.Compile(%s)\n", pad, reVar, errVar, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"regex.Replace: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, fmt.Sprintf("%s.ReplaceAllString(%s, %s)", reVar, input, replacement), "string"))
		return b.String(), true

	case "base64.Encode":
		typed, err := typedActionAs[flowir.Base64Encode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		return renderFlowAssignTarget(st, pad, typed.Output, fmt.Sprintf("base64.StdEncoding.EncodeToString([]byte(%s))", input), "string"), true

	case "base64.Decode":
		typed, err := typedActionAs[flowir.Base64Decode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		bytesVar, errVar := "_b64Bytes"+sfx, "_b64Err"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := base64.StdEncoding.DecodeString(%s)\n", pad, bytesVar, errVar, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"base64.Decode: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, "string("+bytesVar+")", "string"))
		return b.String(), true

	case "url.Parse":
		typed, err := typedActionAs[flowir.URLParse](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		urlVar, errVar := "_url"+sfx, "_urlErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := url.Parse(%s)\n", pad, urlVar, errVar, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"url.Parse: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, urlVar, "*url.URL"))
		return b.String(), true

	case "path.Base":
		typed, err := typedActionAs[flowir.PathBase](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		normVar := "_pathNorm" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ReplaceAll(%s, \"\\\\\", \"/\")\n", pad, normVar, input))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, "path.Base("+normVar+")", "string"))
		return b.String(), true

	case "url.Build":
		typed, err := typedActionAs[flowir.URLBuild](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		base, pathExpr := normalizeFlowExpr(typed.Base.Source), normalizeFlowExpr(typed.Path.Source)
		segments := make([]string, 0, len(typed.Segments))
		for _, item := range typed.Segments {
			segments = append(segments, normalizeFlowExpr(item.Source))
		}
		urlVar, errVar := "_url"+sfx, "_urlErr"+sfx
		queryVar := "_urlQ" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := url.Parse(%s)\n", pad, urlVar, errVar, base))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"url.Build: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if len(segments) > 0 {
			parts := make([]string, 0, len(segments)+1)
			parts = append(parts, urlVar+".Path")
			parts = append(parts, segments...)
			b.WriteString(fmt.Sprintf("%s%s.Path = path.Join(%s)\n", pad, urlVar, strings.Join(parts, ", ")))
		} else if pathExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s.Path = %s\n", pad, urlVar, pathExpr))
		}
		if len(typed.Query) > 0 {
			b.WriteString(fmt.Sprintf("%s%s := %s.Query()\n", pad, queryVar, urlVar))
			for key, value := range typed.Query {
				b.WriteString(fmt.Sprintf("%s%s.Set(%q, %s)\n", pad, queryVar, key, normalizeFlowExpr(value.Source)))
			}
			b.WriteString(fmt.Sprintf("%s%s.RawQuery = %s.Encode()\n", pad, urlVar, queryVar))
		}
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, urlVar+".String()", "string"))
		return b.String(), true

	case "query.Encode":
		typed, err := typedActionAs[flowir.QueryEncode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		outVar := "_qsOut" + sfx
		valVar := "_qsVal" + sfx
		mapVar := "_qsMap" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%svar %s string\n", pad, outVar))
		b.WriteString(fmt.Sprintf("%sswitch %s := any(%s).(type) {\n", pad, valVar, input))
		b.WriteString(fmt.Sprintf("%scase url.Values:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = %s.Encode()\n", pad, outVar, valVar))
		b.WriteString(fmt.Sprintf("%scase map[string]string:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := url.Values{}\n", pad, mapVar))
		b.WriteString(fmt.Sprintf("%s\tfor _k, _v := range %s { %s.Set(_k, _v) }\n", pad, valVar, mapVar))
		b.WriteString(fmt.Sprintf("%s\t%s = %s.Encode()\n", pad, outVar, mapVar))
		b.WriteString(fmt.Sprintf("%scase map[string]any:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s := url.Values{}\n", pad, mapVar))
		b.WriteString(fmt.Sprintf("%s\tfor _k, _v := range %s { %s.Set(_k, fmt.Sprint(_v)) }\n", pad, valVar, mapVar))
		b.WriteString(fmt.Sprintf("%s\t%s = %s.Encode()\n", pad, outVar, mapVar))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"query.Encode: input must be url.Values or map[string]...\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, outVar, "string"))
		return b.String(), true

	case "query.Decode":
		typed, err := typedActionAs[flowir.QueryDecode](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		valsVar, errVar := "_qsVals"+sfx, "_qsErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := url.ParseQuery(%s)\n", pad, valsVar, errVar, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"query.Decode: %w\", "+errVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, valsVar, "url.Values"))
		return b.String(), true

	case "hash.Sum":
		typed, err := typedActionAs[flowir.HashSum](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, algorithm := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Algorithm.Source)
		algVar := "_hashAlg" + sfx
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "string"
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, algVar, algorithm))
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, algVar))
		b.WriteString(fmt.Sprintf("%scase \"sha256\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_sum := sha256.Sum256([]byte(%s))\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s %s hex.EncodeToString(_sum[:])\n", pad, typed.Output, assign))
		b.WriteString(fmt.Sprintf("%scase \"sha1\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_sum := sha1.Sum([]byte(%s))\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s %s hex.EncodeToString(_sum[:])\n", pad, typed.Output, assign))
		b.WriteString(fmt.Sprintf("%scase \"md5\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_sum := md5.Sum([]byte(%s))\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s %s hex.EncodeToString(_sum[:])\n", pad, typed.Output, assign))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"hash.Sum: unsupported algorithm %q\", "+algVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "hash.HMAC":
		typed, err := typedActionAs[flowir.HashHMAC](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, key, algorithm := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Algorithm.Source)
		algVar := "_hmacAlg" + sfx
		hashVar := "_hmac" + sfx
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "string"
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, algVar, algorithm))
		b.WriteString(fmt.Sprintf("%svar %s hash.Hash\n", pad, hashVar))
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, algVar))
		b.WriteString(fmt.Sprintf("%scase \"sha256\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = hmac.New(sha256.New, []byte(%s))\n", pad, hashVar, key))
		b.WriteString(fmt.Sprintf("%scase \"sha1\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = hmac.New(sha1.New, []byte(%s))\n", pad, hashVar, key))
		b.WriteString(fmt.Sprintf("%scase \"md5\":\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = hmac.New(md5.New, []byte(%s))\n", pad, hashVar, key))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"hash.HMAC: unsupported algorithm %q\", "+algVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s_, _ = %s.Write([]byte(%s))\n", pad, hashVar, input))
		b.WriteString(fmt.Sprintf("%s%s %s hex.EncodeToString(%s.Sum(nil))\n", pad, typed.Output, assign, hashVar))
		return b.String(), true

	case "uuid.New":
		typed, err := typedActionAs[flowir.UUIDNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, typed.Output, "uuid.NewString()", "string"), true

	case "ulid.New":
		typed, err := typedActionAs[flowir.ULIDNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		rawVar := "_ulidRaw" + sfx
		msVar := "_ulidMs" + sfx
		encVar := "_ulidEnc" + sfx
		outVar := "_ulidOut" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, 16)\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s%s := uint64(time.Now().UTC().UnixMilli())\n", pad, msVar))
		b.WriteString(fmt.Sprintf("%s%s[0] = byte(%s >> 40)\n", pad, rawVar, msVar))
		b.WriteString(fmt.Sprintf("%s%s[1] = byte(%s >> 32)\n", pad, rawVar, msVar))
		b.WriteString(fmt.Sprintf("%s%s[2] = byte(%s >> 24)\n", pad, rawVar, msVar))
		b.WriteString(fmt.Sprintf("%s%s[3] = byte(%s >> 16)\n", pad, rawVar, msVar))
		b.WriteString(fmt.Sprintf("%s%s[4] = byte(%s >> 8)\n", pad, rawVar, msVar))
		b.WriteString(fmt.Sprintf("%s%s[5] = byte(%s)\n", pad, rawVar, msVar))
		b.WriteString(fmt.Sprintf("%sif _, _ulidErr := cryptorand.Read(%s[6:]); _ulidErr != nil {\n", pad, rawVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"ulid.New: %w\", _ulidErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := base32.NewEncoding(\"0123456789ABCDEFGHJKMNPQRSTVWXYZ\").WithPadding(base32.NoPadding)\n", pad, encVar))
		b.WriteString(fmt.Sprintf("%s%s := %s.EncodeToString(%s)\n", pad, outVar, encVar, rawVar))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, outVar, "string"))
		return b.String(), true

	case "time.Now":
		typed, err := typedActionAs[flowir.TimeNow](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		format := normalizeFlowExpr(typed.Format)
		if format != "" {
			return renderFlowAssignTarget(st, pad, typed.Output, "time.Now().UTC().Format("+format+")", "string"), true
		}
		return renderFlowAssignTarget(st, pad, typed.Output, "time.Now().UTC()", "time.Time"), true

	case "time.Format":
		typed, err := typedActionAs[flowir.TimeFormat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, format := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Format)
		if format == "" {
			format = "time.RFC3339"
		}
		timezone := normalizeFlowExpr(typed.Timezone)
		if typed.Zero == "empty" {
			if timezone != "" {
				expr := fmt.Sprintf("func() string { if %s.IsZero() { return \"\" }; _tz, _ := time.LoadLocation(fmt.Sprint(%s)); if _tz == nil { _tz = time.UTC }; return %s.In(_tz).Format(%s) }()", input, timezone, input, format)
				return renderFlowAssignTarget(st, pad, typed.Output, expr, "string"), true
			}
			expr := fmt.Sprintf("func() string { if %s.IsZero() { return \"\" }; return %s.Format(%s) }()", input, input, format)
			return renderFlowAssignTarget(st, pad, typed.Output, expr, "string"), true
		}
		if timezone != "" {
			locVar := "_tzLoc" + sfx
			timeVar := "_tzTime" + sfx
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s, _ := time.LoadLocation(fmt.Sprint(%s))\n", pad, locVar, timezone))
			b.WriteString(fmt.Sprintf("%sif %s == nil { %s = time.UTC }\n", pad, locVar, locVar))
			b.WriteString(fmt.Sprintf("%s%s := %s.In(%s)\n", pad, timeVar, input, locVar))
			b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, timeVar+".Format("+format+")", "string"))
			return b.String(), true
		}
		return renderFlowAssignTarget(st, pad, typed.Output, input+".Format("+format+")", "string"), true

	case "time.InZone":
		typed, err := typedActionAs[flowir.TimeInZone](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, timezone := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Timezone)
		locVar := "_inZoneLoc" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, _ := time.LoadLocation(fmt.Sprint(%s))\n", pad, locVar, timezone))
		b.WriteString(fmt.Sprintf("%sif %s == nil { %s = time.UTC }\n", pad, locVar, locVar))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, input+".In("+locVar+")", "time.Time"))
		return b.String(), true

	case "math.Op":
		typed, err := typedActionAs[flowir.MathOperation](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		operation := normalizeFlowExpr(typed.Operation.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "float64"

		a := normalizeFlowExpr(typed.A.Source)
		bExpr := normalizeFlowExpr(typed.B.Source)
		value := normalizeFlowExpr(typed.Value.Source)
		minValue := normalizeFlowExpr(typed.Min.Source)
		maxValue := normalizeFlowExpr(typed.Max.Source)
		opVar := "_mathOp" + sfx
		factorVar := "_roundFactor" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, opVar, operation))
		b.WriteString(fmt.Sprintf("%sswitch %s {\n", pad, opVar))
		if a != "" && bExpr != "" {
			b.WriteString(fmt.Sprintf("%scase \"min\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Min(float64(%s), float64(%s))\n", pad, typed.Output, assign, a, bExpr))
			b.WriteString(fmt.Sprintf("%scase \"max\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Max(float64(%s), float64(%s))\n", pad, typed.Output, assign, a, bExpr))
		}
		if value != "" && minValue != "" && maxValue != "" {
			b.WriteString(fmt.Sprintf("%scase \"clamp\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Max(float64(%s), math.Min(float64(%s), float64(%s)))\n", pad, typed.Output, assign, minValue, maxValue, value))
		}
		if value != "" {
			b.WriteString(fmt.Sprintf("%scase \"round\":\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s := math.Pow(10, float64(%d))\n", pad, factorVar, typed.Precision))
			b.WriteString(fmt.Sprintf("%s\t%s %s math.Round(float64(%s)*%s) / %s\n", pad, typed.Output, assign, value, factorVar, factorVar))
		}
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"math.Op: unsupported op %q\", "+opVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "num.Add", "num.Sub", "num.Mul", "num.Div":
		typed, err := typedActionAs[flowir.NumberBinary](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		a := normalizeFlowExpr(typed.A.Source)
		bExpr := normalizeFlowExpr(typed.B.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "float64"
		switch step.Name {
		case "num.Add":
			return fmt.Sprintf("%s%s %s float64(%s) + float64(%s)\n", pad, typed.Output, assign, a, bExpr), true
		case "num.Sub":
			return fmt.Sprintf("%s%s %s float64(%s) - float64(%s)\n", pad, typed.Output, assign, a, bExpr), true
		case "num.Mul":
			return fmt.Sprintf("%s%s %s float64(%s) * float64(%s)\n", pad, typed.Output, assign, a, bExpr), true
		default:
			denVar := "_numDivDen" + sfx
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s := float64(%s)\n", pad, denVar, bExpr))
			b.WriteString(fmt.Sprintf("%sif %s == 0 {\n", pad, denVar))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"num.Div: division by zero\")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s %s float64(%s) / %s\n", pad, typed.Output, assign, a, denVar))
			return b.String(), true
		}

	case "jsonpath.Get":
		typed, err := typedActionAs[flowir.JSONPathGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, pathExpr := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Path.Source)
		valVar := "_jsonPathVal" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, _jsonPathErr%s := helpers.JSONPathGet(%s, %s)\n", pad, valVar, sfx, input, pathExpr))
		b.WriteString(fmt.Sprintf("%sif _jsonPathErr%s != nil {\n", pad, sfx))
		b.WriteString(errReturn(st, pad+"\t", "_jsonPathErr"+sfx))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, valVar, "any"))
		return b.String(), true

	case "jsonpath.Set":
		typed, err := typedActionAs[flowir.JSONPathSet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input, pathExpr, value := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Path.Source), normalizeFlowExpr(typed.Value.Source)
		pathVar := "_jpSetPath" + sfx
		partsVar := "_jpSetParts" + sfx
		rootVar := "_jpRoot" + sfx
		curVar := "_jpCurMap" + sfx
		partVar := "_jpPart" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, pathVar, pathExpr))
		b.WriteString(fmt.Sprintf("%s%s = strings.TrimPrefix(%s, \"$\")\n", pad, pathVar, pathVar))
		b.WriteString(fmt.Sprintf("%s%s = strings.TrimPrefix(%s, \".\")\n", pad, pathVar, pathVar))
		b.WriteString(fmt.Sprintf("%s%s, _ok := any(%s).(map[string]any)\n", pad, rootVar, input))
		b.WriteString(fmt.Sprintf("%sif !_ok {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jsonpath.Set: input must be map[string]any\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, pathVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jsonpath.Set: path is empty\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := strings.Split(%s, \".\")\n", pad, partsVar, pathVar))
		b.WriteString(fmt.Sprintf("%s%s := %s\n", pad, curVar, rootVar))
		b.WriteString(fmt.Sprintf("%sfor _i, %s := range %s {\n", pad, partVar, partsVar))
		b.WriteString(fmt.Sprintf("%s\tif %s == \"\" { continue }\n", pad, partVar))
		b.WriteString(fmt.Sprintf("%s\tif strings.Contains(%s, \"[\") {\n", pad, partVar))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"jsonpath.Set: array notation is not supported in this version\")"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _i == len(%s)-1 {\n", pad, partsVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s[%s] = %s\n", pad, curVar, partVar, value))
		b.WriteString(fmt.Sprintf("%s\t\tbreak\n", pad))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_next, _ok := %s[%s].(map[string]any)\n", pad, curVar, partVar))
		b.WriteString(fmt.Sprintf("%s\tif !_ok || _next == nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t_next = map[string]any{}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s[%s] = _next\n", pad, curVar, partVar))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = _next\n", pad, curVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, rootVar, "map[string]any"))
		return b.String(), true
	}
	return "", false
}
