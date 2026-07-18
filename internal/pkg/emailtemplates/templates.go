package emailtemplates

import (
	"fmt"
	"github.com/strogmv/ang/internal/pkg/templaterender"
	"reflect"
	"strings"
)

type Template struct {
	Subject      string
	Text         string
	HTML         string
	RequiredVars []string
	OptionalVars []string
}

var Templates = map[string]Template{
	"welcome_email": {
		Subject:      "Welcome to ANG, {{ .Name }}",
		Text:         "Hi {{ .Name }},\n\nWelcome to ANG. Your account is ready.\n\nSign in: {{ if .LoginURL }}{{ .LoginURL }}{{ else }}https://app.ang.local/login{{ end }}\n\nIf you need help, reply to support@ang.local.\n--\nANG Team\nSupport: support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">          <h1 style=\"margin:0 0 16px 0;\">Welcome to ANG, {{ .Name }}</h1>\n          <p style=\"margin:0 0 12px 0;\">Your account is ready and you can start using the system immediately.</p>\n          <p style=\"margin:0 0 20px 0;\">\n            <a href=\"{{ if .LoginURL }}{{ .LoginURL }}{{ else }}https://app.ang.local/login{{ end }}\" style=\"display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;\">Open ANG</a>\n          </p>          <p style=\"margin:0;color:#475569;\">Support: support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"Name"},
		OptionalVars: []string{"LoginURL", "AppName", "SupportEmail"},
	},
	"generic_notice": {
		Subject:      "{{ .Title }} · ANG",
		Text:         "{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}\n\n{{ .Body }}\n--\nANG Team\nSupport: support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">          <h1 style=\"margin:0 0 16px 0;\">{{ .Title }}</h1>\n          <p style=\"margin:0 0 16px 0;\">{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}</p>\n          <div style=\"margin:0 0 20px 0;color:#0f172a;\">{{ .Body }}</div>          <p style=\"margin:0;color:#475569;\">Support: support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"Title", "Body"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
	"password_reset": {
		Subject:      "Reset your ANG password",
		Text:         "Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},\n\nWe received a request to reset your ANG password.\n\nReset link: {{ .ResetURL }}\n\nIf you did not request this, you can ignore this message.\n--\nANG Team\nSupport: support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">          <h1 style=\"margin:0 0 16px 0;\">Reset your password</h1>\n          <p style=\"margin:0 0 16px 0;\">Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},</p>\n          <p style=\"margin:0 0 20px 0;\">We received a request to reset your ANG password.</p>\n          <p style=\"margin:0 0 20px 0;\">\n            <a href=\"{{ .ResetURL }}\" style=\"display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;\">Reset password</a>\n          </p>          <p style=\"margin:0;color:#475569;\">Support: support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"ResetURL"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
	"email_verification": {
		Subject:      "Verify your ANG email address",
		Text:         "Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},\n\nPlease verify your email address to continue using ANG.\n\nVerification link: {{ .VerifyURL }}\n--\nANG Team\nSupport: support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">          <h1 style=\"margin:0 0 16px 0;\">Verify your email address</h1>\n          <p style=\"margin:0 0 16px 0;\">Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},</p>\n          <p style=\"margin:0 0 20px 0;\">Confirm your email to finish setting up your ANG account.</p>\n          <p style=\"margin:0 0 20px 0;\">\n            <a href=\"{{ .VerifyURL }}\" style=\"display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;\">Verify email</a>\n          </p>          <p style=\"margin:0;color:#475569;\">Support: support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"VerifyURL"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
	"invitation_email": {
		Subject:      "{{ .InviterName }} invited you to ANG",
		Text:         "Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},\n\n{{ .InviterName }} invited you to join ANG.\n\nAccept invitation: {{ .InviteURL }}\n--\nANG Team\nSupport: support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">          <h1 style=\"margin:0 0 16px 0;\">You are invited to ANG</h1>\n          <p style=\"margin:0 0 16px 0;\">{{ .InviterName }} wants you to join.</p>\n          <p style=\"margin:0 0 20px 0;\">\n            <a href=\"{{ .InviteURL }}\" style=\"display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;\">Accept invitation</a>\n          </p>          <p style=\"margin:0;color:#475569;\">Support: support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"InviterName", "InviteURL"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
}

// Has reports whether a template with the given name is registered.
func Has(name string) bool {
	_, ok := Templates[name]
	return ok
}

func Render(name string, data any) (Template, error) {
	tpl, ok := Templates[name]
	if !ok {
		return Template{}, fmt.Errorf("unknown email template: %s", name)
	}
	if err := validateRequiredData(data, tpl.RequiredVars); err != nil {
		return Template{}, fmt.Errorf("template %s: %w", name, err)
	}
	subject, err := templaterender.RenderString(tpl.Subject, data)
	if err != nil {
		return Template{}, err
	}
	text, err := templaterender.RenderString(tpl.Text, data)
	if err != nil {
		return Template{}, err
	}
	html, err := templaterender.RenderString(tpl.HTML, data)
	if err != nil {
		return Template{}, err
	}
	return Template{Subject: subject, Text: text, HTML: html}, nil
}

func validateRequiredData(data any, required []string) error {
	for _, path := range required {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if !hasTemplateVar(data, path) {
			return fmt.Errorf("missing required template var %q", path)
		}
	}
	return nil
}

func hasTemplateVar(data any, path string) bool {
	if data == nil {
		return false
	}
	parts := strings.Split(path, ".")
	cur := reflect.ValueOf(data)
	for _, part := range parts {
		cur = derefValue(cur)
		if !cur.IsValid() {
			return false
		}
		switch cur.Kind() {
		case reflect.Map:
			key := reflect.ValueOf(part)
			val := cur.MapIndex(key)
			if !val.IsValid() {
				return false
			}
			cur = val
		case reflect.Struct:
			field := cur.FieldByName(part)
			if !field.IsValid() {
				return false
			}
			cur = field
		default:
			return false
		}
	}
	cur = derefValue(cur)
	return cur.IsValid()
}

func derefValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}
