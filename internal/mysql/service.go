package mysql

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
	"github.com/erayyal/serveray-mcp/internal/shared/sqlsafe"
)

type Service struct {
	cfg Config
	db  *sql.DB
}

func Open(ctx context.Context, cfg Config) (*Service, error) {
	database, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open MySQL connection: %w", err)
	}

	shareddb.ApplyPool(database, cfg.DB.Pool())
	if err := shareddb.Ping(ctx, database, cfg.DB.ConnectTimeout); err != nil {
		database.Close()
		return nil, err
	}

	return &Service{cfg: cfg, db: database}, nil
}

func (s *Service) Close() error {
	return s.db.Close()
}

func (s *Service) DefaultSchema() string {
	if s.cfg.Database != "" {
		return s.cfg.Database
	}
	return "mysql"
}

func (s *Service) Query(ctx context.Context, statement string, maxRows int) (shareddb.QueryResult, error) {
	if err := sqlsafe.ValidateReadOnly(statement); err != nil {
		return shareddb.QueryResult{}, err
	}

	limits := s.cfg.DB.Limits()
	if maxRows > 0 && maxRows < limits.MaxRows {
		limits.MaxRows = maxRows
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.cfg.DB.QueryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, statement)
	if err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	return shareddb.RowsToJSON(rows, limits)
}

func (s *Service) Execute(ctx context.Context, statement string) (int64, error) {
	if !s.cfg.DB.WriteEnabled() {
		return 0, fmt.Errorf("write mode is disabled")
	}
	if err := sqlsafe.ValidateWrite(statement); err != nil {
		return 0, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.cfg.DB.QueryTimeout)
	defer cancel()

	result, err := s.db.ExecContext(queryCtx, statement)
	if err != nil {
		return 0, fmt.Errorf("execute statement: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read rows affected: %w", err)
	}
	return rowsAffected, nil
}

func (s *Service) ListDatabases(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	SCHEMA_NAME AS database_name,
	DEFAULT_CHARACTER_SET_NAME AS default_charset
FROM information_schema.SCHEMATA
ORDER BY SCHEMA_NAME
LIMIT ?
`, limit)
}

func (s *Service) ListSchemas(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.ListDatabases(ctx, limit)
}

func (s *Service) ListTables(ctx context.Context, schema string, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	TABLE_SCHEMA AS schema_name,
	TABLE_NAME AS table_name,
	TABLE_TYPE AS table_type
FROM information_schema.TABLES
WHERE (? = '' OR TABLE_SCHEMA = ?)
ORDER BY TABLE_SCHEMA, TABLE_NAME
LIMIT ?
`, schema, schema, limit)
}

func (s *Service) DescribeTable(ctx context.Context, schema, table string) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	COLUMN_NAME AS column_name,
	COLUMN_TYPE AS column_type,
	IS_NULLABLE AS is_nullable,
	COLUMN_KEY AS column_key,
	EXTRA AS extra,
	COLUMN_DEFAULT AS column_default
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ?
  AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION
`, schema, table)
}

func (s *Service) queryMaps(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.cfg.DB.QueryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute metadata query: %w", err)
	}
	defer rows.Close()

	items, _, err := shareddb.RowsToMaps(rows, s.cfg.DB.Limits())
	if err != nil {
		return nil, err
	}
	return items, nil
}
