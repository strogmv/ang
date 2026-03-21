package compiler

import (
	"context"
	"errors"
	"testing"

	"github.com/strogmv/ang/internal/adapter/notifications"
	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/pkg/emailtemplates"
	"github.com/strogmv/ang/internal/port"
)

func TestEmailTemplateRuntimeSmoke_ValidatesRequiredVars(t *testing.T) {
	t.Parallel()

	if _, err := emailtemplates.Render("password_reset", map[string]any{
		"Name":     "Jane",
		"ResetURL": "https://app.ang.local/reset?token=abc",
	}); err != nil {
		t.Fatalf("expected valid template render, got %v", err)
	}

	if _, err := emailtemplates.Render("password_reset", map[string]any{
		"Name": "Jane",
	}); err == nil {
		t.Fatalf("expected missing required vars error")
	}
}

func TestNotificationDispatcherDryRunSmoke_EmailTemplate(t *testing.T) {
	t.Parallel()

	dispatcher := notifications.NewDispatcher(&config.Config{
		EmailProvider: "ses",
		EmailDryRun:   true,
		SESRegion:     "eu-central-1",
		SESFrom:       "noreply@example.com",
	})

	err := dispatcher.Dispatch(context.Background(), port.NotificationMessage{
		Channels: []string{"email_primary", "email_fallback"},
		Template: "password_reset",
		Metadata: map[string]any{"to": "user@example.com"},
		Payload: map[string]any{
			"Name":     "Jane",
			"ResetURL": "https://app.ang.local/reset?token=abc",
		},
	})
	if err != nil {
		t.Fatalf("dispatch dry-run failed: %v", err)
	}
}

type recordingFailureRecorder struct {
	failures []port.NotificationFailure
}

func (r *recordingFailureRecorder) RecordFailure(_ context.Context, failure port.NotificationFailure) error {
	r.failures = append(r.failures, failure)
	return nil
}

type recordingRetryEnqueuer struct {
	calls []port.NotificationFailure
}

func (r *recordingRetryEnqueuer) EnqueueRetry(_ context.Context, _ port.NotificationMessage, failure port.NotificationFailure) error {
	r.calls = append(r.calls, failure)
	return nil
}

type failingNotificationSink struct {
	err error
}

func (s failingNotificationSink) Send(_ context.Context, _ port.NotificationMessage) error {
	return s.err
}

type successfulNotificationSink struct{}

func (s successfulNotificationSink) Send(_ context.Context, _ port.NotificationMessage) error {
	return nil
}

func TestNotificationDispatcherFallsBackAfterTransientFailure(t *testing.T) {
	t.Parallel()

	recorder := &recordingFailureRecorder{}
	retries := &recordingRetryEnqueuer{}
	dispatcher := &notifications.Dispatcher{
		EmailPrimarySink:  failingNotificationSink{err: context.DeadlineExceeded},
		EmailFallbackSink: successfulNotificationSink{},
		FailureRecorder:   recorder,
		RetryEnqueuer:     retries,
	}

	err := dispatcher.Dispatch(context.Background(), port.NotificationMessage{
		Channels: []string{"email_primary", "email_fallback"},
		Template: "password_reset",
		Metadata: map[string]any{"to": "user@example.com"},
		Payload: map[string]any{
			"ResetURL": "https://app.ang.local/reset?token=abc",
		},
	})
	if err != nil {
		t.Fatalf("expected fallback delivery to succeed, got %v", err)
	}
	if len(recorder.failures) != 1 {
		t.Fatalf("expected one recorded failure, got %d", len(recorder.failures))
	}
	if !recorder.failures[0].Retryable || recorder.failures[0].Kind != "transient" {
		t.Fatalf("expected transient retryable failure, got %+v", recorder.failures[0])
	}
	if len(retries.calls) != 0 {
		t.Fatalf("expected no retry enqueue on successful fallback, got %d", len(retries.calls))
	}
}

func TestNotificationDispatcherEnqueuesRetryWhenAllChannelsFailTransiently(t *testing.T) {
	t.Parallel()

	recorder := &recordingFailureRecorder{}
	retries := &recordingRetryEnqueuer{}
	dispatcher := &notifications.Dispatcher{
		EmailPrimarySink:  failingNotificationSink{err: context.DeadlineExceeded},
		EmailFallbackSink: failingNotificationSink{err: errors.New("connection refused")},
		FailureRecorder:   recorder,
		RetryEnqueuer:     retries,
	}

	err := dispatcher.Dispatch(context.Background(), port.NotificationMessage{
		Channels: []string{"email_primary", "email_fallback"},
		Template: "password_reset",
		Metadata: map[string]any{"to": "user@example.com"},
		Payload: map[string]any{
			"ResetURL": "https://app.ang.local/reset?token=abc",
		},
	})
	if err == nil {
		t.Fatalf("expected dispatch failure when all providers fail")
	}
	if len(recorder.failures) != 2 {
		t.Fatalf("expected two recorded failures, got %d", len(recorder.failures))
	}
	if len(retries.calls) != 1 {
		t.Fatalf("expected one retry enqueue, got %d", len(retries.calls))
	}
	if !retries.calls[0].Retryable || retries.calls[0].Kind != "transient" {
		t.Fatalf("expected transient retry failure, got %+v", retries.calls[0])
	}
}
