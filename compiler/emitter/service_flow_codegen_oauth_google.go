package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// renderFlowStepOAuthGoogle handles oauth.Google.* actions.
func renderFlowStepOAuthGoogle(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, _ func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Action {
	case "oauth.Google.GetURL":
		return renderOAuthGoogleGetURL(st, step, pad, sfx, arg), true
	case "oauth.Google.Exchange":
		return renderOAuthGoogleExchange(st, step, pad, sfx, arg), true
	case "oauth.Google.UserInfo":
		return renderOAuthGoogleUserInfo(st, step, pad, sfx, arg), true
	}
	return "", false
}

// renderOAuthGoogleGetURL generates the authorization URL for the Google OAuth flow.
//
// CUE usage:
//
//	{ action: "oauth.Google.GetURL", clientID: "s.cfg.GoogleClientID",
//	  redirectURL: "s.cfg.GoogleRedirectURL", state: "stateVar", output: "authURL" }
func renderOAuthGoogleGetURL(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	clientID := arg("clientID")
	redirectURL := arg("redirectURL")
	output := arg("output")
	if clientID == "" || redirectURL == "" || output == "" {
		return renderInvalidFlowStepConfig(st, pad, "oauth.Google.GetURL", "clientID, redirectURL and output are required")
	}

	stateExpr := arg("state")
	if stateExpr == "" {
		stateExpr = `""`
	}
	scopesExpr := arg("scopes")
	if scopesExpr == "" {
		scopesExpr = `"openid email profile"`
	}

	assign := ":="
	if st.declared[output] {
		assign = "="
	}
	st.declared[output] = true
	st.pointers[output] = false
	st.types[output] = "string"

	cfgV := "_oauthCfg" + sfx
	scopesV := "_oauthScopes" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := strings.Fields(%s)\n", pad, scopesV, scopesExpr))
	b.WriteString(fmt.Sprintf("%sif len(%s) == 0 {\n", pad, scopesV))
	b.WriteString(fmt.Sprintf("%s\t%s = []string{\"openid\", \"email\", \"profile\"}\n", pad, scopesV))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := &oauth2.Config{\n", pad, cfgV))
	b.WriteString(fmt.Sprintf("%s\tClientID:    %s,\n", pad, clientID))
	b.WriteString(fmt.Sprintf("%s\tRedirectURL: %s,\n", pad, redirectURL))
	b.WriteString(fmt.Sprintf("%s\tScopes:      %s,\n", pad, scopesV))
	b.WriteString(fmt.Sprintf("%s\tEndpoint:    google.Endpoint,\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s %s %s.AuthCodeURL(%s, oauth2.AccessTypeOnline)\n", pad, output, assign, cfgV, stateExpr))
	return b.String()
}

// renderOAuthGoogleExchange exchanges the authorization code for an *oauth2.Token.
//
// CUE usage:
//
//	{ action: "oauth.Google.Exchange",
//	  clientID: "s.cfg.GoogleClientID", clientSecret: "s.cfg.GoogleClientSecret",
//	  redirectURL: "s.cfg.GoogleRedirectURL", code: "req.Code", output: "token" }
func renderOAuthGoogleExchange(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	clientID := arg("clientID")
	clientSecret := arg("clientSecret")
	redirectURL := arg("redirectURL")
	code := arg("code")
	output := arg("output")
	if clientID == "" || clientSecret == "" || redirectURL == "" || code == "" || output == "" {
		return renderInvalidFlowStepConfig(st, pad, "oauth.Google.Exchange", "clientID, clientSecret, redirectURL, code and output are required")
	}

	scopesExpr := arg("scopes")
	if scopesExpr == "" {
		scopesExpr = `"openid email profile"`
	}

	assign := ":="
	if st.declared[output] {
		assign = "="
	}
	st.declared[output] = true
	st.pointers[output] = true
	st.types[output] = "*oauth2.Token"

	cfgV := "_oauthCfg" + sfx
	scopesV := "_oauthScopes" + sfx
	errV := "_oauthExErr" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := strings.Fields(%s)\n", pad, scopesV, scopesExpr))
	b.WriteString(fmt.Sprintf("%sif len(%s) == 0 {\n", pad, scopesV))
	b.WriteString(fmt.Sprintf("%s\t%s = []string{\"openid\", \"email\", \"profile\"}\n", pad, scopesV))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s := &oauth2.Config{\n", pad, cfgV))
	b.WriteString(fmt.Sprintf("%s\tClientID:     %s,\n", pad, clientID))
	b.WriteString(fmt.Sprintf("%s\tClientSecret: %s,\n", pad, clientSecret))
	b.WriteString(fmt.Sprintf("%s\tRedirectURL:  %s,\n", pad, redirectURL))
	b.WriteString(fmt.Sprintf("%s\tScopes:       %s,\n", pad, scopesV))
	b.WriteString(fmt.Sprintf("%s\tEndpoint:     google.Endpoint,\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%s%s, %s %s %s.Exchange(ctx, %s)\n", pad, output, errV, assign, cfgV, code))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"oauth.Google.Exchange: %%w\", %s)", errV)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

// renderOAuthGoogleUserInfo fetches the Google userinfo profile for a given token.
//
// CUE usage:
//
//	{ action: "oauth.Google.UserInfo", token: "token", output: "profile" }
//
// The output variable has fields: Sub, Email, EmailVerified, Name, GivenName, FamilyName, Picture.
func renderOAuthGoogleUserInfo(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) string {
	tokenExpr := arg("token")
	output := arg("output")
	if tokenExpr == "" || output == "" {
		return renderInvalidFlowStepConfig(st, pad, "oauth.Google.UserInfo", "token and output are required")
	}

	assign := ":="
	if st.declared[output] {
		assign = "="
	}
	st.declared[output] = true
	st.pointers[output] = false
	st.types[output] = "struct{Sub string;Email string;EmailVerified bool;Name string;GivenName string;FamilyName string;Picture string}"

	clientV := "_oauthClient" + sfx
	respV := "_oauthUIResp" + sfx
	errV := "_oauthUIErr" + sfx
	decErrV := "_oauthDecErr" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := oauth2.NewClient(ctx, oauth2.StaticTokenSource(%s))\n", pad, clientV, tokenExpr))
	b.WriteString(fmt.Sprintf("%s%s, %s := %s.Get(\"https://www.googleapis.com/oauth2/v3/userinfo\")\n", pad, respV, errV, clientV))
	b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errV))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"oauth.Google.UserInfo: %%w\", %s)", errV)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	b.WriteString(fmt.Sprintf("%sdefer %s.Body.Close()\n", pad, respV))

	b.WriteString(fmt.Sprintf("%svar %s struct {\n", pad, output))
	b.WriteString(fmt.Sprintf("%s\tSub           string `json:\"sub\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tEmail         string `json:\"email\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tEmailVerified bool   `json:\"email_verified\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tName          string `json:\"name\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tGivenName     string `json:\"given_name\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tFamilyName    string `json:\"family_name\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s\tPicture       string `json:\"picture\"`\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))

	if st.declared[output] && assign == "=" {
		// re-declare in place — reset to zero then decode
		b.WriteString(fmt.Sprintf("%s%s = struct {\n", pad, output))
		b.WriteString(fmt.Sprintf("%s\tSub           string `json:\"sub\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tEmail         string `json:\"email\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tEmailVerified bool   `json:\"email_verified\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tName          string `json:\"name\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tGivenName     string `json:\"given_name\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tFamilyName    string `json:\"family_name\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s\tPicture       string `json:\"picture\"`\n", pad))
		b.WriteString(fmt.Sprintf("%s}{}\n", pad))
	}

	b.WriteString(fmt.Sprintf("%sif %s := json.NewDecoder(%s.Body).Decode(&%s); %s != nil {\n", pad, decErrV, respV, output, decErrV))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"oauth.Google.UserInfo: decode: %%w\", %s)", decErrV)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
