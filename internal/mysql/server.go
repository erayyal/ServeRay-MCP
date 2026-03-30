package mysql

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

	instructions := "MySQL MCP server. Prefer metadata tools and read-only queries. Write operations require explicit unsafe opt-in."
	srv := mcpserver.New(Name, Version, instructions, logger)
	shareddb.RegisterCatalogTools(srv, service, shareddb.ToolsetOptions{
		DefaultLimit: 50,
		MaxLimit:     cfg.DB.MaxRows,
		WriteEnabled: cfg.DB.WriteEnabled(),
	})

	if err := mcpserver.AddJSONResource(srv, "server://mysql/capabilities", "mysql-capabilities", "Effective MySQL server safety configuration.", map[string]any{
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
			"TLS defaults to true when the DSN is built by the server.",
			"Multi-statement execution is blocked.",
			"Users should still prefer least-privileged MySQL accounts.",
		},
	}); err != nil {
		service.Close()
		return nil, nil, err
	}

	mcpserver.AddStaticPrompt(srv, "safe_usage", "MySQL safe usage guidance.", fmt.Sprintf(
		"Use list_databases, list_tables, and describe_table first. Responses are capped at %d rows and %d bytes, and raw SQL is restricted to read-only mode by default.",
		cfg.DB.MaxRows,
		cfg.DB.MaxBytes,
	))

	return srv, service, nil
}
