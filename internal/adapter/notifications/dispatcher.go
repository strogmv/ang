package notifications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/strogmv/ang/internal/adapter/mailer/noop"
	"github.com/strogmv/ang/internal/adapter/mailer/smtp"
	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/pkg/emailtemplates"
	"github.com/strogmv/ang/internal/port"
)

// Dispatcher routes notification messages to configured channel sinks.
type Dispatcher struct {
	EmailFallbackSink port.NotificationEmailFallbackSink
	EmailPrimarySink  port.NotificationEmailPrimarySink
	UserMuteChecker   func(ctx context.Context, userID string, msg port.NotificationMessage) (bool, error)
	FailureRecorder   port.NotificationFailureRecorder
	RetryEnqueuer     port.NotificationRetryEnqueuer
}

type dispatchPolicy struct {
	Event    string
	Type     string
	Audience string
	Channels []string
	Template string
	MuteKey  string
}

type dispatchFailure struct {
	failure port.NotificationFailure
	stop    bool
}

var dispatchPolicies = []dispatchPolicy{
	{
		Event:    "UserRegistered",
		Type:     "user.welcome",
		Audience: "user",
		Channels: []string{
			"email_primary",
			"email_fallback",
		},
		Template: "welcome_email",
		MuteKey:  "user.welcome",
	},
	{
		Event:    "",
		Type:     "auth.password_reset",
		Audience: "user",
		Channels: []string{
			"email_primary",
			"email_fallback",
		},
		Template: "password_reset",
		MuteKey:  "auth.password_reset",
	},
	{
		Event:    "",
		Type:     "auth.email_verification",
		Audience: "user",
		Channels: []string{
			"email_primary",
			"email_fallback",
		},
		Template: "email_verification",
		MuteKey:  "auth.email_verification",
	},
	{
		Event:    "",
		Type:     "user.invitation",
		Audience: "user",
		Channels: []string{
			"email_primary",
			"email_fallback",
		},
		Template: "invitation_email",
		MuteKey:  "user.invitation",
	},
}

// NewDispatcher builds a runtime dispatcher with channel-specific sinks.
func NewDispatcher(cfg *config.Config) *Dispatcher {
	d := &Dispatcher{}
	d.EmailFallbackSink = newEmailSink(cfg, "smtp", "generic_notice")
	d.EmailPrimarySink = newEmailSink(cfg, "ses", "generic_notice")
	return d
}

// Dispatch delivers message to requested channels, or to default channels when omitted.
func (d *Dispatcher) Dispatch(ctx context.Context, msg port.NotificationMessage) error {
	msg = applyDispatchPolicy(msg)
	if d.UserMuteChecker != nil && strings.TrimSpace(msg.UserID) != "" {
		muted, err := d.UserMuteChecker(ctx, msg.UserID, msg)
		if err != nil {
			return fmt.Errorf("resolve notification mute: %w", err)
		}
		if muted {
			return nil
		}
	}
	channels := msg.Channels
	if len(channels) == 0 {
		channels = []string{
			"email_fallback",
			"email_primary",
		}
	}
	failures := make([]port.NotificationFailure, 0, len(channels))
	var retryFailure *port.NotificationFailure
	for idx, channel := range channels {
		failure := d.dispatchChannel(ctx, strings.TrimSpace(channel), msg, idx+1)
		if failure == nil {
			return nil
		}
		failures = append(failures, failure.failure)
		d.recordFailure(ctx, failure.failure)
		if failure.failure.Retryable {
			copied := failure.failure
			retryFailure = &copied
		}
		if failure.stop {
			break
		}
	}
	if len(failures) == 0 {
		return nil
	}
	if retryFailure != nil && d.RetryEnqueuer != nil {
		if err := d.RetryEnqueuer.EnqueueRetry(ctx, msg, *retryFailure); err != nil {
			failures = append(failures, port.NotificationFailure{
				Event:    msg.Event,
				Type:     msg.Type,
				Channel:  retryFailure.Channel,
				Attempt:  retryFailure.Attempt,
				Kind:     "permanent",
				Message:  fmt.Sprintf("enqueue notification retry: %v", err),
				Provider: retryFailure.Provider,
			})
		}
	}
	return aggregateNotificationFailures(failures)
}

func (d *Dispatcher) dispatchChannel(ctx context.Context, channel string, msg port.NotificationMessage, attempt int) *dispatchFailure {
	if channel == "" {
		failure := newNotificationFailure(msg, "", "permanent", false, attempt, "", "notification channel is empty")
		return &dispatchFailure{failure: failure, stop: false}
	}
	switch channel {
	case "email_fallback":
		if d.EmailFallbackSink == nil {
			failure := newNotificationFailure(msg, channel, "permanent", false, attempt, "smtp", "notification sink is not configured")
			return &dispatchFailure{failure: failure, stop: false}
		}
		if err := d.EmailFallbackSink.Send(ctx, msg); err != nil {
			kind, retryable, stop := classifyNotificationError(err)
			failure := newNotificationFailure(msg, channel, kind, retryable, attempt, "smtp", err.Error())
			return &dispatchFailure{failure: failure, stop: stop}
		}
		return nil
	case "email_primary":
		if d.EmailPrimarySink == nil {
			failure := newNotificationFailure(msg, channel, "permanent", false, attempt, "ses", "notification sink is not configured")
			return &dispatchFailure{failure: failure, stop: false}
		}
		if err := d.EmailPrimarySink.Send(ctx, msg); err != nil {
			kind, retryable, stop := classifyNotificationError(err)
			failure := newNotificationFailure(msg, channel, kind, retryable, attempt, "ses", err.Error())
			return &dispatchFailure{failure: failure, stop: stop}
		}
		return nil
	default:
		failure := newNotificationFailure(msg, channel, "permanent", false, attempt, "", fmt.Sprintf("notification channel %q is not supported", channel))
		return &dispatchFailure{failure: failure, stop: false}
	}
}

func (d *Dispatcher) recordFailure(ctx context.Context, failure port.NotificationFailure) {
	if d == nil || d.FailureRecorder == nil {
		return
	}
	_ = d.FailureRecorder.RecordFailure(ctx, failure)
}

func aggregateNotificationFailures(failures []port.NotificationFailure) error {
	if len(failures) == 0 {
		return nil
	}
	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		parts = append(parts, fmt.Sprintf("%s[%s]: %s", failure.Channel, failure.Kind, failure.Message))
	}
	return fmt.Errorf("notification delivery failed: %s", strings.Join(parts, "; "))
}

func newNotificationFailure(msg port.NotificationMessage, channel, kind string, retryable bool, attempt int, provider, message string) port.NotificationFailure {
	return port.NotificationFailure{
		Event:     strings.TrimSpace(msg.Event),
		Type:      strings.TrimSpace(msg.Type),
		Channel:   strings.TrimSpace(channel),
		Template:  strings.TrimSpace(msg.Template),
		Recipient: extractRecipient(msg),
		Attempt:   attempt,
		Kind:      strings.TrimSpace(kind),
		Retryable: retryable,
		Message:   strings.TrimSpace(message),
		Provider:  strings.TrimSpace(provider),
	}
}

func applyDispatchPolicy(msg port.NotificationMessage) port.NotificationMessage {
	for _, rule := range dispatchPolicies {
		if strings.TrimSpace(rule.Event) != "" && !strings.EqualFold(strings.TrimSpace(rule.Event), strings.TrimSpace(msg.Event)) {
			continue
		}
		if strings.TrimSpace(rule.Type) != "" && !strings.EqualFold(strings.TrimSpace(rule.Type), strings.TrimSpace(msg.Type)) {
			continue
		}
		if strings.TrimSpace(rule.Audience) != "" && !strings.EqualFold(strings.TrimSpace(rule.Audience), strings.TrimSpace(msg.Audience)) {
			continue
		}
		if strings.TrimSpace(msg.Type) == "" && strings.TrimSpace(rule.Type) != "" {
			msg.Type = strings.TrimSpace(rule.Type)
		}
		if len(msg.Channels) == 0 && len(rule.Channels) > 0 {
			msg.Channels = append([]string(nil), rule.Channels...)
		}
		if strings.TrimSpace(msg.Template) == "" && strings.TrimSpace(rule.Template) != "" {
			msg.Template = strings.TrimSpace(rule.Template)
		}
		if strings.TrimSpace(msg.MuteKey) == "" && strings.TrimSpace(rule.MuteKey) != "" {
			msg.MuteKey = strings.TrimSpace(rule.MuteKey)
		}
		return msg
	}
	return msg
}

func classifyNotificationError(err error) (kind string, retryable bool, stop bool) {
	if err == nil {
		return "", false, false
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "transient", true, false
	case errors.Is(err, context.Canceled):
		return "permanent", false, true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "transient", true, false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case msg == "":
		return "permanent", false, false
	case strings.Contains(msg, "recipient is required"),
		strings.Contains(msg, "template is required"),
		strings.Contains(msg, "missing required template vars"),
		strings.Contains(msg, "unknown email template"),
		strings.Contains(msg, "email sink is not configured"):
		return "permanent", false, true
	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "tempor"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "tls handshake"),
		strings.Contains(msg, "eof"),
		strings.Contains(msg, "no such host"):
		return "transient", true, false
	default:
		return "permanent", false, false
	}
}

type emailSink struct {
	mailer          port.Mailer
	defaultTemplate string
	provider        string
	initErr         error
}

func newEmailSink(cfg *config.Config, driver string, defaultTemplate string) *emailSink {
	mailer, provider, err := resolveMailer(cfg, driver)
	return &emailSink{
		mailer:          mailer,
		defaultTemplate: strings.TrimSpace(defaultTemplate),
		provider:        strings.TrimSpace(provider),
		initErr:         err,
	}
}

func (s *emailSink) Send(ctx context.Context, msg port.NotificationMessage) error {
	if s == nil {
		return fmt.Errorf("email sink is not configured")
	}
	if s.initErr != nil {
		return s.initErr
	}
	if s.mailer == nil {
		return fmt.Errorf("email sink is not configured")
	}
	to := extractRecipient(msg)
	if to == "" {
		return fmt.Errorf("email recipient is required")
	}
	templateID := strings.TrimSpace(msg.Template)
	if templateID == "" {
		templateID = strings.TrimSpace(s.defaultTemplate)
	}
	if templateID == "" {
		return fmt.Errorf("email template is required")
	}
	data := msg.Payload
	if data == nil {
		data = msg.Metadata
	}
	tpl, err := emailtemplates.Render(templateID, data)
	if err != nil {
		return err
	}
	return s.mailer.Send(ctx, port.EmailMessage{
		To:      to,
		Subject: tpl.Subject,
		Text:    tpl.Text,
		HTML:    tpl.HTML,
	})
}

func resolveMailer(cfg *config.Config, driver string) (port.Mailer, string, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	provider := normalizeMailerProvider(driver, cfg)
	if cfg.EmailDryRun || strings.EqualFold(strings.TrimSpace(os.Getenv("EMAIL_DRY_RUN")), "true") {
		return &dryRunMailer{}, provider, nil
	}
	switch provider {
	case "", "none", "noop", "disabled":
		return noop.New(), provider, nil
	case "smtp":
		if strings.TrimSpace(cfg.SMTPHost) == "" {
			return nil, provider, fmt.Errorf("smtp host not configured")
		}
		if strings.TrimSpace(cfg.SMTPFrom) == "" && strings.TrimSpace(cfg.SMTPUser) == "" {
			return nil, provider, fmt.Errorf("smtp sender not configured (set SMTP_FROM or SMTP_USER)")
		}
		return smtp.New(cfg), provider, nil
	case "ses":
		region := firstNonEmpty(strings.TrimSpace(cfg.SESRegion), strings.TrimSpace(os.Getenv("SES_REGION")))
		accessKey := firstNonEmpty(strings.TrimSpace(cfg.SESAccessKeyID), strings.TrimSpace(os.Getenv("SES_ACCESS_KEY_ID")))
		secretKey := firstNonEmpty(strings.TrimSpace(cfg.SESSecretAccessKey), strings.TrimSpace(os.Getenv("SES_SECRET_ACCESS_KEY")))
		from := firstNonEmpty(strings.TrimSpace(cfg.SESFrom), strings.TrimSpace(os.Getenv("SES_FROM")), strings.TrimSpace(cfg.SMTPFrom))
		if region == "" || accessKey == "" || secretKey == "" || from == "" {
			return nil, provider, fmt.Errorf("ses credentials are incomplete")
		}
		derived := *cfg
		derived.SMTPHost = firstNonEmpty(strings.TrimSpace(os.Getenv("SMTP_HOST")), "email-smtp."+region+".amazonaws.com")
		derived.SMTPPort = firstNonEmpty(strings.TrimSpace(os.Getenv("SMTP_PORT")), "587")
		derived.SMTPUser = accessKey
		derived.SMTPPass = deriveSESSMTPPassword(secretKey, region)
		derived.SMTPFrom = from
		return smtp.New(&derived), provider, nil
	default:
		return nil, provider, fmt.Errorf("unsupported email provider: %s", provider)
	}
}

func normalizeMailerProvider(driver string, cfg *config.Config) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(
		strings.TrimSpace(driver),
		strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")),
		strings.TrimSpace(cfg.EmailProvider),
		"noop",
	)))
}

func extractRecipient(msg port.NotificationMessage) string {
	if msg.Metadata != nil {
		for _, key := range []string{"to", "email", "recipient"} {
			if v, ok := msg.Metadata[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	if payload, ok := msg.Payload.(map[string]any); ok {
		for _, key := range []string{"to", "email", "recipient"} {
			if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func deriveSESSMTPPassword(secret, region string) string {
	kDate := hmacSHA256([]byte("AWS4"+secret), "11111111")
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "ses")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hmacSHA256(kSigning, "SendRawEmail")
	raw := append([]byte{0x04}, signature...)
	return base64.StdEncoding.EncodeToString(raw)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type dryRunMailer struct{}

func (d *dryRunMailer) Send(_ context.Context, msg port.EmailMessage) error {
	_ = msg
	return nil
}
