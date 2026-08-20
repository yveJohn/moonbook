package payment

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const EnvCallbackStaleMinutes = "MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES"

type AuditConfig struct {
	StaleAfter time.Duration
}

func LoadAuditConfig(lookup func(string) (string, bool)) (AuditConfig, error) {
	minutes := 5
	if raw, exists := lookup(EnvCallbackStaleMinutes); exists {
		parsed, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || parsed < 1 || parsed > 1440 {
			return AuditConfig{}, fmt.Errorf("%s must be an integer between 1 and 1440", EnvCallbackStaleMinutes)
		}
		minutes = parsed
	}
	return AuditConfig{StaleAfter: time.Duration(minutes) * time.Minute}, nil
}
