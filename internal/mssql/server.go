package mssql

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
	"github.com/erayyal/serveray-mcp/internal/shared/logging"
	"github.com/erayyal/serveray-mcp/internal/shared/mcpserver"
)

func New(ctx context.Context, logger *logging.Logger) (*server.MCPServer, *Service, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	service, err := Open(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	instructions := "Microsoft SQL Server MCP server. Prefer metadata tools first. Query tool is safe-mode read-only unless explicit write mode has been enabled."
	srv := mcpserver.New(Name, Version, instructions, logger)
	shareddb.RegisterCatalogTools(srv, service, shareddb.ToolsetOptions{
		DefaultLimit: 50,
		MaxLimit:     cfg.DB.MaxRows,
		WriteEnabled: cfg.DB.WriteEnabled(),
	})

	if err := mcpserver.AddJSONResource(srv, "server://mssql/capabilities", "mssql-capabilities", "Effective MSSQL server safety configuration.", map[string]any{
		"server":        Name,
		"version":       Version,
		"default_mode":  "read-only safe mode",
		"write_enabled": cfg.DB.WriteEnabled(),
		"timeouts": map[string]any{
			"connect_timeout": cfg.DB.ConnectTimeout.String(),
			"query_timeout":   cfg.DB.QueryTimeout.String(),
		},
		"limits": map[string]any{
			"max_rows":       cfg.DB.MaxRows,
			"max_bytes":      cfg.DB.MaxBytes,
			"max_cell_chars": cfg.DB.MaxCellChars,
		},
		"notes": []string{
			"SQL writes are disabled by default.",
			"Multi-statement execution is blocked.",
			"READ UNCOMMITTED and lock timeout safeguards are applied to user queries.",
		},
	}); err != nil {
		service.Close()
		return nil, nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "MSSQL safe usage guidance.", fmt.Sprintf(
		"Use metadata tools before raw SQL. Keep queries narrow, add WHERE clauses, and expect results to be capped at %d rows and %d bytes. Write operations are only available after explicit unsafe opt-in.",
		cfg.DB.MaxRows,
		cfg.DB.MaxBytes,
	))

	return srv, service, nil
}
