package redaction

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const Placeholder = "[REDACTED]"

var (
	bearerPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+\-/]+=*`)
	kvPattern     = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret|api[_-]?key|authorization|cookie|dsn|connection[_-]?string|webhook)\b([\s:=\"]+)([^,\s\"}]+)`)
	queryPattern  = regexp.MustCompile(`(?i)([?&](?:access_token|token|api_key|apikey|key|password|signature|sig)=)([^&\s]+)`)
)

type Redactor struct {
	sensitiveKeys map[string]struct{}
}

func New() *Redactor {
	keys := []string{
		"authorization",
		"apikey",
		"apitoken",
		"bearer",
		"connectionstring",
		"cookie",
		"dsn",
		"password",
		"passwd",
		"pwd",
		"secret",
		"token",
		"webhook",
	}
	index := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		index[key] = struct{}{}
	}
	return &Redactor{sensitiveKeys: index}
}

func (r *Redactor) IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	_, ok := r.sensitiveKeys[normalized]
	return ok
}

func (r *Redactor) RedactText(text string) string {
	text = bearerPattern.ReplaceAllString(text, "Bearer "+Placeholder)
	text = kvPattern.ReplaceAllString(text, "$1$2"+Placeholder)
	text = queryPattern.ReplaceAllString(text, "$1"+Placeholder)
	text = redactURL(text)
	return text
}

func (r *Redactor) RedactValue(key string, value any) any {
	if r.IsSensitiveKey(key) {
		return Placeholder
	}

	switch typed := value.(type) {
	case string:
		return r.RedactText(typed)
	case fmt.Stringer:
		return r.RedactText(typed.String())
	case []string:
		out := make([]string, len(typed))
		for i := range typed {
			out[i] = r.RedactText(typed[i])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for innerKey, innerValue := range typed {
			out[innerKey] = r.RedactValue(innerKey, innerValue)
		}
		return out
	default:
		return value
	}
}

func normalizeKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}

func redactURL(text string) string {
	if !strings.Contains(text, "://") {
		return text
	}

	words := strings.Fields(text)
	changed := false
	for i, word := range words {
		parsed, err := url.Parse(word)
		if err != nil || parsed.User == nil {
			continue
		}
		username := parsed.User.Username()
		if username == "" {
			continue
		}
		if _, hasPassword := parsed.User.Password(); !hasPassword {
			continue
		}
		parsed.User = url.UserPassword(username, Placeholder)
		words[i] = parsed.String()
		changed = true
	}
	if !changed {
		return text
	}
	return strings.Join(words, " ")
}
