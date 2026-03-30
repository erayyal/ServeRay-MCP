package limits

import (
	"encoding/json"
	"fmt"
)

func Clamp(value, fallback, min, max int) int {
	if value == 0 {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func TruncateString(text string, max int) (string, bool) {
	if max <= 0 || len(text) <= max {
		return text, false
	}
	return text[:max] + "…", true
}

func PrettyJSON(value any, maxBytes int) (string, bool, error) {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", false, fmt.Errorf("marshal JSON: %w", err)
	}
	if maxBytes > 0 && len(payload) > maxBytes {
		return string(payload[:maxBytes]) + "\n…", true, nil
	}
	return string(payload), false, nil
}

func Notice(text, message string, shouldAdd bool) string {
	if !shouldAdd || message == "" {
		return text
	}
	if text == "" {
		return message
	}
	return text + "\n\n" + message
}
