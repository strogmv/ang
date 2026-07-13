package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/flowir"
)

func TestTypedDispatchUsesPredecodedAction(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "uuid.New",
		Action: flowir.UUIDNew{Output: "generatedID"},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "generatedID") {
		t.Fatalf("typed action was not used:\n%s", got)
	}
}

func TestTypedStateDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "state.Delete",
		Action: flowir.StateDelete{Key: flowir.Expression{Source: `"typed-key"`}},
		ScalarArgs: map[string]flowir.ScalarArg{
			"key": {Kind: flowir.ScalarString, String: `"legacy-key"`},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, `"typed-key"`) || strings.Contains(got, `"legacy-key"`) {
		t.Fatalf("state renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedCacheDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "cache.Del",
		Action: flowir.CacheDelete{Key: flowir.Expression{Source: `"typed-cache-key"`}},
		ScalarArgs: map[string]flowir.ScalarArg{
			"key": {Kind: flowir.ScalarString, String: `"legacy-cache-key"`},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, `"typed-cache-key"`) || strings.Contains(got, `"legacy-cache-key"`) {
		t.Fatalf("cache renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedStorageDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "storage.GetURL",
		Action: flowir.StorageGetURL{Key: flowir.Expression{Source: `"typed-storage-key"`}, Output: "typedURL"},
		ScalarArgs: map[string]flowir.ScalarArg{
			"key":    {Kind: flowir.ScalarString, String: `"legacy-storage-key"`},
			"output": {Kind: flowir.ScalarString, String: "legacyURL"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, `"typed-storage-key"`) || !strings.Contains(got, "typedURL") || strings.Contains(got, "legacy") {
		t.Fatalf("storage renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedCoreDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "session.Get",
		Action: flowir.SessionGet{Output: "typedSessionID"},
		ScalarArgs: map[string]flowir.ScalarArg{
			"output": {Kind: flowir.ScalarString, String: "legacySessionID"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "typedSessionID") || strings.Contains(got, "legacySessionID") {
		t.Fatalf("core renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedSerializationDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "cast.ToString",
			Action: flowir.CastToString{
				Input:  flowir.Expression{Source: "req.TypedID"},
				Format: `"%s"`,
				Output: "typedString",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyID"},
				"output": {Kind: flowir.ScalarString, String: "legacyString"},
			},
		},
		{
			Name: "template.Render",
			Action: flowir.TemplateRender{
				Template: flowir.Expression{Source: `"Hello {{.Name}}"`},
				Data:     flowir.Expression{Source: "typedData"},
				Output:   "typedBody",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"template": {Kind: flowir.ScalarString, String: `"Legacy"`},
				"data":     {Kind: flowir.ScalarString, String: "legacyData"},
				"output":   {Kind: flowir.ScalarString, String: "legacyBody"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "typedString") || !strings.Contains(got, "req.TypedID") || !strings.Contains(got, "typedBody") || !strings.Contains(got, "typedData") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("serialization renderers did not use typed actions directly:\n%s", got)
	}
}

func TestTypedDataTransformDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "regex.Match",
			Action: flowir.RegexMatch{
				Input:   flowir.Expression{Source: "req.TypedEmail"},
				Pattern: flowir.Expression{Source: `"^[^@]+@typed$"`},
				Output:  "typedOK",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":   {Kind: flowir.ScalarString, String: "req.LegacyEmail"},
				"pattern": {Kind: flowir.ScalarString, String: `"legacy"`},
				"output":  {Kind: flowir.ScalarString, String: "legacyOK"},
			},
		},
		{
			Name: "regex.Replace",
			Action: flowir.RegexReplace{
				Input:       flowir.Expression{Source: "req.TypedName"},
				Pattern:     flowir.Expression{Source: `"\\s+"`},
				Replacement: flowir.Expression{Source: `"-"`},
				Output:      "typedSlug",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyName"},
				"repl":   {Kind: flowir.ScalarString, String: `"legacy"`},
				"output": {Kind: flowir.ScalarString, String: "legacySlug"},
			},
		},
		{
			Name: "base64.Encode",
			Action: flowir.Base64Encode{
				Input:  flowir.Expression{Source: "req.TypedPayload"},
				Output: "typedB64",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"output": {Kind: flowir.ScalarString, String: "legacyB64"},
			},
		},
		{
			Name: "base64.Decode",
			Action: flowir.Base64Decode{
				Input:  flowir.Expression{Source: "req.TypedEncoded"},
				Output: "typedRaw",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyEncoded"},
				"output": {Kind: flowir.ScalarString, String: "legacyRaw"},
			},
		},
		{
			Name: "url.Parse",
			Action: flowir.URLParse{
				Input:  flowir.Expression{Source: "req.TypedURL"},
				Output: "typedURL",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyURL"},
				"output": {Kind: flowir.ScalarString, String: "legacyURL"},
			},
		},
		{
			Name: "path.Base",
			Action: flowir.PathBase{
				Input:  flowir.Expression{Source: "req.TypedPath"},
				Output: "typedBase",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyPath"},
				"output": {Kind: flowir.ScalarString, String: "legacyBase"},
			},
		},
		{
			Name: "url.Build",
			Action: flowir.URLBuild{
				Base:   flowir.Expression{Source: `"https://typed.example"`},
				Path:   flowir.Expression{Source: `"/typed"`},
				Output: "typedBuiltURL",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"base":   {Kind: flowir.ScalarString, String: `"https://legacy.example"`},
				"path":   {Kind: flowir.ScalarString, String: `"/legacy"`},
				"output": {Kind: flowir.ScalarString, String: "legacyBuiltURL"},
			},
		},
		{
			Name: "query.Encode",
			Action: flowir.QueryEncode{
				Input:  flowir.Expression{Source: "typedQueryMap"},
				Output: "typedRawQuery",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyQueryMap"},
				"output": {Kind: flowir.ScalarString, String: "legacyRawQuery"},
			},
		},
		{
			Name: "query.Decode",
			Action: flowir.QueryDecode{
				Input:  flowir.Expression{Source: "typedRawInput"},
				Output: "typedQueryVals",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyRawInput"},
				"output": {Kind: flowir.ScalarString, String: "legacyQueryVals"},
			},
		},
		{
			Name: "hash.Sum",
			Action: flowir.HashSum{
				Input:     flowir.Expression{Source: "req.TypedPayload"},
				Algorithm: flowir.Expression{Source: `"sha256"`},
				Output:    "typedDigest",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":     {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"algorithm": {Kind: flowir.ScalarString, String: `"md5"`},
				"output":    {Kind: flowir.ScalarString, String: "legacyDigest"},
			},
		},
		{
			Name: "hash.HMAC",
			Action: flowir.HashHMAC{
				Input:     flowir.Expression{Source: "req.TypedPayload"},
				Key:       flowir.Expression{Source: "req.TypedSecret"},
				Algorithm: flowir.Expression{Source: `"sha1"`},
				Output:    "typedSignature",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":     {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"key":       {Kind: flowir.ScalarString, String: "req.LegacySecret"},
				"algorithm": {Kind: flowir.ScalarString, String: `"md5"`},
				"output":    {Kind: flowir.ScalarString, String: "legacySignature"},
			},
		},
		{
			Name:   "uuid.New",
			Action: flowir.UUIDNew{Output: "typedUUID"},
			ScalarArgs: map[string]flowir.ScalarArg{
				"output": {Kind: flowir.ScalarString, String: "legacyUUID"},
			},
		},
		{
			Name:   "ulid.New",
			Action: flowir.ULIDNew{Output: "typedULID"},
			ScalarArgs: map[string]flowir.ScalarArg{
				"output": {Kind: flowir.ScalarString, String: "legacyULID"},
			},
		},
		{
			Name:   "time.Now",
			Action: flowir.TimeNow{Output: "typedNow"},
			ScalarArgs: map[string]flowir.ScalarArg{
				"output": {Kind: flowir.ScalarString, String: "legacyNow"},
			},
		},
		{
			Name: "time.Format",
			Action: flowir.TimeFormat{
				Input:  flowir.Expression{Source: "typedNow"},
				Format: "time.RFC3339",
				Output: "typedFormatted",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyNow"},
				"output": {Kind: flowir.ScalarString, String: "legacyFormatted"},
			},
		},
		{
			Name: "time.InZone",
			Action: flowir.TimeInZone{
				Input:    flowir.Expression{Source: "typedNow"},
				Timezone: `"Europe/Berlin"`,
				Output:   "typedBerlinTime",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":    {Kind: flowir.ScalarString, String: "legacyNow"},
				"timezone": {Kind: flowir.ScalarString, String: `"UTC"`},
				"output":   {Kind: flowir.ScalarString, String: "legacyBerlinTime"},
			},
		},
		{
			Name: "math.Op",
			Action: flowir.MathOperation{
				Operation: flowir.Expression{Source: `"clamp"`},
				Value:     flowir.Expression{Source: "req.TypedScore"},
				Min:       flowir.Expression{Source: "0"},
				Max:       flowir.Expression{Source: "100"},
				Output:    "typedClamped",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"op":     {Kind: flowir.ScalarString, String: `"round"`},
				"value":  {Kind: flowir.ScalarString, String: "req.LegacyScore"},
				"output": {Kind: flowir.ScalarString, String: "legacyClamped"},
			},
		},
		{
			Name: "num.Add",
			Action: flowir.NumberBinary{
				Operation: "num.Add",
				A:         flowir.Expression{Source: "req.TypedA"},
				B:         flowir.Expression{Source: "req.TypedB"},
				Output:    "typedSum",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"a":      {Kind: flowir.ScalarString, String: "req.LegacyA"},
				"b":      {Kind: flowir.ScalarString, String: "req.LegacyB"},
				"output": {Kind: flowir.ScalarString, String: "legacySum"},
			},
		},
		{
			Name: "num.Div",
			Action: flowir.NumberBinary{
				Operation: "num.Div",
				A:         flowir.Expression{Source: "req.TypedNumerator"},
				B:         flowir.Expression{Source: "req.TypedDenominator"},
				Output:    "typedRatio",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"a":      {Kind: flowir.ScalarString, String: "req.LegacyNumerator"},
				"b":      {Kind: flowir.ScalarString, String: "req.LegacyDenominator"},
				"output": {Kind: flowir.ScalarString, String: "legacyRatio"},
			},
		},
		{
			Name: "jsonpath.Get",
			Action: flowir.JSONPathGet{
				Input:  flowir.Expression{Source: "req.TypedPayload"},
				Path:   flowir.Expression{Source: `"$.typed.email"`},
				Output: "typedEmail",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"path":   {Kind: flowir.ScalarString, String: `"$.legacy.email"`},
				"output": {Kind: flowir.ScalarString, String: "legacyEmail"},
			},
		},
		{
			Name: "jsonpath.Set",
			Action: flowir.JSONPathSet{
				Input:  flowir.Expression{Source: "typedPayloadMap"},
				Path:   flowir.Expression{Source: `"$.typed.role"`},
				Value:  flowir.Expression{Source: `"admin"`},
				Output: "typedPatched",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyPayloadMap"},
				"path":   {Kind: flowir.ScalarString, String: `"$.legacy.role"`},
				"output": {Kind: flowir.ScalarString, String: "legacyPatched"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "req.TypedEmail") || !strings.Contains(got, "typedOK") || !strings.Contains(got, "req.TypedName") || !strings.Contains(got, "typedSlug") || !strings.Contains(got, "req.TypedPayload") || !strings.Contains(got, "typedB64") || !strings.Contains(got, "req.TypedEncoded") || !strings.Contains(got, "typedRaw") || !strings.Contains(got, "req.TypedURL") || !strings.Contains(got, "typedURL") || !strings.Contains(got, "req.TypedPath") || !strings.Contains(got, "typedBase") || !strings.Contains(got, "typed.example") || !strings.Contains(got, "typedBuiltURL") || !strings.Contains(got, "typedQueryMap") || !strings.Contains(got, "typedRawQuery") || !strings.Contains(got, "typedRawInput") || !strings.Contains(got, "typedQueryVals") || !strings.Contains(got, "typedDigest") || !strings.Contains(got, "req.TypedSecret") || !strings.Contains(got, "typedSignature") || !strings.Contains(got, "typedUUID") || !strings.Contains(got, "typedULID") || !strings.Contains(got, "typedNow") || !strings.Contains(got, "typedFormatted") || !strings.Contains(got, "typedBerlinTime") || !strings.Contains(got, "typedClamped") || !strings.Contains(got, "req.TypedA") || !strings.Contains(got, "typedSum") || !strings.Contains(got, "req.TypedNumerator") || !strings.Contains(got, "typedRatio") || !strings.Contains(got, "$.typed.email") || !strings.Contains(got, "typedEmail") || !strings.Contains(got, "typedPayloadMap") || !strings.Contains(got, "typedPatched") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("data transform renderers did not use typed actions directly:\n%s", got)
	}
}

func TestTypedSecurityDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "jwt.Sign",
			Action: flowir.JWTSign{
				Claims: flowir.Expression{Source: `map[string]any{"sub": req.TypedUserID}`},
				Secret: flowir.Expression{Source: `"typed-jwt-secret"`},
				TTL:    flowir.Expression{Source: `"1h"`},
				Output: "typedJWT",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"claims": {Kind: flowir.ScalarString, String: `map[string]any{"sub": req.LegacyUserID}`},
				"secret": {Kind: flowir.ScalarString, String: `"legacy-jwt-secret"`},
				"output": {Kind: flowir.ScalarString, String: "legacyJWT"},
			},
		},
		{
			Name: "jwt.Verify",
			Action: flowir.JWTVerify{
				Token:  flowir.Expression{Source: "typedJWT"},
				Secret: flowir.Expression{Source: `"typed-jwt-secret"`},
				Output: "typedJWTClaims",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"token":  {Kind: flowir.ScalarString, String: "legacyJWT"},
				"secret": {Kind: flowir.ScalarString, String: `"legacy-jwt-secret"`},
				"output": {Kind: flowir.ScalarString, String: "legacyJWTClaims"},
			},
		},
		{
			Name: "token.Generate",
			Action: flowir.TokenGenerate{
				Subject: flowir.Expression{Source: "req.TypedUserID"},
				Purpose: flowir.Expression{Source: `"verify_email"`},
				Secret:  flowir.Expression{Source: `"typed-token-secret"`},
				TTL:     flowir.Expression{Source: `"30m"`},
				Output:  "typedToken",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"subject": {Kind: flowir.ScalarString, String: "req.LegacyUserID"},
				"secret":  {Kind: flowir.ScalarString, String: `"legacy-token-secret"`},
				"output":  {Kind: flowir.ScalarString, String: "legacyToken"},
			},
		},
		{
			Name: "token.Verify",
			Action: flowir.TokenVerify{
				Token:   flowir.Expression{Source: "typedToken"},
				Purpose: flowir.Expression{Source: `"verify_email"`},
				Secret:  flowir.Expression{Source: `"typed-token-secret"`},
				Output:  "typedTokenClaims",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"token":  {Kind: flowir.ScalarString, String: "legacyToken"},
				"secret": {Kind: flowir.ScalarString, String: `"legacy-token-secret"`},
				"output": {Kind: flowir.ScalarString, String: "legacyTokenClaims"},
			},
		},
		{
			Name: "crypto.Hash",
			Action: flowir.CryptoHash{
				Input:     flowir.Expression{Source: "req.TypedPassword"},
				Algorithm: "bcrypt",
				Output:    "typedPasswordHash",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyPassword"},
				"algo":   {Kind: flowir.ScalarString, String: "sha256"},
				"output": {Kind: flowir.ScalarString, String: "legacyPasswordHash"},
			},
		},
		{
			Name: "oauth2.Token",
			Action: flowir.OAuth2Token{OAuth2Fields: flowir.OAuth2Fields{
				TokenURL:     flowir.Expression{Source: `"https://typed.example/token"`},
				ClientID:     flowir.Expression{Source: `"typed-id"`},
				ClientSecret: flowir.Expression{Source: `"typed-secret"`},
				Output:       "typedOAuthToken",
			}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"tokenURL": {Kind: flowir.ScalarString, String: `"https://legacy.example/token"`},
				"output":   {Kind: flowir.ScalarString, String: "legacyOAuthToken"},
			},
		},
		{
			Name: "oauth2.Refresh",
			Action: flowir.OAuth2Refresh{OAuth2Fields: flowir.OAuth2Fields{
				TokenURL:     flowir.Expression{Source: `"https://typed.example/token"`},
				RefreshToken: flowir.Expression{Source: "typedRefreshToken"},
				Output:       "typedOAuthRefresh",
			}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"tokenURL":     {Kind: flowir.ScalarString, String: `"https://legacy.example/token"`},
				"refreshToken": {Kind: flowir.ScalarString, String: "legacyRefreshToken"},
				"output":       {Kind: flowir.ScalarString, String: "legacyOAuthRefresh"},
			},
		},
		{
			Name: "crypto.Encrypt",
			Action: flowir.CryptoCipher{
				Input:  flowir.Expression{Source: "req.TypedPayload"},
				Key:    flowir.Expression{Source: `"typed-encryption-key"`},
				Output: "typedCipher",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"key":    {Kind: flowir.ScalarString, String: `"legacy-encryption-key"`},
				"output": {Kind: flowir.ScalarString, String: "legacyCipher"},
			},
		},
		{
			Name: "crypto.Decrypt",
			Action: flowir.CryptoCipher{
				Decrypt: true,
				Input:   flowir.Expression{Source: "typedCipher"},
				Key:     flowir.Expression{Source: `"typed-encryption-key"`},
				Output:  "typedPlain",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyCipher"},
				"key":    {Kind: flowir.ScalarString, String: `"legacy-encryption-key"`},
				"output": {Kind: flowir.ScalarString, String: "legacyPlain"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "req.TypedUserID") || !strings.Contains(got, "typedJWT") || !strings.Contains(got, "typedJWTClaims") || !strings.Contains(got, "typedToken") || !strings.Contains(got, "typedTokenClaims") || !strings.Contains(got, "req.TypedPassword") || !strings.Contains(got, "typedPasswordHash") || !strings.Contains(got, "typed.example") || !strings.Contains(got, "typedOAuthToken") || !strings.Contains(got, "typedRefreshToken") || !strings.Contains(got, "typedOAuthRefresh") || !strings.Contains(got, "req.TypedPayload") || !strings.Contains(got, "typedCipher") || !strings.Contains(got, "typedPlain") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("security renderer did not use typed action directly:\n%s", got)
	}
}

func TestTypedCallDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name: "logic.Call",
		Action: flowir.LogicCall{
			Function:    flowir.Expression{Source: "typedFunction"},
			Arguments:   []flowir.Expression{{Source: "req.Value"}},
			CallOptions: flowir.CallOptions{Output: "typedResult"},
		},
		ScalarArgs: map[string]flowir.ScalarArg{
			"func":   {Kind: flowir.ScalarString, String: "legacyFunction"},
			"output": {Kind: flowir.ScalarString, String: "legacyResult"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "typedResult, err := typedFunction(req.Value)") || strings.Contains(got, "legacy") {
		t.Fatalf("call renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedRepositoryDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name: "repo.Get",
		Action: flowir.RepositoryCall{
			Operation: flowir.RepoGet,
			Entity:    "TypedEntity",
			Input:     flowir.Expression{Source: "req.TypedID"},
			Output:    "typedEntity",
		},
		ScalarArgs: map[string]flowir.ScalarArg{
			"source": {Kind: flowir.ScalarString, String: "LegacyEntity"},
			"input":  {Kind: flowir.ScalarString, String: "req.LegacyID"},
			"output": {Kind: flowir.ScalarString, String: "legacyEntity"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "s.TypedEntityRepo.FindByID(ctx, req.TypedID)") || !strings.Contains(got, "typedEntity") || strings.Contains(got, "Legacy") || strings.Contains(got, "legacyEntity") {
		t.Fatalf("repository renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedMappingDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name: "mapping.Assign",
		Action: flowir.MappingAssign{
			Target:  flowir.Expression{Source: "typedTarget"},
			Value:   flowir.Expression{Source: "req.TypedValue"},
			Declare: true,
		},
		ScalarArgs: map[string]flowir.ScalarArg{
			"to":    {Kind: flowir.ScalarString, String: "legacyTarget"},
			"value": {Kind: flowir.ScalarString, String: "req.LegacyValue"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "typedTarget := req.TypedValue") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("mapping renderer did not use the typed action directly:\n%s", got)
	}
}

func TestTypedHTTPCallDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{declared: map[string]bool{"resp": true, "err": true}, pointers: map[string]bool{}, types: map[string]string{}, stepN: &n}
	step := flowir.TypedStep{
		Name:       "http.Call",
		Action:     flowir.HTTPCall{Method: "GET", URL: flowir.Expression{Source: `"https://typed.invalid"`}, Output: "typedBody"},
		ScalarArgs: map[string]flowir.ScalarArg{"url": {Kind: flowir.ScalarString, String: `"https://legacy.invalid"`}},
	}
	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "typed.invalid") || !strings.Contains(got, "typedBody") || strings.Contains(got, "legacy.invalid") {
		t.Fatalf("http.Call did not use typed action:\n%s", got)
	}
}

func TestTypedConcurrencyAndDeliveryDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name:   "parallel.Run",
			Action: flowir.ParallelRun{MaxConcurrency: 2},
			Branches: map[string][]flowir.TypedStep{
				"typedBranch": {{
					Name: "mapping.Assign",
					Action: flowir.MappingAssign{
						Target:  flowir.Expression{Source: "typedBranchValue"},
						Value:   flowir.Expression{Source: "req.TypedValue"},
						Declare: true,
					},
				}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"maxConcurrency": {Kind: flowir.ScalarInt, Int: 99},
			},
		},
		{
			Name: "pdf.Render",
			Action: flowir.PDFRender{
				Template: flowir.Expression{Source: `"typed-template"`},
				Data:     flowir.Expression{Source: "req.TypedReport"},
				Output:   "typedPDF",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"data":   {Kind: flowir.ScalarString, String: "req.LegacyReport"},
				"output": {Kind: flowir.ScalarString, String: "legacyPDF"},
			},
		},
		{
			Name: "queue.Enqueue",
			Action: flowir.QueueEnqueue{
				Subject: flowir.Expression{Source: `"typed.events"`},
				Payload: flowir.Expression{Source: "req.TypedPayload"},
				Timeout: flowir.Expression{Source: "time.Second"},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"subject": {Kind: flowir.ScalarString, String: `"legacy.events"`},
				"payload": {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "typedBranchValue") || !strings.Contains(got, "req.TypedValue") || !strings.Contains(got, "make(chan struct{}, 2)") || !strings.Contains(got, "req.TypedReport") || !strings.Contains(got, "typedPDF") || !strings.Contains(got, `"typed.events"`) || !strings.Contains(got, "req.TypedPayload") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("concurrency and delivery renderer did not use typed actions directly:\n%s", got)
	}
}

func TestTypedEventOrchestrationDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "notify.Email",
			Action: flowir.NotifyEmail{
				To:      flowir.Expression{Source: "req.TypedEmail"},
				Text:    flowir.Expression{Source: `"typed notification"`},
				Subject: flowir.Expression{Source: `"typed subject"`},
				Output:  "typedNotificationID",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"to":     {Kind: flowir.ScalarString, String: "req.LegacyEmail"},
				"text":   {Kind: flowir.ScalarString, String: `"legacy notification"`},
				"output": {Kind: flowir.ScalarString, String: "legacyNotificationID"},
			},
		},
		{
			Name: "event.Outbox",
			Action: flowir.EventOutbox{
				Name:    flowir.Expression{Source: `"TypedEvent"`},
				Payload: flowir.Expression{Source: "req.TypedPayload"},
				ID:      flowir.Expression{Source: "req.TypedID"},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"name":    {Kind: flowir.ScalarString, String: `"LegacyEvent"`},
				"payload": {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"id":      {Kind: flowir.ScalarString, String: "req.LegacyID"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "req.TypedEmail") || !strings.Contains(got, "typed notification") || !strings.Contains(got, "typedNotificationID") || !strings.Contains(got, `"TypedEvent"`) || !strings.Contains(got, "req.TypedPayload") || !strings.Contains(got, "req.TypedID") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("event orchestration renderer did not use typed actions directly:\n%s", got)
	}
}

func TestTypedPolicyDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "policy.Check",
			Action: flowir.PolicyCheck{
				Policy:   "TypedPolicy",
				User:     flowir.Expression{Source: "typedUser"},
				Output:   "typedAllowed",
				Resolved: true,
				Roles:    []string{"typed-role"},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"policy": {Kind: flowir.ScalarString, String: "LegacyPolicy"},
				"user":   {Kind: flowir.ScalarString, String: "legacyUser"},
				"output": {Kind: flowir.ScalarString, String: "legacyAllowed"},
			},
		},
		{
			Name: "policy.Require",
			Action: flowir.PolicyDecisionAction{
				Alias:     "policy.Require",
				PolicyKey: flowir.Expression{Source: `"typed.policy"`},
				Subject:   flowir.Expression{Source: "req.TypedUserID"},
				Operation: flowir.Expression{Source: `"write"`},
				Throw:     flowir.Expression{Source: `"typed policy denied"`},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"policyKey": {Kind: flowir.ScalarString, String: `"legacy.policy"`},
				"subject":   {Kind: flowir.ScalarString, String: "req.LegacyUserID"},
				"throw":     {Kind: flowir.ScalarString, String: `"legacy policy denied"`},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "TypedPolicy") || !strings.Contains(got, "typedUser") || !strings.Contains(got, "typedAllowed") || !strings.Contains(got, "typed-role") || !strings.Contains(got, `"typed.policy"`) || !strings.Contains(got, "req.TypedUserID") || !strings.Contains(got, "typed policy denied") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("policy renderer did not use typed actions directly:\n%s", got)
	}
}

func TestTypedReliabilityDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name:   "dedupe.Once",
			Action: flowir.DedupeOnce{Key: flowir.Expression{Source: "typedKey"}, TTL: flowir.Expression{Source: "time.Minute"}},
			Children: map[string][]flowir.TypedStep{
				"_do": {{
					Name: "mapping.Assign",
					Action: flowir.MappingAssign{
						Target:  flowir.Expression{Source: "typedDeduped"},
						Value:   flowir.Expression{Source: "req.TypedValue"},
						Declare: true,
					},
				}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"key": {Kind: flowir.ScalarString, String: "legacyKey"},
			},
		},
		{
			Name: "log.Emit",
			Action: flowir.LogEmit{
				Message: flowir.Expression{Source: `"typed log"`},
				Level:   "info",
				Fields:  map[string]flowir.Expression{"typedField": {Source: "req.TypedID"}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"message": {Kind: flowir.ScalarString, String: `"legacy log"`},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "typedKey") || !strings.Contains(got, "typedDeduped") || !strings.Contains(got, "req.TypedValue") || !strings.Contains(got, "typed log") || !strings.Contains(got, "typedField") || !strings.Contains(got, "req.TypedID") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("reliability renderer did not use typed actions directly:\n%s", got)
	}
}

func TestTypedOAuthGoogleDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{declared: map[string]bool{"resp": true, "err": true}, pointers: map[string]bool{}, types: map[string]string{}, stepN: &n}
	step := flowir.TypedStep{
		Name: "oauth.Google.GetURL",
		Action: flowir.OAuthGoogleGetURL{
			ClientID:    flowir.Expression{Source: `"typed-client"`},
			RedirectURL: flowir.Expression{Source: `"https://typed.example/callback"`},
			State:       flowir.Expression{Source: "typedState"},
			Scopes:      flowir.Expression{Source: `"email"`},
			Output:      "typedURL",
		},
		ScalarArgs: map[string]flowir.ScalarArg{
			"clientID":    {Kind: flowir.ScalarString, String: `"legacy-client"`},
			"redirectURL": {Kind: flowir.ScalarString, String: `"https://legacy.example/callback"`},
			"output":      {Kind: flowir.ScalarString, String: "legacyURL"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "typed-client") || !strings.Contains(got, "typed.example") || !strings.Contains(got, "typedState") || !strings.Contains(got, "typedURL") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("Google OAuth renderer did not use typed action directly:\n%s", got)
	}
}
