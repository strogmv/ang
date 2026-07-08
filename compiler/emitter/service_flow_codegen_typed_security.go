package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepSecurity(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
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
