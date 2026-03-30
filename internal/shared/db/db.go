package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/erayyal/serveray-mcp/internal/shared/limits"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type BaseConfig struct {
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	MaxRows         int
	MaxBytes        int
	MaxCellChars    int
	EnableWrite     bool
	WriteAck        string
}

type QueryLimits struct {
	MaxRows      int
	MaxBytes     int
	MaxCellChars int
}

type QueryResult struct {
	JSON      string
	Rows      int
	Truncated bool
}

func (c BaseConfig) Pool() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		ConnMaxIdleTime: c.ConnMaxIdleTime,
	}
}

func (c BaseConfig) Limits() QueryLimits {
	return QueryLimits{
		MaxRows:      c.MaxRows,
		MaxBytes:     c.MaxBytes,
		MaxCellChars: c.MaxCellChars,
	}
}

func (c BaseConfig) WriteEnabled() bool {
	return c.EnableWrite && c.WriteAck == "ENABLE_UNSAFE_WRITE_OPERATIONS"
}

func ApplyPool(database *sql.DB, cfg PoolConfig) {
	database.SetMaxOpenConns(cfg.MaxOpenConns)
	database.SetMaxIdleConns(cfg.MaxIdleConns)
	database.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	database.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

func Ping(ctx context.Context, database *sql.DB, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := database.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func RowsToJSON(rows *sql.Rows, cfg QueryLimits) (QueryResult, error) {
	maps, truncated, err := RowsToMaps(rows, cfg)
	if err != nil {
		return QueryResult{}, err
	}
	text, _, err := limits.PrettyJSON(maps, 0)
	if err != nil {
		return QueryResult{}, err
	}
	return QueryResult{
		JSON:      text,
		Rows:      len(maps),
		Truncated: truncated,
	}, nil
}

func RowsToMaps(rows *sql.Rows, cfg QueryLimits) ([]map[string]any, bool, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, false, fmt.Errorf("read columns: %w", err)
	}

	results := make([]map[string]any, 0)
	estimatedBytes := 2
	truncated := false

	for rows.Next() {
		if cfg.MaxRows > 0 && len(results) >= cfg.MaxRows {
			truncated = true
			break
		}

		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, false, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = normalizeValue(values[i], cfg.MaxCellChars)
		}

		payload, err := json.Marshal(row)
		if err != nil {
			return nil, false, fmt.Errorf("marshal row: %w", err)
		}
		if cfg.MaxBytes > 0 && estimatedBytes+len(payload)+1 > cfg.MaxBytes {
			truncated = true
			break
		}

		estimatedBytes += len(payload) + 1
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate rows: %w", err)
	}
	return results, truncated, nil
}

func ParseQualifiedName(input, defaultSchema string) (schema string, name string, err error) {
	parts := strings.Split(strings.TrimSpace(input), ".")
	switch len(parts) {
	case 1:
		schema = defaultSchema
		name = parts[0]
	case 2:
		schema = parts[0]
		name = parts[1]
	default:
		return "", "", fmt.Errorf("expected name or schema.name")
	}

	if err := ValidateIdentifier(schema); err != nil {
		return "", "", fmt.Errorf("invalid schema: %w", err)
	}
	if err := ValidateIdentifier(name); err != nil {
		return "", "", fmt.Errorf("invalid table: %w", err)
	}
	return schema, name, nil
}

func ValidateIdentifier(value string) error {
	if !identifierPattern.MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("identifier %q contains unsupported characters", value)
	}
	return nil
}

func normalizeValue(value any, maxCellChars int) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		text, _ := limits.TruncateString(string(typed), maxCellChars)
		return text
	case string:
		text, _ := limits.TruncateString(typed, maxCellChars)
		return text
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	default:
		return typed
	}
}
