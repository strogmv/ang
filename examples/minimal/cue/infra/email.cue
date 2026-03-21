package infra

_appName:      "ANG Minimal Example"
_supportEmail: "minimal-support@ang.local"

_welcomeSubject: "Welcome to \(_appName), {{ .Name }}"
_welcomeText: """
Hi {{ .Name }},

Welcome to \(_appName).

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
          <p style="margin:0 0 20px 0;">Use the link below to reset your password.</p>
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

_inviteSubject: "{{ .InviterName }} invited you to \(_appName)"
_inviteText: """
Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},

{{ .InviterName }} invited you to join \(_appName).

Accept invitation: {{ .InviteURL }}

--
\(_appName) Team
Support: \(_supportEmail)
"""
_inviteHTML: """
<html>
  <body style="font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;">
    <table width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;">
      <tr>
        <td style="padding:32px;">
          <h1 style="margin:0 0 16px 0;">You are invited to \(_appName)</h1>
          <p style="margin:0 0 20px 0;">{{ .InviterName }} wants you to join.</p>
          <p style="margin:0 0 20px 0;">
            <a href="{{ .InviteURL }}" style="display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;">Accept invitation</a>
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
		optionalVars: ["AppName", "SupportEmail"]
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
			type:     "system.notice"
			audience: "user"
			channels: ["email_primary", "email_fallback"]
			template: "generic_notice"
			muteKey:  "system.notice"
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
			type:     "user.invitation"
			audience: "user"
			channels: ["email_primary", "email_fallback"]
			template: "invitation_email"
			muteKey:  "user.invitation"
		},
	]
}
