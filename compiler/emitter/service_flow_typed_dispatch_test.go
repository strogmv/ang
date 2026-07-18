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

func TestTypedRepositoryAdvancedDispatchUsesTypedArgumentsAndChildren(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "repo.Query",
			Action: flowir.RepositoryCall{
				Operation: flowir.RepoQuery,
				Entity:    "TypedEntity",
				Method:    "FindByEmail",
				Arguments: []flowir.Expression{{Source: "req.TypedEmail"}, {Source: "req.TypedCompanyID"}},
				Output:    "typedQueryResult",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"source": {Kind: flowir.ScalarString, String: "LegacyEntity"},
				"method": {Kind: flowir.ScalarString, String: "FindByLegacy"},
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyEmail"},
				"output": {Kind: flowir.ScalarString, String: "legacyQueryResult"},
			},
		},
		{
			Name: "repo.Upsert",
			Action: flowir.RepositoryCall{
				Operation: flowir.RepoUpsert,
				Entity:    "TypedEntity",
				Find:      flowir.Expression{Source: "req.TypedID"},
				Input:     flowir.Expression{Source: "typedInput"},
				Output:    "typedUpsertResult",
			},
			Children: map[string][]flowir.TypedStep{
				"_ifNew": {{
					Name: "mapping.Assign",
					Action: flowir.MappingAssign{
						Target:  flowir.Expression{Source: "typedUpsertMarker"},
						Value:   flowir.Expression{Source: `"new"`},
						Declare: true,
					},
				}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"source": {Kind: flowir.ScalarString, String: "LegacyEntity"},
				"find":   {Kind: flowir.ScalarString, String: "req.LegacyID"},
				"input":  {Kind: flowir.ScalarString, String: "legacyInput"},
				"output": {Kind: flowir.ScalarString, String: "legacyUpsertResult"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"FindByEmail", "req.TypedEmail", "req.TypedCompanyID", "typedQueryResult", "req.TypedID", "typedInput", "typedUpsertResult", "typedUpsertMarker"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("advanced repository renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("advanced repository renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedDBDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "db.Get",
			Action: flowir.DBGet{DBFields: flowir.DBFields{
				Source: "TypedEntity", Input: flowir.Expression{Source: "req.TypedID"}, Output: "typedEntity",
			}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"source": {Kind: flowir.ScalarString, String: "LegacyEntity"},
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyID"},
				"output": {Kind: flowir.ScalarString, String: "legacyEntity"},
			},
		},
		{
			Name: "db.Update",
			Action: flowir.DBUpdate{DBFields: flowir.DBFields{
				Source: "TypedEntity", Input: flowir.Expression{Source: "typedEntity"},
			}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"source": {Kind: flowir.ScalarString, String: "LegacyEntity"},
				"input":  {Kind: flowir.ScalarString, String: "legacyEntity"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"TypedEntityRepo.FindByID", "req.TypedID", "typedEntity", "TypedEntityRepo.Update"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("DB renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("DB renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedDomainPrimitivesDoNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name:   "list.Len",
			Action: flowir.ListLen{Input: flowir.Expression{Source: "typedItems"}, Output: "typedCount"},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyItems"},
				"output": {Kind: flowir.ScalarString, String: "legacyCount"},
			},
		},
		{
			Name: "map.Set",
			Action: flowir.MapSet{
				Input:  flowir.Expression{Source: "typedMap"},
				Key:    flowir.Expression{Source: `"typed-key"`},
				Value:  flowir.Expression{Source: "typedValue"},
				Output: "typedMapCopy",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyMap"},
				"key":    {Kind: flowir.ScalarString, String: `"legacy-key"`},
				"value":  {Kind: flowir.ScalarString, String: "legacyValue"},
				"output": {Kind: flowir.ScalarString, String: "legacyMapCopy"},
			},
		},
		{
			Name: "map.Get",
			Action: flowir.MapGet{
				Input:   flowir.Expression{Source: "typedMap"},
				Key:     flowir.Expression{Source: `"typed-key"`},
				Output:  "typedMapValue",
				Into:    "string",
				Default: flowir.Expression{Source: `"typed-default"`},
				Found:   "typedFound",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":   {Kind: flowir.ScalarString, String: "legacyMap"},
				"key":     {Kind: flowir.ScalarString, String: `"legacy-key"`},
				"output":  {Kind: flowir.ScalarString, String: "legacyMapValue"},
				"default": {Kind: flowir.ScalarString, String: `"legacy-default"`},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typedItems", "typedCount", "typedMap", "typed-key", "typedValue", "typedMapCopy", "typedMapValue", "typed-default", "typedFound"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("domain primitive renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("domain primitive renderer reconstructed legacy arguments:\n%s", got)
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

func TestTypedMappingMapConstructsEntityWithoutInput(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "mapping.Map",
		Action: flowir.MappingMap{Output: "newUser", Entity: "User"},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "var newUser domain.User") {
		t.Fatalf("entity construction did not declare a domain value:\n%s", got)
	}
	if strings.Contains(got, "helpers.Assign") {
		t.Fatalf("entity construction without input emitted invalid assignment:\n%s", got)
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

func TestTypedControlResilienceDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name:   "flow.Try",
			Action: flowir.FlowTry{Retries: 2, BackoffMS: 7},
			Children: map[string][]flowir.TypedStep{
				"_do": {{
					Name: "mapping.Assign",
					Action: flowir.MappingAssign{
						Target:  flowir.Expression{Source: "typedTryValue"},
						Value:   flowir.Expression{Source: "req.TypedValue"},
						Declare: true,
					},
				}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"retries":   {Kind: flowir.ScalarInt, Int: 99},
				"backoffMs": {Kind: flowir.ScalarInt, Int: 999},
			},
		},
		{
			Name:   "flow.Timeout",
			Action: flowir.FlowTimeout{Duration: flowir.Expression{Source: "time.Second"}},
			Children: map[string][]flowir.TypedStep{
				"_do": {{
					Name: "mapping.Assign",
					Action: flowir.MappingAssign{
						Target:  flowir.Expression{Source: "typedTimeoutValue"},
						Value:   flowir.Expression{Source: "req.TypedTimeoutValue"},
						Declare: true,
					},
				}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"duration": {Kind: flowir.ScalarString, String: "24 * time.Hour"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	if !strings.Contains(got, "_tryMax_0 := 2") || !strings.Contains(got, "typedTryValue") || !strings.Contains(got, "req.TypedValue") || !strings.Contains(got, "time.Second") || !strings.Contains(got, "typedTimeoutValue") || !strings.Contains(got, "req.TypedTimeoutValue") || strings.Contains(got, "999") || strings.Contains(got, "24 * time.Hour") || strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("control resilience renderer did not use typed actions and children directly:\n%s", got)
	}
}

func TestTypedControlFlowBasicDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	assign := func(target, value string) flowir.TypedStep {
		return flowir.TypedStep{
			Name: "mapping.Assign",
			Action: flowir.MappingAssign{
				Target:  flowir.Expression{Source: target},
				Value:   flowir.Expression{Source: value},
				Declare: true,
			},
		}
	}
	steps := []flowir.TypedStep{
		{
			Name:   "flow.If",
			Action: flowir.FlowIf{Condition: flowir.Expression{Source: "typedCondition"}},
			Children: map[string][]flowir.TypedStep{
				"_then": {assign("typedIfValue", "req.TypedIfValue")},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"condition": {Kind: flowir.ScalarString, String: "legacyCondition"},
			},
		},
		{
			Name:   "flow.Switch",
			Action: flowir.FlowSwitch{Value: flowir.Expression{Source: "typedStatus"}, Match: "exact"},
			Branches: map[string][]flowir.TypedStep{
				"typed": {assign("typedSwitchValue", "req.TypedSwitchValue")},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"value": {Kind: flowir.ScalarString, String: "legacyStatus"},
				"match": {Kind: flowir.ScalarString, String: "prefix"},
			},
		},
		{
			Name:   "flow.For",
			Action: flowir.FlowFor{Each: flowir.Expression{Source: "typedItems"}, As: "typedItem"},
			Children: map[string][]flowir.TypedStep{
				"_do": {assign("typedForValue", "typedItem")},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"each": {Kind: flowir.ScalarString, String: "legacyItems"},
				"as":   {Kind: flowir.ScalarString, String: "legacyItem"},
			},
		},
		{
			Name:   "flow.While",
			Action: flowir.FlowWhile{Condition: flowir.Expression{Source: "typedContinue"}},
			Children: map[string][]flowir.TypedStep{
				"_do": {assign("typedWhileValue", "req.TypedWhileValue")},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"condition": {Kind: flowir.ScalarString, String: "legacyContinue"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typedCondition", "typedIfValue", "typedStatus", "typedSwitchValue", "typedItems", "typedItem", "typedForValue", "typedContinue", "typedWhileValue"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("control-flow renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("control-flow renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedStatefulControlDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "flow.Checkpoint",
			Action: flowir.FlowCheckpoint{
				Name: "typed-checkpoint",
				Data: flowir.Expression{Source: "req.TypedCheckpoint"},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"name": {Kind: flowir.ScalarString, String: "legacy-checkpoint"},
				"data": {Kind: flowir.ScalarString, String: "req.LegacyCheckpoint"},
			},
		},
		{
			Name: "flow.RecordEvent",
			Action: flowir.FlowRecordEvent{
				Name:    flowir.Expression{Source: `"typed-event"`},
				Payload: flowir.Expression{Source: "req.TypedPayload"},
				Output:  "typedEvent",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"name":    {Kind: flowir.ScalarString, String: `"legacy-event"`},
				"payload": {Kind: flowir.ScalarString, String: "req.LegacyPayload"},
				"output":  {Kind: flowir.ScalarString, String: "legacyEvent"},
			},
		},
		{
			Name: "flow.Validate",
			Action: flowir.FlowValidate{
				Condition: flowir.Expression{Source: "typedValid"},
				Message:   "typed validation failed",
				Code:      "TYPED_INVALID",
				Status:    flowir.Expression{Source: "http.StatusBadRequest"},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"condition": {Kind: flowir.ScalarString, String: "legacyValid"},
				"message":   {Kind: flowir.ScalarString, String: "legacy validation failed"},
			},
		},
		{
			Name:   "flow.SuggestNext",
			Action: flowir.FlowSuggestNext{Options: []string{"typed-next"}, Output: "typedNext"},
			ScalarArgs: map[string]flowir.ScalarArg{
				"output":    {Kind: flowir.ScalarString, String: "legacyNext"},
				"options.0": {Kind: flowir.ScalarString, String: "legacy-next"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typed-checkpoint", "req.TypedCheckpoint", "typed-event", "req.TypedPayload", "typedEvent", "typedValid", "typed validation failed", "typedNext", "typed-next"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("stateful control renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("stateful control renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedParallelControlDispatchUsesTypedBranches(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	branch := func(target, value string) []flowir.TypedStep {
		return []flowir.TypedStep{{
			Name: "mapping.Assign",
			Action: flowir.MappingAssign{
				Target:  flowir.Expression{Source: target},
				Value:   flowir.Expression{Source: value},
				Declare: true,
			},
		}}
	}
	steps := []flowir.TypedStep{
		{
			Name:   "flow.Parallel",
			Action: flowir.FlowParallel{},
			Branches: map[string][]flowir.TypedStep{
				"typed-first":  branch("typedParallelFirst", "req.TypedParallelFirst"),
				"typed-second": branch("typedParallelSecond", "req.TypedParallelSecond"),
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"legacy": {Kind: flowir.ScalarString, String: "legacy-parallel"},
			},
		},
		{
			Name:   "flow.Race",
			Action: flowir.FlowRace{},
			Branches: map[string][]flowir.TypedStep{
				"typed-race": branch("typedRaceValue", "req.TypedRaceValue"),
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"legacy": {Kind: flowir.ScalarString, String: "legacy-race"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typedParallelFirst", "req.TypedParallelFirst", "typedParallelSecond", "req.TypedParallelSecond", "typedRaceValue", "req.TypedRaceValue", "_fp_0Wg", "_fr_1Wg"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("parallel control renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("parallel control renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedSchedulingControlDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name:   "flow.Delay",
			Action: flowir.FlowDelay{Duration: flowir.Expression{Source: "typedDelay"}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"duration": {Kind: flowir.ScalarString, String: "legacyDelay"},
			},
		},
		{
			Name:   "flow.Schedule",
			Action: flowir.FlowSchedule{At: flowir.Expression{Source: "typedAt"}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"at": {Kind: flowir.ScalarString, String: "legacyAt"},
			},
		},
		{
			Name:   "flow.Tag",
			Action: flowir.FlowTag{Name: flowir.Expression{Source: `"typed-tag"`}, Value: flowir.Expression{Source: "typedTagValue"}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"name":  {Kind: flowir.ScalarString, String: `"legacy-tag"`},
				"value": {Kind: flowir.ScalarString, String: "legacyTagValue"},
			},
		},
		{
			Name:   "flow.Return",
			Action: flowir.FlowReturn{Set: flowir.Expression{Source: "resp.TypedStatus"}, Value: flowir.Expression{Source: `"typed-ok"`}},
			ScalarArgs: map[string]flowir.ScalarArg{
				"set":   {Kind: flowir.ScalarString, String: "resp.LegacyStatus"},
				"value": {Kind: flowir.ScalarString, String: `"legacy-ok"`},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typedDelay", "typedAt", "typed-tag", "typedTagValue", "resp.TypedStatus", "typed-ok"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("scheduling control renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("scheduling control renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedSagaControlDispatchUsesTypedChildren(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "flow.Saga",
		Action: flowir.FlowSaga{},
		Children: map[string][]flowir.TypedStep{
			"_do": {{
				Name:   "flow.Compensate",
				Action: flowir.FlowCompensate{},
				Children: map[string][]flowir.TypedStep{
					"_do": {{
						Name: "mapping.Assign",
						Action: flowir.MappingAssign{
							Target:  flowir.Expression{Source: "typedCompensationValue"},
							Value:   flowir.Expression{Source: "req.TypedCompensationValue"},
							Declare: true,
						},
					}},
				},
			}, {
				Name:   "flow.Rollback",
				Action: flowir.FlowRollback{Error: flowir.Expression{Source: `fmt.Errorf("typed rollback")`}},
			}},
		},
		ScalarArgs: map[string]flowir.ScalarArg{
			"legacy": {Kind: flowir.ScalarString, String: "legacy-saga"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	for _, expected := range []string{"_sagaCompensations_0", "typedCompensationValue", "req.TypedCompensationValue", "typed rollback"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("saga renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("saga renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedCollectionDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "list.Append",
			Action: flowir.ListAppend{
				Target: flowir.Expression{Source: "typedItems"},
				Item:   flowir.Expression{Source: "req.TypedItem"},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"to":   {Kind: flowir.ScalarString, String: "legacyItems"},
				"item": {Kind: flowir.ScalarString, String: "req.LegacyItem"},
			},
		},
		{
			Name: "str.Normalize",
			Action: flowir.StringNormalize{
				Input:  flowir.Expression{Source: "req.TypedName"},
				Mode:   "upper",
				Output: "typedName",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "req.LegacyName"},
				"mode":   {Kind: flowir.ScalarString, String: "lower"},
				"output": {Kind: flowir.ScalarString, String: "legacyName"},
			},
		},
		{
			Name: "value.Coalesce",
			Action: flowir.ValueCoalesce{
				Values: []flowir.Expression{{Source: "typedPrimary"}, {Source: "typedFallback"}},
				Output: "typedCoalesced",
				Mode:   "non_nil",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"values.0": {Kind: flowir.ScalarString, String: "legacyPrimary"},
				"output":   {Kind: flowir.ScalarString, String: "legacyCoalesced"},
			},
		},
		{
			Name:   "batch.Run",
			Action: flowir.BatchRun{From: flowir.Expression{Source: "typedBatchItems"}, Size: flowir.Expression{Source: "typedBatchSize"}, As: "typedBatch"},
			Children: map[string][]flowir.TypedStep{
				"_do": {{
					Name: "mapping.Assign",
					Action: flowir.MappingAssign{
						Target:  flowir.Expression{Source: "typedBatchValue"},
						Value:   flowir.Expression{Source: "typedBatch[0]"},
						Declare: true,
					},
				}},
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"from": {Kind: flowir.ScalarString, String: "legacyBatchItems"},
				"size": {Kind: flowir.ScalarString, String: "legacyBatchSize"},
				"as":   {Kind: flowir.ScalarString, String: "legacyBatch"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typedItems", "req.TypedItem", "req.TypedName", "typedName", "typedPrimary", "typedFallback", "typedCoalesced", "typedBatchItems", "typedBatchSize", "typedBatch", "typedBatchValue"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("collection renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("collection renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedDomainErrorDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	steps := []flowir.TypedStep{
		{
			Name: "errors.New",
			Action: flowir.ErrorNew{
				Message: flowir.Expression{Source: `"typed failure"`},
				Status:  flowir.Expression{Source: "http.StatusTeapot"},
				Code:    flowir.Expression{Source: `"TYPED_FAILURE"`},
				Output:  "typedErr",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"message": {Kind: flowir.ScalarString, String: `"legacy failure"`},
				"status":  {Kind: flowir.ScalarString, String: "http.StatusInternalServerError"},
				"code":    {Kind: flowir.ScalarString, String: `"LEGACY_FAILURE"`},
				"output":  {Kind: flowir.ScalarString, String: "legacyErr"},
			},
		},
		{
			Name: "errors.Wrap",
			Action: flowir.ErrorWrap{
				Error:   flowir.Expression{Source: "typedErr"},
				Message: flowir.Expression{Source: `"typed context"`},
				Output:  "typedWrappedErr",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"error":   {Kind: flowir.ScalarString, String: "legacyErr"},
				"message": {Kind: flowir.ScalarString, String: `"legacy context"`},
				"output":  {Kind: flowir.ScalarString, String: "legacyWrappedErr"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typed failure", "http.StatusTeapot", "TYPED_FAILURE", "typedErr", "typed context", "typedWrappedErr"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("domain error renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("domain error renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedDomainAuthDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name: "auth.CheckRole",
		Action: flowir.AuthCheckRole{
			User:      flowir.Expression{Source: "typedUser"},
			Roles:     flowir.Expression{Source: "typedRoles"},
			CompanyID: flowir.Expression{Source: "typedCompanyID"},
		},
		ScalarArgs: map[string]flowir.ScalarArg{
			"user":      {Kind: flowir.ScalarString, String: "legacyUser"},
			"roles":     {Kind: flowir.ScalarString, String: "legacyRoles"},
			"companyID": {Kind: flowir.ScalarString, String: "legacyCompanyID"},
		},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	for _, expected := range []string{"typedUser", "typedRoles", "typedCompanyID"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("domain auth renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("domain auth renderer reconstructed legacy arguments:\n%s", got)
	}
}

func TestTypedInfrastructureDispatchDoesNotReconstructLegacyArgs(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared:    map[string]bool{"resp": true, "err": true},
		pointers:    map[string]bool{},
		types:       map[string]string{},
		stepN:       &n,
		isStreaming: true,
	}
	steps := []flowir.TypedStep{
		{
			Name: "openai.Chat",
			Action: flowir.OpenAIChat{
				UserMessage: flowir.Expression{Source: "typedOpenAIPrompt"},
				Model:       flowir.Expression{Source: `"typed-openai"`},
				Output:      "typedOpenAIReply",
				MaxTokens:   222,
				MaxRounds:   1,
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"user_message": {Kind: flowir.ScalarString, String: "legacyOpenAIPrompt"},
				"output":       {Kind: flowir.ScalarString, String: "legacyOpenAIReply"},
			},
		},
		{
			Name: "openai.Stream",
			Action: flowir.OpenAIStream{
				UserMessage: flowir.Expression{Source: "typedStreamPrompt"},
				Model:       flowir.Expression{Source: `"typed-stream"`},
				Output:      "typedStreamReply",
				MaxTokens:   333,
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"user_message": {Kind: flowir.ScalarString, String: "legacyStreamPrompt"},
				"output":       {Kind: flowir.ScalarString, String: "legacyStreamReply"},
			},
		},
		{
			Name: "claude.Chat",
			Action: flowir.ClaudeChat{
				UserMessage: flowir.Expression{Source: "typedClaudePrompt"},
				Model:       flowir.Expression{Source: `"typed-claude"`},
				Output:      "typedClaudeReply",
				MaxTokens:   111,
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"user_message": {Kind: flowir.ScalarString, String: "legacyClaudePrompt"},
				"output":       {Kind: flowir.ScalarString, String: "legacyClaudeReply"},
			},
		},
		{
			Name: "openai.Embed",
			Action: flowir.OpenAIEmbed{
				Input:      flowir.Expression{Source: "typedEmbeddingInput"},
				Model:      flowir.Expression{Source: `"typed-embedding"`},
				Output:     "typedEmbedding",
				Dimensions: 42,
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyEmbeddingInput"},
				"output": {Kind: flowir.ScalarString, String: "legacyEmbedding"},
			},
		},
		{
			Name: "plan.BuildAutomata",
			Action: flowir.PlanBuildAutomata{
				Input:  flowir.Expression{Source: "typedUsecases"},
				Output: "typedAutomata",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"input":  {Kind: flowir.ScalarString, String: "legacyUsecases"},
				"output": {Kind: flowir.ScalarString, String: "legacyAutomata"},
			},
		},
		{
			Name: "locale.Resolve",
			Action: flowir.LocaleResolve{
				Sources: "typedLocaleSource",
				Default: flowir.Expression{Source: `"typed-default"`},
				Output:  "typedLocale",
			},
			ScalarArgs: map[string]flowir.ScalarArg{
				"sources": {Kind: flowir.ScalarString, String: "legacyLocaleSource"},
				"default": {Kind: flowir.ScalarString, String: `"legacy-default"`},
				"output":  {Kind: flowir.ScalarString, String: "legacyLocale"},
			},
		},
	}

	got := renderTypedFlowSteps(state, steps, 0)
	for _, expected := range []string{"typedOpenAIPrompt", "typedOpenAIReply", "typedStreamPrompt", "typedStreamReply", "typedClaudePrompt", "typedClaudeReply", "typedEmbeddingInput", "typedEmbedding", "typedUsecases", "typedAutomata", "typedLocaleSource", "typed-default", "typedLocale"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("infrastructure renderer omitted %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "legacy") || strings.Contains(got, "Legacy") {
		t.Fatalf("infrastructure renderer reconstructed legacy arguments:\n%s", got)
	}
}
