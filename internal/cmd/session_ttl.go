package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseSessionTTL(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("--session-ttl must be greater than zero")
		}
		return time.Duration(seconds) * time.Second, nil
	}
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --session-ttl %q: use a duration like 3h or seconds like 10800", value)
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("--session-ttl must be greater than zero")
	}
	if ttl%time.Second != 0 {
		return 0, fmt.Errorf("--session-ttl must resolve to whole seconds")
	}
	return ttl, nil
}
