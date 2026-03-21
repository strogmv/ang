package infra

_appName:      "ANG Blog Example"
_supportEmail: "blog-support@ang.local"
_loginURL:     "https://blog.ang.local/login"

_welcomeSubject: "Welcome to \(_appName), {{ .Name }}"
_welcomeText: """
Hi {{ .Name }},

Welcome to \(_appName). Your account is ready.

Sign in: {{ if .LoginURL }}{{ .LoginURL }}{{ else }}\(_loginURL){{ end }}

If you need help, reply to \(_supportEmail).

--
\(_appName) Team
"""
_welcomeHTML: """
<html>
  <body style="font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;">
    <table width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;">
      <tr>
        <td style="padding:32px;">
          <h1 style="margin:0 0 16px 0;">Welcome to \(_appName), {{ .Name }}</h1>
          <p style="margin:0 0 12px 0;">Your author account is ready.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ if .LoginURL }}{{ .LoginURL }}{{ else }}\(_loginURL){{ end }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Open blog</a>
          </p>
          <p style="margin:0;color:#475569;">Need help? Contact \(_supportEmail).</p>
        </td>
      </tr>
    </table>
  </body>
</html>
"""

_noticeSubject: "{{ .Title }} · \(_appName)"
_noticeText: """
{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}

{{ .Body }}

--
\(_appName) Team
Support: \(_supportEmail)
"""
_noticeHTML: """
<html>
  <body style="font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;">
    <table width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;">
      <tr>
        <td style="padding:32px;">
          <h1 style="margin:0 0 16px 0;">{{ .Title }}</h1>
          <p style="margin:0 0 16px 0;">{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}</p>
          <div style="margin:0 0 20px 0;color:#0f172a;">{{ .Body }}</div>
          <p style="margin:0;color:#475569;">Support: \(_supportEmail)</p>
        </td>
      </tr>
    </table>
  </body>
</html>
"""

_resetSubject: "Reset your \(_appName) password"
_resetText: """
Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},

We received a request to reset your \(_appName) password.

Reset link: {{ .ResetURL }}

If this was not you, ignore this email.

--
\(_appName) Team
Support: \(_supportEmail)
"""
_resetHTML: """
<html>
  <body style="font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;">
    <table width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;">
      <tr>
        <td style="padding:32px;">
          <h1 style="margin:0 0 16px 0;">Reset your password</h1>
          <p style="margin:0 0 16px 0;">Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},</p>
          <p style="margin:0 0 20px 0;">Use the link below to reset your \(_appName) password.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ .ResetURL }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Reset password</a>
          </p>
          <p style="margin:0;color:#475569;">Support: \(_supportEmail)</p>
        </td>
      </tr>
    </table>
  </body>
</html>
"""

_verifySubject: "Verify your \(_appName) email address"
_verifyText: """
Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},

Please verify your email address to activate your \(_appName) account.

Verification link: {{ .VerifyURL }}

--
\(_appName) Team
Support: \(_supportEmail)
"""
_verifyHTML: """
<html>
  <body style="font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;">
    <table width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;">
      <tr>
        <td style="padding:32px;">
          <h1 style="margin:0 0 16px 0;">Verify your email address</h1>
          <p style="margin:0 0 20px 0;">Finish setting up your \(_appName) account.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ .VerifyURL }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Verify email</a>
          </p>
          <p style="margin:0;color:#475569;">Support: \(_supportEmail)</p>
        </td>
      </tr>
    </table>
  </body>
</html>
"""

#Templates: [
	{
		id:      "welcome_email"
		kind:    "email"
		channel: "email"
		subject: _welcomeSubject
		text:    _welcomeText
		html:    _welcomeHTML
		requiredVars: ["Name"]
		optionalVars: ["LoginURL", "AppName", "SupportEmail"]
	},
	{
		id:      "generic_notice"
		kind:    "email"
		channel: "email"
		subject: _noticeSubject
		text:    _noticeText
		html:    _noticeHTML
		requiredVars: ["Title", "Body"]
		optionalVars: ["Name", "AppName", "SupportEmail"]
	},
	{
		id:      "password_reset"
		kind:    "email"
		channel: "email"
		subject: _resetSubject
		text:    _resetText
		html:    _resetHTML
		requiredVars: ["ResetURL"]
		optionalVars: ["Name", "AppName", "SupportEmail"]
	},
	{
		id:      "email_verification"
		kind:    "email"
		channel: "email"
		subject: _verifySubject
		text:    _verifyText
		html:    _verifyHTML
		requiredVars: ["VerifyURL"]
		optionalVars: ["Name", "AppName", "SupportEmail"]
	},
]

#EmailTemplates: [
	{
		name:    "welcome_email"
		subject: _welcomeSubject
		text:    _welcomeText
		html:    _welcomeHTML
	},
	{
		name:    "generic_notice"
		subject: _noticeSubject
		text:    _noticeText
		html:    _noticeHTML
	},
	{
		name:    "password_reset"
		subject: _resetSubject
		text:    _resetText
		html:    _resetHTML
	},
	{
		name:    "email_verification"
		subject: _verifySubject
		text:    _verifyText
		html:    _verifyHTML
	},
]

#NotificationChannels: {
	enabled: true
	defaultChannels: ["email_primary", "email_fallback"]
	channels: {
		email_primary: {
			enabled:  true
			driver:   "ses"
			template: "generic_notice"
		}
		email_fallback: {
			enabled:  true
			driver:   "smtp"
			template: "generic_notice"
		}
	}
}

#NotificationPolicies: {
	enabled: true
	rules: [
		{
			enabled:  true
			event:    "UserRegistered"
			type:     "user.welcome"
			audience: "user"
			channels: ["email_primary", "email_fallback"]
			template: "welcome_email"
			muteKey:  "user.welcome"
		},
		{
			enabled:  true
			type:     "auth.password_reset"
			audience: "user"
			channels: ["email_primary", "email_fallback"]
			template: "password_reset"
			muteKey:  "auth.password_reset"
		},
		{
			enabled:  true
			type:     "auth.email_verification"
			audience: "user"
			channels: ["email_primary", "email_fallback"]
			template: "email_verification"
			muteKey:  "auth.email_verification"
		},
	]
}
