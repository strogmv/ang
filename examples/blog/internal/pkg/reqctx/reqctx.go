package reqctx

import "context"

type contextKey int

const (
	skipAutoVerifyKey contextKey = iota
	sessionIDKey
	localeKey
	timezoneKey
)

// WithSkipAutoVerify returns a new context with the skip-auto-verify flag set.
// Used by TestHeadersMiddleware to bypass JWT verification in tests.
func WithSkipAutoVerify(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipAutoVerifyKey, true)
}

// SkipAutoVerify reports whether the context has skip-auto-verify set.
func SkipAutoVerify(ctx context.Context) bool {
	v, _ := ctx.Value(skipAutoVerifyKey).(bool)
	return v
}

// WithSessionID stores an anonymous session ID in the context.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// SessionID retrieves the anonymous session ID from the context.
// Returns an empty string if no session ID is set.
func SessionID(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey).(string)
	return v
}

// WithLocale stores the resolved locale in the context.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey, locale)
}

// Locale retrieves the locale from the context.
// Returns an empty string if not set.
func Locale(ctx context.Context) string {
	v, _ := ctx.Value(localeKey).(string)
	return v
}

// WithTimezone stores the resolved timezone in the context.
func WithTimezone(ctx context.Context, tz string) context.Context {
	return context.WithValue(ctx, timezoneKey, tz)
}

// Timezone retrieves the timezone from the context. Returns empty string if not set.
func Timezone(ctx context.Context) string {
	v, _ := ctx.Value(timezoneKey).(string)
	return v
}
