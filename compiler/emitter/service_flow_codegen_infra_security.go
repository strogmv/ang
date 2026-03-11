package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepInfraSecurity(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "jwt.Sign":
		claims := arg("claims")
		output := arg("output")
		if claims == "" || output == "" {
			return "", true
		}
		alg := arg("alg")
		if alg == "" {
			alg = `"HS256"`
		}
		secretExpr := arg("secret")
		ttl := arg("ttl")

		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		algV := "_jwtAlg" + sfx
		secretV := "_jwtSecret" + sfx
		claimsV := "_jwtClaims" + sfx
		ttlV := "_jwtTTL" + sfx
		ttlErrV := "_jwtTTLErr" + sfx
		headerJV := "_jwtHeaderJSON" + sfx
		payloadJV := "_jwtPayloadJSON" + sfx
		unsignedV := "_jwtUnsigned" + sfx
		sigV := "_jwtSig" + sfx
		encV := "_jwtEnc" + sfx
		hmacV := "_jwtHMAC" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := strings.ToUpper(strings.TrimSpace(%s))\n", pad, algV, alg))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, algV))
		b.WriteString(fmt.Sprintf("%s\t%s = \"HS256\"\n", pad, algV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s != \"HS256\" {\n", pad, algV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Sign: unsupported alg %q\", "+algV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, secretV))
		if secretExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, secretV, secretExpr))
		} else {
			b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"JWT_PRIVATE_KEY\")\n", pad, secretV))
			b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretV))
			b.WriteString(fmt.Sprintf("%s\t%s = os.Getenv(\"JWT_PUBLIC_KEY\")\n", pad, secretV))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "JWT_SECRET_MISSING", "jwt.Sign requires secret or JWT_PRIVATE_KEY")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, claimsV))
		b.WriteString(fmt.Sprintf("%sswitch _v := any(%s).(type) {\n", pad, claims))
		b.WriteString(fmt.Sprintf("%scase map[string]any:\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = make(map[string]any, len(_v))\n", pad, claimsV))
		b.WriteString(fmt.Sprintf("%s\tfor _k, _val := range _v {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t\t%s[_k] = _val\n", pad, claimsV))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%sdefault:\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_CLAIMS", "jwt.Sign claims must be map[string]any")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		if ttl != "" {
			b.WriteString(fmt.Sprintf("%s%s, %s := time.ParseDuration(%s)\n", pad, ttlV, ttlErrV, ttl))
			b.WriteString(fmt.Sprintf("%sif %s != nil || %s <= 0 {\n", pad, ttlErrV, ttlV))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Sign: invalid ttl: %w\", "+ttlErrV+")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%sif _, _hasExp := %s[\"exp\"]; !_hasExp {\n", pad, claimsV))
			b.WriteString(fmt.Sprintf("%s\t%s[\"exp\"] = time.Now().Add(%s).Unix()\n", pad, claimsV, ttlV))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}

		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(map[string]any{\"alg\": \"HS256\", \"typ\": \"JWT\"})\n", pad, headerJV))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, payloadJV, claimsV))
		b.WriteString(fmt.Sprintf("%s%s := base64.RawURLEncoding\n", pad, encV))
		b.WriteString(fmt.Sprintf("%s%s := %s.EncodeToString(%s) + \".\" + %s.EncodeToString(%s)\n", pad, unsignedV, encV, headerJV, encV, payloadJV))
		b.WriteString(fmt.Sprintf("%s%s := hmac.New(sha256.New, []byte(%s))\n", pad, hmacV, secretV))
		b.WriteString(fmt.Sprintf("%s_, _ = %s.Write([]byte(%s))\n", pad, hmacV, unsignedV))
		b.WriteString(fmt.Sprintf("%s%s := %s.Sum(nil)\n", pad, sigV, hmacV))
		b.WriteString(fmt.Sprintf("%s%s %s %s + \".\" + %s.EncodeToString(%s)\n", pad, output, assign, unsignedV, encV, sigV))
		return b.String(), true

	case "jwt.Verify":
		tokenExpr := arg("token")
		output := arg("output")
		if tokenExpr == "" || output == "" {
			return "", true
		}
		secretExpr := arg("secret")
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "map[string]any"

		secretV := "_jwtSecret" + sfx
		partsV := "_jwtParts" + sfx
		unsignedV := "_jwtUnsigned" + sfx
		sigV := "_jwtSig" + sfx
		sigErrV := "_jwtSigErr" + sfx
		payloadV := "_jwtPayload" + sfx
		payloadErrV := "_jwtPayloadErr" + sfx
		claimsV := "_jwtClaims" + sfx
		encV := "_jwtEnc" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, secretV))
		if secretExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, secretV, secretExpr))
		} else {
			b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"JWT_PRIVATE_KEY\")\n", pad, secretV))
			b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretV))
			b.WriteString(fmt.Sprintf("%s\t%s = os.Getenv(\"JWT_PUBLIC_KEY\")\n", pad, secretV))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, secretV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "JWT_SECRET_MISSING", "jwt.Verify requires secret or JWT_PRIVATE_KEY")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		b.WriteString(fmt.Sprintf("%s%s := strings.Split(%s, \".\")\n", pad, partsV, tokenExpr))
		b.WriteString(fmt.Sprintf("%sif len(%s) != 3 {\n", pad, partsV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "JWT_INVALID", "jwt.Verify: malformed token")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := base64.RawURLEncoding\n", pad, encV))
		b.WriteString(fmt.Sprintf("%s%s, %s := %s.DecodeString(%s[2])\n", pad, sigV, sigErrV, encV, partsV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, sigErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Verify: decode signature: %w\", "+sigErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := %s[0] + \".\" + %s[1]\n", pad, unsignedV, partsV, partsV))
		b.WriteString(fmt.Sprintf("%s_hmac := hmac.New(sha256.New, []byte(%s))\n", pad, secretV))
		b.WriteString(fmt.Sprintf("%s_, _ = _hmac.Write([]byte(%s))\n", pad, unsignedV))
		b.WriteString(fmt.Sprintf("%sif !hmac.Equal(%s, _hmac.Sum(nil)) {\n", pad, sigV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "JWT_INVALID_SIGNATURE", "jwt.Verify: invalid signature")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := %s.DecodeString(%s[1])\n", pad, payloadV, payloadErrV, encV, partsV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, payloadErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"jwt.Verify: decode payload: %w\", "+payloadErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, claimsV))
		b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, payloadV, claimsV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusUnauthorized, "JWT_INVALID", "jwt.Verify: invalid payload")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _expRaw, _ok := %s[\"exp\"]; _ok {\n", pad, claimsV))
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
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, claimsV))
		return b.String(), true

	case "crypto.Hash":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return "", true
		}
		algo := arg("algo")
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"
		var b strings.Builder
		if algo == "bcrypt" {
			hashV := "_bcHash" + sfx
			hashErrV := "_bcHashErr" + sfx
			b.WriteString(fmt.Sprintf("%s%s, %s := bcrypt.GenerateFromPassword([]byte(%s), 12)\n", pad, hashV, hashErrV, input))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, hashErrV))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Hash bcrypt: %w\", "+hashErrV+")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, hashV))
		} else {
			// default: sha256 hex
			b.WriteString(fmt.Sprintf("%s_sha256Sum%s := sha256.Sum256([]byte(%s))\n", pad, sfx, input))
			b.WriteString(fmt.Sprintf("%s%s %s hex.EncodeToString(_sha256Sum%s[:])\n", pad, output, assign, sfx))
		}
		return b.String(), true

	case "oauth2.Token":
		return renderOAuthTokenLike(st, step, pad, sfx, arg, "oauth2.Token", false), true
	case "oauth2.Refresh":
		return renderOAuthTokenLike(st, step, pad, sfx, arg, "oauth2.Refresh", true), true

	case "crypto.Encrypt":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return "", true
		}
		keyExpr := arg("key")
		aadExpr := arg("aad")
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		keyV := "_encKey" + sfx
		keySumV := "_encKeySum" + sfx
		blockV := "_encBlock" + sfx
		blockErrV := "_encBlockErr" + sfx
		gcmV := "_encGCM" + sfx
		gcmErrV := "_encGCMErr" + sfx
		nonceV := "_encNonce" + sfx
		nonceErrV := "_encNonceErr" + sfx
		aadV := "_encAAD" + sfx
		cipherV := "_encCipher" + sfx
		payloadV := "_encPayload" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, keyV))
		if keyExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, keyV, keyExpr))
		} else {
			b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"APP_ENCRYPTION_KEY\")\n", pad, keyV))
		}
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, keyV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "ENCRYPTION_KEY_MISSING", "crypto.Encrypt requires key or APP_ENCRYPTION_KEY")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := sha256.Sum256([]byte(%s))\n", pad, keySumV, keyV))
		b.WriteString(fmt.Sprintf("%s%s, %s := aes.NewCipher(%s[:])\n", pad, blockV, blockErrV, keySumV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, blockErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Encrypt: cipher init: %w\", "+blockErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := cipher.NewGCM(%s)\n", pad, gcmV, gcmErrV, blockV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, gcmErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Encrypt: gcm init: %w\", "+gcmErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := make([]byte, %s.NonceSize())\n", pad, nonceV, gcmV))
		b.WriteString(fmt.Sprintf("%sif _, %s := cryptorand.Read(%s); %s != nil {\n", pad, nonceErrV, nonceV, nonceErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Encrypt: nonce: %w\", "+nonceErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if aadExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s := []byte(%s)\n", pad, aadV, aadExpr))
		} else {
			b.WriteString(fmt.Sprintf("%s%s := []byte(nil)\n", pad, aadV))
		}
		b.WriteString(fmt.Sprintf("%s%s := %s.Seal(nil, %s, []byte(%s), %s)\n", pad, cipherV, gcmV, nonceV, input, aadV))
		b.WriteString(fmt.Sprintf("%s%s := append(%s, %s...)\n", pad, payloadV, nonceV, cipherV))
		b.WriteString(fmt.Sprintf("%s%s %s base64.RawStdEncoding.EncodeToString(%s)\n", pad, output, assign, payloadV))
		return b.String(), true

	case "crypto.Decrypt":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return "", true
		}
		keyExpr := arg("key")
		aadExpr := arg("aad")
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "string"

		keyV := "_decKey" + sfx
		keySumV := "_decKeySum" + sfx
		rawV := "_decRaw" + sfx
		rawErrV := "_decRawErr" + sfx
		blockV := "_decBlock" + sfx
		blockErrV := "_decBlockErr" + sfx
		gcmV := "_decGCM" + sfx
		gcmErrV := "_decGCMErr" + sfx
		nonceV := "_decNonce" + sfx
		cipherV := "_decCipher" + sfx
		aadV := "_decAAD" + sfx
		plainV := "_decPlain" + sfx
		plainErrV := "_decPlainErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, keyV))
		if keyExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, keyV, keyExpr))
		} else {
			b.WriteString(fmt.Sprintf("%s%s = os.Getenv(\"APP_ENCRYPTION_KEY\")\n", pad, keyV))
		}
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, keyV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "ENCRYPTION_KEY_MISSING", "crypto.Decrypt requires key or APP_ENCRYPTION_KEY")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := sha256.Sum256([]byte(%s))\n", pad, keySumV, keyV))
		b.WriteString(fmt.Sprintf("%s%s, %s := base64.RawStdEncoding.DecodeString(%s)\n", pad, rawV, rawErrV, input))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rawErrV))
		b.WriteString(fmt.Sprintf("%s\t%s, %s = base64.StdEncoding.DecodeString(%s)\n", pad, rawV, rawErrV, input))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rawErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Decrypt: decode input: %w\", "+rawErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := aes.NewCipher(%s[:])\n", pad, blockV, blockErrV, keySumV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, blockErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Decrypt: cipher init: %w\", "+blockErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s, %s := cipher.NewGCM(%s)\n", pad, gcmV, gcmErrV, blockV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, gcmErrV))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"crypto.Decrypt: gcm init: %w\", "+gcmErrV+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif len(%s) < %s.NonceSize() {\n", pad, rawV, gcmV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "INVALID_CIPHERTEXT", "crypto.Decrypt: invalid ciphertext")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s := %s[:%s.NonceSize()]\n", pad, nonceV, rawV, gcmV))
		b.WriteString(fmt.Sprintf("%s%s := %s[%s.NonceSize():]\n", pad, cipherV, rawV, gcmV))
		if aadExpr != "" {
			b.WriteString(fmt.Sprintf("%s%s := []byte(%s)\n", pad, aadV, aadExpr))
		} else {
			b.WriteString(fmt.Sprintf("%s%s := []byte(nil)\n", pad, aadV))
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := %s.Open(nil, %s, %s, %s)\n", pad, plainV, plainErrV, gcmV, nonceV, cipherV, aadV))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, plainErrV))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusBadRequest, "DECRYPT_FAILED", "crypto.Decrypt: unable to decrypt payload")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, plainV))
		return b.String(), true
	}

	return "", false
}

func renderOAuthTokenLike(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string, action string, refreshOnly bool) string {
	tokenURL := arg("tokenURL")
	output := arg("output")
	if tokenURL == "" || output == "" {
		return ""
	}
	clientID := arg("clientID")
	clientSecret := arg("clientSecret")
	scope := arg("scope")
	audience := arg("audience")
	grantType := arg("grantType")
	refreshToken := arg("refreshToken")
	code := arg("code")
	redirectURI := arg("redirectURI")
	username := arg("username")
	password := arg("password")

	assign := ":="
	if st.declared[output] {
		assign = "="
	}
	st.declared[output] = true
	st.pointers[output] = false
	st.types[output] = "map[string]any"

	formV := "_oauthForm" + sfx
	grantV := "_oauthGrant" + sfx
	reqV := "_oauthReq" + sfx
	reqErrV := "_oauthReqErr" + sfx
	respV := "_oauthResp" + sfx
	httpErrV := "_oauthErr" + sfx
	bodyV := "_oauthBody" + sfx
	bodyErrV := "_oauthBodyErr" + sfx
	tokenV := "_oauthToken" + sfx
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s%s := url.Values{}\n", pad, formV))
	if refreshOnly {
		b.WriteString(fmt.Sprintf("%s%s := \"refresh_token\"\n", pad, grantV))
	} else {
		b.WriteString(fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, grantV, grantTypeOrDefault(grantType)))
		b.WriteString(fmt.Sprintf("%sif %s == \"\" {\n", pad, grantV))
		b.WriteString(fmt.Sprintf("%s\t%s = \"client_credentials\"\n", pad, grantV))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	b.WriteString(fmt.Sprintf("%s%s.Set(\"grant_type\", %s)\n", pad, formV, grantV))
	if refreshOnly {
		if refreshToken == "" {
			b.WriteString(errReturn(st, pad, `errors.New(http.StatusBadRequest, "MISSING_REFRESH_TOKEN", "oauth2.Refresh missing refreshToken")`))
			return b.String()
		}
		b.WriteString(fmt.Sprintf("%s%s.Set(\"refresh_token\", %s)\n", pad, formV, refreshToken))
	} else {
		if refreshToken != "" {
			b.WriteString(fmt.Sprintf("%s%s.Set(\"refresh_token\", %s)\n", pad, formV, refreshToken))
		}
		if code != "" {
			b.WriteString(fmt.Sprintf("%s%s.Set(\"code\", %s)\n", pad, formV, code))
		}
		if redirectURI != "" {
			b.WriteString(fmt.Sprintf("%s%s.Set(\"redirect_uri\", %s)\n", pad, formV, redirectURI))
		}
		if username != "" {
			b.WriteString(fmt.Sprintf("%s%s.Set(\"username\", %s)\n", pad, formV, username))
		}
		if password != "" {
			b.WriteString(fmt.Sprintf("%s%s.Set(\"password\", %s)\n", pad, formV, password))
		}
	}
	if clientID != "" {
		b.WriteString(fmt.Sprintf("%s%s.Set(\"client_id\", %s)\n", pad, formV, clientID))
	}
	if clientSecret != "" {
		b.WriteString(fmt.Sprintf("%s%s.Set(\"client_secret\", %s)\n", pad, formV, clientSecret))
	}
	if scope != "" {
		b.WriteString(fmt.Sprintf("%s%s.Set(\"scope\", %s)\n", pad, formV, scope))
	}
	if audience != "" {
		b.WriteString(fmt.Sprintf("%s%s.Set(\"audience\", %s)\n", pad, formV, audience))
	}

	b.WriteString(fmt.Sprintf("%s%s, %s := http.NewRequestWithContext(ctx, http.MethodPost, %s, strings.NewReader(%s.Encode()))\n", pad, reqV, reqErrV, tokenURL, formV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, reqErrV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\""+action+": request build: %w\", "+reqErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s.Header.Set(\"Content-Type\", \"application/x-www-form-urlencoded\")\n", pad, reqV))
	b.WriteString(fmt.Sprintf("%s%s, %s := http.DefaultClient.Do(%s)\n", pad, respV, httpErrV, reqV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, httpErrV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\""+action+": http: %w\", "+httpErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, respV))
	b.WriteString(fmt.Sprintf("%s%s, %s := io.ReadAll(%s.Body)\n", pad, bodyV, bodyErrV, respV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, bodyErrV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\""+action+": read body: %w\", "+bodyErrV+")"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sif %s.StatusCode < 200 || %s.StatusCode >= 300 {\n", pad, respV, respV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\""+action+": status %d: %s\", "+respV+".StatusCode, string("+bodyV+"))"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, tokenV))
	b.WriteString(fmt.Sprintf("%sif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, bodyV, tokenV))
	b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\""+action+": decode: %w\", err)"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, tokenV))
	return b.String()
}

func grantTypeOrDefault(grantType string) string {
	if grantType == "" {
		return `""`
	}
	return grantType
}
