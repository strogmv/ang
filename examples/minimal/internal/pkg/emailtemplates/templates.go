package emailtemplates

import (
	"fmt"
	"github.com/example/minimal/internal/pkg/templaterender"
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
		Subject:      "Welcome to ANG Minimal Example, {{ .Name }}",
		Text:         "Hi {{ .Name }},\n\nWelcome to ANG Minimal Example.\n\nIf you need help, reply to minimal-support@ang.local.\n\n--\nANG Minimal Example Team",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">\n          <h1 style=\"margin:0 0 16px 0;\">Welcome to ANG Minimal Example, {{ .Name }}</h1>\n          <p style=\"margin:0;color:#475569;\">Need help? Contact minimal-support@ang.local.</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"Name"},
		OptionalVars: []string{"AppName", "SupportEmail"},
	},
	"generic_notice": {
		Subject:      "{{ .Title }} · ANG Minimal Example",
		Text:         "{{ if .Name }}Hi {{ .Name }},{{ else }}Hello,{{ end }}\n\n{{ .Body }}\n\n--\nANG Minimal Example Team\nSupport: minimal-support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">\n          <h1 style=\"margin:0 0 16px 0;\">{{ .Title }}</h1>\n          <div style=\"margin:0 0 20px 0;color:#0f172a;\">{{ .Body }}</div>\n          <p style=\"margin:0;color:#475569;\">Support: minimal-support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"Title", "Body"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
	"password_reset": {
		Subject:      "Reset your ANG Minimal Example password",
		Text:         "Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},\n\nWe received a request to reset your ANG Minimal Example password.\n\nReset link: {{ .ResetURL }}\n\n--\nANG Minimal Example Team\nSupport: minimal-support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">\n          <h1 style=\"margin:0 0 16px 0;\">Reset your password</h1>\n          <p style=\"margin:0 0 20px 0;\">Use the link below to reset your password.</p>\n          <p style=\"margin:0 0 20px 0;\">\n            <a href=\"{{ .ResetURL }}\" style=\"display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;\">Reset password</a>\n          </p>\n          <p style=\"margin:0;color:#475569;\">Support: minimal-support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"ResetURL"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
	"invitation_email": {
		Subject:      "{{ .InviterName }} invited you to ANG Minimal Example",
		Text:         "Hi {{ if .Name }}{{ .Name }}{{ else }}there{{ end }},\n\n{{ .InviterName }} invited you to join ANG Minimal Example.\n\nAccept invitation: {{ .InviteURL }}\n\n--\nANG Minimal Example Team\nSupport: minimal-support@ang.local",
		HTML:         "<html>\n  <body style=\"font-family:Arial,sans-serif;background:#f6f8fb;padding:24px;\">\n    <table width=\"100%%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;\">\n      <tr>\n        <td style=\"padding:32px;\">\n          <h1 style=\"margin:0 0 16px 0;\">You are invited to ANG Minimal Example</h1>\n          <p style=\"margin:0 0 20px 0;\">{{ .InviterName }} wants you to join.</p>\n          <p style=\"margin:0 0 20px 0;\">\n            <a href=\"{{ .InviteURL }}\" style=\"display:inline-block;padding:12px 18px;background:#0f766e;color:#ffffff;text-decoration:none;border-radius:10px;\">Accept invitation</a>\n          </p>\n          <p style=\"margin:0;color:#475569;\">Support: minimal-support@ang.local</p>\n        </td>\n      </tr>\n    </table>\n  </body>\n</html>",
		RequiredVars: []string{"InviterName", "InviteURL"},
		OptionalVars: []string{"Name", "AppName", "SupportEmail"},
	},
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
