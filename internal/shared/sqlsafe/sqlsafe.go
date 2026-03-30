package sqlsafe

import (
	"fmt"
	"strings"
	"unicode"
)

var readOnlyBlockedTokens = []string{
	"ALTER",
	"CALL",
	"CREATE",
	"DELETE",
	"DROP",
	"EXEC",
	"EXECUTE",
	"GRANT",
	"INSERT",
	"MERGE",
	"REVOKE",
	"TRUNCATE",
	"UPDATE",
	"XP_CMDSHELL",
}

var writeBlockedTokens = []string{
	"CALL",
	"EXEC",
	"EXECUTE",
	"GRANT",
	"REVOKE",
	"XP_CMDSHELL",
}

func ValidateReadOnly(query string) error {
	cleaned := sanitize(query)
	if cleaned == "" {
		return fmt.Errorf("query is empty")
	}
	if hasMultipleStatements(cleaned) {
		return fmt.Errorf("multi-statement execution is blocked")
	}

	tokens := tokenize(cleaned)
	if len(tokens) == 0 {
		return fmt.Errorf("query is empty")
	}
	if blocked := firstBlockedToken(tokens, readOnlyBlockedTokens); blocked != "" {
		return fmt.Errorf("%s is blocked in safe mode", blocked)
	}

	switch tokens[0] {
	case "SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN":
		return nil
	case "WITH":
		if !containsToken(tokens, "SELECT") {
			return fmt.Errorf("WITH queries must remain read-only")
		}
		return nil
	default:
		return fmt.Errorf("%s is not allowed in safe mode", tokens[0])
	}
}

func ValidateWrite(query string) error {
	cleaned := sanitize(query)
	if cleaned == "" {
		return fmt.Errorf("statement is empty")
	}
	if hasMultipleStatements(cleaned) {
		return fmt.Errorf("multi-statement execution is blocked")
	}

	tokens := tokenize(cleaned)
	if len(tokens) == 0 {
		return fmt.Errorf("statement is empty")
	}
	if blocked := firstBlockedToken(tokens, writeBlockedTokens); blocked != "" {
		return fmt.Errorf("%s is blocked even in write mode", blocked)
	}

	switch tokens[0] {
	case "ALTER", "CREATE", "DELETE", "DROP", "INSERT", "MERGE", "SELECT", "TRUNCATE", "UPDATE", "WITH":
		return nil
	default:
		return fmt.Errorf("%s is not supported by the write tool", tokens[0])
	}
}

func sanitize(input string) string {
	type mode int

	const (
		modeNormal mode = iota
		modeSingleQuote
		modeDoubleQuote
		modeBracket
		modeLineComment
		modeBlockComment
	)

	runes := []rune(input)
	var builder strings.Builder
	state := modeNormal

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		switch state {
		case modeNormal:
			switch {
			case ch == '\'':
				state = modeSingleQuote
				builder.WriteRune(' ')
			case ch == '"':
				state = modeDoubleQuote
				builder.WriteRune(' ')
			case ch == '[':
				state = modeBracket
				builder.WriteRune(' ')
			case ch == '-' && i+1 < len(runes) && runes[i+1] == '-':
				state = modeLineComment
				builder.WriteString("  ")
				i++
			case ch == '/' && i+1 < len(runes) && runes[i+1] == '*':
				state = modeBlockComment
				builder.WriteString("  ")
				i++
			default:
				builder.WriteRune(unicode.ToUpper(ch))
			}
		case modeSingleQuote:
			builder.WriteRune(' ')
			if ch == '\'' {
				if i+1 < len(runes) && runes[i+1] == '\'' {
					builder.WriteRune(' ')
					i++
					continue
				}
				state = modeNormal
			}
		case modeDoubleQuote:
			builder.WriteRune(' ')
			if ch == '"' {
				state = modeNormal
			}
		case modeBracket:
			builder.WriteRune(' ')
			if ch == ']' {
				state = modeNormal
			}
		case modeLineComment:
			if ch == '\n' {
				builder.WriteRune('\n')
				state = modeNormal
			} else {
				builder.WriteRune(' ')
			}
		case modeBlockComment:
			builder.WriteRune(' ')
			if ch == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				builder.WriteRune(' ')
				i++
				state = modeNormal
			}
		}
	}

	return strings.TrimSpace(builder.String())
}

func hasMultipleStatements(cleaned string) bool {
	trimmed := strings.TrimSpace(cleaned)
	trimmed = strings.TrimRight(trimmed, " \t\r\n;")
	return strings.Contains(trimmed, ";")
}

func tokenize(cleaned string) []string {
	return strings.FieldsFunc(cleaned, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_')
	})
}

func containsToken(tokens []string, target string) bool {
	for _, token := range tokens {
		if token == target {
			return true
		}
	}
	return false
}

func firstBlockedToken(tokens, blocked []string) string {
	for _, token := range blocked {
		if containsToken(tokens, token) {
			return token
		}
	}
	return ""
}
