package sqlsafe

import "testing"

func TestValidateReadOnlyAcceptsSelect(t *testing.T) {
	if err := ValidateReadOnly("SELECT * FROM users"); err != nil {
		t.Fatalf("expected select to pass, got %v", err)
	}
}

func TestValidateReadOnlyBlocksWriteStatements(t *testing.T) {
	tests := []string{
		"DROP TABLE users",
		"DELETE FROM users",
		"SELECT * FROM users; DELETE FROM users",
		"WITH doomed AS (SELECT 1) DELETE FROM users",
	}

	for _, query := range tests {
		if err := ValidateReadOnly(query); err == nil {
			t.Fatalf("expected query %q to be rejected", query)
		}
	}
}

func TestValidateWriteStillBlocksExec(t *testing.T) {
	if err := ValidateWrite("EXEC xp_cmdshell 'dir'"); err == nil {
		t.Fatalf("expected EXEC to remain blocked in write mode")
	}
}
