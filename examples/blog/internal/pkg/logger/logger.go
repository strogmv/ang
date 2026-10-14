package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// parseLevel maps LOG_LEVEL (debug/info/warn/error) to an slog level; default info.
// ERROR is always emitted since it is >= every level.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Init() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: parseLevel(os.Getenv("LOG_LEVEL")),
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
	return logger
}

func From(ctx context.Context) *slog.Logger {
	logger := slog.Default()

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		logger = logger.With(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	return logger
}
