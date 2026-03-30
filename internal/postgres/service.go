package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
	"github.com/erayyal/serveray-mcp/internal/shared/sqlsafe"
)

type Service struct {
	cfg Config
	db  *sql.DB
}

func Open(ctx context.Context, cfg Config) (*Service, error) {
	database, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
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
	return "public"
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

	tx, err := s.db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("begin read-only transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := applyReadOnlyGuards(queryCtx, tx, s.cfg.DB.QueryTimeout); err != nil {
		return shareddb.QueryResult{}, err
	}

	rows, err := tx.QueryContext(queryCtx, statement)
	if err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	result, err := shareddb.RowsToJSON(rows, limits)
	if err != nil {
		return shareddb.QueryResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("commit read-only transaction: %w", err)
	}
	return result, nil
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

	tx, err := s.db.BeginTx(queryCtx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := applyWriteGuards(queryCtx, tx, s.cfg.DB.QueryTimeout); err != nil {
		return 0, err
	}

	result, err := tx.ExecContext(queryCtx, statement)
	if err != nil {
		return 0, fmt.Errorf("execute statement: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read rows affected: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return rowsAffected, nil
}

func (s *Service) ListDatabases(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	d.datname AS database_name,
	PG_SIZE_PRETTY(pg_database_size(d.datname)) AS size,
	d.datallowconn AS allow_connections
FROM pg_database d
WHERE d.datistemplate = false
ORDER BY d.datname
LIMIT $1
`, limit)
}

func (s *Service) ListSchemas(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	schema_name
FROM information_schema.schemata
WHERE schema_name NOT IN ('information_schema', 'pg_catalog')
ORDER BY schema_name
LIMIT $1
`, limit)
}

func (s *Service) ListTables(ctx context.Context, schema string, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	table_schema AS schema_name,
	table_name,
	table_type
FROM information_schema.tables
WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
  AND ($1 = '' OR table_schema = $1)
ORDER BY table_schema, table_name
LIMIT $2
`, schema, limit)
}

func (s *Service) DescribeTable(ctx context.Context, schema, table string) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	column_name,
	data_type,
	is_nullable,
	column_default,
	udt_name
FROM information_schema.columns
WHERE table_schema = $1
  AND table_name = $2
ORDER BY ordinal_position
`, schema, table)
}

func (s *Service) queryMaps(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.cfg.DB.QueryTimeout)
	defer cancel()

	tx, err := s.db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin read-only transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := applyReadOnlyGuards(queryCtx, tx, s.cfg.DB.QueryTimeout); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute metadata query: %w", err)
	}
	defer rows.Close()

	items, _, err := shareddb.RowsToMaps(rows, s.cfg.DB.Limits())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit read-only transaction: %w", err)
	}
	return items, nil
}

func applyReadOnlyGuards(ctx context.Context, tx *sql.Tx, timeout time.Duration) error {
	if err := setLocalInt(ctx, tx, "statement_timeout", timeout.Milliseconds()); err != nil {
		return fmt.Errorf("set statement timeout: %w", err)
	}
	if err := setLocalInt(ctx, tx, "lock_timeout", 5000); err != nil {
		return fmt.Errorf("set lock timeout: %w", err)
	}
	if err := setLocalInt(ctx, tx, "idle_in_transaction_session_timeout", 5000); err != nil {
		return fmt.Errorf("set idle transaction timeout: %w", err)
	}
	return nil
}

func applyWriteGuards(ctx context.Context, tx *sql.Tx, timeout time.Duration) error {
	if err := setLocalInt(ctx, tx, "statement_timeout", timeout.Milliseconds()); err != nil {
		return fmt.Errorf("set statement timeout: %w", err)
	}
	if err := setLocalInt(ctx, tx, "lock_timeout", 5000); err != nil {
		return fmt.Errorf("set lock timeout: %w", err)
	}
	return nil
}

func setLocalInt(ctx context.Context, tx *sql.Tx, key string, value int64) error {
	if value <= 0 {
		value = 1
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL %s = %d", key, value)); err != nil {
		return err
	}
	return nil
}
