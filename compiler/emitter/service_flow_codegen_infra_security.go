package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderOAuthTokenLike(st *flowRenderState, fields flowir.OAuth2Fields, pad, sfx, action string, refreshOnly bool) string {
	expr := func(v flowir.Expression) string { return normalizeFlowExpr(v.Source) }
	tokenURL, output := expr(fields.TokenURL), fields.Output
	clientID, clientSecret, scope, audience := expr(fields.ClientID), expr(fields.ClientSecret), expr(fields.Scope), expr(fields.Audience)
	grantType, refreshToken, code, redirectURI := expr(fields.GrantType), expr(fields.RefreshToken), expr(fields.Code), expr(fields.RedirectURI)
	username, password := expr(fields.Username), expr(fields.Password)

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
