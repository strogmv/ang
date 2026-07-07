package testpp

import (
	"fmt"
	"strings"
)

type apiSecrets struct {
	apiKey     string
	signingKey string
}

func parseSecrets(secret string) (*apiSecrets, error) {
	parts := strings.SplitN(strings.TrimSpace(secret), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("%s: expected secret format '%s', got %d parts", ppSID, "key:sign", len(parts))
	}
	return &apiSecrets{
		apiKey:     parts[0],
		signingKey: parts[1],
	}, nil
}
