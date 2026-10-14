package emitter

import (
	"fmt"
	"strings"
)

func flowHTTPStatusExpr(raw any, fallback string) string {
	switch v := raw.(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%d", int(v))
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return fallback
		}
		switch strings.ToLower(s) {
		case "400", "bad_request":
			return "http.StatusBadRequest"
		case "401", "unauthorized":
			return "http.StatusUnauthorized"
		case "403", "forbidden":
			return "http.StatusForbidden"
		case "404", "not_found":
			return "http.StatusNotFound"
		case "409", "conflict":
			return "http.StatusConflict"
		case "422", "unprocessable_entity":
			return "http.StatusUnprocessableEntity"
		case "429", "too_many_requests":
			return "http.StatusTooManyRequests"
		case "500", "internal", "internal_error":
			return "http.StatusInternalServerError"
		case "503", "service_unavailable":
			return "http.StatusServiceUnavailable"
		default:
			return s
		}
	default:
		return fallback
	}
}
