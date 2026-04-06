package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
)

type notificationChannelTemplate struct {
	Name      string
	TypeName  string
	Driver    string
	Template  string
	EmailLike bool
}

type notificationPolicyTemplate struct {
	Event    string
	Type     string
	Audience string
	Channels []string
	Template string
	MuteKey  string
}

// EmitNotificationDispatchPorts generates NotificationDispatcher and channel sink interfaces.
func (e *Emitter) EmitNotificationDispatchPorts(cfg *ir.NotificationsConfig) error {
	channels, _, err := collectNotificationChannels(cfg)
	if err != nil {
		return err
	}

	tmplPath := filepath.Join(e.TemplatesDir, "notification_dispatcher_port.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/notification_dispatcher_port.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	t, err := template.New("notification_dispatcher_port").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := e.outDir("internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, struct {
		Channels []notificationChannelTemplate
	}{
		Channels: channels,
	}); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "notification_dispatcher.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Notification Dispatch Ports: %s\n", path)
	return nil
}

// EmitNotificationDispatcherRuntime generates a default channel dispatcher adapter.
func (e *Emitter) EmitNotificationDispatcherRuntime(cfg *ir.NotificationsConfig) error {
	channels, defaultChannels, err := collectNotificationChannels(cfg)
	if err != nil {
		return err
	}
	policies := collectNotificationPolicies(cfg)

	tmplPath := filepath.Join(e.TemplatesDir, "notification_dispatcher_runtime.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/notification_dispatcher_runtime.tmpl"
	}

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	t, err := template.New("notification_dispatcher_runtime").Funcs(template.FuncMap{
		"GoModule": func() string { return e.GoModule },
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := e.outDir("internal", "adapter", "notifications")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, struct {
		Channels        []notificationChannelTemplate
		DefaultChannels []string
		HasEmailChannel bool
		Policies        []notificationPolicyTemplate
	}{
		Channels:        channels,
		DefaultChannels: defaultChannels,
		HasEmailChannel: hasEmailTransport(channels),
		Policies:        policies,
	}); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "dispatcher.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Notification Dispatcher Runtime: %s\n", path)
	return nil
}

func hasEmailTransport(channels []notificationChannelTemplate) bool {
	for _, ch := range channels {
		if ch.EmailLike {
			return true
		}
	}
	return false
}

func collectNotificationChannels(cfg *ir.NotificationsConfig) ([]notificationChannelTemplate, []string, error) {
	channelSet := map[string]bool{}
	defaultChannels := make([]string, 0, 4)
	defaultSet := map[string]bool{}
	if cfg != nil && cfg.Channels != nil {
		for _, ch := range cfg.Channels.DefaultChannels {
			ch = strings.TrimSpace(ch)
			if ch == "" {
				continue
			}
			channelSet[ch] = true
			if !defaultSet[ch] {
				defaultSet[ch] = true
				defaultChannels = append(defaultChannels, ch)
			}
		}
		for name, spec := range cfg.Channels.Channels {
			if !spec.Enabled {
				continue
			}
			name = strings.TrimSpace(name)
			if name != "" {
				channelSet[name] = true
			}
		}
	}
	if cfg != nil && cfg.Policies != nil {
		for _, rule := range cfg.Policies.Rules {
			if !rule.Enabled {
				continue
			}
			for _, ch := range rule.Channels {
				ch = strings.TrimSpace(ch)
				if ch != "" {
					channelSet[ch] = true
				}
			}
		}
	}
	if len(channelSet) == 0 {
		channelSet["in_app"] = true
	}
	if len(defaultChannels) == 0 {
		defaultChannels = append(defaultChannels, "in_app")
		channelSet["in_app"] = true
	}

	channels := make([]notificationChannelTemplate, 0, len(channelSet))
	for name := range channelSet {
		spec := ir.NotificationChannelSpec{}
		if cfg != nil && cfg.Channels != nil && cfg.Channels.Channels != nil {
			spec = cfg.Channels.Channels[name]
		}
		channels = append(channels, notificationChannelTemplate{
			Name:      name,
			TypeName:  channelTypeName(name),
			Driver:    strings.TrimSpace(spec.Driver),
			Template:  strings.TrimSpace(spec.Template),
			EmailLike: isEmailDriver(spec.Driver),
		})
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Name < channels[j].Name })
	sort.Strings(defaultChannels)
	return channels, defaultChannels, nil
}

func channelTypeName(name string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(name), func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})
	if len(parts) == 0 {
		return "Channel"
	}
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteString(ExportName(strings.ToLower(p)))
	}
	if b.Len() == 0 {
		return "Channel"
	}
	return b.String()
}

func collectNotificationPolicies(cfg *ir.NotificationsConfig) []notificationPolicyTemplate {
	if cfg == nil || cfg.Policies == nil || !cfg.Policies.Enabled {
		return nil
	}
	out := make([]notificationPolicyTemplate, 0, len(cfg.Policies.Rules))
	for _, rule := range cfg.Policies.Rules {
		if !rule.Enabled {
			continue
		}
		channels := make([]string, 0, len(rule.Channels))
		for _, ch := range rule.Channels {
			ch = strings.TrimSpace(ch)
			if ch != "" {
				channels = append(channels, ch)
			}
		}
		out = append(out, notificationPolicyTemplate{
			Event:    strings.TrimSpace(rule.Event),
			Type:     strings.TrimSpace(rule.Type),
			Audience: strings.TrimSpace(rule.Audience),
			Channels: channels,
			Template: strings.TrimSpace(rule.Template),
			MuteKey:  strings.TrimSpace(rule.MuteKey),
		})
	}
	return out
}

func isEmailDriver(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "smtp", "ses", "noop", "disabled", "none":
		return true
	default:
		return false
	}
}
