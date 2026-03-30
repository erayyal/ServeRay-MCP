package redaction

import "testing"

func TestRedactText(t *testing.T) {
	redactor := New()

	input := `Authorization: Bearer secret-token password=my-password dsn=sqlserver://sa:TopSecret@db.example.com:1433?password=hunter2`
	output := redactor.RedactText(input)

	if output == input {
		t.Fatalf("expected redaction to change the text")
	}
	if containsSensitive(output, "secret-token", "my-password", "TopSecret", "hunter2") {
		t.Fatalf("expected secrets to be redacted, got %q", output)
	}
}

func TestRedactValueByKey(t *testing.T) {
	redactor := New()

	value := redactor.RedactValue("api_token", "should-not-appear")
	if value != Placeholder {
		t.Fatalf("expected sensitive key to be fully redacted, got %v", value)
	}
}

func containsSensitive(text string, values ...string) bool {
	for _, value := range values {
		if value != "" && len(text) > 0 && contains(text, value) {
			return true
		}
	}
	return false
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0
}

func stringIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
