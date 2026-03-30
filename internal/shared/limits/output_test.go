package limits

import "testing"

func TestPrettyJSONTruncates(t *testing.T) {
	value := map[string]any{"text": "abcdefghijklmnopqrstuvwxyz"}
	output, truncated, err := PrettyJSON(value, 10)
	if err != nil {
		t.Fatalf("PrettyJSON: %v", err)
	}
	if !truncated {
		t.Fatalf("expected output to be truncated")
	}
	if len(output) <= 10 {
		t.Fatalf("expected notice to be appended, got %q", output)
	}
}
