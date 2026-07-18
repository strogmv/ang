package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepSecurity(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "jwt.Sign":
		typed, err := typedActionAs[flowir.JWTSign](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedJWTSign(st, typed, pad, sfx), true

	case "jwt.Verify":
		typed, err := typedActionAs[flowir.JWTVerify](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedJWTVerify(st, typed, pad, sfx), true

	case "token.Generate":
		typed, err := typedActionAs[flowir.TokenGenerate](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedTokenGenerate(st, typed, pad, sfx), true

	case "token.Verify":
		typed, err := typedActionAs[flowir.TokenVerify](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderTypedTokenVerify(st, typed, pad, sfx), true

	case "crypto.Hash":
		typed, err := typedActionAs[flowir.CryptoHash](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		assign := ":="
		if st.declared[typed.Output] {
			assign = "="
		}
		st.declared[typed.Output] = true
		st.pointers[typed.Output] = false
		st.types[typed.Output] = "string"

		var b strings.Builder
		if typed.Algorithm == "bcrypt" {
			hashVar := "_bcHash" + sfx
			errVar := "_bcHashErr" + sfx
			b.WriteString(fmt.Sprintf("%s%s, %s := bcrypt.GenerateFromPassword([]byte(%s), 12)\n", pad, hashVar, errVar, input))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Hash bcrypt: %w\", "+errVar+")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, typed.Output, assign, hashVar))
			return b.String(), true
		}
		b.WriteString(fmt.Sprintf("%s_sha256Sum%s := sha256.Sum256([]byte(%s))\n", pad, sfx, input))
		b.WriteString(fmt.Sprintf("%s%s %s hex.EncodeToString(_sha256Sum%s[:])\n", pad, typed.Output, assign, sfx))
		return b.String(), true

	case "oauth2.Token":
		typed, err := typedActionAs[flowir.OAuth2Token](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderOAuthTokenLike(st, typed.OAuth2Fields, pad, sfx, step.Name, false), true

	case "oauth2.Refresh":
		typed, err := typedActionAs[flowir.OAuth2Refresh](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderOAuthTokenLike(st, typed.OAuth2Fields, pad, sfx, step.Name, true), true

	case "crypto.Encrypt", "crypto.Decrypt":
		typed, err := typedActionAs[flowir.CryptoCipher](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		if typed.Decrypt {
			return renderTypedCryptoDecrypt(st, typed, pad, sfx), true
		}
		return renderTypedCryptoEncrypt(st, typed, pad, sfx), true
	}
	return "", false
}

func renderTypedJWTSign(st *flowRenderState, typed flowir.JWTSign, pad, sfx string) string {
	claims := normalizeFlowExpr(typed.Claims.Source)
	alg := normalizeFlowExpr(typed.Algorithm.Source)
	secretExpr := normalizeFlowExpr(typed.Secret.Source)
	ttl := normalizeFlowExpr(typed.TTL.Source)
	assign := ":="
	if st.declared[typed.Output] {
		assign = "="
	}
	st.declared[typed.Output] = true
	st.pointers[typed.Output] = false
	st.types[typed.Output] = "string"

	algVar := "_jwtAlg" + sfx
	secretVar := "_jwtSecret" + sfx
	claimsVar := "_jwtClaims" + sfx
	ttlVar := "_jwtTTL" + sfx
	ttlErrVar := "_jwtTTLErr" + sfx
	headerJSONVar := "_jwtHeaderJSON" + sfx
	payloadJSONVar := "_jwtPayloadJSON" + sfx
	unsignedVar := "_jwtUnsigned" + sfx
	sigVar := "_jwtSig" + sfx
	encVar := "_jwtEnc" + sfx
	hmacVar := "_jwtHMAC" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := strings.ToUpper(strings.TrimSpace(%s))\n", pad, algVar, alg))
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, algVar))
	b.WriteString(fmt.Sprintf("%s\t%s = \"HS256\"\n", pad, algVar))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif %s != \"HS256\" {\n", pad, algVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Sign: unsupported alg %q\", "+algVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, secretVar))
	if secretExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, secretVar, secretExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"JWT_PRIVATE_KEY\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s\t%s = os.Getenv(\"JWT_PUBLIC_KEY\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "JWT_SECRET_MISSING", "jwt.Sign requires secret or JWT_PRIVATE_KEY")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%sswitch _v := any(%s).(type) {\n", pad, claims))
	b.WriteString(fmt.Sprintf("%scase map[string]any:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s = make(map[string]any, len(_v))\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%s\tfor _k, _val := range _v {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t%s[_k] = _val\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_CLAIMS", "jwt.Sign claims must be map[string]any")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	if ttl != "" {
		b.WriteString(fmt.Sprintf("%s%s, %s := time.ParseDuration(%s)\n", pad, ttlVar, ttlErrVar, ttl))
		b.WriteString(fmt.Sprintf("%sif %s != nil || %s <= 0 {\n", pad, ttlErrVar, ttlVar))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Sign: invalid ttl: %w\", "+ttlErrVar+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _, _hasExp := %s[\"exp\"]; !_hasExp {\n", pad, claimsVar))
		b.WriteString(fmt.Sprintf("%s\t%s[\"exp\"] = time.Now().Add(%s).Unix()\n", pad, claimsVar, ttlVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}

	b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(map[string]any{\"alg\": \"HS256\", \"typ\": \"JWT\"})\n", pad, headerJSONVar))
	b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, payloadJSONVar, claimsVar))
	b.WriteString(fmt.Sprintf("%s%s := base64.RawURLEncoding\n", pad, encVar))
	b.WriteString(fmt.Sprintf("%s%s := %s.EncodeToString(%s) + \".\" + %s.EncodeToString(%s)\n", pad, unsignedVar, encVar, headerJSONVar, encVar, payloadJSONVar))
	b.WriteString(fmt.Sprintf("%s%s := hmac.New(sha256.New, []byte(%s))\n", pad, hmacVar, secretVar))
	b.WriteString(fmt.Sprintf("%s_, _ = %s.Write([]byte(%s))\n", pad, hmacVar, unsignedVar))
	b.WriteString(fmt.Sprintf("%s%s := %s.Sum(nil)\n", pad, sigVar, hmacVar))
	b.WriteString(fmt.Sprintf("%s%s %s %s + \".\" + %s.EncodeToString(%s)\n", pad, typed.Output, assign, unsignedVar, encVar, sigVar))
	return b.String()
}

func renderTypedJWTVerify(st *flowRenderState, typed flowir.JWTVerify, pad, sfx string) string {
	tokenExpr := normalizeFlowExpr(typed.Token.Source)
	secretExpr := normalizeFlowExpr(typed.Secret.Source)
	assign := ":="
	if st.declared[typed.Output] {
		assign = "="
	}
	st.declared[typed.Output] = true
	st.pointers[typed.Output] = false
	st.types[typed.Output] = "map[string]any"

	secretVar := "_jwtSecret" + sfx
	partsVar := "_jwtParts" + sfx
	unsignedVar := "_jwtUnsigned" + sfx
	sigVar := "_jwtSig" + sfx
	sigErrVar := "_jwtSigErr" + sfx
	payloadVar := "_jwtPayload" + sfx
	payloadErrVar := "_jwtPayloadErr" + sfx
	claimsVar := "_jwtClaims" + sfx
	encVar := "_jwtEnc" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, secretVar))
	if secretExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, secretVar, secretExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"JWT_PRIVATE_KEY\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s\t%s = os.Getenv(\"JWT_PUBLIC_KEY\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "JWT_SECRET_MISSING", "jwt.Verify requires secret or JWT_PRIVATE_KEY")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	b.WriteString(fmt.Sprintf("%s%s := strings.Split(%s, \".\")\n", pad, partsVar, tokenExpr))
	b.WriteString(fmt.Sprintf("%sif len(%s) != 3 {\n", pad, partsVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "JWT_INVALID", "jwt.Verify: malformed token")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := base64.RawURLEncoding\n", pad, encVar))
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.DecodeString(%s[2])\n", pad, sigVar, sigErrVar, encVar, partsVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sigErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Verify: decode signature: %w\", "+sigErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := %s[0] + \".\" + %s[1]\n", pad, unsignedVar, partsVar, partsVar))
	b.WriteString(fmt.Sprintf("%s_hmac := hmac.New(sha256.New, []byte(%s))\n", pad, secretVar))
	b.WriteString(fmt.Sprintf("%s_, _ = _hmac.Write([]byte(%s))\n", pad, unsignedVar))
	b.WriteString(fmt.Sprintf("%sif !hmac.Equal(%s, _hmac.Sum(nil)) {\n", pad, sigVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "JWT_INVALID_SIGNATURE", "jwt.Verify: invalid signature")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.DecodeString(%s[1])\n", pad, payloadVar, payloadErrVar, encVar, partsVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, payloadErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Verify: decode payload: %w\", "+payloadErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, payloadVar, claimsVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "JWT_INVALID", "jwt.Verify: invalid payload")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif _expRaw, _ok := %s[\"exp\"]; _ok {\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%s\tswitch _exp := _expRaw.(type) {\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase float64:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif int64(_exp) < time.Now().Unix() {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", `errors.New(http.StatusUnauthorized, "JWT_EXPIRED", "jwt.Verify: token expired")`))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase int64:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif _exp < time.Now().Unix() {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", `errors.New(http.StatusUnauthorized, "JWT_EXPIRED", "jwt.Verify: token expired")`))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase int:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif int64(_exp) < time.Now().Unix() {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", `errors.New(http.StatusUnauthorized, "JWT_EXPIRED", "jwt.Verify: token expired")`))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, typed.Output, assign, claimsVar))
	return b.String()
}

func renderTypedTokenGenerate(st *flowRenderState, typed flowir.TokenGenerate, pad, sfx string) string {
	subject := normalizeFlowExpr(typed.Subject.Source)
	purposeExpr := normalizeFlowExpr(typed.Purpose.Source)
	claimsExpr := normalizeFlowExpr(typed.Claims.Source)
	secretExpr := normalizeFlowExpr(typed.Secret.Source)
	ttlExpr := normalizeFlowExpr(typed.TTL.Source)
	secretVar := "_tokenSecret" + sfx
	payloadVar := "_tokenPayload" + sfx
	ttlVar := "_tokenTTL" + sfx
	ttlErrVar := "_tokenTTLErr" + sfx
	nowVar := "_tokenNow" + sfx
	payloadJSONVar := "_tokenPayloadJSON" + sfx
	unsignedVar := "_tokenUnsigned" + sfx
	sigVar := "_tokenSig" + sfx
	encVar := "_tokenEnc" + sfx
	hmacVar := "_tokenHMAC" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, secretVar))
	if secretExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, secretVar, secretExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"APP_TOKEN_SECRET\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s\t%s = os.Getenv(\"JWT_PRIVATE_KEY\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "TOKEN_SECRET_MISSING", "token.Generate requires secret, APP_TOKEN_SECRET, or JWT_PRIVATE_KEY")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s := time.ParseDuration(%s)\n", pad, ttlVar, ttlErrVar, ttlExpr))
	b.WriteString(fmt.Sprintf("%sif %s != nil || %s <= 0 {\n", pad, ttlErrVar, ttlVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"token.Generate: invalid ttl: %w\", "+ttlErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := make(map[string]any)\n", pad, payloadVar))
	if claimsExpr != "" {
		b.WriteString(fmt.Sprintf("%sswitch _claims := any(%s).(type) {\n", pad, claimsExpr))
		b.WriteString(fmt.Sprintf("%scase map[string]any:\n", pad))
		b.WriteString(fmt.Sprintf("%s\tfor _k, _v := range _claims {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s[_k] = _v\n", pad, payloadVar))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_CLAIMS", "token.Generate claims must be map[string]any")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%s%s[\"sub\"] = %s\n", pad, payloadVar, subject))
	if purposeExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s[\"purpose\"] = %s\n", pad, payloadVar, purposeExpr))
	}
	b.WriteString(fmt.Sprintf("%s%s := time.Now().UTC()\n", pad, nowVar))
	b.WriteString(fmt.Sprintf("%s%s[\"iat\"] = %s.Unix()\n", pad, payloadVar, nowVar))
	b.WriteString(fmt.Sprintf("%s%s[\"exp\"] = %s.Add(%s).Unix()\n", pad, payloadVar, nowVar, ttlVar))
	b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, payloadJSONVar, payloadVar))
	b.WriteString(fmt.Sprintf("%s%s := base64.RawURLEncoding\n", pad, encVar))
	b.WriteString(fmt.Sprintf("%s%s := \"tk1.\" + %s.EncodeToString(%s)\n", pad, unsignedVar, encVar, payloadJSONVar))
	b.WriteString(fmt.Sprintf("%s%s := hmac.New(sha256.New, []byte(%s))\n", pad, hmacVar, secretVar))
	b.WriteString(fmt.Sprintf("%s_, _ = %s.Write([]byte(%s))\n", pad, hmacVar, unsignedVar))
	b.WriteString(fmt.Sprintf("%s%s := %s.Sum(nil)\n", pad, sigVar, hmacVar))
	b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, unsignedVar+"+\".\"+"+encVar+".EncodeToString("+sigVar+")", "string"))
	return b.String()
}

func renderTypedTokenVerify(st *flowRenderState, typed flowir.TokenVerify, pad, sfx string) string {
	tokenExpr := normalizeFlowExpr(typed.Token.Source)
	secretExpr := normalizeFlowExpr(typed.Secret.Source)
	purposeExpr := normalizeFlowExpr(typed.Purpose.Source)
	secretVar := "_tokenSecret" + sfx
	partsVar := "_tokenParts" + sfx
	unsignedVar := "_tokenUnsigned" + sfx
	sigVar := "_tokenSig" + sfx
	sigErrVar := "_tokenSigErr" + sfx
	payloadVar := "_tokenPayload" + sfx
	payloadErrVar := "_tokenPayloadErr" + sfx
	claimsVar := "_tokenClaims" + sfx
	encVar := "_tokenEnc" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, secretVar))
	if secretExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, secretVar, secretExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"APP_TOKEN_SECRET\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s\t%s = os.Getenv(\"JWT_PRIVATE_KEY\")\n", pad, secretVar))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "TOKEN_SECRET_MISSING", "token.Verify requires secret, APP_TOKEN_SECRET, or JWT_PRIVATE_KEY")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := strings.Split(%s, \".\")\n", pad, partsVar, tokenExpr))
	b.WriteString(fmt.Sprintf("%sif len(%s) != 3 || %s[0] != \"tk1\" {\n", pad, partsVar, partsVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "TOKEN_INVALID", "token.Verify: malformed token")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := base64.RawURLEncoding\n", pad, encVar))
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.DecodeString(%s[2])\n", pad, sigVar, sigErrVar, encVar, partsVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sigErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"token.Verify: decode signature: %w\", "+sigErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := %s[0] + \".\" + %s[1]\n", pad, unsignedVar, partsVar, partsVar))
	b.WriteString(fmt.Sprintf("%s_hmac := hmac.New(sha256.New, []byte(%s))\n", pad, secretVar))
	b.WriteString(fmt.Sprintf("%s_, _ = _hmac.Write([]byte(%s))\n", pad, unsignedVar))
	b.WriteString(fmt.Sprintf("%sif !hmac.Equal(%s, _hmac.Sum(nil)) {\n", pad, sigVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "TOKEN_INVALID_SIGNATURE", "token.Verify: invalid signature")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.DecodeString(%s[1])\n", pad, payloadVar, payloadErrVar, encVar, partsVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, payloadErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"token.Verify: decode payload: %w\", "+payloadErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, payloadVar, claimsVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "TOKEN_INVALID", "token.Verify: invalid payload")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif _expRaw, _ok := %s[\"exp\"]; _ok {\n", pad, claimsVar))
	b.WriteString(fmt.Sprintf("%s\tswitch _exp := _expRaw.(type) {\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase float64:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif int64(_exp) < time.Now().Unix() {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", `errors.New(http.StatusUnauthorized, "TOKEN_EXPIRED", "token.Verify: token expired")`))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase int64:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif _exp < time.Now().Unix() {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", `errors.New(http.StatusUnauthorized, "TOKEN_EXPIRED", "token.Verify: token expired")`))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase int:\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif int64(_exp) < time.Now().Unix() {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", `errors.New(http.StatusUnauthorized, "TOKEN_EXPIRED", "token.Verify: token expired")`))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	if purposeExpr != "" {
		b.WriteString(fmt.Sprintf("%sif fmt.Sprint(%s[\"purpose\"]) != fmt.Sprint(%s) {\n", pad, claimsVar, purposeExpr))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "TOKEN_PURPOSE_MISMATCH", "token.Verify: purpose mismatch")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(renderFlowAssignTarget(st, pad, typed.Output, claimsVar, "map[string]any"))
	return b.String()
}

func renderTypedCryptoEncrypt(st *flowRenderState, typed flowir.CryptoCipher, pad, sfx string) string {
	input, keyExpr, aadExpr := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.AAD.Source)
	assign := ":="
	if st.declared[typed.Output] {
		assign = "="
	}
	st.declared[typed.Output] = true
	st.pointers[typed.Output] = false
	st.types[typed.Output] = "string"

	keyVar := "_encKey" + sfx
	keySumVar := "_encKeySum" + sfx
	blockVar := "_encBlock" + sfx
	blockErrVar := "_encBlockErr" + sfx
	gcmVar := "_encGCM" + sfx
	gcmErrVar := "_encGCMErr" + sfx
	nonceVar := "_encNonce" + sfx
	nonceErrVar := "_encNonceErr" + sfx
	aadVar := "_encAAD" + sfx
	cipherVar := "_encCipher" + sfx
	payloadVar := "_encPayload" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, keyVar))
	if keyExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, keyVar, keyExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"APP_ENCRYPTION_KEY\")\n", pad, keyVar))
	}
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, keyVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "ENCRYPTION_KEY_MISSING", "crypto.Encrypt requires key or APP_ENCRYPTION_KEY")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := sha256.Sum256([]byte(%s))\n", pad, keySumVar, keyVar))
	b.WriteString(fmt.Sprintf("%s%s, %s := aes.NewCipher(%s[:])\n", pad, blockVar, blockErrVar, keySumVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, blockErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Encrypt: cipher init: %w\", "+blockErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s := cipher.NewGCM(%s)\n", pad, gcmVar, gcmErrVar, blockVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, gcmErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Encrypt: gcm init: %w\", "+gcmErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := make([]byte, %s.NonceSize())\n", pad, nonceVar, gcmVar))
	b.WriteString(fmt.Sprintf("%sif _, %s := cryptorand.Read(%s); %s != nil {\n", pad, nonceErrVar, nonceVar, nonceErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Encrypt: nonce: %w\", "+nonceErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	if aadExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s := []byte(%s)\n", pad, aadVar, aadExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s := []byte(nil)\n", pad, aadVar))
	}
	b.WriteString(fmt.Sprintf("%s%s := %s.Seal(nil, %s, []byte(%s), %s)\n", pad, cipherVar, gcmVar, nonceVar, input, aadVar))
	b.WriteString(fmt.Sprintf("%s%s := append(%s, %s...)\n", pad, payloadVar, nonceVar, cipherVar))
	b.WriteString(fmt.Sprintf("%s%s %s base64.RawStdEncoding.EncodeToString(%s)\n", pad, typed.Output, assign, payloadVar))
	return b.String()
}

func renderTypedCryptoDecrypt(st *flowRenderState, typed flowir.CryptoCipher, pad, sfx string) string {
	input, keyExpr, aadExpr := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.AAD.Source)
	assign := ":="
	if st.declared[typed.Output] {
		assign = "="
	}
	st.declared[typed.Output] = true
	st.pointers[typed.Output] = false
	st.types[typed.Output] = "string"

	keyVar := "_decKey" + sfx
	keySumVar := "_decKeySum" + sfx
	rawVar := "_decRaw" + sfx
	rawErrVar := "_decRawErr" + sfx
	blockVar := "_decBlock" + sfx
	blockErrVar := "_decBlockErr" + sfx
	gcmVar := "_decGCM" + sfx
	gcmErrVar := "_decGCMErr" + sfx
	nonceVar := "_decNonce" + sfx
	cipherVar := "_decCipher" + sfx
	aadVar := "_decAAD" + sfx
	plainVar := "_decPlain" + sfx
	plainErrVar := "_decPlainErr" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, keyVar))
	if keyExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, keyVar, keyExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"APP_ENCRYPTION_KEY\")\n", pad, keyVar))
	}
	b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, keyVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "ENCRYPTION_KEY_MISSING", "crypto.Decrypt requires key or APP_ENCRYPTION_KEY")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := sha256.Sum256([]byte(%s))\n", pad, keySumVar, keyVar))
	b.WriteString(fmt.Sprintf("%s%s, %s := base64.RawStdEncoding.DecodeString(%s)\n", pad, rawVar, rawErrVar, input))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rawErrVar))
	b.WriteString(fmt.Sprintf("%s\t%s, %s = base64.StdEncoding.DecodeString(%s)\n", pad, rawVar, rawErrVar, input))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rawErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Decrypt: decode input: %w\", "+rawErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s := aes.NewCipher(%s[:])\n", pad, blockVar, blockErrVar, keySumVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, blockErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Decrypt: cipher init: %w\", "+blockErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s := cipher.NewGCM(%s)\n", pad, gcmVar, gcmErrVar, blockVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, gcmErrVar))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Decrypt: gcm init: %w\", "+gcmErrVar+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif len(%s) < %s.NonceSize() {\n", pad, rawVar, gcmVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_CIPHERTEXT", "crypto.Decrypt: invalid ciphertext")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := %s[:%s.NonceSize()]\n", pad, nonceVar, rawVar, gcmVar))
	b.WriteString(fmt.Sprintf("%s%s := %s[%s.NonceSize():]\n", pad, cipherVar, rawVar, gcmVar))
	if aadExpr != "" {
		b.WriteString(fmt.Sprintf("%s%s := []byte(%s)\n", pad, aadVar, aadExpr))
	} else {
		b.WriteString(fmt.Sprintf("%s%s := []byte(nil)\n", pad, aadVar))
	}
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.Open(nil, %s, %s, %s)\n", pad, plainVar, plainErrVar, gcmVar, nonceVar, cipherVar, aadVar))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, plainErrVar))
	b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "DECRYPT_FAILED", "crypto.Decrypt: unable to decrypt payload")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, typed.Output, assign, plainVar))
	return b.String()
}
