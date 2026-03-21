package infra

_appName:      "ANG"
_supportEmail: "support@ang.local"
_loginURL:     "https://app.ang.local/login"

_textSignature: """

--
\(_appName) Team
Support: \(_supportEmail)
"""

_htmlFrameStart: """
<html>
  <body style="font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;">
    <table width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;">
      <tr>
        <td style="padding:32px;">
"""

_htmlFrameEnd: """
          <p style="margin:0;color:#475569;">Support: \(_supportEmail)</p>
        </td>
      </tr>
    </table>
  </body>
</html>
"""

_welcomeSubject: "Welcome to \(_appName), {{ .Name }}"
_welcomeText: """
Hi {{ .Name }},

Welcome to \(_appName). Your account is ready.

Sign in: {{ if .LoginURL }}{{ .LoginURL }}{{ else }}\(_loginURL){{ end }}

If you need help, reply to \(_supportEmail).
""" + _textSignature
_welcomeHTML: _htmlFrameStart + """
          <h1 style="margin:0 0 16px 0;">Welcome to \(_appName), {{ .Name }}</h1>
          <p style="margin:0 0 12px 0;">Your account is ready and you can start using the system immediately.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ if .LoginURL }}{{ .LoginURL }}{{ else }}\(_loginURL){{ end }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Open \(_appName)</a>
          </p>
""" + _htmlFrameEnd

_noticeSubject: "{{ .Title }} · \(_appName)"
_noticeText: """
{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}

{{ .Body }}
""" + _textSignature
_noticeHTML: _htmlFrameStart + """
          <h1 style="margin:0 0 16px 0;">{{ .Title }}</h1>
          <p style="margin:0 0 16px 0;">{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}</p>
          <div style="margin:0 0 20px 0;color:#0f172a;">{{ .Body }}</div>
""" + _htmlFrameEnd

_resetSubject: "Reset your \(_appName) password"
_resetText: """
Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},

We received a request to reset your \(_appName) password.

Reset link: {{ .ResetURL }}

If you did not request this, you can ignore this message.
""" + _textSignature
_resetHTML: _htmlFrameStart + """
          <h1 style="margin:0 0 16px 0;">Reset your password</h1>
          <p style="margin:0 0 16px 0;">Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},</p>
          <p style="margin:0 0 20px 0;">We received a request to reset your \(_appName) password.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ .ResetURL }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Reset password</a>
          </p>
""" + _htmlFrameEnd

_verifySubject: "Verify your \(_appName) email address"
_verifyText: """
Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},

Please verify your email address to continue using \(_appName).

Verification link: {{ .VerifyURL }}
""" + _textSignature
_verifyHTML: _htmlFrameStart + """
          <h1 style="margin:0 0 16px 0;">Verify your email address</h1>
          <p style="margin:0 0 16px 0;">Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},</p>
          <p style="margin:0 0 20px 0;">Confirm your email to finish setting up your \(_appName) account.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ .VerifyURL }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Verify email</a>
          </p>
""" + _htmlFrameEnd

_inviteSubject: "{{ .InviterName }} invited you to \(_appName)"
_inviteText: """
Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},

{{ .InviterName }} invited you to join \(_appName).

Accept invitation: {{ .InviteURL }}
""" + _textSignature
_inviteHTML: _htmlFrameStart + """
          <h1 style="margin:0 0 16px 0;">You are invited to \(_appName)</h1>
          <p style="margin:0 0 16px 0;">{{ .InviterName }} wants you to join.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ .InviteURL }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Accept invitation</a>
          </p>
""" + _htmlFrameEnd

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
	{
		id:      "invitation_email"
		kind:    "email"
		channel: "email"
		subject: _inviteSubject
		text:    _inviteText
		html:    _inviteHTML
		requiredVars: ["InviterName", "InviteURL"]
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
	{
		name:    "invitation_email"
		subject: _inviteSubject
		text:    _inviteText
		html:    _inviteHTML
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
		{
			enabled:  true
			type:     "user.invitation"
			audience: "user"
			channels: ["email_primary", "email_fallback"]
			template: "invitation_email"
			muteKey:  "user.invitation"
		},
	]
}
