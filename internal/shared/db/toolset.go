package db

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/erayyal/serveray-mcp/internal/shared/limits"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

type CatalogService interface {
	Query(ctx context.Context, sql string, maxRows int) (QueryResult, error)
	Execute(ctx context.Context, sql string) (int64, error)
	ListDatabases(ctx context.Context, limit int) ([]map[string]any, error)
	ListSchemas(ctx context.Context, limit int) ([]map[string]any, error)
	ListTables(ctx context.Context, schema string, limit int) ([]map[string]any, error)
	DescribeTable(ctx context.Context, schema, table string) ([]map[string]any, error)
	DefaultSchema() string
}

type ToolsetOptions struct {
	DefaultLimit int
	MaxLimit     int
	WriteEnabled bool
}

func RegisterCatalogTools(srv *server.MCPServer, service CatalogService, opts ToolsetOptions) {
	srv.AddTool(queryTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("sql")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		limitValue := request.GetInt("limit", 0)
		maxRows := limits.Clamp(limitValue, opts.DefaultLimit, 1, opts.MaxLimit)

		result, err := service.Query(ctx, query, maxRows)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}

		text := result.JSON
		if result.Truncated {
			text = limits.Notice(text, fmt.Sprintf("Result was truncated to %d row(s) or the configured byte budget.", result.Rows), true)
		}
		return mcp.NewToolResultText(text), nil
	})

	if opts.WriteEnabled {
		srv.AddTool(writeTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			statement, err := request.RequireString("sql")
			if err != nil {
				return mcpserver.ErrorResult(err.Error()), nil
			}

			rowsAffected, err := service.Execute(ctx, statement)
			if err != nil {
				return mcpserver.ErrorResult(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Write completed. Rows affected: %d", rowsAffected)), nil
		})
	}

	srv.AddTool(listDatabasesTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limitValue := request.GetInt("limit", 0)
		items, err := service.ListDatabases(ctx, limits.Clamp(limitValue, opts.DefaultLimit, 1, opts.MaxLimit))
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(listSchemasTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limitValue := request.GetInt("limit", 0)
		items, err := service.ListSchemas(ctx, limits.Clamp(limitValue, opts.DefaultLimit, 1, opts.MaxLimit))
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(listTablesTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limitValue := request.GetInt("limit", 0)
		schema := request.GetString("schema", "")
		items, err := service.ListTables(ctx, schema, limits.Clamp(limitValue, opts.DefaultLimit, 1, opts.MaxLimit))
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})

	srv.AddTool(describeTableTool(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tableName, err := request.RequireString("table")
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		schema, table, err := ParseQualifiedName(tableName, service.DefaultSchema())
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		items, err := service.DescribeTable(ctx, schema, table)
		if err != nil {
			return mcpserver.ErrorResult(err.Error()), nil
		}
		return mcpserver.JSONResult(items)
	})
}

func queryTool() mcp.Tool {
	return mcp.NewTool("query",
		mcp.WithDescription("Execute a read-only SQL query. Safe mode blocks writes, multi-statement input, and dangerous keywords."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("A single read-only SQL query.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower row cap. The server still enforces its own maximum.")),
	)
}

func writeTool() mcp.Tool {
	return mcp.NewTool("execute_write",
		mcp.WithDescription("Execute a single write statement. This tool only appears when explicit unsafe write mode is enabled."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("A single write statement. EXEC and multi-statement input remain blocked.")),
	)
}

func listDatabasesTool() mcp.Tool {
	return mcp.NewTool("list_databases",
		mcp.WithDescription("List visible databases and basic metadata."),
		mcp.WithNumber("limit", mcp.Description("Optional lower row cap.")),
	)
}

func listSchemasTool() mcp.Tool {
	return mcp.NewTool("list_schemas",
		mcp.WithDescription("List visible schemas."),
		mcp.WithNumber("limit", mcp.Description("Optional lower row cap.")),
	)
}

func listTablesTool() mcp.Tool {
	return mcp.NewTool("list_tables",
		mcp.WithDescription("List visible tables and basic table metadata."),
		mcp.WithString("schema", mcp.Description("Optional schema filter.")),
		mcp.WithNumber("limit", mcp.Description("Optional lower row cap.")),
	)
}

func describeTableTool() mcp.Tool {
	return mcp.NewTool("describe_table",
		mcp.WithDescription("Describe the columns for a table. Use schema.table when needed."),
		mcp.WithString("table", mcp.Required(), mcp.Description("Table name or schema.table.")),
	)
}
