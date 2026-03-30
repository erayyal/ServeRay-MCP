package mssql

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/microsoft/go-mssqldb"

	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
	"github.com/erayyal/serveray-mcp/internal/shared/sqlsafe"
)

type Service struct {
	cfg Config
	db  *sql.DB
}

func Open(ctx context.Context, cfg Config) (*Service, error) {
	database, err := sql.Open("sqlserver", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open MSSQL connection: %w", err)
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
	return "dbo"
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

	conn, err := s.db.Conn(queryCtx)
	if err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(queryCtx, "SET DEADLOCK_PRIORITY LOW"); err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("set deadlock priority: %w", err)
	}
	if _, err := conn.ExecContext(queryCtx, "SET LOCK_TIMEOUT 5000"); err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("set lock timeout: %w", err)
	}
	if _, err := conn.ExecContext(queryCtx, "SET TRANSACTION ISOLATION LEVEL READ UNCOMMITTED"); err != nil {
		return shareddb.QueryResult{}, fmt.Errorf("set isolation level: %w", err)
	}

	rows, err := conn.QueryContext(queryCtx, statement)
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

	conn, err := s.db.Conn(queryCtx)
	if err != nil {
		return 0, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(queryCtx, "SET DEADLOCK_PRIORITY LOW"); err != nil {
		return 0, fmt.Errorf("set deadlock priority: %w", err)
	}
	if _, err := conn.ExecContext(queryCtx, "SET LOCK_TIMEOUT 5000"); err != nil {
		return 0, fmt.Errorf("set lock timeout: %w", err)
	}

	result, err := conn.ExecContext(queryCtx, statement)
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
SELECT TOP (@limit)
	name AS database_name,
	state_desc AS state
FROM sys.databases
ORDER BY name
`, sql.Named("limit", limit))
}

func (s *Service) ListSchemas(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT TOP (@limit)
	s.name AS schema_name,
	COUNT(t.object_id) AS table_count
FROM sys.schemas s
LEFT JOIN sys.tables t ON s.schema_id = t.schema_id
GROUP BY s.name
ORDER BY s.name
`, sql.Named("limit", limit))
}

func (s *Service) ListTables(ctx context.Context, schema string, limit int) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT TOP (@limit)
	s.name AS schema_name,
	t.name AS table_name,
	p.rows AS estimated_rows
FROM sys.tables t
JOIN sys.schemas s ON t.schema_id = s.schema_id
JOIN sys.partitions p ON t.object_id = p.object_id AND p.index_id IN (0,1)
WHERE (@schema = '' OR s.name = @schema)
ORDER BY s.name, t.name
`, sql.Named("limit", limit), sql.Named("schema", schema))
}

func (s *Service) DescribeTable(ctx context.Context, schema, table string) ([]map[string]any, error) {
	return s.queryMaps(ctx, `
SELECT
	c.name AS column_name,
	tp.name AS data_type,
	c.max_length,
	c.precision,
	c.scale,
	c.is_nullable,
	c.is_identity,
	ISNULL(dc.definition, '') AS default_value,
	CASE WHEN pk.column_id IS NOT NULL THEN 1 ELSE 0 END AS primary_key
FROM sys.columns c
JOIN sys.types tp ON c.user_type_id = tp.user_type_id
JOIN sys.tables t ON c.object_id = t.object_id
JOIN sys.schemas s ON t.schema_id = s.schema_id
LEFT JOIN sys.default_constraints dc ON c.default_object_id = dc.object_id
LEFT JOIN (
	SELECT ic.object_id, ic.column_id
	FROM sys.index_columns ic
	JOIN sys.indexes i ON ic.object_id = i.object_id AND ic.index_id = i.index_id
	WHERE i.is_primary_key = 1
) pk ON c.object_id = pk.object_id AND c.column_id = pk.column_id
WHERE s.name = @schema AND t.name = @table
ORDER BY c.column_id
`, sql.Named("schema", schema), sql.Named("table", table))
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
